package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// MigrationReporter receives migration progress updates.
type MigrationReporter interface {
	CurrentVersion(version uint, dirty bool)
	Applied(version uint)
	NoChange(version uint, dirty bool)
}

func readMigrationVersion(m *migrate.Migrate) (uint, bool, error) {
	v, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return v, dirty, nil
}

//gocyclo:ignore
func availableUpVersions(migrationsDir string) ([]uint, error) {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	versions := make([]uint, 0)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".up.sql") {
			continue
		}
		parts := strings.SplitN(name, "_", 2)
		if len(parts) < 2 {
			continue
		}
		n, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			continue
		}
		versions = append(versions, uint(n))
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	return versions, nil
}

// RunMigrations applies SQL migrations from migrationsDir using the given MySQL migrate URL.
//
//gocyclo:ignore
func RunMigrations(migrationsDir, mysqlMigrateURL string, r MigrationReporter) error {
	abs, err := filepath.Abs(migrationsDir)
	if err != nil {
		return fmt.Errorf("migrations path: %w", err)
	}
	sourceURL := MigrationsFileURL(abs)
	m, err := migrate.New(sourceURL, mysqlMigrateURL)
	if err != nil {
		return fmt.Errorf("migrate new: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	current, dirty, err := readMigrationVersion(m)
	if err != nil {
		return fmt.Errorf("read current migration version: %w", err)
	}
	if r != nil {
		r.CurrentVersion(current, dirty)
	}

	known, err := availableUpVersions(abs)
	if err != nil {
		return err
	}
	for _, target := range known {
		if target <= current {
			continue
		}
		if migrateErr := m.Migrate(target); migrateErr != nil && !errors.Is(migrateErr, migrate.ErrNoChange) {
			return fmt.Errorf("migrate to version %d: %w", target, migrateErr)
		}
		if r != nil {
			r.Applied(target)
		}
		current = target
	}
	finalVersion, finalDirty, err := readMigrationVersion(m)
	if err != nil {
		return fmt.Errorf("read final migration version: %w", err)
	}
	if r != nil {
		r.NoChange(finalVersion, finalDirty)
	}
	return nil
}
