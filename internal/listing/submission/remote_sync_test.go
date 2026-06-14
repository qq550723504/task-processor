package submission

import (
	"testing"
	"time"
)

type testRemoteRecord struct {
	RequestID string
	Message   string
}

func TestApplyRemoteSyncAlwaysAppliesReportState(t *testing.T) {
	t.Parallel()

	var gotStatus string
	var gotCheckedAt time.Time
	state := ActionRemoteSyncState{
		RemoteStatus: "confirmed",
		CheckedAt:    time.Date(2026, 6, 14, 18, 0, 0, 0, time.UTC),
	}

	mutated := ApplyRemoteSync(
		ActionRecordSlots[testRemoteRecord]{},
		"publish",
		"req-1",
		func(record *testRemoteRecord) ActionRecordView {
			return ActionRecordView{RequestID: record.RequestID}
		},
		state,
		func(state ActionRemoteSyncState) {
			gotStatus = state.RemoteStatus
			gotCheckedAt = state.CheckedAt
		},
		func(record *testRemoteRecord, state ActionRemoteSyncState) {
			record.Message = state.RemoteStatus
		},
	)

	if mutated {
		t.Fatal("mutated = true, want false without matching record")
	}
	if gotStatus != "confirmed" || !gotCheckedAt.Equal(state.CheckedAt) {
		t.Fatalf("report state = %q/%v, want confirmed/%v", gotStatus, gotCheckedAt, state.CheckedAt)
	}
}

func TestApplyRemoteSyncMutatesOnlyMatchingRecord(t *testing.T) {
	t.Parallel()

	record := &testRemoteRecord{RequestID: "req-1"}
	state := ActionRemoteSyncState{
		RemoteStatus: "pending",
		CheckedAt:    time.Date(2026, 6, 14, 18, 5, 0, 0, time.UTC),
	}

	mutated := ApplyRemoteSync(
		ActionRecordSlots[testRemoteRecord]{Publish: record},
		"publish",
		"req-1",
		func(record *testRemoteRecord) ActionRecordView {
			return ActionRecordView{RequestID: record.RequestID}
		},
		state,
		nil,
		func(record *testRemoteRecord, state ActionRemoteSyncState) {
			record.Message = state.RemoteStatus
		},
	)

	if !mutated {
		t.Fatal("mutated = false, want true")
	}
	if record.Message != "pending" {
		t.Fatalf("record message = %q, want pending", record.Message)
	}
}
