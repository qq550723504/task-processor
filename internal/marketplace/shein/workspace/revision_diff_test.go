package workspace

import "testing"

func TestBuildRevisionDiffBetweenRevisionsDetectsChangedTitle(t *testing.T) {
	beforeTitle := "before"
	afterTitle := "after"

	diff := BuildRevisionDiffBetweenRevisions(
		&EditorRevisionSkeleton{Shein: &RevisionInput{SpuName: &beforeTitle}},
		&EditorRevisionSkeleton{Shein: &RevisionInput{SpuName: &afterTitle}},
	)

	if diff == nil || diff.ChangeCount != 1 || len(diff.Changes) != 1 {
		t.Fatalf("diff = %#v", diff)
	}
	if diff.Changes[0].FieldPath != "shein.spu_name" || diff.Changes[0].Before != beforeTitle || diff.Changes[0].After != afterTitle {
		t.Fatalf("change = %#v", diff.Changes[0])
	}
}
