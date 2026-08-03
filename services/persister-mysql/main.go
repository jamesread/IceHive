package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/go-sql-driver/mysql"
	"github.com/icehive/icehive/services/common/pkg/amqpctl"
	"github.com/icehive/icehive/services/common/pkg/bootstrap"
	"github.com/icehive/icehive/services/common/pkg/persist"
	"github.com/knadh/koanf/v2"
	"github.com/sirupsen/logrus"
)

func main() {
	persist.Main(persist.MainConfig{
		ID:            "mysql",
		DefaultListen: ":8084",
		ConfigYAML:    "persister-mysql.yaml",
		Work:          mysqlWork,
	})
}

type sourceHash struct {
	HashValue string `json:"hash_value"`
	HashType  string `json:"hash_type"`
}

type collectorMetadata struct {
	RecollectSpec       *string    `json:"recollect_spec"`
	SourceHash          sourceHash `json:"source_hash"`
	EntityType          string     `json:"entity_type"`
	SourceSystem        string     `json:"source_system"`
	SourceCollectorType string     `json:"source_collector_type"`
	SourceUniqueID      string     `json:"source_unique_id"`
	ObservedUnixMS      int64      `json:"observed_unix_ms"`
}

type fieldDescriptor struct {
	Type string `json:"type"`
}

type entityMessage struct {
	Structure     map[string]fieldDescriptor `json:"structure"`
	Values        map[string]any             `json:"values"`
	Type          string                     `json:"type"`
	SchemaVersion string                     `json:"schema_version"`
	Metadata      collectorMetadata          `json:"collectormetadata"`
}

type mysqlPersister struct {
	db      *sql.DB
	log     *logrus.Logger
	tableMu sync.Mutex
}

var identifierPattern = regexp.MustCompile(`[^a-z0-9_]`)

//gocyclo:ignore
func mysqlWork(ctx context.Context, _ *koanf.Koanf, log *logrus.Logger, boot *bootstrap.WorkerRuntime, amqpClient *amqpctl.Client) error {
	if boot == nil {
		return fmt.Errorf("controller bootstrap settings are required")
	}
	db, err := openTargetDB(ctx, boot)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	p := &mysqlPersister{db: db, log: log}
	queueName := amqpctl.QueueName("persister-mysql-entities")
	if err := amqpClient.EnsureQueue(queueName, amqpctl.RoutingKeyCollectorEntities); err != nil {
		return fmt.Errorf("declare entity queue: %w", err)
	}
	log.WithFields(logrus.Fields{
		"exchange":      boot.AMQPExchange,
		"queue":         queueName,
		"routing_key":   amqpctl.RoutingKeyCollectorEntities,
		"mysql_host":    boot.MySQLHost,
		"mysql_db":      boot.MySQLDatabase,
		"controller_db": "isolated",
	}).Info("MySQL persister consuming entity stream")

	consumeErr := amqpClient.ConsumeJSON(ctx, queueName, amqpctl.RoutingKeyCollectorEntities, func(hctx context.Context, body []byte) error {
		var msg entityMessage
		if err := json.Unmarshal(body, &msg); err != nil {
			log.WithError(err).WithField("body_len", len(body)).Warn("entity message: JSON decode failed")
			return fmt.Errorf("decode entity json: %w", err)
		}
		if err := p.persistEntity(hctx, &msg); err != nil {
			log.WithError(err).WithFields(logrus.Fields{
				"entity_type":           msg.Metadata.EntityType,
				"source_unique_id":      msg.Metadata.SourceUniqueID,
				"source_collector_type": msg.Metadata.SourceCollectorType,
				"source_system":         msg.Metadata.SourceSystem,
				"schema_version":        msg.SchemaVersion,
				"envelope_type":         msg.Type,
			}).Error("entity persist failed")
			return err
		}
		return nil
	})
	if consumeErr != nil && ctx.Err() != nil {
		return nil
	}
	return consumeErr
}

//gocyclo:ignore
func openTargetDB(ctx context.Context, boot *bootstrap.WorkerRuntime) (*sql.DB, error) {
	if strings.TrimSpace(boot.MySQLHost) == "" || strings.TrimSpace(boot.MySQLUser) == "" || strings.TrimSpace(boot.MySQLDatabase) == "" {
		return nil, fmt.Errorf("missing persister MySQL settings from controller bootstrap (need host,user,database,password)")
	}
	port := boot.MySQLPort
	if port <= 0 {
		port = 3306
	}
	cfg := mysql.NewConfig()
	cfg.User = boot.MySQLUser
	cfg.Passwd = boot.MySQLPassword
	cfg.Net = "tcp"
	cfg.Addr = boot.MySQLHost + ":" + strconv.Itoa(port)
	cfg.DBName = boot.MySQLDatabase
	cfg.Params = map[string]string{"parseTime": "true"}
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sql ping: %w", err)
	}
	return db, nil
}

//gocyclo:ignore
func (p *mysqlPersister) persistEntity(ctx context.Context, msg *entityMessage) error {
	if msg.Type != "Entity" || msg.SchemaVersion != "v1" {
		return fmt.Errorf("unsupported entity envelope type=%q schema_version=%q", msg.Type, msg.SchemaVersion)
	}
	tableName := sinkTableName(msg.Metadata.EntityType)
	if tableName == "" {
		return fmt.Errorf("empty entity_type")
	}
	if err := p.ensureTable(ctx, tableName, msg.Structure); err != nil {
		return err
	}
	if err := p.upsertEntity(ctx, tableName, msg); err != nil {
		return err
	}
	p.log.WithFields(logrus.Fields{
		"entity_type":           msg.Metadata.EntityType,
		"source_unique_id":      msg.Metadata.SourceUniqueID,
		"source_hash_value":     msg.Metadata.SourceHash.HashValue,
		"source_collector_type": msg.Metadata.SourceCollectorType,
		"table":                 tableName,
	}).Info("persisted entity write")
	return nil
}

