package shein

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildSubmissionResponseSummaryTreatsCodeZeroWithoutValidationNotesAsSuccess(t *testing.T) {
	t.Parallel()

	summary := BuildSubmissionResponseSummary(&sheinproduct.SheinResponse{
		Code: "0",
		Msg:  "OK",
		Info: sheinproduct.ResponseInfo{
			Success: false,
			SPUName: "SPU-123",
		},
	})

	if summary == nil {
		t.Fatal("expected summary")
	}
	if !summary.Success {
		t.Fatalf("summary success = %v, want true", summary.Success)
	}
}

func TestBuildSubmissionResponseSummaryKeepsValidationFailuresAsUnsuccessful(t *testing.T) {
	t.Parallel()

	summary := BuildSubmissionResponseSummary(&sheinproduct.SheinResponse{
		Code: "0",
		Msg:  "OK",
		Info: sheinproduct.ResponseInfo{
			Success: false,
			PreValidResult: []sheinproduct.PreValidResult{{
				Messages: []string{"图片只能上传一张"},
			}},
		},
	})

	if summary == nil {
		t.Fatal("expected summary")
	}
	if summary.Success {
		t.Fatalf("summary success = %v, want false", summary.Success)
	}
	if len(summary.ValidationNotes) != 1 || summary.ValidationNotes[0] != "图片只能上传一张" {
		t.Fatalf("validation notes = %+v", summary.ValidationNotes)
	}
}
