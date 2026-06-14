package submission

import (
	"testing"
	"time"
)

type testActionRecord struct {
	ID string
}

type testActionResult struct {
	OK bool
}

func TestRecordForActionReturnsExpectedSlot(t *testing.T) {
	t.Parallel()

	saveDraft := &testActionRecord{ID: "save"}
	publish := &testActionRecord{ID: "publish"}
	slots := ActionRecordSlots[testActionRecord]{
		SaveDraft: saveDraft,
		Publish:   publish,
	}

	if got := RecordForAction(slots, "save_draft"); got != saveDraft {
		t.Fatalf("save_draft record = %+v, want %+v", got, saveDraft)
	}
	if got := RecordForAction(slots, "publish"); got != publish {
		t.Fatalf("publish record = %+v, want %+v", got, publish)
	}
	if got := RecordForAction(slots, "unknown"); got != nil {
		t.Fatalf("unknown record = %+v, want nil", got)
	}
}

func TestApplyRecordStateSyncsLastFieldsAndSelectedSlot(t *testing.T) {
	t.Parallel()

	saveDraft := &testActionRecord{ID: "save"}
	publish := &testActionRecord{ID: "publish"}
	result := &testActionResult{OK: true}
	submittedAt := time.Date(2026, 6, 14, 10, 0, 0, 0, time.UTC)
	report := &ReportState[testActionRecord, testActionResult]{
		Slots: ActionRecordSlots[testActionRecord]{
			SaveDraft: saveDraft,
		},
	}

	ApplyRecordState(report, publish, ReportRecordState[testActionResult]{
		Action:      "publish",
		Status:      "success",
		Error:       "",
		SubmittedAt: submittedAt,
		Result:      result,
	})

	if report.LastAction != "publish" || report.LastStatus != "success" || report.LastError != "" {
		t.Fatalf("report last state = %+v", report)
	}
	if report.SubmittedAt == nil || !report.SubmittedAt.Equal(submittedAt) {
		t.Fatalf("submitted at = %+v, want %v", report.SubmittedAt, submittedAt)
	}
	if report.LastResult != result {
		t.Fatalf("last result = %+v, want %+v", report.LastResult, result)
	}
	if report.Slots.SaveDraft != saveDraft {
		t.Fatalf("save_draft slot = %+v, want %+v", report.Slots.SaveDraft, saveDraft)
	}
	if report.Slots.Publish != publish {
		t.Fatalf("publish slot = %+v, want %+v", report.Slots.Publish, publish)
	}
}

func TestActionSucceededChecksSelectedSlotStatus(t *testing.T) {
	t.Parallel()

	saveDraft := &testActionRecord{ID: "save"}
	publish := &testActionRecord{ID: "publish"}
	statusByID := map[string]string{
		"save":    "failed",
		"publish": "success",
	}
	slots := ActionRecordSlots[testActionRecord]{
		SaveDraft: saveDraft,
		Publish:   publish,
	}

	view := func(record *testActionRecord) ActionRecordView {
		return ActionRecordView{
			RequestID: record.ID,
			Status:    statusByID[record.ID],
		}
	}

	if !ActionSucceeded(slots, "publish", view, "success") {
		t.Fatal("publish should be treated as successful")
	}
	if ActionSucceeded(slots, "save_draft", view, "success") {
		t.Fatal("save_draft should not be treated as successful")
	}
}

func TestFindCompletedRecordByRequestIDRequiresMatchingFinishedRecord(t *testing.T) {
	t.Parallel()

	finishedAt := time.Date(2026, 6, 14, 11, 0, 0, 0, time.UTC)
	saveDraft := &testActionRecord{ID: "save"}
	publish := &testActionRecord{ID: "publish"}
	finishedByID := map[string]*time.Time{
		"save":    nil,
		"publish": &finishedAt,
	}
	slots := ActionRecordSlots[testActionRecord]{
		SaveDraft: saveDraft,
		Publish:   publish,
	}

	view := func(record *testActionRecord) ActionRecordView {
		return ActionRecordView{
			RequestID:  record.ID,
			FinishedAt: finishedByID[record.ID],
		}
	}

	if got := FindCompletedRecordByRequestID(slots, "publish", "publish", view); got != publish {
		t.Fatalf("publish completed record = %+v, want %+v", got, publish)
	}
	if got := FindCompletedRecordByRequestID(slots, "save_draft", "save", view); got != nil {
		t.Fatalf("unfinished save_draft record = %+v, want nil", got)
	}
	if got := FindCompletedRecordByRequestID(slots, "publish", "other", view); got != nil {
		t.Fatalf("mismatched request record = %+v, want nil", got)
	}
}

func TestMutateMatchingRecordRequiresMatchingRequestID(t *testing.T) {
	t.Parallel()

	publish := &testActionRecord{ID: "publish"}
	slots := ActionRecordSlots[testActionRecord]{
		Publish: publish,
	}
	view := func(record *testActionRecord) ActionRecordView {
		return ActionRecordView{RequestID: record.ID}
	}

	mutated := MutateMatchingRecord(slots, "publish", "publish", view, func(record *testActionRecord) {
		record.ID = "updated"
	})
	if !mutated {
		t.Fatal("expected matching record mutation to run")
	}
	if publish.ID != "updated" {
		t.Fatalf("publish record = %+v, want updated id", publish)
	}

	mutated = MutateMatchingRecord(slots, "publish", "other", func(record *testActionRecord) ActionRecordView {
		return ActionRecordView{RequestID: record.ID}
	}, func(record *testActionRecord) {
		record.ID = "should-not-change"
	})
	if mutated {
		t.Fatal("expected mismatched request mutation to be skipped")
	}
	if publish.ID != "updated" {
		t.Fatalf("publish record = %+v, want unchanged updated id", publish)
	}
}
