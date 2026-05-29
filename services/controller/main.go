package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/jamesread/golure/pkg/dirs"
	"github.com/jamesread/golure/pkg/listenaddr"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/icehive/icehive/services/common/pkg/amqpctl"
	"github.com/icehive/icehive/services/common/pkg/buildinfo"
	"github.com/icehive/icehive/services/common/pkg/common"
	controlv1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/control/v1"
	icehivev1 "github.com/icehive/icehive/services/common/pkg/gen/icehive/v1"
	"github.com/icehive/icehive/services/common/pkg/gen/icehive/v1/icehivev1connect"
	"github.com/icehive/icehive/services/common/pkg/httpshim"
	"github.com/icehive/icehive/services/common/pkg/logging"
	"github.com/icehive/icehive/services/controller/internal/db"
)

type controllerSrv struct {
	mu         sync.RWMutex
	db         *sql.DB
	k          *koanf.Koanf
	amqpClient *amqpctl.Client
}

type migrationLogger struct {
	log *logrus.Logger
}

func persisterMySQLSettingsFromConfig(k *koanf.Koanf) (db.PersisterMySQLSettings, bool) {
	if k == nil || !k.Exists("persister_mysql.host") {
		return db.PersisterMySQLSettings{}, false
	}
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
	}, true
}

func (m migrationLogger) CurrentVersion(version uint, dirty bool) {
	m.log.WithFields(logrus.Fields{
		"version": version,
		"dirty":   dirty,
	}).Info("database migration current version")
}

func (m migrationLogger) Applied(version uint) {
	m.log.WithField("version", version).Info("database migration applied")
}

func (m migrationLogger) NoChange(version uint, dirty bool) {
	m.log.WithFields(logrus.Fields{
		"version": version,
		"dirty":   dirty,
	}).Info("database migration no change")
}

func heartbeatStatus(latestMs int64) string {
	if latestMs <= 0 {
		return "unknown"
	}
	if time.Now().UnixMilli()-latestMs <= 30000 {
		return "healthy"
	}
	return "stale"
}

func (s *controllerSrv) Health(
	ctx context.Context,
	_ *connect.Request[icehivev1.HealthRequest],
) (*connect.Response[icehivev1.HealthResponse], error) {
	if err := s.db.PingContext(ctx); err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, err)
	}
	return connect.NewResponse(&icehivev1.HealthResponse{Status: "ok"}), nil
}

func (s *controllerSrv) WorkerBootstrap(
	ctx context.Context,
	req *connect.Request[icehivev1.WorkerBootstrapRequest],
) (*connect.Response[icehivev1.WorkerBootstrapResponse], error) {
	_ = req.Msg

	settings, err := db.LoadAMQPBootstrapSettings(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	url := strings.TrimSpace(settings.URL)
	ex := strings.TrimSpace(settings.Exchange)
	if ex == "" {
		ex = amqpctl.DefaultControlExchange
	}
	rk := strings.TrimSpace(settings.RoutingKeyControlEvents)
	if rk == "" {
		rk = amqpctl.RoutingKeyControlEvents
	}
	resp := &icehivev1.WorkerBootstrapResponse{
		Amqp: &icehivev1.AMQPSettings{
			Url:                     url,
			Exchange:                ex,
			RoutingKeyControlEvents: rk,
		},
	}
	if strings.EqualFold(strings.TrimSpace(req.Msg.GetWorkerKind()), "persister") {
		mysqlSettings, mysqlErr := db.LoadPersisterMySQLSettings(ctx, s.db)
		if mysqlErr != nil {
			if fallback, ok := persisterMySQLSettingsFromConfig(s.k); ok {
				mysqlSettings = fallback
			} else {
				return nil, connect.NewError(connect.CodeFailedPrecondition, mysqlErr)
			}
		}
		resp.Mysql = &icehivev1.MySQLSettings{
			Host:     mysqlSettings.Host,
			Port:     int32(mysqlSettings.Port),
			User:     mysqlSettings.User,
			Password: mysqlSettings.Password,
			Database: mysqlSettings.Database,
		}
	}
	return connect.NewResponse(resp), nil
}

func configKeyRedacted(key string) bool {
	k := strings.ToLower(key)
	return strings.Contains(k, "password") || strings.Contains(k, "secret") || strings.Contains(k, "token")
}

func formatConfigValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	default:
		return fmt.Sprint(x)
	}
}

