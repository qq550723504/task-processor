package shein

import "testing"

func TestRepairCenterAccessors(t *testing.T) {
	t.Parallel()

	center := &RepairCenter[string, string, string, string, string]{
		Stats: &RepairCenterStats{
			TotalActions:       3,
			DirectApplyActions: 1,
		},
		PrimaryPlan: &RepairPlan{Status: "mixed"},
		Session:     &RepairSession{Status: "guided_mixed"},
	}

	if got := RepairCenterActionCount(center); got != 3 {
		t.Fatalf("RepairCenterActionCount() = %d, want 3", got)
	}
	if got := RepairCenterDirectApplyCount(center); got != 1 {
		t.Fatalf("RepairCenterDirectApplyCount() = %d, want 1", got)
	}
	if got := RepairCenterPrimaryPlanStatus(center); got != "mixed" {
		t.Fatalf("RepairCenterPrimaryPlanStatus() = %q, want mixed", got)
	}
	if got := RepairCenterSessionStatus(center); got != "guided_mixed" {
		t.Fatalf("RepairCenterSessionStatus() = %q, want guided_mixed", got)
	}
}
