package productimage

import (
	"context"
	"fmt"
	"sort"
	"strings"

	productenrich "task-processor/internal/productenrich"
)

type sourceParserAdapter struct {
	inputParser productenrich.InputParser
}

func NewSourceParser(inputParser productenrich.InputParser) (SourceParser, error) {
	if inputParser == nil {
		return nil, fmt.Errorf("input parser cannot be nil")
	}
	return &sourceParserAdapter{inputParser: inputParser}, nil
}

func (a *sourceParserAdapter) Parse(ctx context.Context, req *ImageProcessRequest) (*SourceBundle, error) {
	parsedInput, err := a.inputParser.ParseInput(ctx, &productenrich.GenerateRequest{
		ImageURLs:  append([]string(nil), req.ImageURLs...),
		Text:       req.Text,
		ProductURL: req.ProductURL,
	})
	if err != nil {
		return nil, err
	}
	return &SourceBundle{
		Images:      append([]string(nil), parsedInput.Images...),
		Text:        parsedInput.Text,
		TitleHint:   sourceTitleHintFromParsedInput(parsedInput),
		ProductURL:  req.ProductURL,
		Marketplace: req.Marketplace,
		Country:     req.Country,
		ParsedInput: parsedInput,
	}, nil
}

type productContextAnalyzerAdapter struct {
	understanding productenrich.ProductUnderstanding
}

func NewProductContextAnalyzer(understanding productenrich.ProductUnderstanding) (ProductContextAnalyzer, error) {
	if understanding == nil {
		return nil, fmt.Errorf("product understanding cannot be nil")
	}
	return &productContextAnalyzerAdapter{understanding: understanding}, nil
}

func (a *productContextAnalyzerAdapter) Analyze(ctx context.Context, source *SourceBundle) (*productenrich.ProductAnalysis, error) {
	if source == nil {
		return nil, fmt.Errorf("source cannot be nil")
	}
	if source.ParsedInput == nil {
		return nil, fmt.Errorf("parsed input is required for context analysis")
	}
	return a.understanding.AnalyzeProduct(ctx, source.ParsedInput)
}

type defaultImageInspector struct{}

func NewDefaultImageInspector() ImageInspector { return &defaultImageInspector{} }

func (i *defaultImageInspector) Inspect(_ context.Context, source *SourceBundle, imageURL string) (*ImageAudit, error) {
	lower := strings.ToLower(imageURL)
	audit := &ImageAudit{
		ImageURL:          imageURL,
		IsWhiteBackground: containsAny(lower, "white", "whitebg", "white-background"),
		HasOverlayText:    containsAny(lower, "text", "poster", "caption", "label", "desc"),
		HasPromoBadge:     containsAny(lower, "promo", "sale", "discount", "coupon", "price", "badge"),
		HasLogo:           containsAny(lower, "logo", "watermark", "brandmark"),
		IsCollage:         containsAny(lower, "collage", "grid", "contact-sheet", "mosaic"),
		SharpnessScore:    0.8,
		QualityScore:      0.8,
		PrimaryObject:     sourceTitleHint(source),
	}
	if audit.IsWhiteBackground {
		audit.QualityScore += 0.15
	}
	if audit.HasOverlayText {
		audit.QualityScore -= 0.2
		audit.Issues = append(audit.Issues, "overlay_text")
	}
	if audit.HasPromoBadge {
		audit.QualityScore -= 0.25
		audit.Issues = append(audit.Issues, "promo_badge")
	}
	if audit.HasLogo {
		audit.QualityScore -= 0.2
		audit.Issues = append(audit.Issues, "logo_or_watermark")
	}
	if audit.IsCollage {
		audit.QualityScore -= 0.35
		audit.Issues = append(audit.Issues, "collage")
	}
	if audit.QualityScore < 0.1 {
		audit.QualityScore = 0.1
	}
	return audit, nil
}

type defaultImageRanker struct{}

