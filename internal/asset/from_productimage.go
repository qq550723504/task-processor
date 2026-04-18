package asset

import (
	"fmt"
	"strings"

	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func BuildBundle(canonical *productenrich.CanonicalProduct, result *productimage.ImageProcessResult) *Bundle {
	bundle := &Bundle{
		Selection: &Selection{},
	}

	if canonical != nil {
		for idx, image := range canonical.Images {
			asset := Asset{
				ID:        fmt.Sprintf("source-%d", idx+1),
				Kind:      KindSourceImage,
				URL:       image.URL,
				Role:      firstNonEmpty(image.Role, "source"),
				Generator: "canonical_product",
				Labels:    uniqueStrings([]string{image.Role}),
			}
			bundle.Assets = append(bundle.Assets, asset)
			bundle.Selection.SourceAssetIDs = append(bundle.Selection.SourceAssetIDs, asset.ID)
		}
	}

	if result != nil {
		if result.MainImage != nil {
			asset := buildProcessedAsset("main", KindMainImage, "primary", "productimage_pipeline", result.MainImage)
			bundle.Assets = append(bundle.Assets, asset)
			bundle.Selection.MainAssetID = asset.ID
		}
		if result.WhiteBgImage != nil {
			asset := buildProcessedAsset("white-bg", KindWhiteBgImage, "white_bg", "productimage_pipeline", result.WhiteBgImage)
			bundle.Assets = append(bundle.Assets, asset)
			bundle.Selection.WhiteBgAssetID = asset.ID
		}
		if result.SubjectCutout != nil {
			asset := buildProcessedAsset("subject-cutout", KindSubjectCutout, "cutout", "productimage_pipeline", result.SubjectCutout)
			bundle.Assets = append(bundle.Assets, asset)
			bundle.Selection.SubjectCutoutAssetID = asset.ID
		}
		for idx, image := range result.GalleryImages {
			asset := buildProcessedAsset(fmt.Sprintf("gallery-%d", idx+1), KindGalleryImage, "gallery", "productimage_pipeline", &image)
			bundle.Assets = append(bundle.Assets, asset)
			bundle.Selection.GalleryAssetIDs = append(bundle.Selection.GalleryAssetIDs, asset.ID)
		}
		bundle.Review = buildReviewSummary(result.Review)
		bundle.Compliance = buildComplianceSummary(result.Compliance)
		bundle.Quality = buildQualitySummary(result.Quality)
		bundle.IPRisk = buildIPRiskSummary(result.IPRisk)
	}

	bundle.Stats = buildStats(bundle.Assets)
	if bundle.Review == nil {
		bundle.Review = &ReviewSummary{}
	}
	return bundle
}

func buildProcessedAsset(id string, kind Kind, role, generator string, asset *productimage.ImageAsset) Asset {
	if asset == nil {
		return Asset{}
	}
	metadata := make(map[string]string, len(asset.Metadata))
	for key, value := range asset.Metadata {
		metadata[key] = value
	}
	labels := append([]string{role}, asset.Metadata["stage"])
	return Asset{
		ID:             id,
		Kind:           kind,
		URL:            asset.URL,
		Role:           role,
		Generator:      generator,
		SourceURL:      asset.SourceURL,
		SourceAssetIDs: firstNonEmptySlice(asset.Metadata["source_asset_id"]),
		Operations:     append([]string(nil), asset.Operations...),
		Labels:         uniqueStrings(labels),
		Width:          asset.Width,
		Height:         asset.Height,
		Metadata:       metadata,
	}
}

func buildReviewSummary(review *productimage.ReviewDecision) *ReviewSummary {
	if review == nil {
		return nil
	}
	return &ReviewSummary{
		NeedsReview: review.NeedsReview,
		Reasons:     append([]string(nil), review.Reasons...),
	}
}

func buildComplianceSummary(report *productimage.ComplianceReport) *ComplianceSummary {
	if report == nil {
		return nil
	}
	items := make([]IssueDigest, 0, len(report.Issues))
	for _, issue := range report.Issues {
		items = append(items, IssueDigest{
			Code:     issue.Code,
			Message:  issue.Message,
			Severity: issue.Severity,
			ImageURL: issue.ImageURL,
		})
	}
	return &ComplianceSummary{
		Marketplace: report.Marketplace,
		Passed:      report.Passed,
		Issues:      items,
	}
}

func buildQualitySummary(quality *productimage.QualityAssessment) *QualitySummary {
	if quality == nil {
		return nil
	}
	return &QualitySummary{
		OverallScore: quality.OverallScore,
		MainScore:    quality.MainScore,
		WhiteBgScore: quality.WhiteBgScore,
		Issues:       append([]string(nil), quality.Issues...),
	}
}

func buildIPRiskSummary(report *productimage.IPRiskReport) *IPRiskSummary {
	if report == nil {
		return nil
	}
	return &IPRiskSummary{
		Level:   report.Level,
		Score:   report.Score,
		Reasons: append([]string(nil), report.Reasons...),
	}
}

func buildStats(assets []Asset) *Stats {
	stats := &Stats{TotalAssets: len(assets)}
	for _, asset := range assets {
		switch originForAsset(asset) {
		case OriginSource:
			stats.SourceAssets++
		case OriginGenerated:
			stats.GeneratedAssets++
		default:
			stats.DerivedAssets++
		}
	}
	return stats
}

func uniqueStrings(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptySlice(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
