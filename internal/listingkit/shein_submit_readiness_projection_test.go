package listingkit

import "testing"

func TestBuildSheinSubmitReadinessProjectionWithPodBuildsSharedProjection(t *testing.T) {
	t.Parallel()

	pkg := &SheinPackage{
		Inspection: &SheinInspection{
			NeedsReview: true,
			Summary:     []string{"需要继续确认"},
		},
		ReviewNotes: []string{"人工备注待处理"},
	}

	projection := buildSheinSubmitReadinessProjectionWithPod(pkg, nil)
	if projection == nil {
		t.Fatal("projection = nil")
	}
	if projection.Readiness == nil {
		t.Fatal("readiness = nil")
	}
	if projection.Checklist == nil {
		t.Fatal("checklist = nil")
	}
	if projection.SubmitState == nil {
		t.Fatal("submit state = nil")
	}
	if projection.StatusOverview == nil {
		t.Fatal("status overview = nil")
	}
}

func TestBuildSheinSubmitReadinessProjectionWithPodNilPackage(t *testing.T) {
	t.Parallel()

	if projection := buildSheinSubmitReadinessProjectionWithPod(nil, nil); projection != nil {
		t.Fatalf("projection = %+v, want nil", projection)
	}
}
