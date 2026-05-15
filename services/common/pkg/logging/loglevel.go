// Package logging centralizes log configuration for IceHive binaries.
package logging

import (
	"fmt"
	"os"
	"strings"

	"github.com/icehive/icehive/services/common/pkg/common"
	"github.com/sirupsen/logrus"
)

// EnvLogLevel is the environment variable used to set the minimum log level.
// Values match logrus.ParseLevel (case-insensitive): trace, debug, info, warn, error, fatal, panic.
const EnvLogLevel = "LOG_LEVEL"

// ApplyEnvLogLevel sets the level on log (when non-nil) and on the logrus standard logger
// (for code that logs with the package-level logrus API). Empty EnvLogLevel defaults to info.
// Invalid values default to info and a single warning line is written to stderr.
func ApplyEnvLogLevel(log *logrus.Logger) {
	level, warn := resolveLevelFromEnv()
	if warn != "" {
		fmt.Fprintln(os.Stderr, warn)
	}
	if log != nil {
		log.SetLevel(level)
	}
	logrus.SetLevel(level)
}

func resolveLevelFromEnv() (logrus.Level, string) {
	raw := strings.TrimSpace(os.Getenv(EnvLogLevel))
	if raw == "" {
		return logrus.InfoLevel, ""
	}
	level, err := logrus.ParseLevel(strings.ToLower(raw))
	if err != nil {
		return logrus.InfoLevel, fmt.Sprintf(
			"%s: invalid %s=%q (%v); using info",
			common.ProjectName, EnvLogLevel, raw, err,
		)
	}
	return level, ""
}
