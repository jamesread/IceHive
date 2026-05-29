package controllerurl

import (
	"crypto/tls"
	"net/http"
	"os"
	"strings"
	"sync"
)

const skipVerifyEnvVar = "ICEHIVE_CONTROLLER_SKIPVERIFY"

// SkipVerify reports whether ICEHIVE_CONTROLLER_SKIPVERIFY is set (non-empty).
func SkipVerify() bool {
	return strings.TrimSpace(os.Getenv(skipVerifyEnvVar)) != ""
}

var (
	httpClient     *http.Client
	httpClientOnce sync.Once
)

// HTTPClient returns an HTTP client for Connect calls to the controller.
// When ICEHIVE_CONTROLLER_SKIPVERIFY is set, TLS certificate verification is skipped.
func HTTPClient() *http.Client {
	httpClientOnce.Do(func() {
		tr, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			tr = &http.Transport{}
		} else {
			tr = tr.Clone()
		}
		if SkipVerify() {
			if tr.TLSClientConfig == nil {
				tr.TLSClientConfig = &tls.Config{}
			}
			tr.TLSClientConfig.InsecureSkipVerify = true
		}
		httpClient = &http.Client{Transport: tr}
	})
	return httpClient
}
