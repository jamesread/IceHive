package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var cronStandardParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// CollectionSourceRow is a row in icehive_collection_sources.
type CollectionSourceRow struct {
	ID                string
	CollectorType     string
	SourceSpec        string
	CronLine          string
	Enabled           bool
	LastRunUnixMs     sql.NullInt64
	LastSuccessUnixMs sql.NullInt64
	LastError         sql.NullString
	NextDueUnixMs     sql.NullInt64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func validateCronLine(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		// Empty cron: source is run-now only (EnqueueCollectionRequest), never polled on a schedule.
		return nil
	}
	if len(s) > 256 {
		return fmt.Errorf("cron_line exceeds 256 characters")
	}
	if _, err := cronStandardParser.Parse(s); err != nil {
		return fmt.Errorf("invalid cron_line: %w", err)
	}
	return nil
}

// ListCollectionSources returns sources, optionally filtered by collector_type.
func ListCollectionSources(ctx context.Context, db *sql.DB, collectorType string) ([]CollectionSourceRow, error) {
	if db == nil {
		return nil, fmt.Errorf("nil DB")
	}
	collectorType = strings.TrimSpace(collectorType)
	var rows *sql.Rows
	var err error
	if collectorType == "" {
		rows, err = db.QueryContext(ctx, `
			SELECT id, collector_type, source_spec, cron_line, enabled,
			       last_run_unix_ms, last_success_unix_ms, last_error, next_due_unix_ms,
			       created_at, updated_at
			FROM icehive_collection_sources
			ORDER BY collector_type, id`)
	} else {
		rows, err = db.QueryContext(ctx, `
			SELECT id, collector_type, source_spec, cron_line, enabled,
			       last_run_unix_ms, last_success_unix_ms, last_error, next_due_unix_ms,
			       created_at, updated_at
			FROM icehive_collection_sources
			WHERE collector_type = ?
			ORDER BY id`, collectorType)
	}
	if err != nil {
		return nil, fmt.Errorf("list collection sources: %w", err)
	}
	defer rows.Close()

	var out []CollectionSourceRow
	for rows.Next() {
		var r CollectionSourceRow
		var enabled int
		if err := rows.Scan(
			&r.ID, &r.CollectorType, &r.SourceSpec, &r.CronLine, &enabled,
			&r.LastRunUnixMs, &r.LastSuccessUnixMs, &r.LastError, &r.NextDueUnixMs,
			&r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan collection source: %w", err)
		}
		r.Enabled = enabled != 0
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate collection sources: %w", err)
	}
	return out, nil
}

// UpsertCollectionSource inserts or updates a collection source.
func UpsertCollectionSource(ctx context.Context, db *sql.DB, r CollectionSourceRow) (CollectionSourceRow, error) {
	if db == nil {
		return CollectionSourceRow{}, fmt.Errorf("nil DB")
	}
	r.CollectorType = strings.TrimSpace(r.CollectorType)
	r.SourceSpec = strings.TrimSpace(r.SourceSpec)
	r.CronLine = strings.TrimSpace(r.CronLine)
	if r.CollectorType == "" {
		return CollectionSourceRow{}, fmt.Errorf("collector_type is required")
	}
	if r.SourceSpec == "" {
		return CollectionSourceRow{}, fmt.Errorf("source_spec is required")
	}
	if err := validateCronLine(r.CronLine); err != nil {
		return CollectionSourceRow{}, err
	}
	id := strings.TrimSpace(r.ID)
	if id == "" {
		var err error
		id, err = randomID()
		if err != nil {
			return CollectionSourceRow{}, fmt.Errorf("generate id: %w", err)
		}
	}
	enabled := 0
	if r.Enabled {
		enabled = 1
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO icehive_collection_sources (
			id, collector_type, source_spec, cron_line, enabled,
			last_run_unix_ms, last_success_unix_ms, last_error, next_due_unix_ms
		) VALUES (?, ?, ?, ?, ?, NULL, NULL, NULL, NULL)
		ON DUPLICATE KEY UPDATE
			collector_type = VALUES(collector_type),
			source_spec = VALUES(source_spec),
			cron_line = VALUES(cron_line),
			enabled = VALUES(enabled)`,
		id, r.CollectorType, r.SourceSpec, r.CronLine, enabled,
	)
	if err != nil {
		return CollectionSourceRow{}, fmt.Errorf("upsert collection source: %w", err)
	}
	return GetCollectionSourceByID(ctx, db, id)
}

// GetCollectionSourceByID loads one row by id.
func GetCollectionSourceByID(ctx context.Context, db *sql.DB, id string) (CollectionSourceRow, error) {
	if db == nil {
		return CollectionSourceRow{}, fmt.Errorf("nil DB")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return CollectionSourceRow{}, fmt.Errorf("empty id")
	}
	var r CollectionSourceRow
	var enabled int
	err := db.QueryRowContext(ctx, `
		SELECT id, collector_type, source_spec, cron_line, enabled,
		       last_run_unix_ms, last_success_unix_ms, last_error, next_due_unix_ms,
		       created_at, updated_at
		FROM icehive_collection_sources WHERE id = ?`, id,
	).Scan(
		&r.ID, &r.CollectorType, &r.SourceSpec, &r.CronLine, &enabled,
		&r.LastRunUnixMs, &r.LastSuccessUnixMs, &r.LastError, &r.NextDueUnixMs,
		&r.CreatedAt, &r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return CollectionSourceRow{}, fmt.Errorf("collection source not found: %s", id)
	}
	if err != nil {
		return CollectionSourceRow{}, fmt.Errorf("get collection source: %w", err)
	}
	r.Enabled = enabled != 0
	return r, nil
}

// DeleteCollectionSource removes a source by id.
func DeleteCollectionSource(ctx context.Context, db *sql.DB, id string) error {
	if db == nil {
		return fmt.Errorf("nil DB")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("empty id")
	}
	res, err := db.ExecContext(ctx, `DELETE FROM icehive_collection_sources WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete collection source: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("collection source not found: %s", id)
	}
	return nil
}

// ReportCollectionSourceRun updates run metadata after a collector attempt.
func ReportCollectionSourceRun(ctx context.Context, db *sql.DB, id string, runUnixMs int64, success bool, errMsg string, nextDueUnixMs int64) error {
	if db == nil {
		return fmt.Errorf("nil DB")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("empty id")
	}
	if runUnixMs <= 0 {
		return fmt.Errorf("invalid run_unix_ms")
	}
	var lastErr interface{}
	if success {
		lastErr = nil
	} else {
		lastErr = errMsg
	}
	var nextDue interface{}
	if nextDueUnixMs > 0 {
		nextDue = nextDueUnixMs
	} else {
		nextDue = nil
	}
	var res sql.Result
	var execErr error
	if success {
		res, execErr = db.ExecContext(ctx, `
			UPDATE icehive_collection_sources SET
				last_run_unix_ms = ?,
				last_success_unix_ms = ?,
				last_error = NULL,
				next_due_unix_ms = ?
			WHERE id = ?`,
			runUnixMs, runUnixMs, nextDue, id,
		)
	} else {
		res, execErr = db.ExecContext(ctx, `
			UPDATE icehive_collection_sources SET
				last_run_unix_ms = ?,
				last_error = ?,
				next_due_unix_ms = ?
			WHERE id = ?`,
			runUnixMs, lastErr, nextDue, id,
		)
	}
	if execErr != nil {
		return fmt.Errorf("report collection source run: %w", execErr)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("collection source not found: %s", id)
	}
	return nil
}