func NewDefaultImageRanker() ImageRanker { return &defaultImageRanker{} }

func (r *defaultImageRanker) Select(_ context.Context, source *SourceBundle, audits []ImageAudit, _ *productenrich.ProductAnalysis) (*ImageCandidateSet, error) {
	if source == nil {
		return nil, fmt.Errorf("source cannot be nil")
	}
	result := &ImageCandidateSet{}
	if len(source.Images) == 0 {
		return result, nil
	}
	type scored struct {
		url   string
		score float64
	}
	accepted := make([]scored, 0, len(audits))
	for _, audit := range audits {
		if audit.IsCollage || (audit.HasPromoBadge && audit.HasOverlayText) {
			result.RejectedImages = append(result.RejectedImages, audit.ImageURL)
			continue
		}
		score := audit.QualityScore
		if audit.IsWhiteBackground {
			score += 0.2
		}
		accepted = append(accepted, scored{url: audit.ImageURL, score: score})
	}
	if len(accepted) == 0 {
		accepted = append(accepted, scored{url: source.Images[0], score: 0})
	}
	sort.SliceStable(accepted, func(i, j int) bool { return accepted[i].score > accepted[j].score })
	result.PrimarySource = accepted[0].url
	for idx, item := range accepted {
		if idx == 0 {
			result.HeroCandidates = append(result.HeroCandidates, item.url)
			continue
		}
		result.SceneCandidates = append(result.SceneCandidates, item.url)
	}
	return result, nil
}

type defaultSubjectExtractor struct{}

func NewDefaultSubjectExtractor() SubjectExtractor { return &defaultSubjectExtractor{} }

func (e *defaultSubjectExtractor) Extract(_ context.Context, imageURL string, analysis *productenrich.ProductAnalysis) (*ImageAsset, error) {
	if imageURL == "" {
		return nil, fmt.Errorf("image URL cannot be empty")
	}
	metadata := map[string]string{"mode": "placeholder"}
	if analysis != nil && analysis.Representation != nil && analysis.Representation.ProductType != "" {
		metadata["product_type"] = analysis.Representation.ProductType
	}
	return &ImageAsset{URL: imageURL, Type: AssetTypeSubjectCutout, SourceURL: imageURL, Operations: []string{"select_subject", "extract_subject_placeholder"}, Metadata: metadata}, nil
}

type defaultImageCleaner struct{}

func NewDefaultImageCleaner() ImageCleaner { return &defaultImageCleaner{} }

func (c *defaultImageCleaner) Clean(_ context.Context, asset *ImageAsset, _ *productenrich.ProductAnalysis) (*ImageAsset, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset cannot be nil")
	}
	cleaned := &ImageAsset{
		URL:        asset.URL,
		Type:       AssetTypeMainImage,
		SourceURL:  asset.SourceURL,
		Operations: append([]string{}, asset.Operations...),
		Metadata:   cloneMetadata(asset.Metadata),
	}
	cleaned.Operations = append(cleaned.Operations, "cleanup_placeholder")
	lower := strings.ToLower(cleaned.SourceURL)
	if containsAny(lower, "text", "poster", "caption", "label", "desc") {
		cleaned.Operations = append(cleaned.Operations, "remove_overlay_text_placeholder")
		cleaned.Metadata["overlay_text_removed"] = "true"
	}
	if containsAny(lower, "promo", "sale", "discount", "coupon", "price", "badge") {
		cleaned.Operations = append(cleaned.Operations, "remove_promo_badge_placeholder")
		cleaned.Metadata["promo_badge_removed"] = "true"
	}
	if containsAny(lower, "logo", "watermark", "brandmark") {
		cleaned.Operations = append(cleaned.Operations, "remove_logo_overlay_placeholder")
		cleaned.Metadata["logo_overlay_removed"] = "true"
	}
	return cleaned, nil
}

type defaultWhiteBackgroundRenderer struct{}

func NewDefaultWhiteBackgroundRenderer() WhiteBackgroundRenderer {
	return &defaultWhiteBackgroundRenderer{}
}

