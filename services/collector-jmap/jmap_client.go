package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/text/unicode/norm"
)

const (
	capMail = "urn:ietf:params:jmap:mail"

	// inboxEmailThreadLimit is how many recent threads to fetch per run (read-only Email/Thread queries).
	inboxEmailThreadLimit = 10
)

// jmapRuntime holds resolved session parameters for API calls.
type jmapRuntime struct {
	HTTPClient *http.Client
	APIURL     string
	AccountID  string
	Bearer     string
	// APIURLRewrittenFromSession is true when the configured URL pointed at a JMAP session resource
	// (GET-only) and was rewritten to the corresponding .../jmap/api/ POST endpoint.
	APIURLRewrittenFromSession bool
}

func bearerFromEnv() string {
	t := strings.TrimSpace(os.Getenv("ICEHIVE_JMAP_BEARER_TOKEN"))
	if t != "" {
		return t
	}
	return strings.TrimSpace(os.Getenv("ICEHIVE_JMAP_API_KEY"))
}

// normalizeJMAPAPIPostURL ensures POSTs go to the JMAP API endpoint, not the session resource.
// RFC 8620: GET the session URL; method calls are POSTed to apiUrl from the session (e.g. Fastmail
// session is .../jmap/session, API is .../jmap/api/). Misconfigured ICEHIVE_JMAP_API_URL often
// repeats the session URL and yields HTTP 405.
func normalizeJMAPAPIPostURL(raw string) (out string, rewritten bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	s = strings.TrimRight(s, "/")
	low := strings.ToLower(s)
	const marker = "/jmap/session"
	if j := strings.Index(low, marker); j >= 0 {
		tail := low[j+len(marker):]
		// Require /jmap/session as a path segment (not .../jmap/sessionbackup).
		if tail == "" || tail[0] == '/' || tail[0] == '?' || tail[0] == '#' {
			base := s[:j+len("/jmap")] + "/api"
			if tail != "" && (tail[0] == '?' || tail[0] == '#') {
				base += tail
			}
			s = base
			rewritten = true
		}
	}
	return s + "/", rewritten
}

// resolveAPIURLAgainstSession joins a relative session apiUrl to the session request origin.
func resolveAPIURLAgainstSession(sessionRequestURL, apiURL string) string {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		return ""
	}
	u, err := url.Parse(apiURL)
	if err != nil || u.IsAbs() {
		return apiURL
	}
	base, err := url.Parse(sessionRequestURL)
	if err != nil || base == nil {
		return apiURL
	}
	return base.ResolveReference(u).String()
}

func jmapRuntimeFromEnv(ctx context.Context) (*jmapRuntime, error) {
	bearer := bearerFromEnv()
	if bearer == "" {
		return nil, errors.New("set ICEHIVE_JMAP_BEARER_TOKEN or ICEHIVE_JMAP_API_KEY for JMAP Authorization")
	}
	sessionURL := strings.TrimSpace(os.Getenv("ICEHIVE_JMAP_SESSION_URL"))
	if sessionURL == "" {
		sessionURL = strings.TrimSpace(os.Getenv("ICEHIVE_JMAP_CONNECT_URL"))
	}
	apiURL := strings.TrimSpace(os.Getenv("ICEHIVE_JMAP_API_URL"))
	accountID := strings.TrimSpace(os.Getenv("ICEHIVE_JMAP_ACCOUNT_ID"))

	if sessionURL != "" {
		s, err := fetchJMAPSession(ctx, sessionURL, bearer)
		if err != nil {
			return nil, err
		}
		if s.APIURL == "" {
			return nil, errors.New("JMAP session response missing apiUrl")
		}
		apiURL = resolveAPIURLAgainstSession(sessionURL, s.APIURL)
		if accountID == "" {
			accountID = s.PrimaryMailAccount
		}
	}
	if apiURL == "" {
		return nil, errors.New("set ICEHIVE_JMAP_SESSION_URL (or ICEHIVE_JMAP_CONNECT_URL) or ICEHIVE_JMAP_API_URL")
	}
	if accountID == "" {
		return nil, errors.New("set ICEHIVE_JMAP_ACCOUNT_ID or use a session URL that exposes primaryAccounts for mail")
	}

	apiURL, rewritten := normalizeJMAPAPIPostURL(apiURL)
	return &jmapRuntime{
		HTTPClient:                 http.DefaultClient,
		APIURL:                     apiURL,
		AccountID:                  norm.NFC.String(accountID),
		Bearer:                     bearer,
		APIURLRewrittenFromSession: rewritten,
	}, nil
}

