package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// CollectorSourceSchemaRow is a persisted collector SourceSchema document.
type CollectorSourceSchemaRow struct {
	CollectorType string
	SchemaVersion string
	BodyJSON      []byte
	UpdatedUnixMs int64
}

// UpsertCollectorSourceSchema stores or replaces the JSON document for a collector_type.
//
//gocyclo:ignore
func UpsertCollectorSourceSchema(ctx context.Context, db *sql.DB, collectorType, schemaVersion string, bodyJSON []byte, updatedUnixMs int64) error {
	if db == nil {
		return fmt.Errorf("nil DB")
	}
	collectorType = strings.TrimSpace(collectorType)
	if collectorType == "" {
		return fmt.Errorf("empty collector_type")
	}
	schemaVersion = strings.TrimSpace(schemaVersion)
	if schemaVersion == "" {
		return fmt.Errorf("empty schema_version")
	}
	if len(bodyJSON) == 0 {
		return fmt.Errorf("empty body_json")
	}
	if !json.Valid(bodyJSON) {
		return fmt.Errorf("body_json is not valid JSON")
	}
	if updatedUnixMs <= 0 {
		return fmt.Errorf("invalid updated_unix_ms")
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO icehive_collector_source_schemas (collector_type, schema_version, body_json, updated_unix_ms)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			schema_version = VALUES(schema_version),
			body_json = VALUES(body_json),
			updated_unix_ms = VALUES(updated_unix_ms)`,
		collectorType, schemaVersion, string(bodyJSON), updatedUnixMs,
	)
	if err != nil {
		return fmt.Errorf("upsert collector source schema: %w", err)
	}
	return nil
}

// ListCollectorSourceSchemas returns stored schemas, optionally filtered by collector_type.
//
//gocyclo:ignore
func ListCollectorSourceSchemas(ctx context.Context, db *sql.DB, collectorType string) ([]CollectorSourceSchemaRow, error) {
	if db == nil {
		return nil, fmt.Errorf("nil DB")
	}
	collectorType = strings.TrimSpace(collectorType)
	var rows *sql.Rows
	var err error
	if collectorType == "" {
		rows, err = db.QueryContext(ctx, `
			SELECT collector_type, schema_version, body_json, updated_unix_ms
			FROM icehive_collector_source_schemas
			ORDER BY collector_type`)
	} else {
		rows, err = db.QueryContext(ctx, `
			SELECT collector_type, schema_version, body_json, updated_unix_ms
			FROM icehive_collector_source_schemas
			WHERE collector_type = ?
			ORDER BY collector_type`, collectorType)
	}
	if err != nil {
		return nil, fmt.Errorf("list collector source schemas: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []CollectorSourceSchemaRow
	for rows.Next() {
		var r CollectorSourceSchemaRow
		var raw string
		if err := rows.Scan(&r.CollectorType, &r.SchemaVersion, &raw, &r.UpdatedUnixMs); err != nil {
			return nil, fmt.Errorf("scan collector source schema: %w", err)
		}
		r.BodyJSON = []byte(raw)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collector source schemas: %w", err)
	}
	return out, nil
}
