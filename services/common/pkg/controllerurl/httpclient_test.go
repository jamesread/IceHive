package controllerurl

import "testing"

func TestSkipVerify(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		t.Setenv(skipVerifyEnvVar, "")
		if SkipVerify() {
			t.Fatal("expected false")
		}
	})

	t.Run("set", func(t *testing.T) {
		t.Setenv(skipVerifyEnvVar, "1")
		if !SkipVerify() {
			t.Fatal("expected true")
		}
	})

	t.Run("whitespace only", func(t *testing.T) {
		t.Setenv(skipVerifyEnvVar, "   ")
		if SkipVerify() {
			t.Fatal("expected false for whitespace")
		}
	})
}
