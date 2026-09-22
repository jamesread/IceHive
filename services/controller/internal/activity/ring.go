// Package activity provides a process-local ring buffer of recent Controller control-plane events.
package activity

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	KindHeartbeatReceived  = "heartbeat_received"
	KindCollectionEnqueued = "collection_enqueued"
	KindCollectionRun      = "collection_run"

	DefaultCapacity  = 200
	DefaultListLimit = 50
	MaxListLimit     = 100
)

// Event is one activity feed line.
type Event struct {
	Success       *bool
	ID            string
	Kind          string
	Summary       string
	ServiceName   string
	CollectorType string
	SourceID      string
	UnixMs        int64
}

// Ring is a thread-safe circular buffer of Events (newest last internally; List returns newest first).
type Ring struct {
	lastHB   map[string]int64
	buf      []Event
	hbMinGap time.Duration
	mu       sync.Mutex
	seq      atomic.Uint64
	cap      int
	next     int
	full     bool
}

// NewRing returns a ring with the given capacity (clamped to at least 1).
func NewRing(capacity int) *Ring {
	if capacity < 1 {
		capacity = DefaultCapacity
	}
	return &Ring{
		buf:      make([]Event, capacity),
		cap:      capacity,
		lastHB:   make(map[string]int64),
		hbMinGap: 10 * time.Second,
	}
}

// Record appends an event (fills id and unix_ms when unset).
func (r *Ring) Record(e Event) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if e.UnixMs <= 0 {
		e.UnixMs = time.Now().UnixMilli()
	}
	if e.ID == "" {
		e.ID = fmt.Sprintf("%d", r.seq.Add(1))
	}
	r.buf[r.next] = e
	r.next = (r.next + 1) % r.cap
	if r.next == 0 {
		r.full = true
	}
}

func (r *Ring) acceptHeartbeat(serviceName string, unixMs int64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	last, ok := r.lastHB[serviceName]
	if ok && unixMs-last < r.hbMinGap.Milliseconds() {
		return false
	}
	r.lastHB[serviceName] = unixMs
	return true
}

func heartbeatSummary(serviceName, version string) string {
	if v := strings.TrimSpace(version); v != "" {
		return fmt.Sprintf("Heartbeat from %s (%s)", serviceName, v)
	}
	return fmt.Sprintf("Heartbeat from %s", serviceName)
}

// RecordHeartbeat records a heartbeat_received event, throttled per service.
func (r *Ring) RecordHeartbeat(serviceName, version string, unixMs int64) {
	if r == nil {
		return
	}
	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return
	}
	if unixMs <= 0 {
		unixMs = time.Now().UnixMilli()
	}
	if !r.acceptHeartbeat(serviceName, unixMs) {
		return
	}
	r.Record(Event{
		UnixMs:      unixMs,
		Kind:        KindHeartbeatReceived,
		Summary:     heartbeatSummary(serviceName, version),
		ServiceName: serviceName,
	})
}

func enqueueSummary(collectorType, label string) string {
	if collectorType != "" && label != collectorType {
		return fmt.Sprintf("Published collection request → %s (%s)", collectorType, label)
	}
	return fmt.Sprintf("Published collection request for %s", label)
}

func enqueueLabel(collectorType, sourceID, sourceSpec string) string {
	if sourceSpec != "" {
		return sourceSpec
	}
	if sourceID != "" {
		return sourceID
	}
	return collectorType
}

// RecordCollectionEnqueued records a collection_enqueued event after AMQP publish.
func (r *Ring) RecordCollectionEnqueued(collectorType, sourceID, sourceSpec string) {
	if r == nil {
		return
	}
	collectorType = strings.TrimSpace(collectorType)
	sourceID = strings.TrimSpace(sourceID)
	sourceSpec = strings.TrimSpace(sourceSpec)
	label := enqueueLabel(collectorType, sourceID, sourceSpec)
	r.Record(Event{
		Kind:          KindCollectionEnqueued,
		Summary:       enqueueSummary(collectorType, label),
		CollectorType: collectorType,
		SourceID:      sourceID,
	})
}

func truncateErr(errMsg string) string {
	e := strings.TrimSpace(errMsg)
	if len(e) > 80 {
		return e[:80] + "…"
	}
	return e
}

func runSummary(sourceID, collectorType, errMsg string, success bool) string {
	summary := fmt.Sprintf("Collection run ok for %s", sourceID)
	if !success {
		summary = fmt.Sprintf("Collection run failed for %s", sourceID)
		if e := truncateErr(errMsg); e != "" {
			summary = summary + ": " + e
		}
	}
	if collectorType != "" {
		return fmt.Sprintf("%s [%s]", summary, collectorType)
	}
	return summary
}

// RecordCollectionRun records a collection_run event after a collector reports a run.
func (r *Ring) RecordCollectionRun(sourceID, collectorType string, success bool, errMsg string) {
	if r == nil {
		return
	}
	sourceID = strings.TrimSpace(sourceID)
	collectorType = strings.TrimSpace(collectorType)
	ok := success
	r.Record(Event{
		Kind:          KindCollectionRun,
		Summary:       runSummary(sourceID, collectorType, errMsg, success),
		CollectorType: collectorType,
		SourceID:      sourceID,
		Success:       &ok,
	})
}

func clampListLimit(limit int) int {
	if limit <= 0 {
		return DefaultListLimit
	}
	if limit > MaxListLimit {
		return MaxListLimit
	}
	return limit
}

func (r *Ring) filledCount() int {
	if r.full {
		return r.cap
	}
	return r.next
}

func (r *Ring) newestSlice(limit int) []Event {
	n := r.filledCount()
	if n == 0 {
		return nil
	}
	if limit > n {
		limit = n
	}
	out := make([]Event, 0, limit)
	for i := 0; i < limit; i++ {
		idx := (r.next - 1 - i + r.cap) % r.cap
		out = append(out, r.buf[idx])
	}
	return out
}

// List returns up to limit events, newest first.
func (r *Ring) List(limit int) []Event {
	if r == nil {
		return nil
	}
	limit = clampListLimit(limit)
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.newestSlice(limit)
}
