package main

import "testing"

func TestNormalizeJMAPAPIPostURL_FastmailSessionToAPI(t *testing.T) {
	for _, raw := range []string{
		"https://api.fastmail.com/jmap/session",
		"https://api.fastmail.com/jmap/session/",
	} {
		out, rew := normalizeJMAPAPIPostURL(raw)
		if !rew {
			t.Fatalf("expected rewrite for %q", raw)
		}
		if out != "https://api.fastmail.com/jmap/api/" {
			t.Fatalf("got %q from %q", out, raw)
		}
	}
}

func TestNormalizeJMAPAPIPostURL_AlreadyAPI(t *testing.T) {
	out, rew := normalizeJMAPAPIPostURL("https://api.fastmail.com/jmap/api/")
	if rew {
		t.Fatal("unexpected rewrite")
	}
	if out != "https://api.fastmail.com/jmap/api/" {
		t.Fatalf("got %q", out)
	}
}

func TestNormalizeJMAPAPIPostURL_NoFalsePositiveOnSessionPrefix(t *testing.T) {
	out, rew := normalizeJMAPAPIPostURL("https://example.com/jmap/sessionbackup")
	if rew {
		t.Fatalf("unexpected rewrite: %q", out)
	}
}

func TestResolveAPIURLAgainstSession_Relative(t *testing.T) {
	got := resolveAPIURLAgainstSession("https://api.fastmail.com/jmap/session", "/jmap/api/")
	if got != "https://api.fastmail.com/jmap/api/" {
		t.Fatalf("got %q", got)
	}
}
