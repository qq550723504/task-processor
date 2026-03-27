package productimage_test

import (
	"context"
	"testing"

	productenrich "task-processor/internal/productenrich"
	productimage "task-processor/internal/productimage"
)

type mockInputParser struct {
	result *productenrich.ParsedInput
	err    error
}

func (m *mockInputParser) ParseInput(_ context.Context, _ *productenrich.GenerateRequest) (*productenrich.ParsedInput, error) {
	return m.result, m.err
}
func (m *mockInputParser) CollectImages(_ context.Context, urls []string) ([]string, error) {
	return urls, nil
}
func (m *mockInputParser) CleanText(text string) string { return text }
func (m *mockInputParser) Scrape1688(_ context.Context, _ string) (*productenrich.ScrapedData, error) {
	return nil, nil
}

type mockProductUnderstanding struct {
	result *productenrich.ProductAnalysis
	err    error
}

func (m *mockProductUnderstanding) AnalyzeProduct(_ context.Context, _ *productenrich.ParsedInput) (*productenrich.ProductAnalysis, error) {
	return m.result, m.err
}
func (m *mockProductUnderstanding) AnalyzeImage(_ context.Context, _ string) (*productenrich.ImageAttributes, error) {
	return nil, nil
}
func (m *mockProductUnderstanding) ExtractTextAttributes(_ context.Context, _ string) (*productenrich.TextAttributes, error) {
	return nil, nil
}
func (m *mockProductUnderstanding) FuseMultimodal(_ context.Context, _ *productenrich.ImageAttributes, _ *productenrich.TextAttributes) (*productenrich.ProductRepresentation, error) {
	return nil, nil
}

