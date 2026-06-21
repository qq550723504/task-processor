package workspace

import "testing"

func TestBuildRepairRevisionSeedBuildsMinimalSkeleton(t *testing.T) {
	t.Parallel()

	status := "resolved"
	categoryID := 123
	seed := BuildRepairRevisionSeed("resolve_category", &RepairPatchPayload{
		CategoryResolution: &CategoryResolutionPatch{
			Status:     &status,
			CategoryID: &categoryID,
		},
		ReviewNotes: []string{"确认类目"},
	})

	if seed.Input == nil {
		t.Fatal("Input = nil, want full input")
	}
	if seed.Skeleton == nil || seed.Skeleton.Shein == nil {
		t.Fatalf("Skeleton = %+v, want minimal skeleton", seed.Skeleton)
	}
	if seed.Skeleton.Platform != "shein" || seed.Skeleton.Actor != "desktop-client" || seed.Skeleton.Reason != "repair: resolve_category" {
		t.Fatalf("Skeleton metadata = %+v, want SHEIN desktop repair metadata", seed.Skeleton)
	}
	if seed.Skeleton.Shein.CategoryResolution == nil || seed.Skeleton.Shein.CategoryResolution.CategoryID == nil || *seed.Skeleton.Shein.CategoryResolution.CategoryID != categoryID {
		t.Fatalf("Skeleton SHEIN category patch = %+v, want category id", seed.Skeleton.Shein.CategoryResolution)
	}
}

func TestBuildRepairRevisionSeedSkipsEmptyPayload(t *testing.T) {
	t.Parallel()

	seed := BuildRepairRevisionSeed("", &RepairPatchPayload{})

	if seed.Input != nil || seed.Skeleton != nil {
		t.Fatalf("seed = %+v, want empty seed", seed)
	}
}

func TestBuildRepairReason(t *testing.T) {
	t.Parallel()

	if got := BuildRepairReason(""); got != "repair suggested issue" {
		t.Fatalf("BuildRepairReason(empty) = %q", got)
	}
	if got := BuildRepairReason("resolve_attributes"); got != "repair: resolve_attributes" {
		t.Fatalf("BuildRepairReason(action) = %q", got)
	}
}