func (r *defaultWhiteBackgroundRenderer) Render(_ context.Context, asset *ImageAsset, _ *productenrich.ProductAnalysis) (*ImageAsset, error) {
	if asset == nil {
		return nil, fmt.Errorf("asset cannot be nil")
	}
	metadata := cloneMetadata(asset.Metadata)
	metadata["mode"] = "placeholder"
	metadata["background"] = "white"
	return &ImageAsset{URL: asset.URL, Type: AssetTypeWhiteBgImage, SourceURL: asset.SourceURL, Operations: append(append([]string{}, asset.Operations...), "render_white_bg_placeholder"), Metadata: metadata}, nil
}

type defaultMarketplaceValidator struct{}

func NewDefaultMarketplaceValidator() MarketplaceValidator { return &defaultMarketplaceValidator{} }

func (v *defaultMarketplaceValidator) Validate(_ context.Context, req *ImageProcessRequest, result *ImageProcessResult) (*ComplianceReport, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if result == nil {
		return nil, fmt.Errorf("result cannot be nil")
	}
	report := &ComplianceReport{Marketplace: req.Marketplace, Passed: true}
	if result.MainImage == nil {
		report.Passed = false
		report.Issues = append(report.Issues, ImageIssue{Code: "missing_main_image", Message: "main image is required", Severity: "error"})
	}
	if result.WhiteBgImage == nil {
		report.Passed = false
		report.Issues = append(report.Issues, ImageIssue{Code: "missing_white_bg", Message: "white background image is required", Severity: "error"})
	}
	if result.WhiteBgImage != nil && result.WhiteBgImage.Metadata["background"] != "white" {
		report.Passed = false
		report.Issues = append(report.Issues, ImageIssue{Code: "non_white_background", Message: "white background image must declare white background", Severity: "error"})
	}
	if result.MainImage != nil {
		lower := strings.ToLower(result.MainImage.SourceURL)
		if containsAny(lower, "collage", "grid", "contact-sheet", "mosaic") {
			report.Passed = false
			report.Issues = append(report.Issues, ImageIssue{Code: "collage_risk", Message: "main image looks like a collage", Severity: "error"})
		}
		if containsAny(lower, "promo", "sale", "discount", "coupon", "price", "badge") && result.MainImage.Metadata["promo_badge_removed"] != "true" {
			report.Passed = false
			report.Issues = append(report.Issues, ImageIssue{Code: "promo_badge_present", Message: "main image still appears to contain promo badge", Severity: "error"})
		}
		if containsAny(lower, "text", "poster", "caption", "label", "desc") && result.MainImage.Metadata["overlay_text_removed"] != "true" {
			report.Passed = false
			report.Issues = append(report.Issues, ImageIssue{Code: "overlay_text_present", Message: "main image still appears to contain overlay text", Severity: "error"})
		}
		if containsAny(lower, "logo", "watermark", "brandmark") && result.MainImage.Metadata["logo_overlay_removed"] != "true" {
			report.Passed = false
			report.Issues = append(report.Issues, ImageIssue{Code: "logo_present", Message: "main image still appears to contain logo or watermark", Severity: "error"})
		}
	}
	return report, nil
}

type defaultQualityAssessor struct{}

func NewDefaultQualityAssessor() QualityAssessor { return &defaultQualityAssessor{} }