func sinkTableName(entityType string) string {
	base := strings.ToLower(strings.TrimSpace(entityType))
	base = identifierPattern.ReplaceAllString(base, "")
	if base == "" {
		return ""
	}
	return base + "s"
}

func safeColumnName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	n = strings.ReplaceAll(n, "-", "_")
	n = identifierPattern.ReplaceAllString(n, "")
	return n
}

func quoteIdent(id string) string {
	return "`" + strings.ReplaceAll(id, "`", "``") + "`"
}

func sqlType(t string) string {
	switch t {
	case "int64":
		return "BIGINT"
	case "float64":
		return "DOUBLE"
	case "bool":
		return "BOOLEAN"
	default:
		return "TEXT"
	}
}

//gocyclo:ignore
func (p *mysqlPersister) ensureTable(ctx context.Context, tableName string, structure map[string]fieldDescriptor) error {
	p.tableMu.Lock()
	defer p.tableMu.Unlock()

	existing, err := p.tableColumns(ctx, tableName)
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		return p.createTable(ctx, tableName, structure)
	}
	if !existing["recollect_spec"] {
		stmt := fmt.Sprintf(
			"ALTER TABLE %s ADD COLUMN %s TEXT NULL",
			quoteIdent(tableName), quoteIdent("recollect_spec"),
		)
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("alter table %s add column recollect_spec: %w", tableName, err)
		}
		existing["recollect_spec"] = true
	}
	for field, desc := range structure {
		col := safeColumnName(field)
		if col == "" || existing[col] {
			continue
		}
		stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s NULL", quoteIdent(tableName), quoteIdent(col), sqlType(desc.Type))
		if _, err := p.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("alter table %s add column %s: %w", tableName, col, err)
		}
	}
	return nil
}

func (p *mysqlPersister) createTable(ctx context.Context, tableName string, structure map[string]fieldDescriptor) error {
	cols := []string{
		"`source_hash_value` CHAR(64) NOT NULL",
		"`source_unique_id` TEXT NOT NULL",
		"`source_collector_type` VARCHAR(255) NOT NULL",
		"`source_system` VARCHAR(255) NOT NULL",
		"`observed_unix_ms` BIGINT NOT NULL",
		"`recollect_spec` TEXT NULL",
	}
	for field, desc := range structure {
		col := safeColumnName(field)
		if col == "" {
			continue
		}
		cols = append(cols, fmt.Sprintf("%s %s NULL", quoteIdent(col), sqlType(desc.Type)))
	}
	cols = append(cols,
		"`created_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP",
		"`updated_at` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP",
		"UNIQUE KEY `uq_source_hash_value` (`source_hash_value`)",
	)
	stmt := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", quoteIdent(tableName), strings.Join(cols, ","))
	if _, err := p.db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("create table %s: %w", tableName, err)
	}
	return nil
}

func (p *mysqlPersister) tableColumns(ctx context.Context, tableName string) (map[string]bool, error) {
	rows, err := p.db.QueryContext(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = DATABASE() AND table_name = ?`, tableName)
	if err != nil {
		return nil, fmt.Errorf("query columns for %s: %w", tableName, err)
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		out[c] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate columns: %w", err)
	}
	return out, nil
}

//gocyclo:ignore
func (p *mysqlPersister) upsertEntity(ctx context.Context, tableName string, msg *entityMessage) error {
	meta := msg.Metadata
	columns := []string{"source_hash_value", "source_unique_id", "source_collector_type", "source_system", "observed_unix_ms", "recollect_spec"}
	var recollectVal any
	if meta.RecollectSpec != nil {
		recollectVal = *meta.RecollectSpec
	} else {
		recollectVal = nil
	}
	values := []any{
		meta.SourceHash.HashValue,
		meta.SourceUniqueID,
		meta.SourceCollectorType,
		meta.SourceSystem,
		meta.ObservedUnixMS,
		recollectVal,
	}
	for field := range msg.Structure {
		col := safeColumnName(field)
		if col == "" {
			continue
		}
		columns = append(columns, col)
		values = append(values, msg.Values[field])
	}
	quotedCols := make([]string, 0, len(columns))
	placeholders := make([]string, 0, len(columns))
	updates := make([]string, 0, len(columns))
	for _, col := range columns {
		quotedCols = append(quotedCols, quoteIdent(col))
		placeholders = append(placeholders, "?")
		if col == "source_hash_value" {
			continue
		}
		updates = append(updates, fmt.Sprintf("%s=VALUES(%s)", quoteIdent(col), quoteIdent(col)))
	}
	stmt := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		quoteIdent(tableName),
		strings.Join(quotedCols, ","),
		strings.Join(placeholders, ","),
		strings.Join(updates, ","),
	)
	if _, err := p.db.ExecContext(ctx, stmt, values...); err != nil {
		return fmt.Errorf("upsert %s entity %s: %w", tableName, meta.SourceUniqueID, err)
	}
	return nil
}
