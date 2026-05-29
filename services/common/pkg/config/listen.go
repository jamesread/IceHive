package config

import "os"

const (
	containerListenAddr  = "0.0.0.0:8080"
	containerImageEnvVar = "ICEHIVE_CONTAINER_IMAGE"
)

func runningInContainer() bool {
	return os.Getenv(containerImageEnvVar) != ""
}

// DefaultWorkerListen returns 0.0.0.0:8080 when ICEHIVE_CONTAINER_IMAGE is set (container image),
// otherwise the per-binary development fallback.
func DefaultWorkerListen(fallback string) string {
	if runningInContainer() {
		return containerListenAddr
	}
	return fallback
}
