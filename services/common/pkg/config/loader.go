// Package config loads optional Koanf YAML from a config directory for IceHive binaries.
package config

import (
	"fmt"
	"path/filepath"

	"github.com/jamesread/golure/pkg/dirs"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/sirupsen/logrus"
)

func absOrSelf(path string) string {
	if p, err := filepath.Abs(path); err == nil {
		return p
	}
	return path
}

// LoadOptionalYAML loads fileName from cfgDir when present; otherwise returns an empty Koanf.
func LoadOptionalYAML(log *logrus.Logger, cfgDir, searchLabel, fileName string) (*koanf.Koanf, error) {
	k := koanf.New(".")
	path, err := dirs.GetFirstExistingFileFromDirs(searchLabel, []string{cfgDir}, fileName)
	if err != nil {
		log.WithFields(logrus.Fields{
			"config_dir": absOrSelf(cfgDir),
			"yaml_file":  fileName,
		}).Info("optional YAML not found; using defaults")
		return k, nil //nolint:nilerr // missing optional configuration uses defaults.
	}
	if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("load %s: %w", fileName, err)
	}
	log.WithField("yaml_config_path", absOrSelf(path)).Info("YAML configuration loaded")
	return k, nil
}