func (s *controllerSrv) ListConfig(
	ctx context.Context,
	_ *connect.Request[icehivev1.ListConfigRequest],
) (*connect.Response[icehivev1.ListConfigResponse], error) {
	rows, err := db.ListMeta(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	vars := make([]*icehivev1.ConfigVar, 0, len(rows))
	for _, kv := range rows {
		key := kv[0]
		raw := formatConfigValue(kv[1])
		redacted := configKeyRedacted(key)
		if redacted {
			raw = ""
		}
		vars = append(vars, &icehivev1.ConfigVar{
			Key:      key,
			Value:    raw,
			Redacted: redacted,
		})
	}
	return connect.NewResponse(&icehivev1.ListConfigResponse{Vars: vars}), nil
}

func (s *controllerSrv) GetConfig(
	ctx context.Context,
	req *connect.Request[icehivev1.GetConfigRequest],
) (*connect.Response[icehivev1.GetConfigResponse], error) {
	key := strings.TrimSpace(req.Msg.GetKey())
	if key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("empty config key"))
	}
	created := false
	value, found, err := db.GetMeta(ctx, s.db, key)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !found {
		if err := db.SetMeta(ctx, s.db, key, ""); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		value = ""
		created = true
	}
	redacted := configKeyRedacted(key)
	logrus.WithFields(logrus.Fields{
		"key":      key,
		"created":  created,
		"redacted": redacted,
	}).Info("config key get request")
	return connect.NewResponse(&icehivev1.GetConfigResponse{
		Var: &icehivev1.ConfigVar{
			Key:      key,
			Value:    value,
			Redacted: redacted,
		},
	}), nil
}

func (s *controllerSrv) SetConfig(
	ctx context.Context,
	req *connect.Request[icehivev1.SetConfigRequest],
) (*connect.Response[icehivev1.SetConfigResponse], error) {
	key := strings.TrimSpace(req.Msg.GetKey())
	if key == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("empty config key"))
	}

	if err := db.SetMeta(ctx, s.db, key, req.Msg.GetValue()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&icehivev1.SetConfigResponse{}), nil
}

func (s *controllerSrv) ListServices(
	ctx context.Context,
	_ *connect.Request[icehivev1.ListServicesRequest],
) (*connect.Response[icehivev1.ListServicesResponse], error) {
	rows, err := db.ListHeartbeats(ctx, s.db)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*icehivev1.ServiceStatus, 0, len(rows))
	for _, r := range rows {
		out = append(out, &icehivev1.ServiceStatus{
			ServiceName:           r.ServiceName,
			LatestHeartbeatUnixMs: r.LatestHeartbeatUnixMs,
			Status:                heartbeatStatus(r.LatestHeartbeatUnixMs),
		})
	}
	return connect.NewResponse(&icehivev1.ListServicesResponse{Services: out}), nil
}

func collectionSourceToProto(r db.CollectionSourceRow) *icehivev1.CollectionSource {
	p := &icehivev1.CollectionSource{
		Id:            r.ID,
		CollectorType: r.CollectorType,
		SourceSpec:    r.SourceSpec,
		CronLine:      r.CronLine,
		Enabled:       r.Enabled,
		CreatedUnixMs: r.CreatedAt.UnixMilli(),
		UpdatedUnixMs: r.UpdatedAt.UnixMilli(),
	}
	if r.LastRunUnixMs.Valid {
		p.LastRunUnixMs = r.LastRunUnixMs.Int64
	}
	if r.LastSuccessUnixMs.Valid {
		p.LastSuccessUnixMs = r.LastSuccessUnixMs.Int64
	}
	if r.LastError.Valid {
		p.LastError = r.LastError.String
	}
	if r.NextDueUnixMs.Valid {
		p.NextDueUnixMs = r.NextDueUnixMs.Int64
	}
	return p
}

