package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"go.yaml.in/yaml/v3"
)

var unsafePathSegment = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// entityDirName returns the per-entity-type directory name (PascalCase entity_type from the bus).
func entityDirName(entityType string) (string, error) {
	dir := strings.TrimSpace(entityType)
	if dir == "" {
		return "", fmt.Errorf("empty entity_type")
	}
	if strings.Contains(dir, string(os.PathSeparator)) || dir == "." || dir == ".." {
		return "", fmt.Errorf("invalid entity_type %q", entityType)
	}
	return dir, nil
}

// entityFileName returns <entity-name>.yaml for the entity instance (stable per source_unique_id).
func entityFileName(sourceUniqueID, hashValue string) (string, error) {
	stem := strings.TrimSpace(sourceUniqueID)
	if stem == "" {
		stem = strings.TrimSpace(hashValue)
	}
	stem = unsafePathSegment.ReplaceAllString(stem, "_")
	stem = strings.Trim(stem, "._-")
	if stem == "" {
		return "", fmt.Errorf("empty entity name (source_unique_id and hash)")
	}
	return stem + ".yaml", nil
}

func entityFilePath(root, entityType, sourceUniqueID, hashValue string) (string, error) {
	dir, err := entityDirName(entityType)
	if err != nil {
		return "", err
	}
	name, err := entityFileName(sourceUniqueID, hashValue)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, dir, name), nil
}

type yamlStore struct {
	root string
	mu   sync.Mutex
}

func newYAMLStore(root string) (*yamlStore, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("empty data_dir")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve data_dir: %w", err)
	}
	return &yamlStore{root: abs}, nil
}

func (s *yamlStore) writeEntity(_ context.Context, msg *entityMessage) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeEntityLocked(msg)
}

func (s *yamlStore) writeEntityLocked(msg *entityMessage) (string, error) {
	if msg.Type != "Entity" || msg.SchemaVersion != "v1" {
		return "", fmt.Errorf("unsupported entity envelope type=%q schema_version=%q", msg.Type, msg.SchemaVersion)
	}
	meta := msg.Metadata
	path, err := entityFilePath(s.root, meta.EntityType, meta.SourceUniqueID, meta.SourceHash.HashValue)
	if err != nil {
		return "", err
	}
	body, err := yaml.Marshal(toPersistedEntity(msg))
	if err != nil {
		return "", fmt.Errorf("marshal yaml: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("rename %s: %w", path, err)
	}
	return path, nil
}
