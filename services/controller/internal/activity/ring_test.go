package activity

import (
	"testing"
	"time"
)

func TestRingListNewestFirst(t *testing.T) {
	r := NewRing(5)
	r.Record(Event{Kind: KindHeartbeatReceived, Summary: "a", UnixMs: 1})
	r.Record(Event{Kind: KindHeartbeatReceived, Summary: "b", UnixMs: 2})
	r.Record(Event{Kind: KindHeartbeatReceived, Summary: "c", UnixMs: 3})

	got := r.List(10)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if got[0].Summary != "c" || got[1].Summary != "b" || got[2].Summary != "a" {
		t.Fatalf("order=%v", []string{got[0].Summary, got[1].Summary, got[2].Summary})
	}
}

func TestRingWrapsAndCapsList(t *testing.T) {
	r := NewRing(3)
	for i := 1; i <= 5; i++ {
		r.Record(Event{Kind: KindCollectionRun, Summary: string(rune('a' + i - 1)), UnixMs: int64(i)})
	}
	got := r.List(2)
	if len(got) != 2 {
		t.Fatalf("len=%d want 2", len(got))
	}
	if got[0].Summary != "e" || got[1].Summary != "d" {
		t.Fatalf("got %q %q", got[0].Summary, got[1].Summary)
	}
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
