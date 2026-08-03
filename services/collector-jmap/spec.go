package main

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/text/unicode/norm"
)

const prefixMailboxLower = "mailbox:"

// parseJmapMailboxSourceSpec returns a concrete mailbox id, or resolveRFCInbox=true to look up
// the account mailbox with JMAP role "inbox" (RFC 8621) at runtime via read-only Mailbox/query.
//
//gocyclo:ignore
func parseJmapMailboxSourceSpec(sourceSpec string) (mailboxID string, resolveRFCInbox bool, err error) {
	s := strings.TrimSpace(sourceSpec)
	if s == "" {
		return mailboxOrInboxFromEnv()
	}
	lower := strings.ToLower(s)
	if !strings.HasPrefix(lower, prefixMailboxLower) {
		return "", false, fmt.Errorf(`unsupported source_spec %q (expected mailbox:<id>, mailbox:env, mailbox:inbox, or empty for inbox/env)`, s)
	}
	rest := strings.TrimSpace(s[len(prefixMailboxLower):])
	if rest == "" {
		return "", false, fmt.Errorf(`missing value after %q`, prefixMailboxLower)
	}
	if strings.EqualFold(rest, "inbox") {
		return "", true, nil
	}
	if strings.EqualFold(rest, "env") {
		return mailboxOrInboxFromEnv()
	}
	return norm.NFC.String(rest), false, nil
}

func mailboxOrInboxFromEnv() (mailboxID string, resolveRFCInbox bool, err error) {
	id := strings.TrimSpace(os.Getenv("ICEHIVE_JMAP_MAILBOX_ID"))
	if id == "" {
		return "", true, nil
	}
	return norm.NFC.String(id), false, nil
}
