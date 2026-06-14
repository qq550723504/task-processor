package listingkit

import (
	"context"
	"strings"
	"testing"

	"task-processor/internal/productenrich"
)

func TestRunWorkflowOptimizesSheinContentBeforeFinalReview(t *testing.T) {
	t.Parallel()

	productTask := &productenrich.Task{
		ID: "product-task-shein-copy",
		Request: &productenrich.GenerateRequest{
			ImageURLs: []string{"https://example.com/pillow.jpg"},
			Text:      "pillow cover",
		},
	}
	productSvc := &stubWorkflowProductService{
		task: productTask,
		product: &productenrich.ProductJSON{
			Title:         "Envelope style pillow cover",
			Description:   "Simple pillow cover for home decor.",
			Category:      []string{"Home", "Textiles", "Pillow Covers"},
			Images:        []string{"https://example.com/pillow.jpg"},
			SellingPoints: []string{"Soft polyester", "Botanical print"},
			Attributes:    map[string]string{"brand": "DemoBrand"},
		},
	}
	ai := &stubSheinContentAI{
		response: `{"title":"Botanical Envelope Pillow Cover for Sofa Couch Bedroom Decor, Soft Polyester Accent Cushion Case","description":"A soft polyester envelope pillow cover designed to refresh sofas, beds, and reading corners with a botanical accent print. The overlap closure keeps the insert tucked in while making everyday styling changes easy."}`,
	}

	svc := seedWorkflowServices(seedWorkflowDepsFromMirrors(&service{
		mirrors: serviceDependencyMirrors{
			assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
			assetRecipeResolver: newDefaultAssetRecipeResolver(),
			assetBundleBuilder:  newDefaultAssetBundleBuilder(),
			assetGenerator:      newDefaultAssetGenerationService(),
		},
		workflowDeps: workflowDependencies{
			sheinContentOptimizer: ai,
		},
	}), productSvc, nil)

	task := &Task{
		ID: "listingkit-task-shein-copy",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/pillow.jpg"},
			Text:      "pillow cover",
			Platforms: []string{"shein"},
			Country:   "US",
			Language:  "en_US",
			Options:   &GenerateOptions{ProcessImages: false},
		},
	}

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}
	if ai.calls != 1 {
		t.Fatal("shein content optimizer was not called")
	}
	if result.Shein == nil {
		t.Fatal("shein package = nil")
	}
	if got := result.Shein.ProductNameEn; !strings.Contains(got, "Botanical Envelope Pillow Cover") {
		t.Fatalf("shein title = %q", got)
	}
	if got := result.Shein.Description; !strings.Contains(got, "reading corners") {
		t.Fatalf("shein description = %q", got)
	}

	preview := buildSheinPreviewPayload(result.Shein, result.PodExecution, result.CanonicalProduct, nil, nil)
	if preview == nil || preview.FinalReview == nil {
		t.Fatalf("preview final review = %+v", preview)
	}
	if got := preview.FinalReview.Title; !strings.Contains(got, "Botanical Envelope Pillow Cover") {
		t.Fatalf("final review title = %q", got)
	}
	if got := preview.FinalReview.Description; !strings.Contains(got, "reading corners") {
		t.Fatalf("final review description = %q", got)
	}
}
