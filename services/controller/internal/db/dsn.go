package db

import (
	"net/url"
	"path/filepath"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// SQLDSN returns a DSN for database/sql with the go-sql-driver/mysql driver.
func SQLDSN(s Settings) (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	cfg := mysqldriver.NewConfig()
	cfg.User = s.User
	cfg.Passwd = s.Password
	cfg.Net = "tcp"
	cfg.Addr = s.JoinHostPort()
	cfg.DBName = s.Database
	cfg.Params = map[string]string{
		"parseTime":       "true",
		"multiStatements": "true",
	}
	return cfg.FormatDSN(), nil
}

// MigrateDatabaseURL returns a URL for golang-migrate's MySQL database driver.
func MigrateDatabaseURL(s Settings) (string, error) {
	if err := s.Validate(); err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("parseTime", "true")
	q.Set("multiStatements", "true")
	addr := "tcp(" + s.JoinHostPort() + ")"
	u := &url.URL{
		Scheme:   "mysql",
		User:     url.UserPassword(s.User, s.Password),
		Host:     addr,
		Path:     "/" + s.Database,
		RawQuery: q.Encode(),
	}
	return u.String(), nil
}

// MigrationsFileURL returns a file:// URL for the migrations directory (absolute path).
func MigrationsFileURL(absMigrationsDir string) string {
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(absMigrationsDir)}
	return u.String()
}
