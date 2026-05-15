package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// AMQPBootstrapSettings are controller-provided AMQP settings loaded from icehive_meta.
type AMQPBootstrapSettings struct {
	URL                     string
	Exchange                string
	RoutingKeyControlEvents string
}

// PersisterMySQLSettings are controller-provided sink DB settings loaded from icehive_meta.
type PersisterMySQLSettings struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

type HeartbeatRow struct {
	ServiceName           string
	LatestHeartbeatUnixMs int64
}

// LoadAMQPBootstrapSettings reads amqp.* keys from icehive_meta.
// Expected keys are:
// - amqp.url (preferred) OR amqp.host (+ optional amqp.port, amqp.user, amqp.password, amqp.vhost)
// - amqp.exchange
// - amqp.routing_key_control_events
func LoadAMQPBootstrapSettings(ctx context.Context, db *sql.DB) (AMQPBootstrapSettings, error) {
	if db == nil {
		return AMQPBootstrapSettings{}, fmt.Errorf("nil DB")
	}
	rows, err := db.QueryContext(ctx, `SELECT k, v FROM icehive_meta WHERE k LIKE 'amqp.%'`)
	if err != nil {
		return AMQPBootstrapSettings{}, fmt.Errorf("query icehive_meta amqp keys: %w", err)
	}
	defer rows.Close()

	vals := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return AMQPBootstrapSettings{}, fmt.Errorf("scan icehive_meta row: %w", err)
		}
		vals[k] = strings.TrimSpace(v)
	}
	if err := rows.Err(); err != nil {
		return AMQPBootstrapSettings{}, fmt.Errorf("iterate icehive_meta rows: %w", err)
	}

	s := AMQPBootstrapSettings{
		URL:                     vals["amqp.url"],
		Exchange:                vals["amqp.exchange"],
		RoutingKeyControlEvents: vals["amqp.routing_key_control_events"],
	}
	if s.URL == "" {
		host := vals["amqp.host"]
		if host == "" {
			return AMQPBootstrapSettings{}, fmt.Errorf("missing amqp.url or amqp.host in icehive_meta")
		}
		port := vals["amqp.port"]
		if port == "" {
			port = "5672"
		}
		user := vals["amqp.user"]
		pass := vals["amqp.password"]
		vhost := strings.TrimPrefix(vals["amqp.vhost"], "/")
		if vhost == "" {
			vhost = "/"
		}

		u := &url.URL{Scheme: "amqp", Host: host + ":" + port}
		if user != "" {
			u.User = url.UserPassword(user, pass)
		}
		if vhost == "/" {
			u.Path = "/"
		} else {
			u.Path = "/" + url.PathEscape(vhost)
		}
		s.URL = u.String()
	}
	return s, nil
}

// LoadPersisterMySQLSettings reads persister_mysql.* keys from icehive_meta.
func LoadPersisterMySQLSettings(ctx context.Context, db *sql.DB) (PersisterMySQLSettings, error) {
	if db == nil {
		return PersisterMySQLSettings{}, fmt.Errorf("nil DB")
	}
	rows, err := db.QueryContext(ctx, `SELECT k, v FROM icehive_meta WHERE k LIKE 'persister_mysql.%'`)
	if err != nil {
		return PersisterMySQLSettings{}, fmt.Errorf("query icehive_meta persister mysql keys: %w", err)
	}
	defer rows.Close()

	vals := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return PersisterMySQLSettings{}, fmt.Errorf("scan icehive_meta row: %w", err)
		}
		vals[k] = strings.TrimSpace(v)
	}
	if err := rows.Err(); err != nil {
		return PersisterMySQLSettings{}, fmt.Errorf("iterate icehive_meta rows: %w", err)
	}

	port := 3306
	if p := vals["persister_mysql.port"]; p != "" {
		n, convErr := strconv.Atoi(p)
		if convErr != nil {
			return PersisterMySQLSettings{}, fmt.Errorf("invalid persister_mysql.port %q", p)
		}
		port = n
	}
	s := PersisterMySQLSettings{
		Host:     vals["persister_mysql.host"],
		Port:     port,
		User:     vals["persister_mysql.user"],
		Password: vals["persister_mysql.password"],
		Database: vals["persister_mysql.database"],
	}
	if s.Host == "" || s.User == "" || s.Database == "" {
		return PersisterMySQLSettings{}, fmt.Errorf("missing persister_mysql settings in icehive_meta (need host,user,database,password)")
	}
	return s, nil
}

