package preview

import "testing"

func TestBuildResultProjectionExtractsGenericPreviewFields(t *testing.T) {
	t.Parallel()

	base := &Preview{
		NeedsReview: true,
		Attachment:  &Attachment{},
		Overview: &Header{
			StatusMessage: "ready",
			Warnings:      []string{"warn-1"},
			ReviewReasons: []string{"review-1"},
			PlatformCards: []PlatformCard{{Platform: "shein", Status: "ready"}},
		},
		RevisionHistoryMeta: &RevisionHistoryMeta{
			TotalRecords:    3,
			ReturnedRecords: 2,
			HasMore:         true,
			IsTruncated:     true,
			MaxRecords:      2,
		},
	}

	projection := BuildResultProjection(ResultProjectionInput{Preview: base})

	if !projection.NeedsReview {
		t.Fatal("NeedsReview = false, want true")
	}
	if projection.Attachment != base.Attachment {
		t.Fatalf("Attachment = %+v, want original neutral attachment", projection.Attachment)
	}
	if projection.Overview == nil || projection.Overview.StatusMessage != "ready" {
		t.Fatalf("Overview = %+v, want copied header", projection.Overview)
	}
	if projection.RevisionHistoryMeta == nil || projection.RevisionHistoryMeta.TotalRecords != 3 {
		t.Fatalf("RevisionHistoryMeta = %+v, want copied meta", projection.RevisionHistoryMeta)
	}

	base.Overview.Warnings[0] = "mutated"
	base.Overview.PlatformCards[0].Status = "mutated"
	if projection.Overview.Warnings[0] != "warn-1" {
		t.Fatalf("Warnings were aliased: %+v", projection.Overview.Warnings)
	}
	if projection.Overview.PlatformCards[0].Status != "ready" {
		t.Fatalf("PlatformCards were aliased: %+v", projection.Overview.PlatformCards)
	}
}

func TestBuildResultProjectionHandlesNilPreview(t *testing.T) {
	t.Parallel()

	projection := BuildResultProjection(ResultProjectionInput{})

	if projection.NeedsReview || projection.Attachment != nil || projection.Overview != nil || projection.RevisionHistoryMeta != nil {
		t.Fatalf("projection = %+v, want zero value", projection)
	}
}
