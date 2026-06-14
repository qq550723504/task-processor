package submission

import (
	"testing"
	"time"
)

type testAttemptRecord struct {
	RequestID   string
	SubmittedAt time.Time
	StartedAt   time.Time
	Attempt     int
}

func TestResolveAttemptStartedAtPrefersInFlightStart(t *testing.T) {
	t.Parallel()

	fallback := time.Date(2026, 6, 14, 19, 0, 0, 0, time.UTC)
	inFlight := fallback.Add(-time.Minute)

	got := ResolveAttemptStartedAt(fallback, &inFlight)
	if !got.Equal(inFlight) {
		t.Fatalf("startedAt = %v, want %v", got, inFlight)
	}
}

func TestResolveAttemptRecordForRequestReusesMatchingRecord(t *testing.T) {
	t.Parallel()

	record := &testAttemptRecord{RequestID: "req-1", Attempt: 3}
	got := ResolveAttemptRecordForRequest(
		ActionRecordSlots[testAttemptRecord]{Publish: record},
		"publish",
		"req-1",
		func(record *testAttemptRecord) ActionRecordView {
			return ActionRecordView{RequestID: record.RequestID}
		},
		func(seed AttemptRecordSeed) *testAttemptRecord {
			t.Fatalf("build should not be called for matching record, got seed %+v", seed)
			return nil
		},
		AttemptSeedState{AttemptCount: 9},
		time.Now(),
	)

	if got != record {
		t.Fatalf("record = %+v, want original %+v", got, record)
	}
}

func TestResolveAttemptRecordForRequestBuildsFallbackFromSeedState(t *testing.T) {
	t.Parallel()

	fallback := time.Date(2026, 6, 14, 19, 5, 0, 0, time.UTC)
	inFlight := fallback.Add(-2 * time.Minute)

	got := ResolveAttemptRecordForRequest(
		ActionRecordSlots[testAttemptRecord]{},
		"publish",
		"req-2",
		func(record *testAttemptRecord) ActionRecordView {
			return ActionRecordView{RequestID: record.RequestID}
		},
		func(seed AttemptRecordSeed) *testAttemptRecord {
			return &testAttemptRecord{
				RequestID:   seed.RequestID,
				SubmittedAt: seed.SubmittedAt,
				StartedAt:   seed.StartedAt,
				Attempt:     seed.Attempt,
			}
		},
		AttemptSeedState{
			AttemptCount:      4,
			InFlightStartedAt: &inFlight,
		},
		fallback,
	)

	if got == nil {
		t.Fatal("record = nil, want fallback record")
	}
	if got.RequestID != "req-2" || got.Attempt != 4 {
		t.Fatalf("record = %+v, want req-2 attempt 4", got)
	}
	if !got.SubmittedAt.Equal(inFlight) || !got.StartedAt.Equal(inFlight) {
		t.Fatalf("record timing = %+v, want in-flight started at %v", got, inFlight)
	}
}
