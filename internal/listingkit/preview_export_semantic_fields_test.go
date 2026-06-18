package listingkit

import (
	"encoding/json"
	"strings"
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildSheinPreviewPayloadPopulatesSemanticFields(t *testing.T) {
	pkg := &sheinpub.Package{
		SpuName:      "Semantic Preview Product",
		DraftPayload: &sheinpub.RequestDraft{SpuName: "Semantic Preview Product"},
		PreviewPayload: &sheinproduct.Product{
			SPUName: "Semantic Preview Product",
		},
		SubmissionState: &sheinpub.SubmissionReport{LastStatus: sheinpub.SubmissionStatusSuccess},
	}
	sheinpub.NormalizePackageSemanticFields(pkg)

	payload := buildSheinPreviewPayload(pkg, nil, nil, nil, nil)
	if payload == nil {
		t.Fatal("payload = nil")
	}
	if payload.DraftPayload == nil || payload.DraftPayload.SpuName != "Semantic Preview Product" {
		t.Fatalf("draft payload = %+v", payload.DraftPayload)
	}
	if payload.PreviewPayload == nil || payload.PreviewPayload.SPUName != "Semantic Preview Product" {
		t.Fatalf("preview payload = %+v", payload.PreviewPayload)
	}
	if payload.SubmissionState == nil || payload.SubmissionState.LastStatus != sheinpub.SubmissionStatusSuccess {
		t.Fatalf("submission state = %+v", payload.SubmissionState)
	}
}

func TestBuildListingKitExportPopulatesSemanticFields(t *testing.T) {
	task := &Task{
		ID: "task-semantic-export",
		Result: &ListingKitResult{
			Shein: &sheinpub.Package{
				DraftPayload:   &sheinpub.RequestDraft{SpuName: "Semantic Export Product"},
				PreviewPayload: &sheinproduct.Product{SPUName: "Semantic Export Product"},
			},
		},
	}
	sheinpub.NormalizePackageSemanticFields(task.Result.Shein)

	export, err := buildListingKitExport(task, "shein")
	if err != nil {
		t.Fatalf("buildListingKitExport() error = %v", err)
	}
	if export == nil || export.Shein == nil {
		t.Fatalf("export = %+v", export)
	}
	if export.Shein.DraftPayload == nil || export.Shein.DraftPayload.SpuName != "Semantic Export Product" {
		t.Fatalf("draft payload = %+v", export.Shein.DraftPayload)
	}
	if export.Shein.PreviewPayload == nil || export.Shein.PreviewPayload.SPUName != "Semantic Export Product" {
		t.Fatalf("preview payload = %+v", export.Shein.PreviewPayload)
	}
}

func TestPreviewAndExportJSONIncludeLegacyAndSemanticFieldNames(t *testing.T) {
	pkg := &sheinpub.Package{
		SpuName:         "Semantic JSON Product",
		DraftPayload:    &sheinpub.RequestDraft{SpuName: "Semantic JSON Product"},
		PreviewPayload:  &sheinproduct.Product{SPUName: "Semantic JSON Product"},
		SubmissionState: &sheinpub.SubmissionReport{LastStatus: sheinpub.SubmissionStatusSuccess},
	}
	sheinpub.NormalizePackageSemanticFields(pkg)

	previewPayload := buildSheinPreviewPayload(pkg, nil, nil, nil, nil)
	if previewPayload == nil {
		t.Fatal("preview payload = nil")
	}
	previewJSON, err := json.Marshal(previewPayload)
	if err != nil {
		t.Fatalf("json.Marshal(preview) error = %v", err)
	}
	previewText := string(previewJSON)
	for _, key := range []string{
		`"request_draft"`,
		`"draft_payload"`,
		`"preview_product"`,
		`"preview_payload"`,
		`"submission"`,
		`"submission_state"`,
	} {
		if !strings.Contains(previewText, key) {
			t.Fatalf("preview json missing %s: %s", key, previewText)
		}
	}

	exportPayload := normalizeSheinExportPayloadSemanticFields(&SheinExportPayload{
		RequestDraft:   pkg.RequestDraft,
		DraftPayload:   pkg.DraftPayload,
		PreviewProduct: pkg.PreviewProduct,
		PreviewPayload: pkg.PreviewPayload,
	})
	exportJSON, err := json.Marshal(exportPayload)
	if err != nil {
		t.Fatalf("json.Marshal(export) error = %v", err)
	}
	exportText := string(exportJSON)
	for _, key := range []string{
		`"request_draft"`,
		`"draft_payload"`,
		`"preview_product"`,
		`"preview_payload"`,
	} {
		if !strings.Contains(exportText, key) {
			t.Fatalf("export json missing %s: %s", key, exportText)
		}
	}
}

func TestBuildSheinPreviewPayloadDoesNotFallbackHeadlineOrFinalReviewTitle(t *testing.T) {
	pkg := &sheinpub.Package{
		SpuName:          "Fallback SPU Name",
		Description:      "Preview description",
		TitleDiagnostics: &sheinpub.TitleDiagnostics{Source: "unresolved_prompt_title"},
		DraftPayload:     &sheinpub.RequestDraft{SpuName: "Fallback SPU Name"},
	}
	sheinpub.NormalizePackageSemanticFields(pkg)

	payload := buildSheinPreviewPayload(pkg, nil, nil, nil, nil)
	if payload == nil {
		t.Fatal("payload = nil")
	}
	if payload.Headline != "" {
		t.Fatalf("headline = %q, want empty without generated title fallback", payload.Headline)
	}
	if payload.FinalReview == nil {
		t.Fatal("final review = nil")
	}
	if payload.FinalReview.Title != "" {
		t.Fatalf("final review title = %q, want empty without generated title fallback", payload.FinalReview.Title)
	}
}
