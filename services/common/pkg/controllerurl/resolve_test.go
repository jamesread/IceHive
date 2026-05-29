package controllerurl

import "testing"

func TestResolve(t *testing.T) {
	t.Run("flag wins over env", func(t *testing.T) {
		t.Setenv(envVar, "http://env:8080")
		got := Resolve("http://flag:8080")
		if got.URL != "http://flag:8080" || got.Source != flagSource {
			t.Fatalf("got %+v, want URL=http://flag:8080 source=%q", got, flagSource)
		}
	})

	t.Run("env when flag empty", func(t *testing.T) {
		t.Setenv(envVar, "http://env:8080")
		got := Resolve("")
		if got.URL != "http://env:8080" || got.Source != envSource {
			t.Fatalf("got %+v, want URL=http://env:8080 source=%q", got, envSource)
		}
	})

	t.Run("default when unset", func(t *testing.T) {
		t.Setenv(envVar, "")
		got := Resolve("")
		if got.URL != defaultURL || got.Source != defaultSource {
			t.Fatalf("got %+v, want URL=%q source=%q", got, defaultURL, defaultSource)
		}
	})

	t.Run("trims whitespace", func(t *testing.T) {
		t.Setenv(envVar, "")
		got := Resolve("  http://flag:8080  ")
		if got.URL != "http://flag:8080" || got.Source != flagSource {
			t.Fatalf("got %+v", got)
		}
	})
}
