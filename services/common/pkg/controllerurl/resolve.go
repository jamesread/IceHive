package controllerurl

import (
	"os"
	"strings"
)

const (
	defaultURL    = "http://localhost:8080"
	envVar        = "ICEHIVE_CONTROLLER_URL"
	flagSource    = "-controller flag"
	envSource     = "ICEHIVE_CONTROLLER_URL"
	defaultSource = "default (" + defaultURL + ")"
)

// Resolved is a controller Connect base URL and where it was loaded from.
type Resolved struct {
	URL    string
	Source string
}

// Resolve returns the controller Connect base URL from flagValue, ICEHIVE_CONTROLLER_URL,
// or the localhost default, plus a short label for logs.
func Resolve(flagValue string) Resolved {
	if v := strings.TrimSpace(flagValue); v != "" {
		return Resolved{URL: v, Source: flagSource}
	}
	if v := strings.TrimSpace(os.Getenv(envVar)); v != "" {
		return Resolved{URL: v, Source: envSource}
	}
	return Resolved{URL: defaultURL, Source: defaultSource}
}