type sessionDoc struct {
	APIURL             string            `json:"apiUrl"`
	PrimaryAccounts    map[string]string `json:"primaryAccounts"`
	PrimaryMailAccount string
}

func fetchJMAPSession(ctx context.Context, sessionURL, bearer string) (*sessionDoc, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sessionURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("JMAP session GET %s: status %d: %s", sessionURL, res.StatusCode, truncateBytes(body, 512))
	}
	var s sessionDoc
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("JMAP session JSON: %w", err)
	}
	if strings.TrimSpace(s.APIURL) == "" {
		var top map[string]json.RawMessage
		if err := json.Unmarshal(body, &top); err == nil {
			for _, key := range []string{"apiUrl", "apiURL"} {
				raw, ok := top[key]
				if !ok {
					continue
				}
				var u string
				if json.Unmarshal(raw, &u) == nil {
					u = strings.TrimSpace(u)
					if u != "" {
						s.APIURL = u
						break
					}
				}
			}
		}
	}
	if s.PrimaryAccounts != nil {
		s.PrimaryMailAccount = strings.TrimSpace(s.PrimaryAccounts[capMail])
	}
	return &s, nil
}

func truncateBytes(b []byte, max int) string {
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "…"
}

type jmapEnvelope struct {
	MethodResponses []json.RawMessage `json:"methodResponses"`
}

func (rt *jmapRuntime) invokeJMAP(ctx context.Context, methodCalls []any) ([][]json.RawMessage, error) {
	payload := map[string]any{
		"using":        []string{"urn:ietf:params:jmap:core", capMail},
		"methodCalls":  methodCalls,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rt.APIURL, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+rt.Bearer)
	res, err := rt.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("JMAP POST %s: status %d: %s", rt.APIURL, res.StatusCode, truncateBytes(body, 1024))
	}
	var env jmapEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("JMAP response JSON: %w", err)
	}
	out := make([][]json.RawMessage, 0, len(env.MethodResponses))
	for _, mr := range env.MethodResponses {
		var triple []json.RawMessage
		if err := json.Unmarshal(mr, &triple); err != nil {
			return nil, fmt.Errorf("methodResponse tuple: %w", err)
		}
		if len(triple) < 2 {
			return nil, errors.New("short methodResponse")
		}
		out = append(out, triple)
	}
	return out, nil
}

func parseUTCDateString(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().UnixMilli()
		}
	}
	return 0
}

// resolveInboxMailboxID returns the mailbox id for role "inbox" (read-only Mailbox/query).
func (rt *jmapRuntime) resolveInboxMailboxID(ctx context.Context) (string, error) {
	queryArgs := map[string]any{
		"accountId": rt.AccountID,
		"filter":    map[string]any{"role": "inbox"},
		"limit":     1,
	}
	responses, err := rt.invokeJMAP(ctx, []any{[]any{"Mailbox/query", queryArgs, "mq1"}})
	if err != nil {
		return "", err
	}
	if len(responses) == 0 {
		return "", errors.New("empty JMAP methodResponses for Mailbox/query")
	}
	ids, err := parseMailboxQueryResponse(responses[0])
	if err != nil {
		return "", err
	}
	if len(ids) == 0 || strings.TrimSpace(ids[0]) == "" {
		return "", errors.New("no mailbox with role inbox found (Mailbox/query returned no ids)")
	}
	return norm.NFC.String(strings.TrimSpace(ids[0])), nil
}

func parseMailboxQueryResponse(triple []json.RawMessage) ([]string, error) {
	var name string
	if err := json.Unmarshal(triple[0], &name); err != nil {
		return nil, err
	}
	if name == "error" {
		return nil, fmt.Errorf("Mailbox/query: %s", string(triple[1]))
	}
	var args struct {
		IDs []string `json:"ids"`
	}
	if err := json.Unmarshal(triple[1], &args); err != nil {
		return nil, fmt.Errorf("Mailbox/query args: %w", err)
	}
	return args.IDs, nil
}