// SetMeta updates/creates a key in icehive_meta.
func SetMeta(ctx context.Context, db *sql.DB, key, value string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("empty meta key")
	}
	_, err := db.ExecContext(
		ctx,
		`INSERT INTO icehive_meta (k, v) VALUES (?, ?)
		 ON DUPLICATE KEY UPDATE v = VALUES(v), updated_at = CURRENT_TIMESTAMP`,
		key,
		value,
	)
	if err != nil {
		return fmt.Errorf("upsert icehive_meta %q: %w", key, err)
	}
	return nil
}

// ListMeta returns all key/value rows from icehive_meta sorted by key.
func ListMeta(ctx context.Context, db *sql.DB) ([][2]string, error) {
	if db == nil {
		return nil, fmt.Errorf("nil DB")
	}
	rows, err := db.QueryContext(ctx, `SELECT k, v FROM icehive_meta`)
	if err != nil {
		return nil, fmt.Errorf("query icehive_meta: %w", err)
	}
	defer rows.Close()

	out := make([][2]string, 0)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan icehive_meta row: %w", err)
		}
		out = append(out, [2]string{k, v})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate icehive_meta rows: %w", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i][0] < out[j][0] })
	return out, nil
}

// GetMeta returns a single key from icehive_meta.
func GetMeta(ctx context.Context, db *sql.DB, key string) (string, bool, error) {
	if db == nil {
		return "", false, fmt.Errorf("nil DB")
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false, fmt.Errorf("empty meta key")
	}
	var value string
	err := db.QueryRowContext(ctx, `SELECT v FROM icehive_meta WHERE k = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("query icehive_meta %q: %w", key, err)
	}
	return value, true, nil
}

func UpsertHeartbeat(ctx context.Context, db *sql.DB, serviceName string, latestUnixMs int64) error {
	if db == nil {
		return fmt.Errorf("nil DB")
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return fmt.Errorf("empty service name")
	}
	if latestUnixMs <= 0 {
		return fmt.Errorf("invalid heartbeat timestamp")
	}
	_, err := db.ExecContext(
		ctx,
		`INSERT INTO icehive_heartbeats (service_name, latest_heartbeat_unix_ms)
		 VALUES (?, ?)
		 ON DUPLICATE KEY UPDATE latest_heartbeat_unix_ms = VALUES(latest_heartbeat_unix_ms), updated_at = CURRENT_TIMESTAMP`,
		serviceName,
		latestUnixMs,
	)
	if err != nil {
		return fmt.Errorf("upsert heartbeat %q: %w", serviceName, err)
	}
	return nil
}

func ListHeartbeats(ctx context.Context, db *sql.DB) ([]HeartbeatRow, error) {
	if db == nil {
		return nil, fmt.Errorf("nil DB")
	}
	rows, err := db.QueryContext(ctx, `SELECT service_name, latest_heartbeat_unix_ms FROM icehive_heartbeats`)
	if err != nil {
		return nil, fmt.Errorf("query heartbeats: %w", err)
	}
	defer rows.Close()

	out := make([]HeartbeatRow, 0)
	for rows.Next() {
		var r HeartbeatRow
		if err := rows.Scan(&r.ServiceName, &r.LatestHeartbeatUnixMs); err != nil {
			return nil, fmt.Errorf("scan heartbeat row: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate heartbeat rows: %w", err)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ServiceName < out[j].ServiceName })
	return out, nil
}
