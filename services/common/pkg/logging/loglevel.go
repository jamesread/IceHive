// Package logging centralizes log configuration for IceHive binaries.
package logging

import (
	"fmt"
	"os"
	"strings"

	"github.com/icehive/icehive/services/common/pkg/common"
	"github.com/jamesread/golure/pkg/logsetup"
	"github.com/sirupsen/logrus"
)

// EnvLogLevel is the environment variable used to set the minimum log level.
// Values match logrus.ParseLevel (case-insensitive): trace, debug, info, warn, error, fatal, panic.
const EnvLogLevel = "LOG_LEVEL"

// ApplyEnvLogLevel sets the level on log (when non-nil) and on the logrus standard logger
// (for code that logs with the package-level logrus API). Empty EnvLogLevel defaults to info.
// Invalid values default to info and a single warning line is written to stderr.
func ApplyEnvLogLevel(log *logrus.Logger) {
	if err := logsetup.ApplyEnvLevel(log, EnvLogLevel, logrus.InfoLevel); err != nil {
		fmt.Fprintf(os.Stderr, "%s: invalid %s=%q (%v); using info\n",
			common.ProjectName, EnvLogLevel, strings.TrimSpace(os.Getenv(EnvLogLevel)), err)
	}
}