func (a *defaultQualityAssessor) Assess(_ context.Context, source *SourceBundle, audits []ImageAudit, candidates *ImageCandidateSet, result *ImageProcessResult) (*QualityAssessment, error) {
	profile := resolveMarketplaceProfile(source)
	assessment := &QualityAssessment{
		OverallScore: 0.75,
		MainScore:    0.75,
		WhiteBgScore: 0.75,
	}
	primaryAudit := findPrimaryAudit(audits, candidates)
	if primaryAudit != nil {
		assessment.MainScore = primaryAudit.QualityScore
		if primaryAudit.IsCollage {
			assessment.MainScore -= 0.35
			assessment.Issues = append(assessment.Issues, describePrimaryAuditIssue(primaryAudit, "primary image collage risk"))
		}
		if primaryAudit.HasOverlayText {
			assessment.MainScore -= 0.15
			assessment.Issues = append(assessment.Issues, describePrimaryAuditIssue(primaryAudit, "primary image contains overlay text"))
		}
		if primaryAudit.HasPromoBadge {
			assessment.MainScore -= 0.2
			assessment.Issues = append(assessment.Issues, describePrimaryAuditIssue(primaryAudit, "primary image contains promo badge"))
		}
		if primaryAudit.HasLogo {
			assessment.MainScore -= 0.2
			assessment.Issues = append(assessment.Issues, describePrimaryAuditIssue(primaryAudit, "primary image contains logo or watermark"))
		}
	}
	if result != nil && result.WhiteBgImage != nil {
		if result.WhiteBgImage.Metadata["background"] == "white" {
			assessment.WhiteBgScore = 0.9
		}
		if result.WhiteBgImage.Metadata["background_mode"] == "white_canvas" {
			assessment.WhiteBgScore -= profile.WhiteCanvasPenalty
			assessment.Issues = append(assessment.Issues, "white background uses canvas fallback")
		}
	}
	if result != nil {
		for _, trace := range result.ImageTraces {
			if trace.Outcome == "fallback" {
				switch trace.Stage {
				case "extract_subject":
					assessment.MainScore -= 0.05
					assessment.Issues = append(assessment.Issues, "subject extraction fallback used")
				case "cleanup_image":
					assessment.MainScore -= 0.08
					assessment.Issues = append(assessment.Issues, "main image cleanup fallback used")
				case "render_white_bg":
					assessment.WhiteBgScore -= 0.12
					assessment.Issues = append(assessment.Issues, "white background fallback used")
				}
			}
		}
	}
	assessment.MainScore = clampScore(assessment.MainScore)
	assessment.WhiteBgScore = clampScore(assessment.WhiteBgScore)
	assessment.OverallScore = clampScore((assessment.MainScore*0.6 + assessment.WhiteBgScore*0.4))
	assessment.Issues = uniqueStrings(assessment.Issues)
	return assessment, nil
}

type defaultReviewAssessor struct{}

func NewDefaultReviewAssessor() ReviewAssessor { return &defaultReviewAssessor{} }

func (a *defaultReviewAssessor) Assess(_ context.Context, source *SourceBundle, audits []ImageAudit, candidates *ImageCandidateSet, result *ImageProcessResult) (*ReviewDecision, error) {
	profile := resolveMarketplaceProfile(source)
	decision := &ReviewDecision{}
	primaryAudit := findPrimaryAudit(audits, candidates)
	if primaryAudit != nil {
		if primaryAudit.HasOverlayText {
			decision.Reasons = append(decision.Reasons, describePrimaryAuditIssue(primaryAudit, "primary image contains overlay text and was auto-cleaned"))
		}
		if primaryAudit.HasPromoBadge {
			decision.Reasons = append(decision.Reasons, describePrimaryAuditIssue(primaryAudit, "primary image contains promo badge and was auto-cleaned"))
		}
		if primaryAudit.HasLogo {
			decision.Reasons = append(decision.Reasons, describePrimaryAuditIssue(primaryAudit, "primary image contains logo or watermark and was auto-cleaned"))
		}
		if primaryAudit.IsCollage {
			decision.Reasons = append(decision.Reasons, describePrimaryAuditIssue(primaryAudit, "primary image has collage risk"))
		}
	}
	if result != nil {
		if result.Quality != nil {
			if result.Quality.OverallScore < profile.MainReviewThreshold {
				decision.Reasons = append(decision.Reasons, fmt.Sprintf("overall quality score %.2f is below review threshold", result.Quality.OverallScore))
			}
			if result.Quality.MainScore < profile.MainReviewThreshold {
				decision.Reasons = append(decision.Reasons, fmt.Sprintf("main image quality score %.2f is below threshold", result.Quality.MainScore))
			}
			if result.Quality.WhiteBgScore < profile.WhiteBgReviewThreshold {
				decision.Reasons = append(decision.Reasons, fmt.Sprintf("white background quality score %.2f is below threshold", result.Quality.WhiteBgScore))
			}
		}
		for _, trace := range result.ImageTraces {
			if trace.Outcome == "fallback" && (trace.Stage == "extract_subject" || trace.Stage == "cleanup_image" || trace.Stage == "render_white_bg") {
				decision.Reasons = append(decision.Reasons, fmt.Sprintf("%s used fallback path", trace.Stage))
			}
		}
	}
	decision.Reasons = uniqueStrings(decision.Reasons)
	decision.NeedsReview = len(decision.Reasons) > 0
	return decision, nil
}

