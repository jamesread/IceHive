package main

import (
	"os"
	"testing"
)

func TestParseJmapMailboxSourceSpec_Literal(t *testing.T) {
	id, inbox, err := parseJmapMailboxSourceSpec("mailbox:mb-123")
	if err != nil {
		t.Fatal(err)
	}
	if inbox {
		t.Fatal("expected explicit mailbox")
	}
	if id != "mb-123" {
		t.Fatalf("got %q", id)
	}
}

func TestParseJmapMailboxSourceSpec_InboxRole(t *testing.T) {
	id, inbox, err := parseJmapMailboxSourceSpec("mailbox:inbox")
	if err != nil {
		t.Fatal(err)
	}
	if !inbox {
		t.Fatal("expected resolveRFCInbox")
	}
	if id != "" {
		t.Fatalf("expected empty id, got %q", id)
	}
}

func TestParseJmapMailboxSourceSpec_Env(t *testing.T) {
	t.Setenv("ICEHIVE_JMAP_MAILBOX_ID", "inbox-id")
	id, inbox, err := parseJmapMailboxSourceSpec("mailbox:env")
	if err != nil {
		t.Fatal(err)
	}
	if inbox {
		t.Fatal("expected explicit mailbox from env")
	}
	if id != "inbox-id" {
		t.Fatalf("got %q", id)
	}
}

func TestParseJmapMailboxSourceSpec_EmptyUsesEnv(t *testing.T) {
	t.Setenv("ICEHIVE_JMAP_MAILBOX_ID", "mb-x")
	id, inbox, err := parseJmapMailboxSourceSpec("")
	if err != nil {
		t.Fatal(err)
	}
	if inbox {
		t.Fatal("expected explicit mailbox from env")
	}
	if id != "mb-x" {
		t.Fatalf("got %q", id)
	}
}

func TestParseJmapMailboxSourceSpec_EmptyNoEnvMeansInboxQuery(t *testing.T) {
	_ = os.Unsetenv("ICEHIVE_JMAP_MAILBOX_ID")
	id, inbox, err := parseJmapMailboxSourceSpec("")
	if err != nil {
		t.Fatal(err)
	}
	if !inbox {
		t.Fatal("expected resolveRFCInbox when env unset")
	}
	if id != "" {
		t.Fatalf("expected empty id, got %q", id)
	}
}
