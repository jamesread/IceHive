package healthmetrics

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/icehive/icehive/services/common/pkg/obsmetrics"
	"github.com/icehive/icehive/services/controller/internal/db"
	"github.com/knadh/koanf/v2"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

var cronStandardParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

type entityFreshnessProbe struct {
	EntityType   string
	SourceSystem string
	Table        string
	WhereClause  string
}

var defaultEntityProbes = []entityFreshnessProbe{
	{EntityType: "GitRepo", SourceSystem: "github", Table: "gitrepos", WhereClause: "source_collector_type = 'collector-github'"},
	{EntityType: "DependabotIssue", SourceSystem: "github", Table: "dependabotissues", WhereClause: "source_collector_type = 'collector-github'"},
	{EntityType: "GitHubIssue", SourceSystem: "github", Table: "githubissues", WhereClause: "source_collector_type = 'collector-github'"},
}

// Refresher periodically updates controller pipeline health Prometheus gauges.
type Refresher struct {
	log          *logrus.Logger
	metaDB       *sql.DB
	k            *koanf.Koanf
	entitiesDB   *sql.DB
	entityProbes []entityFreshnessProbe
}

// NewRefresher builds a health metrics refresher. entitiesDB may be nil if sink settings are unavailable.
func NewRefresher(log *logrus.Logger, metaDB *sql.DB, k *koanf.Koanf, entitiesDB *sql.DB) *Refresher {
	return &Refresher{
		log:          log,
		metaDB:       metaDB,
		k:            k,
		entitiesDB:   entitiesDB,
		entityProbes: defaultEntityProbes,
	}
}

// OpenEntitiesDB opens a read-only connection to the persister sink database when settings are available.
func OpenEntitiesDB(ctx context.Context, k *koanf.Koanf, metaDB *sql.DB) (*sql.DB, error) {
	settings, err := loadPersisterSettings(ctx, k, metaDB)
	if err != nil {
		return nil, err
	}
	port := settings.Port
	if port <= 0 {
		port = 3306
	}
	cfg := mysql.NewConfig()
	cfg.User = settings.User
	cfg.Passwd = settings.Password
	cfg.Net = "tcp"
	cfg.Addr = settings.Host + ":" + strconv.Itoa(port)
	cfg.DBName = settings.Database
	cfg.Params = map[string]string{"parseTime": "true"}
	dbConn, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	if err := dbConn.PingContext(ctx); err != nil {
		_ = dbConn.Close()
		return nil, err
	}
	return dbConn, nil
}

//gocyclo:ignore
func loadPersisterSettings(ctx context.Context, k *koanf.Koanf, metaDB *sql.DB) (db.PersisterMySQLSettings, error) {
	if metaDB != nil {
		if s, err := db.LoadPersisterMySQLSettings(ctx, metaDB); err == nil {
			return s, nil
		}
	}
	if k != nil && k.Exists("persister_mysql.host") {
		port := k.Int("persister_mysql.port")
		if port <= 0 {
			port = 3306
		}
		return db.PersisterMySQLSettings{
			Host:     strings.TrimSpace(k.String("persister_mysql.host")),
			Port:     port,
			User:     strings.TrimSpace(k.String("persister_mysql.user")),
			Password: k.String("persister_mysql.password"),
			Database: strings.TrimSpace(k.String("persister_mysql.database")),
		}, nil
	}
	return db.PersisterMySQLSettings{}, sql.ErrConnDone
}

// Start runs refresh ticks until ctx is cancelled.
func (r *Refresher) Start(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	r.refresh(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.refresh(ctx)
		}
	}
}

func (r *Refresher) refresh(ctx context.Context) {
	r.refreshHeartbeats(ctx)
	r.refreshCollectionSources(ctx)
	r.refreshEntityFreshness(ctx)
}

func (r *Refresher) refreshHeartbeats(ctx context.Context) {
	rows, err := db.ListHeartbeats(ctx, r.metaDB)
	if err != nil {
		r.log.WithError(err).Warn("health metrics: list heartbeats failed")
		return
	}
	now := time.Now()
	for _, row := range rows {
		age := now.Sub(time.UnixMilli(row.LatestHeartbeatUnixMs)).Seconds()
		if row.LatestHeartbeatUnixMs <= 0 {
			age = -1
		}
		if age < 0 {
			age = 0
		}
		obsmetrics.SetControllerServiceHeartbeatAge(row.ServiceName, age)
	}
}

//gocyclo:ignore
func (r *Refresher) refreshCollectionSources(ctx context.Context) {
	rows, err := db.ListCollectionSources(ctx, r.metaDB, "")
	if err != nil {
		r.log.WithError(err).Warn("health metrics: list collection sources failed")
		return
	}
	now := time.Now()
	for _, src := range rows {
		if !src.Enabled {
			obsmetrics.SetControllerCollectionSourceStale(src.ID, src.CollectorType, false)
			obsmetrics.SetControllerCollectionSourceLastSuccessAge(src.ID, src.CollectorType, 0)
			continue
		}
		cronLine := strings.TrimSpace(src.CronLine)
		if cronLine == "" {
			// Run-now-only sources: staleness based on last success age only when never succeeded.
			age := successAgeSeconds(src, now)
			obsmetrics.SetControllerCollectionSourceLastSuccessAge(src.ID, src.CollectorType, age)
			stale := !src.LastSuccessUnixMs.Valid || src.LastSuccessUnixMs.Int64 <= 0
			obsmetrics.SetControllerCollectionSourceStale(src.ID, src.CollectorType, stale)
			continue
		}
		sched, err := cronStandardParser.Parse(cronLine)
		if err != nil {
			obsmetrics.SetControllerCollectionSourceStale(src.ID, src.CollectorType, true)
			continue
		}
		age := successAgeSeconds(src, now)
		obsmetrics.SetControllerCollectionSourceLastSuccessAge(src.ID, src.CollectorType, age)
		interval := cronInterval(sched, now)
		stale := sourceIsStale(src, now, age, interval)
		obsmetrics.SetControllerCollectionSourceStale(src.ID, src.CollectorType, stale)
	}
}