func cloneMetadata(src map[string]string) map[string]string {
	if len(src) == 0 {
		return map[string]string{}
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func describePrimaryAuditIssue(audit *ImageAudit, message string) string {
	if audit == nil {
		return message
	}
	if object := strings.TrimSpace(audit.PrimaryObject); object != "" {
		return fmt.Sprintf("%s for %s", message, object)
	}
	return message
}

func sourceTitleHint(source *SourceBundle) string {
	if source == nil {
		return ""
	}
	if strings.TrimSpace(source.TitleHint) != "" {
		return strings.TrimSpace(source.TitleHint)
	}
	if source.Analysis != nil {
		if source.Analysis.TextAttributes != nil && strings.TrimSpace(source.Analysis.TextAttributes.Title) != "" {
			return strings.TrimSpace(source.Analysis.TextAttributes.Title)
		}
		if source.Analysis.ScrapedData != nil && strings.TrimSpace(source.Analysis.ScrapedData.Title) != "" {
			return strings.TrimSpace(source.Analysis.ScrapedData.Title)
		}
		if source.Analysis.Representation != nil && strings.TrimSpace(source.Analysis.Representation.ProductType) != "" {
			return strings.TrimSpace(source.Analysis.Representation.ProductType)
		}
	}
	if source.ParsedInput != nil && source.ParsedInput.ScrapedData != nil && strings.TrimSpace(source.ParsedInput.ScrapedData.Title) != "" {
		return strings.TrimSpace(source.ParsedInput.ScrapedData.Title)
	}
	if strings.TrimSpace(source.Text) != "" {
		text := strings.TrimSpace(source.Text)
		runes := []rune(text)
		if len(runes) > 120 {
			return strings.TrimSpace(string(runes[:120]))
		}
		return text
	}
	return ""
}

func sourceTitleHintFromParsedInput(input *productenrich.ParsedInput) string {
	if input == nil {
		return ""
	}
	if input.ScrapedData != nil && strings.TrimSpace(input.ScrapedData.Title) != "" {
		return strings.TrimSpace(input.ScrapedData.Title)
	}
	if strings.TrimSpace(input.Text) != "" {
		text := strings.TrimSpace(input.Text)
		runes := []rune(text)
		if len(runes) > 120 {
			return strings.TrimSpace(string(runes[:120]))
		}
		return text
	}
	return ""
}

func containsAny(s string, parts ...string) bool {
	for _, part := range parts {
		if strings.Contains(s, part) {
			return true
		}
	}
	return false
}

func findPrimaryAudit(audits []ImageAudit, candidates *ImageCandidateSet) *ImageAudit {
	if candidates == nil || candidates.PrimarySource == "" {
		return nil
	}
	for idx := range audits {
		if audits[idx].ImageURL == candidates.PrimarySource {
			return &audits[idx]
		}
	}
	return nil
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func clampScore(value float64) float64 {
	switch {
	case value < 0:
		return 0
	case value > 1:
		return 1
	default:
		return value
	}
}