func (s *controllerSrv) ListCollectionSources(
	ctx context.Context,
	req *connect.Request[icehivev1.ListCollectionSourcesRequest],
) (*connect.Response[icehivev1.ListCollectionSourcesResponse], error) {
	rows, err := db.ListCollectionSources(ctx, s.db, req.Msg.GetCollectorType())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*icehivev1.CollectionSource, 0, len(rows))
	for _, r := range rows {
		out = append(out, collectionSourceToProto(r))
	}
	return connect.NewResponse(&icehivev1.ListCollectionSourcesResponse{Sources: out}), nil
}

func (s *controllerSrv) ListCollectorSourceSchemas(
	ctx context.Context,
	req *connect.Request[icehivev1.ListCollectorSourceSchemasRequest],
) (*connect.Response[icehivev1.ListCollectorSourceSchemasResponse], error) {
	rows, err := db.ListCollectorSourceSchemas(ctx, s.db, req.Msg.GetCollectorType())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	out := make([]*icehivev1.CollectorSourceSchema, 0, len(rows))
	for _, r := range rows {
		out = append(out, &icehivev1.CollectorSourceSchema{
			CollectorType:  r.CollectorType,
			SchemaVersion:  r.SchemaVersion,
			BodyJson:       string(r.BodyJSON),
			UpdatedUnixMs:  r.UpdatedUnixMs,
		})
	}
	return connect.NewResponse(&icehivev1.ListCollectorSourceSchemasResponse{Schemas: out}), nil
}

func (s *controllerSrv) UpsertCollectionSource(
	ctx context.Context,
	req *connect.Request[icehivev1.UpsertCollectionSourceRequest],
) (*connect.Response[icehivev1.UpsertCollectionSourceResponse], error) {
	src := req.Msg.GetSource()
	if src == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("missing source"))
	}
	row := db.CollectionSourceRow{
		ID:            src.GetId(),
		CollectorType: src.GetCollectorType(),
		SourceSpec:    src.GetSourceSpec(),
		CronLine:      src.GetCronLine(),
		Enabled:       src.GetEnabled(),
	}
	saved, err := db.UpsertCollectionSource(ctx, s.db, row)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&icehivev1.UpsertCollectionSourceResponse{
		Source: collectionSourceToProto(saved),
	}), nil
}

func (s *controllerSrv) DeleteCollectionSource(
	ctx context.Context,
	req *connect.Request[icehivev1.DeleteCollectionSourceRequest],
) (*connect.Response[icehivev1.DeleteCollectionSourceResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("empty id"))
	}
	if err := db.DeleteCollectionSource(ctx, s.db, id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&icehivev1.DeleteCollectionSourceResponse{}), nil
}

func (s *controllerSrv) setAMQPClient(c *amqpctl.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.amqpClient = c
}

func (s *controllerSrv) getAMQPClient() *amqpctl.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.amqpClient
}