func TestSourceParserAdapter_Parse(t *testing.T) {
	adapter, err := productimage.NewSourceParser(&mockInputParser{result: &productenrich.ParsedInput{
		Images: []string{"a", "b"},
		Text:   "desc",
		ScrapedData: &productenrich.ScrapedData{
			Title: "Breathable Running Shoes",
		},
	}})
	if err != nil {
		t.Fatalf("NewSourceParser() error = %v", err)
	}
	source, err := adapter.Parse(context.Background(), &productimage.ImageProcessRequest{ImageURLs: []string{"a", "b"}, Marketplace: "amazon"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if len(source.Images) != 2 {
		t.Fatalf("images = %d, want 2", len(source.Images))
	}
	if source.ParsedInput == nil {
		t.Fatal("expected parsed input to be preserved")
	}
	if source.TitleHint != "Breathable Running Shoes" {
		t.Fatalf("title hint = %q, want scraped title", source.TitleHint)
	}
}

func TestProductContextAnalyzer_Analyze(t *testing.T) {
	analysis := &productenrich.ProductAnalysis{Representation: &productenrich.ProductRepresentation{ProductType: "chair"}}
	adapter, err := productimage.NewProductContextAnalyzer(&mockProductUnderstanding{result: analysis})
	if err != nil {
		t.Fatalf("NewProductContextAnalyzer() error = %v", err)
	}
	result, err := adapter.Analyze(context.Background(), &productimage.SourceBundle{ParsedInput: &productenrich.ParsedInput{Images: []string{"a"}}})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if result.Representation.ProductType != "chair" {
		t.Fatalf("product type = %q, want chair", result.Representation.ProductType)
	}
}

func TestDefaultImageRanker_Select(t *testing.T) {
	ranker := productimage.NewDefaultImageRanker()
	result, err := ranker.Select(context.Background(), &productimage.SourceBundle{Images: []string{"promo_badge_text.jpg", "hero_white.jpg", "detail.jpg"}}, []productimage.ImageAudit{
		{ImageURL: "promo_badge_text.jpg", HasPromoBadge: true, HasOverlayText: true, QualityScore: 0.9},
		{ImageURL: "hero_white.jpg", IsWhiteBackground: true, QualityScore: 0.8},
		{ImageURL: "detail.jpg", QualityScore: 0.7},
	}, nil)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if result.PrimarySource != "hero_white.jpg" {
		t.Fatalf("primary = %q, want hero_white.jpg", result.PrimarySource)
	}
	if len(result.RejectedImages) != 1 || result.RejectedImages[0] != "promo_badge_text.jpg" {
		t.Fatalf("unexpected rejected images: %+v", result.RejectedImages)
	}
}

func TestDefaultImageInspector_UsesTitleHintAsPrimaryObject(t *testing.T) {
	inspector := productimage.NewDefaultImageInspector()
	audit, err := inspector.Inspect(context.Background(), &productimage.SourceBundle{
		TitleHint: "Running Shoes",
	}, "hero_white.jpg")
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if audit.PrimaryObject != "Running Shoes" {
		t.Fatalf("primary object = %q, want Running Shoes", audit.PrimaryObject)
	}
}

func TestDefaultCleanerSubjectExtractorWhiteBackgroundAndValidator(t *testing.T) {
	extractor := productimage.NewDefaultSubjectExtractor()
	asset, err := extractor.Extract(context.Background(), "promo_badge_text_logo_hero.jpg", &productenrich.ProductAnalysis{Representation: &productenrich.ProductRepresentation{ProductType: "lamp"}})
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if asset.Type != productimage.AssetTypeSubjectCutout {
		t.Fatalf("asset type = %q, want subject cutout", asset.Type)
	}
	if asset.Metadata["product_type"] != "lamp" {
		t.Fatalf("product_type metadata = %q, want lamp", asset.Metadata["product_type"])
	}

	cleaner := productimage.NewDefaultImageCleaner()
	mainImage, err := cleaner.Clean(context.Background(), asset, nil)
	if err != nil {
		t.Fatalf("Clean() error = %v", err)
	}
	if mainImage.Metadata["promo_badge_removed"] != "true" {
		t.Fatalf("promo badge metadata = %q, want true", mainImage.Metadata["promo_badge_removed"])
	}
	if mainImage.Metadata["overlay_text_removed"] != "true" {
		t.Fatalf("overlay text metadata = %q, want true", mainImage.Metadata["overlay_text_removed"])
	}
	if mainImage.Metadata["logo_overlay_removed"] != "true" {
		t.Fatalf("logo metadata = %q, want true", mainImage.Metadata["logo_overlay_removed"])
	}

	renderer := productimage.NewDefaultWhiteBackgroundRenderer()
	whiteBg, err := renderer.Render(context.Background(), mainImage, nil)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if whiteBg.Type != productimage.AssetTypeWhiteBgImage {
		t.Fatalf("white bg type = %q, want white_bg_image", whiteBg.Type)
	}
	if whiteBg.Metadata["background"] != "white" {
		t.Fatalf("background metadata = %q, want white", whiteBg.Metadata["background"])
	}

	validator := productimage.NewDefaultMarketplaceValidator()
	report, err := validator.Validate(context.Background(), &productimage.ImageProcessRequest{Marketplace: "amazon"}, &productimage.ImageProcessResult{MainImage: mainImage, WhiteBgImage: whiteBg})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if !report.Passed {
		t.Fatalf("expected compliance to pass, issues: %+v", report.Issues)
	}
}

func TestDefaultMarketplaceValidator_FailsForUnsafeMainImage(t *testing.T) {
	validator := productimage.NewDefaultMarketplaceValidator()
	report, err := validator.Validate(context.Background(), &productimage.ImageProcessRequest{Marketplace: "amazon"}, &productimage.ImageProcessResult{
		MainImage:    &productimage.ImageAsset{SourceURL: "promo_badge_text.jpg", Type: productimage.AssetTypeMainImage},
		WhiteBgImage: &productimage.ImageAsset{SourceURL: "promo_badge_text.jpg", Type: productimage.AssetTypeWhiteBgImage, Metadata: map[string]string{"background": "white"}},
	})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if report.Passed {
		t.Fatal("expected compliance to fail")
	}
}

func TestDefaultReviewAssessor_Assess(t *testing.T) {
	assessor := productimage.NewDefaultReviewAssessor()
	decision, err := assessor.Assess(context.Background(), nil, []productimage.ImageAudit{
		{ImageURL: "hero.jpg", HasOverlayText: true, HasPromoBadge: true, PrimaryObject: "Running Shoes"},
	}, &productimage.ImageCandidateSet{PrimarySource: "hero.jpg"}, &productimage.ImageProcessResult{
		Quality: &productimage.QualityAssessment{
			OverallScore: 0.55,
			MainScore:    0.5,
			WhiteBgScore: 0.6,
		},
		ImageTraces: []productimage.ImageStageTrace{
			{Stage: "render_white_bg", Outcome: "fallback"},
		},
	})
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	if !decision.NeedsReview {
		t.Fatal("expected decision to require review")
	}
	if len(decision.Reasons) < 2 {
		t.Fatalf("expected multiple review reasons, got %+v", decision.Reasons)
	}
	foundObject := false
	for _, reason := range decision.Reasons {
		if reason == "primary image contains overlay text and was auto-cleaned for Running Shoes" {
			foundObject = true
			break
		}
	}
	if !foundObject {
		t.Fatalf("expected review reasons to include primary object, got %+v", decision.Reasons)
	}
}

func TestDefaultReviewAssessor_DoesNotFlagRenderGalleryFallbackByItself(t *testing.T) {
	assessor := productimage.NewDefaultReviewAssessor()
	decision, err := assessor.Assess(context.Background(), nil, nil, nil, &productimage.ImageProcessResult{
		Quality: &productimage.QualityAssessment{
			OverallScore: 0.8,
			MainScore:    0.8,
			WhiteBgScore: 0.8,
		},
		ImageTraces: []productimage.ImageStageTrace{
			{Stage: "render_gallery", Outcome: "fallback"},
		},
	})
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	if decision.NeedsReview {
		t.Fatalf("expected render_gallery fallback alone not to require review, got %+v", decision.Reasons)
	}
}

func TestDefaultQualityAssessor_Assess(t *testing.T) {
	assessor := productimage.NewDefaultQualityAssessor()
	assessment, err := assessor.Assess(context.Background(), nil, []productimage.ImageAudit{
		{ImageURL: "hero.jpg", QualityScore: 0.82, HasOverlayText: true, PrimaryObject: "Running Shoes"},
	}, &productimage.ImageCandidateSet{PrimarySource: "hero.jpg"}, &productimage.ImageProcessResult{
		WhiteBgImage: &productimage.ImageAsset{
			Type:     productimage.AssetTypeWhiteBgImage,
			Metadata: map[string]string{"background": "white", "background_mode": "white_canvas"},
		},
		ImageTraces: []productimage.ImageStageTrace{
			{Stage: "render_white_bg", Outcome: "fallback"},
		},
	})
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	if assessment.OverallScore <= 0 || assessment.OverallScore > 1 {
		t.Fatalf("unexpected overall score: %+v", assessment)
	}
	if len(assessment.Issues) == 0 {
		t.Fatal("expected quality issues")
	}
	foundObject := false
	for _, issue := range assessment.Issues {
		if issue == "primary image contains overlay text for Running Shoes" {
			foundObject = true
			break
		}
	}
	if !foundObject {
		t.Fatalf("expected quality issues to include primary object, got %+v", assessment.Issues)
	}
}

func TestDefaultQualityAssessor_UsesCategorySpecificWhiteCanvasPenalty(t *testing.T) {
	assessor := productimage.NewDefaultQualityAssessor()
	softAssessment, err := assessor.Assess(context.Background(), &productimage.SourceBundle{
		Marketplace: "amazon",
		Country:     "US",
		Analysis: &productenrich.ProductAnalysis{
			Representation: &productenrich.ProductRepresentation{ProductType: "Slippers"},
		},
	}, nil, nil, &productimage.ImageProcessResult{
		WhiteBgImage: &productimage.ImageAsset{
			Type:     productimage.AssetTypeWhiteBgImage,
			Metadata: map[string]string{"background": "white", "background_mode": "white_canvas"},
		},
	})
	if err != nil {
		t.Fatalf("Assess() soft error = %v", err)
	}
	hardAssessment, err := assessor.Assess(context.Background(), &productimage.SourceBundle{
		Marketplace: "amazon",
		Country:     "US",
		Analysis: &productenrich.ProductAnalysis{
			Representation: &productenrich.ProductRepresentation{ProductType: "Electronics"},
		},
	}, nil, nil, &productimage.ImageProcessResult{
		WhiteBgImage: &productimage.ImageAsset{
			Type:     productimage.AssetTypeWhiteBgImage,
			Metadata: map[string]string{"background": "white", "background_mode": "white_canvas"},
		},
	})
	if err != nil {
		t.Fatalf("Assess() hard error = %v", err)
	}
	if softAssessment.WhiteBgScore <= hardAssessment.WhiteBgScore {
		t.Fatalf("expected soft goods white bg score > hard goods, got soft=%v hard=%v", softAssessment.WhiteBgScore, hardAssessment.WhiteBgScore)
	}
}

func TestDefaultReviewAssessor_UsesCategorySpecificThresholds(t *testing.T) {
	assessor := productimage.NewDefaultReviewAssessor()
	softDecision, err := assessor.Assess(context.Background(), &productimage.SourceBundle{
		Marketplace: "amazon",
		Country:     "US",
		Analysis: &productenrich.ProductAnalysis{
			Representation: &productenrich.ProductRepresentation{ProductType: "Slippers"},
		},
	}, nil, nil, &productimage.ImageProcessResult{
		Quality: &productimage.QualityAssessment{
			OverallScore: 0.63,
			MainScore:    0.63,
			WhiteBgScore: 0.72,
		},
	})
	if err != nil {
		t.Fatalf("Assess() soft error = %v", err)
	}
	if softDecision.NeedsReview {
		t.Fatalf("expected slippers to pass relaxed threshold, got %+v", softDecision.Reasons)
	}

	hardDecision, err := assessor.Assess(context.Background(), &productimage.SourceBundle{
		Marketplace: "amazon",
		Country:     "US",
		Analysis: &productenrich.ProductAnalysis{
			Representation: &productenrich.ProductRepresentation{ProductType: "Electronics"},
		},
	}, nil, nil, &productimage.ImageProcessResult{
		Quality: &productimage.QualityAssessment{
			OverallScore: 0.63,
			MainScore:    0.63,
			WhiteBgScore: 0.72,
		},
	})
	if err != nil {
		t.Fatalf("Assess() hard error = %v", err)
	}
	if !hardDecision.NeedsReview {
		t.Fatal("expected electronics to require review at same score")
	}
}

func TestMarketplaceProfile_DefaultsWhenMarketplaceUnknown(t *testing.T) {
	assessor := productimage.NewDefaultReviewAssessor()
	decision, err := assessor.Assess(context.Background(), &productimage.SourceBundle{
		Marketplace: "etsy",
		Country:     "US",
		Analysis: &productenrich.ProductAnalysis{
			Representation: &productenrich.ProductRepresentation{ProductType: "Slippers"},
		},
	}, nil, nil, &productimage.ImageProcessResult{
		Quality: &productimage.QualityAssessment{
			OverallScore: 0.63,
			MainScore:    0.63,
			WhiteBgScore: 0.72,
		},
	})
	if err != nil {
		t.Fatalf("Assess() error = %v", err)
	}
	if !decision.NeedsReview {
		t.Fatal("expected unknown marketplace to fall back to default threshold")
	}
}

func TestMarketplaceProfile_FamilyKeywordTableIsLoaded(t *testing.T) {
	footwear := productimage.ProfileKeywordsForTest("footwear")
	electronics := productimage.ProfileKeywordsForTest("electronics")
	if len(footwear) == 0 {
		t.Fatal("expected footwear keywords")
	}
	if len(electronics) == 0 {
		t.Fatal("expected electronics keywords")
	}
}

func TestMarketplaceProfile_ResolvesFineGrainedFamilies(t *testing.T) {
	family, mainThreshold, _, _ := productimage.ResolveMarketplaceProfileForTest(&productimage.SourceBundle{
		Marketplace: "amazon",
		Country:     "US",
		Analysis: &productenrich.ProductAnalysis{
			Representation: &productenrich.ProductRepresentation{ProductType: "Running Shoes"},
		},
	})
	if family != "footwear" {
		t.Fatalf("family = %q, want footwear", family)
	}

	family, electronicsThreshold, _, _ := productimage.ResolveMarketplaceProfileForTest(&productimage.SourceBundle{
		Marketplace: "amazon",
		Country:     "US",
		Analysis: &productenrich.ProductAnalysis{
			Representation: &productenrich.ProductRepresentation{ProductType: "Bluetooth Speaker"},
		},
	})
	if family != "electronics" {
		t.Fatalf("family = %q, want electronics", family)
	}
	if electronicsThreshold <= mainThreshold {
		t.Fatalf("expected electronics threshold > footwear threshold, got electronics=%v footwear=%v", electronicsThreshold, mainThreshold)
	}
}
