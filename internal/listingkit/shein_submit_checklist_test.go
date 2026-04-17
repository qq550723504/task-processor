package listingkit

import "testing"

func TestBuildSheinSubmitChecklistGroupsChecks(t *testing.T) {
	t.Parallel()

	checklist := buildSheinSubmitChecklist(&SheinSubmitReadiness{
		Checks: []SheinReadinessCheck{
			{Key: "category", Label: "类目骨架", Status: "blocking"},
			{Key: "request_draft", Label: "请求草稿", Status: "ready"},
			{Key: "manual_notes", Label: "人工备注", Status: "warning"},
		},
		BlockingItems: []SheinReadinessItem{
			{
				Key:             "category",
				SuggestedAction: "确认类目",
				Reason: &SheinReadinessReason{
					Code:     "category_unresolved",
					Category: "classification",
				},
				RepairHints: []SheinRepairHint{{
					Action:        "确认类目",
					Target:        "editor.category",
					Priority:      "high",
					EditorSection: "category",
					RevisionPath:  "shein.category_resolution",
					Patch: &SheinRepairPatchPayload{
						CategoryResolution: &SheinCategoryResolutionPatch{},
					},
					Skeleton: &SheinEditorRevisionSkeleton{
						Platform: "shein",
						Shein: &SheinRevisionInput{
							CategoryResolution: &SheinCategoryResolutionPatch{},
						},
					},
					Revision: &ApplyRevisionRequest{
						Platform: "shein",
						Shein: &SheinRevisionInput{
							CategoryResolution: &SheinCategoryResolutionPatch{},
						},
					},
					Validation: &SheinRepairValidationPreview{
						Valid:                  true,
						Status:                 "ready",
						AffectedSections:       []string{"category", "inspection"},
						CategoryPreviewEffects: []SheinEditorEffect{{Reason: "refresh category preview"}},
					},
				}},
			},
		},
		WarningItems: []SheinReadinessItem{
			{
				Key:             "manual_notes",
				SuggestedAction: "处理备注",
				Reason: &SheinReadinessReason{
					Code:     "manual_review_pending",
					Category: "manual_review",
				},
			},
		},
	})
	if checklist == nil {
		t.Fatal("expected checklist")
	}
	if len(checklist.Required) != 1 || checklist.Required[0].Key != "category" {
		t.Fatalf("required = %+v", checklist.Required)
	}
	if checklist.Required[0].SuggestedAction != "确认类目" {
		t.Fatalf("required action = %+v", checklist.Required[0])
	}
	if checklist.Required[0].Reason == nil || checklist.Required[0].Reason.Code != "category_unresolved" {
		t.Fatalf("required reason = %+v", checklist.Required[0].Reason)
	}
	if len(checklist.Required[0].RepairHints) != 1 || checklist.Required[0].RepairHints[0].Target != "editor.category" {
		t.Fatalf("required repair hints = %+v", checklist.Required[0].RepairHints)
	}
	if checklist.Required[0].RepairHints[0].EditorSection != "category" || checklist.Required[0].RepairHints[0].Patch == nil {
		t.Fatalf("required repair hint metadata = %+v", checklist.Required[0].RepairHints[0])
	}
	if checklist.Required[0].RepairHints[0].Skeleton == nil || checklist.Required[0].RepairHints[0].Revision == nil {
		t.Fatalf("required repair hint revision payload = %+v", checklist.Required[0].RepairHints[0])
	}
	if checklist.Required[0].RepairHints[0].Validation == nil || !checklist.Required[0].RepairHints[0].Validation.Valid {
		t.Fatalf("required repair hint validation = %+v", checklist.Required[0].RepairHints[0])
	}
	if len(checklist.Recommended) != 1 || checklist.Recommended[0].Key != "request_draft" {
		t.Fatalf("recommended = %+v", checklist.Recommended)
	}
	if len(checklist.Optional) != 1 || checklist.Optional[0].Key != "manual_notes" {
		t.Fatalf("optional = %+v", checklist.Optional)
	}
	if checklist.Optional[0].Reason == nil || checklist.Optional[0].Reason.Code != "manual_review_pending" {
		t.Fatalf("optional reason = %+v", checklist.Optional[0].Reason)
	}
}
