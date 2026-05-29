package controllerurl

import (
	"errors"
	"strings"

	"connectrpc.com/connect"
)

// APIPrefix is the path prefix when the controller is reached through the frontend ingress (/api proxy).
const APIPrefix = "/api"

// HasAPIPrefix reports whether base already ends with /api (after trimming trailing slashes).
func HasAPIPrefix(base string) bool {
	return strings.HasSuffix(strings.TrimRight(strings.TrimSpace(base), "/"), APIPrefix)
}

// WithAPIPrefix appends /api to base when it is not already present.
func WithAPIPrefix(base string) string {
	b := strings.TrimRight(strings.TrimSpace(base), "/")
	if b == "" || HasAPIPrefix(b) {
		return b
	}
	return b + APIPrefix
}

// IsMethodNotAllowed reports whether err is an HTTP 405 from the controller Connect client.
func IsMethodNotAllowed(err error) bool {
	if err == nil {
		return false
	}
	var ce *connect.Error
	if errors.As(err, &ce) {
		return ce.Code() == connect.CodeUnknown && strings.Contains(strings.ToLower(ce.Message()), "405")
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "405") && strings.Contains(msg, "method not allowed")
}
