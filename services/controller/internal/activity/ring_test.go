package activity

import (
	"testing"
	"time"
)

func requireEventSummaries(t *testing.T, got []Event, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d", len(got), len(want))
	}
	for i, s := range want {
		if got[i].Summary != s {
			t.Fatalf("got[%d]=%q want %q", i, got[i].Summary, s)
		}
	}
}

func TestRingListNewestFirst(t *testing.T) {
	r := NewRing(5)
	r.Record(Event{Kind: KindHeartbeatReceived, Summary: "a", UnixMs: 1})
	r.Record(Event{Kind: KindHeartbeatReceived, Summary: "b", UnixMs: 2})
	r.Record(Event{Kind: KindHeartbeatReceived, Summary: "c", UnixMs: 3})
	requireEventSummaries(t, r.List(10), "c", "b", "a")
}

func TestRingWrapsAndCapsList(t *testing.T) {
	r := NewRing(3)
	for i := 1; i <= 5; i++ {
		r.Record(Event{Kind: KindCollectionRun, Summary: string(rune('a' + i - 1)), UnixMs: int64(i)})
	}
	requireEventSummaries(t, r.List(2), "e", "d")
}

func TestRecordHeartbeatThrottled(t *testing.T) {
	r := NewRing(10)
	r.hbMinGap = 10 * time.Second
	now := time.Now().UnixMilli()
	r.RecordHeartbeat("collector-github", "1.0", now)
	r.RecordHeartbeat("collector-github", "1.0", now+1000)
	r.RecordHeartbeat("collector-github", "1.0", now+11_000)
	got := r.List(10)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2 (throttled)", len(got))
	}
}
