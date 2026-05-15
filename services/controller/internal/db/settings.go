package db

import (
	"fmt"
	"net"
	"strconv"

	"github.com/knadh/koanf/v2"
)

// Settings holds MySQL connection parameters from config (keys under mysql.*).
type Settings struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// SettingsFromKoanf returns mysql settings when mysql.host is set.
func SettingsFromKoanf(k *koanf.Koanf) (Settings, bool) {
	if k == nil || !k.Exists("mysql.host") {
		return Settings{}, false
	}
	port := k.Int("mysql.port")
	if port == 0 {
		port = 3306
	}
	return Settings{
		Host:     k.String("mysql.host"),
		Port:     port,
		User:     k.String("mysql.user"),
		Password: k.String("mysql.password"),
		Database: k.String("mysql.database"),
	}, true
}

// Validate returns an error if any required field is empty.
func (s Settings) Validate() error {
	if s.Host == "" {
		return fmt.Errorf("mysql.host is empty")
	}
	if s.User == "" {
		return fmt.Errorf("mysql.user is empty")
	}
	if s.Database == "" {
		return fmt.Errorf("mysql.database is empty")
	}
	return nil
}

// JoinHostPort returns host:port suitable for the go-sql-driver tcp address.
func (s Settings) JoinHostPort() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}