func (s *controllerSrv) EnqueueCollectionRequest(
	ctx context.Context,
	req *connect.Request[icehivev1.EnqueueCollectionRequestRequest],
) (*connect.Response[icehivev1.EnqueueCollectionRequestResponse], error) {
	amqpClient := s.getAMQPClient()
	if amqpClient == nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("AMQP not connected yet"))
	}
	switch t := req.Msg.GetTarget().(type) {
	case *icehivev1.EnqueueCollectionRequestRequest_CollectionSourceId:
		id := strings.TrimSpace(t.CollectionSourceId)
		if id == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("empty collection_source_id"))
		}
		row, err := db.GetCollectionSourceByID(ctx, s.db, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil, connect.NewError(connect.CodeNotFound, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		msg := &icehivev1.CollectionRequest{Source: collectionSourceToProto(row)}
		body, err := protojson.Marshal(msg)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		rk := amqpctl.CollectorCollectionRequestRoutingKey(row.CollectorType)
		if err := amqpClient.PublishJSON(ctx, rk, body); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case *icehivev1.EnqueueCollectionRequestRequest_EphemeralCollection:
		src := t.EphemeralCollection
		if src == nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("ephemeral_collection is empty"))
		}
		collType := strings.TrimSpace(src.GetCollectorType())
		spec := strings.TrimSpace(src.GetSourceSpec())
		if collType == "" || spec == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("ephemeral_collection requires collector_type and source_spec"))
		}
		ephemeral := proto.Clone(src).(*icehivev1.CollectionSource)
		ephemeral.Id = ""
		msg := &icehivev1.CollectionRequest{Source: ephemeral}
		body, err := protojson.Marshal(msg)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		rk := amqpctl.CollectorCollectionRequestRoutingKey(collType)
		if err := amqpClient.PublishJSON(ctx, rk, body); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	case nil:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("specify collection_source_id or ephemeral_collection"))
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown enqueue target"))
	}
	return connect.NewResponse(&icehivev1.EnqueueCollectionRequestResponse{}), nil
}

func (s *controllerSrv) ReportCollectionSourceRun(
	ctx context.Context,
	req *connect.Request[icehivev1.ReportCollectionSourceRunRequest],
) (*connect.Response[icehivev1.ReportCollectionSourceRunResponse], error) {
	id := strings.TrimSpace(req.Msg.GetId())
	if id == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("empty id"))
	}
	if err := db.ReportCollectionSourceRun(
		ctx, s.db, id,
		req.Msg.GetRunUnixMs(),
		req.Msg.GetSuccess(),
		req.Msg.GetError(),
		req.Msg.GetNextDueUnixMs(),
	); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&icehivev1.ReportCollectionSourceRunResponse{}), nil
}

func loadConfig(dir string) (*koanf.Koanf, string, error) {
	k := koanf.New(".")
	for _, name := range []string{"config.yaml", "controller.yaml"} {
		path, err := dirs.GetFirstExistingFileFromDirs("controller", []string{dir}, name)
		if err != nil {
			continue
		}
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return nil, "", fmt.Errorf("load %s: %w", name, err)
		}
		return k, path, nil
	}
	return nil, "", fmt.Errorf("no config.yaml or controller.yaml found under %q", dir)
}

// bundledMigrationsDir holds SQL revisions baked into official container images (see Dockerfile.goreleaser).
const bundledMigrationsDir = "/opt/ih/migrations"

// migrationsDirectory returns bundled image migrations when present, otherwise `<configDir>/migrations/` for local dev and tests.
func migrationsDirectory(configDir string) string {
	if st, err := os.Stat(bundledMigrationsDir); err == nil && st.IsDir() {
		return bundledMigrationsDir
	}
	return filepath.Join(configDir, "migrations")
}

func openMySQLAndMigrate(ctx context.Context, log *logrus.Logger, k *koanf.Koanf, configDir string) (*sql.DB, error) {
	ms, ok := db.SettingsFromKoanf(k)
	if !ok {
		return nil, fmt.Errorf("mysql configuration missing (need mysql.host and related fields)")
	}
	if err := ms.Validate(); err != nil {
		return nil, err
	}
	migrateURL, err := db.MigrateDatabaseURL(ms)
	if err != nil {
		return nil, err
	}
	migrationsDir := migrationsDirectory(configDir)
	log.WithField("migrations_path", migrationsDir).Info("running database migrations")
	if err := db.RunMigrations(migrationsDir, migrateURL, migrationLogger{log: log}); err != nil {
		return nil, fmt.Errorf("migrations: %w", err)
	}
	dsn, err := db.SQLDSN(ms)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.Open(dsn)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	log.Info("connected to MySQL after migrations")
	return sqlDB, nil
}