func successAgeSeconds(src db.CollectionSourceRow, now time.Time) float64 {
	if !src.LastSuccessUnixMs.Valid || src.LastSuccessUnixMs.Int64 <= 0 {
		return -1
	}
	return now.Sub(time.UnixMilli(src.LastSuccessUnixMs.Int64)).Seconds()
}

func cronInterval(sched cron.Schedule, now time.Time) time.Duration {
	next := sched.Next(now)
	following := sched.Next(next)
	if following.After(next) {
		return following.Sub(next)
	}
	return time.Hour
}

//gocyclo:ignore
func sourceIsStale(src db.CollectionSourceRow, now time.Time, successAge float64, interval time.Duration) bool {
	if !src.LastSuccessUnixMs.Valid || src.LastSuccessUnixMs.Int64 <= 0 {
		return true
	}
	threshold := 2 * interval
	if successAge < 0 {
		return true
	}
	if time.Duration(successAge*float64(time.Second)) > threshold {
		return true
	}
	if src.LastRunUnixMs.Valid && src.LastRunUnixMs.Int64 > 0 {
		runAge := now.Sub(time.UnixMilli(src.LastRunUnixMs.Int64))
		if runAge <= threshold && time.Duration(successAge*float64(time.Second)) > threshold/2 {
			return true
		}
	}
	return false
}

func (r *Refresher) refreshEntityFreshness(ctx context.Context) {
	if r.entitiesDB == nil {
		return
	}
	now := time.Now()
	for _, probe := range r.entityProbes {
		age, err := queryEntityFreshnessAge(ctx, r.entitiesDB, probe.Table, probe.WhereClause)
		if err != nil {
			r.log.WithError(err).WithField("table", probe.Table).Debug("health metrics: entity freshness query failed")
			continue
		}
		if age < 0 {
			continue
		}
		obsmetrics.SetControllerEntityFreshnessAge(probe.EntityType, probe.SourceSystem, age)
		_ = now
	}
}

func queryEntityFreshnessAge(ctx context.Context, dbConn *sql.DB, table, whereClause string) (float64, error) {
	q := "SELECT MAX(`updated_at`) FROM `" + table + "`"
	if strings.TrimSpace(whereClause) != "" {
		q += " WHERE " + whereClause
	}
	var ts sql.NullTime
	if err := dbConn.QueryRowContext(ctx, q).Scan(&ts); err != nil {
		return -1, err
	}
	if !ts.Valid {
		return -1, nil
	}
	return time.Since(ts.Time).Seconds(), nil
}

// PipelineHealth holds computed pipeline health for a collection source (RPC/UI).
type PipelineHealth struct {
	SecondsSinceLastSuccess   int64
	IsStale                   bool
	EntityFreshnessAgeSeconds int64
}

// ComputePipelineHealth derives UI/RPC health fields for one collection source.
//
//gocyclo:ignore
func ComputePipelineHealth(src db.CollectionSourceRow, entityFreshnessByCollector map[string]int64, now time.Time) PipelineHealth {
	out := PipelineHealth{}
	if src.LastSuccessUnixMs.Valid && src.LastSuccessUnixMs.Int64 > 0 {
		out.SecondsSinceLastSuccess = int64(now.Sub(time.UnixMilli(src.LastSuccessUnixMs.Int64)).Seconds())
		if out.SecondsSinceLastSuccess < 0 {
			out.SecondsSinceLastSuccess = 0
		}
	}
	cronLine := strings.TrimSpace(src.CronLine)
	if !src.Enabled {
		out.IsStale = false
	} else if cronLine == "" {
		out.IsStale = !src.LastSuccessUnixMs.Valid || src.LastSuccessUnixMs.Int64 <= 0
	} else if sched, err := cronStandardParser.Parse(cronLine); err != nil {
		out.IsStale = true
	} else {
		interval := cronInterval(sched, now)
		out.IsStale = sourceIsStale(src, now, float64(out.SecondsSinceLastSuccess), interval)
	}
	if age, ok := entityFreshnessByCollector[src.CollectorType]; ok && age >= 0 {
		out.EntityFreshnessAgeSeconds = age
	}
	return out
}

// EntityFreshnessByCollector returns the max entity row age in seconds per collector_type.
func (r *Refresher) EntityFreshnessByCollector(ctx context.Context) map[string]int64 {
	out := map[string]int64{}
	if r.entitiesDB == nil {
		return out
	}
	type mapping struct {
		collectorType string
		probe         entityFreshnessProbe
	}
	mappings := []mapping{
		{"collector-github", defaultEntityProbes[0]},
	}
	for _, m := range mappings {
		age, err := queryEntityFreshnessAge(ctx, r.entitiesDB, m.probe.Table, m.probe.WhereClause)
		if err != nil || age < 0 {
			continue
		}
		out[m.collectorType] = int64(age)
	}
	return out
}
