package workspace

import "testing"

func TestBuildRestoreDraftFromSkeletonClonesAndAppliesReason(t *testing.T) {
	spuName := "SPU-1"
	source := &EditorRevisionSkeleton{
		Platform: "shein",
		Actor:    "previous",
		Reason:   "old",
		Shein: &RevisionInput{
			SpuName: &spuName,
		},
	}

	restore := BuildRestoreDraftFromSkeleton("restore version", source)

	if restore == nil || restore.Shein == nil || restore.Shein.SpuName == nil {
		t.Fatalf("restore = %#v", restore)
	}
	if restore.Actor != "desktop-client" || restore.Reason != "restore version" {
		t.Fatalf("restore metadata = %#v", restore)
	}
	*source.Shein.SpuName = "changed"
	if *restore.Shein.SpuName != "SPU-1" {
		t.Fatalf("restore spu = %q, want clone", *restore.Shein.SpuName)
	}
}