func openMySQLUntilReady(ctx context.Context, log *logrus.Logger, k *koanf.Koanf, configDir string) (*sql.DB, error) {
	for attempt := 1; ; attempt++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("shutdown before database ready: %w", ctx.Err())
		default:
		}

		sqlDB, err := openMySQLAndMigrate(ctx, log, k, configDir)
		if err == nil {
			return sqlDB, nil
		}

		log.WithError(err).Warnf("database not ready (attempt %d); retrying in 5s", attempt)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func openAMQUntilReady(ctx context.Context, log *logrus.Logger, sqlDB *sql.DB) (*amqpctl.Client, error) {
	for attempt := 1; ; attempt++ {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("shutdown before amqp ready: %w", ctx.Err())
		default:
		}

		settings, err := db.LoadAMQPBootstrapSettings(ctx, sqlDB)
		if err != nil {
			log.WithError(err).Warnf("AMQP status=disconnected (attempt %d); metadata not ready; retrying in 10s", attempt)
		} else {
			c, connErr := amqpctl.Connect(ctx, amqpctl.Config{
				URL:            strings.TrimSpace(settings.URL),
				Exchange:       strings.TrimSpace(settings.Exchange),
				ConnectionName: "ih_controller",
			})
			if connErr == nil {
				log.WithFields(logrus.Fields{
					"url":      settings.URL,
					"exchange": settings.Exchange,
				}).Info("AMQP status=connected")
				return c, nil
			}
			log.WithError(connErr).Warnf("AMQP status=disconnected (attempt %d); connect failed; retrying in 10s", attempt)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

// controllerHTTPPasteBaseURL returns an http://hostname:port base URL suitable for pasting into
// the IceHive frontend (Connect RPC), using the machine hostname and the TCP port from listenAddr.
func controllerHTTPPasteBaseURL(listenAddr string) string {
	_, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return ""
	}
	hn, err := os.Hostname()
	if err != nil || strings.TrimSpace(hn) == "" {
		hn = "localhost"
	}
	return "http://" + hn + ":" + port
}

// controllerWelcomeHTML is a minimal browser-facing page for GET / (RPC lives under /icehive.v1.ControllerService/).
const controllerWelcomeHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + common.ProjectName + ` Controller</title>
</head>
<body>
<h1>` + common.ProjectName + ` Controller</h1>
<p>This server exposes the Connect RPC API at <code>/api/icehive.v1.ControllerService/</code> (and at <code>/icehive.v1.ControllerService/</code> for direct clients) and Prometheus metrics at <code>/metrics</code>.</p>
</body>
</html>
`

func serveControllerWelcome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	body := []byte(controllerWelcomeHTML)
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func main() {
	flagListen := flag.String("listen", "", `listen address; empty, auto, or :8080 uses golure listenaddr (config listen overrides when set)`)
	configDir := flag.String("configdir", ".", "directory containing config.yaml (migrations packaged at "+bundledMigrationsDir+" in container images)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log := logrus.New()
	logging.ApplyEnvLogLevel(log)
	log.WithFields(logrus.Fields{
		"version": buildinfo.Version,
		"commit":  buildinfo.Commit,
		"date":    buildinfo.Date,
	}).Infof("%s Controller starting", common.ProjectName)

	k, configPath, err := loadConfig(*configDir)
	if err != nil {
		log.WithError(err).Fatal("configuration")
	}
	absConfigPath := configPath
	if p, err := filepath.Abs(configPath); err == nil {
		absConfigPath = p
	}
	log.WithField("yaml_config_path", absConfigPath).Info("YAML configuration loaded")
	log.Infof("mysql.host=%s mysql.user=%s", k.String("mysql.host"), k.String("mysql.user"))

	listenAddr := strings.TrimSpace(*flagListen)
	if k.Exists("listen") {
		if v := strings.TrimSpace(k.String("listen")); v != "" {
			listenAddr = v
		}
	}
	// Empty, auto, or literal :8080 → golure (try PORT, 8080, then a free 8000–8999 port).
	if listenAddr == "" || strings.EqualFold(listenAddr, "auto") || listenAddr == ":8080" {
		auto, err := listenaddr.AvailableListenAddr()
		if err != nil {
			log.WithError(err).Fatal("listen address")
		}
		listenAddr = auto
	}

	sqlDB, err := openMySQLUntilReady(ctx, log, k, *configDir)
	if err != nil {
		log.WithError(err).Fatal("database")
	}
	defer func() { _ = sqlDB.Close() }()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", serveControllerWelcome)
	mux.HandleFunc("HEAD /{$}", serveControllerWelcome)
	mux.Handle("/metrics", promhttp.Handler())

	ctrlSrv := &controllerSrv{db: sqlDB, k: k}
	path, h := icehivev1connect.NewControllerServiceHandler(ctrlSrv)
	mux.Handle(path, h)
	apiInner := http.NewServeMux()
	apiInner.Handle(path, h)
	mux.Handle("/api/", http.StripPrefix("/api", apiInner))

	httpSrv := &http.Server{
		Addr:              listenAddr,
		Handler:           withCORS(httpshim.Wrap(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("http server failed")
		}
	}()
	if paste := controllerHTTPPasteBaseURL(listenAddr); paste != "" {
		log.Infof("listening on %s (frontend controller base URL: %s)", listenAddr, paste)
	} else {
		log.Infof("listening on %s", listenAddr)
	}

	amqpDone := make(chan struct{})
	go func() {
		defer close(amqpDone)
		amqpClient, err := openAMQUntilReady(ctx, log, sqlDB)
		if err != nil {
			log.WithError(err).Warn("amqp background bootstrap stopped")
			return
		}
		ctrlSrv.setAMQPClient(amqpClient)
		amqpClient.StartHeartbeatPublisher(ctx, "controller", 10*time.Second)
		go func() {
			hbQueue := amqpctl.QueueName("controller-heartbeats")
			if err := amqpClient.EnsureQueue(hbQueue, amqpctl.RoutingKeyHeartbeats); err != nil {
				log.WithError(err).Warn("heartbeat queue declare/bind failed")
				return
			}
			err := amqpClient.ConsumeControl(ctx, hbQueue, amqpctl.RoutingKeyHeartbeats, func(hctx context.Context, evt *controlv1.ControlEvent) error {
				p := evt.GetPing()
				if p == nil || strings.TrimSpace(p.GetSourceService()) == "" {
					return nil
				}
				ts := evt.GetCreatedUnixMs()
				if ts <= 0 {
					ts = time.Now().UnixMilli()
				}
				return db.UpsertHeartbeat(hctx, sqlDB, p.GetSourceService(), ts)
			})
			if err != nil && hctxErr(ctx) == nil {
				log.WithError(err).Warn("heartbeat consumer stopped")
			}
		}()
		go func() {
			bind := amqpctl.RoutingKeyCollectorSourceSchemaPrefix + ".#"
			schemaQueue := amqpctl.QueueName("controller-source-schemas")
			if err := amqpClient.EnsureQueue(schemaQueue, bind); err != nil {
				log.WithError(err).Warn("source schema queue declare/bind failed")
				return
			}
			err := amqpClient.ConsumeJSON(ctx, schemaQueue, bind, func(sctx context.Context, body []byte) error {
				return handleCollectorSourceSchemaMessage(sctx, log, sqlDB, body)
			})
			if err != nil && hctxErr(ctx) == nil {
				log.WithError(err).Warn("source schema consumer stopped")
			}
		}()
		<-ctx.Done()
		_ = amqpClient.Close()
		log.Info("AMQP status=disconnected")
	}()

	<-ctx.Done()
	log.Infof("shutdown signal (%v)", ctx.Err())

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.WithError(err).Fatal("graceful shutdown")
	}
	<-amqpDone
}

func hctxErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
