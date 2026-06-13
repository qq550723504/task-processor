package preview

import (
	"slices"
	"testing"
	"time"
)

func TestBuildProjection(t *testing.T) {
	t.Parallel()

	createdAt := time.Now()
	completedAt := createdAt.Add(time.Minute)
	projection := BuildProjection(ProjectionInput{
		Shell: ShellInput{
			TaskID:           "task-1",
			Status:           "completed",
			SelectedPlatform: "amazon",
			Platforms:        []string{"amazon", "shein"},
			CreatedAt:        createdAt,
			CompletedAt:      &completedAt,
		},
		NeedsReview: true,
		Overview: &HeaderInput{
			Country:       "US",
			StatusMessage: "ready",
			ReviewReasons: []string{"needs-manual-check"},
		},
	})
	if projection == nil {
		t.Fatal("projection = nil")
	}
	if projection.TaskID != "task-1" || !projection.NeedsReview {
		t.Fatalf("projection = %+v", projection)
	}
	if !slices.Equal(projection.Platforms, []string{"amazon", "shein"}) {
		t.Fatalf("platforms = %#v", projection.Platforms)
	}
	if projection.Overview == nil || !slices.Equal(projection.Overview.ReviewReasons, []string{"needs-manual-check"}) {
		t.Fatalf("overview = %+v", projection.Overview)
	}
}
