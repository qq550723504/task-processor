package listingkit

import (
	"testing"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildTaskListItemIncludesSheinRemoteSubmissionSummary(t *testing.T) {
	t.Parallel()

	checkedAt := time.Date(2026, 5, 7, 12, 30, 0, 0, time.UTC)
	task := &Task{
		ID:     "task-remote-summary",
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			Shein: &SheinPackage{
				Submission: &sheinpub.SubmissionReport{
					LastAction:      "publish",
					RemoteStatus:    sheinpub.SubmissionRemoteStatusConfirmed,
					RemoteCheckedAt: &checkedAt,
					Publish: &sheinpub.SubmissionRecord{
						Action:         "publish",
						RemoteRecordID: "record-123",
					},
				},
			},
		},
	}

	item := buildTaskListItem(task)

	if item.SheinSubmissionRemoteStatus != sheinpub.SubmissionRemoteStatusConfirmed {
		t.Fatalf("remote status = %q, want confirmed", item.SheinSubmissionRemoteStatus)
	}
	if item.SheinSubmissionRemoteCheckedAt == nil || !item.SheinSubmissionRemoteCheckedAt.Equal(checkedAt) {
		t.Fatalf("remote checked at = %v, want %v", item.SheinSubmissionRemoteCheckedAt, checkedAt)
	}
	if item.SheinSubmissionRemoteRecordID != "record-123" {
		t.Fatalf("remote record id = %q, want record-123", item.SheinSubmissionRemoteRecordID)
	}
}

func TestBuildTaskListItemPrefersRenderedImageCount(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID: "task-rendered-count",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://cdn.example.com/input.png"},
		},
		Status: TaskStatusCompleted,
		Result: &ListingKitResult{
			SDSSync: &SDSSyncSummary{
				Status: "completed",
				MockupImageURLs: []string{
					"https://cdn.example.com/mockup-1.png",
					"https://cdn.example.com/mockup-2.png",
					"https://cdn.example.com/mockup-3.png",
				},
				VariantResults: []SDSSyncSummary{{
					Status:          "completed",
					MockupImageURLs: []string{"https://cdn.example.com/mockup-4.png"},
				}},
			},
		},
	}

	item := buildTaskListItem(task)

	if item.ImageCount != 4 {
		t.Fatalf("image count = %d, want rendered image count 4", item.ImageCount)
	}
}
