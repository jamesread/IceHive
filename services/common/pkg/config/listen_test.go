package config

import "testing"

func TestDefaultWorkerListen(t *testing.T) {
	t.Run("outside container", func(t *testing.T) {
		t.Setenv(containerImageEnvVar, "")

		if got := DefaultWorkerListen(":8081"); got != ":8081" {
			t.Fatalf("got %q, want :8081", got)
		}
	})

	t.Run("inside container", func(t *testing.T) {
		t.Setenv(containerImageEnvVar, "ghcr.io/jamesread/icehive:test")

		if got := DefaultWorkerListen(":8081"); got != containerListenAddr {
			t.Fatalf("got %q, want %s", got, containerListenAddr)
		}
	})
}
