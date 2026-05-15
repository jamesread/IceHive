package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

// Open opens a MySQL connection pool using a driver DSN from SQLDSN.
func Open(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql open: %w", err)
	}
	return db, nil
}
