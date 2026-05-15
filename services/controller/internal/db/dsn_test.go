package db

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLDSN(t *testing.T) {
	s := Settings{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "u",
		Password: "p@ss",
		Database: "icehive",
	}
	dsn, err := SQLDSN(s)
	require.NoError(t, err)
	assert.Contains(t, dsn, "u")
	assert.Contains(t, dsn, "127.0.0.1:3306")
	assert.Contains(t, dsn, "icehive")
}

func TestMigrateDatabaseURL(t *testing.T) {
	s := Settings{
		Host:     "db.internal",
		Port:     3306,
		User:     "root",
		Password: "s3cret",
		Database: "app",
	}
	u, err := MigrateDatabaseURL(s)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(u, "mysql://"))
	assert.Contains(t, u, "tcp(db.internal:3306)")
	assert.Contains(t, u, "/app")
}