// listEmailThreads uses read-only JMAP methods only (Mailbox/query is used separately for inbox
// resolution): Email/query, Email/get, Thread/get. It returns up to inboxEmailThreadLimit threads
// newest-first in the given mailbox.
func (rt *jmapRuntime) listEmailThreads(ctx context.Context, log *logrus.Logger, mailboxID string) ([]emailThreadFields, error) {
	limit := inboxEmailThreadLimit
	queryArgs := map[string]any{
		"accountId": rt.AccountID,
		"filter":    map[string]any{"inMailbox": mailboxID},
		"sort": []any{
			map[string]any{"property": "receivedAt", "isAscending": false},
		},
		"collapseThreads": true,
		"limit":           limit,
	}
	if log != nil {
		log.WithFields(logrus.Fields{
			"jmap_account_id": rt.AccountID,
			"mailbox_id":      mailboxID,
			"query_limit":     limit,
		}).Info("jmap: Email/query (collapseThreads, newest first)")
	}
	responses, err := rt.invokeJMAP(ctx, []any{
		[]any{"Email/query", queryArgs, "q1"},
	})
	if err != nil {
		return nil, err
	}
	if len(responses) == 0 {
		return nil, errors.New("empty JMAP methodResponses")
	}
	ids, queryTotal, err := parseEmailQueryResponse(responses[0])
	if err != nil {
		return nil, err
	}
	if log != nil {
		fields := logrus.Fields{
			"email_query_id_count": len(ids),
			"mailbox_id":           mailboxID,
		}
		if queryTotal != nil {
			fields["email_query_total"] = *queryTotal
		}
		if len(ids) == 0 {
			log.WithFields(fields).Info("jmap: Email/query returned no ids (empty mailbox, no matches, or insufficient mayReadItems on this mailbox)")
		} else {
			log.WithFields(fields).Info("jmap: Email/query returned representative email ids for threads")
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}

	getArgs := map[string]any{
		"accountId":  rt.AccountID,
		"ids":        ids,
		"properties": []string{"threadId", "subject", "preview", "receivedAt"},
	}
	if log != nil {
		log.WithField("email_get_id_count", len(ids)).Info("jmap: Email/get (threadId, subject, preview, receivedAt)")
	}
	responses2, err := rt.invokeJMAP(ctx, []any{[]any{"Email/get", getArgs, "g1"}})
	if err != nil {
		return nil, err
	}
	emails, err := parseEmailGetList(responses2[0])
	if err != nil {
		return nil, err
	}
	if log != nil {
		log.WithField("email_get_list_count", len(emails)).Info("jmap: Email/get parsed list entries")
	}

	threadSeen := make(map[string]struct{})
	var threadOrder []string
	var missingThreadID int
	for _, e := range emails {
		tid := norm.NFC.String(strings.TrimSpace(e.ThreadID))
		if tid == "" {
			missingThreadID++
			continue
		}
		if _, ok := threadSeen[tid]; ok {
			continue
		}
		threadSeen[tid] = struct{}{}
		threadOrder = append(threadOrder, tid)
	}
	if log != nil {
		log.WithFields(logrus.Fields{
			"distinct_thread_ids": len(threadOrder),
			"emails_missing_thread_id": missingThreadID,
		}).Info("jmap: derived thread ids from Email/get")
	}
	if len(threadOrder) == 0 {
		if log != nil && len(emails) > 0 {
			log.Warn("jmap: Email/get returned messages but no usable threadId on any row (unexpected server response shape?)")
		}
		return nil, nil
	}

	threadGetArgs := map[string]any{
		"accountId":  rt.AccountID,
		"ids":        threadOrder,
		"properties": []string{"id", "emailIds"},
	}
	if log != nil {
		log.WithField("thread_get_id_count", len(threadOrder)).Info("jmap: Thread/get (emailIds for message counts)")
	}
	responses3, err := rt.invokeJMAP(ctx, []any{[]any{"Thread/get", threadGetArgs, "t1"}})
	if err != nil {
		return nil, err
	}
	counts, err := parseThreadGetCounts(responses3[0])
	if err != nil {
		return nil, err
	}
	if log != nil {
		log.WithField("thread_get_resolved_count", len(counts)).Info("jmap: Thread/get parsed thread rows")
	}

	rep := make(map[string]emailPreview, len(emails))
	for _, e := range emails {
		tid := norm.NFC.String(strings.TrimSpace(e.ThreadID))
		if tid == "" {
			continue
		}
		if prev, ok := rep[tid]; !ok || e.ReceivedUnixMs > prev.ReceivedUnixMs {
			rep[tid] = emailPreview{
				Subject:          norm.NFC.String(e.Subject),
				Snippet:          norm.NFC.String(e.Preview),
				ReceivedUnixMs:   e.ReceivedUnixMs,
			}
		}
	}

	out := make([]emailThreadFields, 0, len(threadOrder))
	for _, tid := range threadOrder {
		p := rep[tid]
		mc := counts[tid]
		out = append(out, emailThreadFields{
			ThreadID:           tid,
			MailboxID:          norm.NFC.String(mailboxID),
			AccountID:          rt.AccountID,
			MessageCount:       mc,
			Subject:            p.Subject,
			Snippet:            p.Snippet,
			LastReceivedUnixMs: p.ReceivedUnixMs,
		})
	}
	return out, nil
}

type emailPreview struct {
	Subject          string
	Snippet          string
	ReceivedUnixMs   int64
}

type parsedEmail struct {
	ThreadID         string
	Subject          string
	Preview          string
	ReceivedUnixMs   int64
}

func parseEmailQueryResponse(triple []json.RawMessage) (ids []string, total *int64, err error) {
	var name string
	if err := json.Unmarshal(triple[0], &name); err != nil {
		return nil, nil, err
	}
	if name == "error" {
		return nil, nil, fmt.Errorf("Email/query: %s", string(triple[1]))
	}
	var args struct {
		IDs   []string    `json:"ids"`
		Total json.Number `json:"total"`
	}
	if err := json.Unmarshal(triple[1], &args); err != nil {
		return nil, nil, fmt.Errorf("Email/query args: %w", err)
	}
	if args.Total != "" {
		if v, e := args.Total.Int64(); e == nil {
			total = new(int64)
			*total = v
		}
	}
	return args.IDs, total, nil
}

func parseEmailGetList(triple []json.RawMessage) ([]parsedEmail, error) {
	var name string
	if err := json.Unmarshal(triple[0], &name); err != nil {
		return nil, err
	}
	if name == "error" {
		return nil, fmt.Errorf("Email/get: %s", string(triple[1]))
	}
	var args struct {
		List []map[string]any `json:"list"`
	}
	if err := json.Unmarshal(triple[1], &args); err != nil {
		return nil, fmt.Errorf("Email/get args: %w", err)
	}
	var out []parsedEmail
	for _, row := range args.List {
		tid, _ := row["threadId"].(string)
		subj, _ := row["subject"].(string)
		preview, _ := row["preview"].(string)
		var rx int64
		if rs, ok := row["receivedAt"].(string); ok {
			rx = parseUTCDateString(rs)
		}
		out = append(out, parsedEmail{
			ThreadID:       tid,
			Subject:        subj,
			Preview:        preview,
			ReceivedUnixMs: rx,
		})
	}
	return out, nil
}

func parseThreadGetCounts(triple []json.RawMessage) (map[string]int64, error) {
	var name string
	if err := json.Unmarshal(triple[0], &name); err != nil {
		return nil, err
	}
	if name == "error" {
		return nil, fmt.Errorf("Thread/get: %s", string(triple[1]))
	}
	var args struct {
		List []struct {
			ID       string   `json:"id"`
			EmailIDs []string `json:"emailIds"`
		} `json:"list"`
	}
	if err := json.Unmarshal(triple[1], &args); err != nil {
		return nil, fmt.Errorf("Thread/get args: %w", err)
	}
	out := make(map[string]int64, len(args.List))
	for _, t := range args.List {
		id := norm.NFC.String(strings.TrimSpace(t.ID))
		if id == "" {
			continue
		}
		out[id] = int64(len(t.EmailIDs))
	}
	return out, nil
}
