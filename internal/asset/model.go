package asset

type Kind string

const (
	KindSourceImage       Kind = "source_image"
	KindCleanImage        Kind = "clean_image"
	KindMainImage         Kind = "main_image"
	KindWhiteBgImage      Kind = "white_bg_image"
	KindSubjectCutout     Kind = "subject_cutout"
	KindGalleryImage      Kind = "gallery_image"
	KindDetailCrop        Kind = "detail_crop"
	KindSceneImage        Kind = "scene_image"
	KindSellingPointImage Kind = "selling_point_image"
	KindSizeSceneImage    Kind = "size_scene_image"
	KindModelImage        Kind = "model_image"
)

type Bundle struct {
	Assets     []Asset            `json:"assets,omitempty"`
	Selection  *Selection         `json:"selection,omitempty"`
	Stats      *Stats             `json:"stats,omitempty"`
	Review     *ReviewSummary     `json:"review,omitempty"`
	Compliance *ComplianceSummary `json:"compliance,omitempty"`
	Quality    *QualitySummary    `json:"quality,omitempty"`
	IPRisk     *IPRiskSummary     `json:"ip_risk,omitempty"`
}

type Asset struct {
	ID             string            `json:"id,omitempty"`
	Kind           Kind              `json:"kind,omitempty"`
	URL            string            `json:"url,omitempty"`
	Role           string            `json:"role,omitempty"`
	Generator      string            `json:"generator,omitempty"`
	RecipeID       string            `json:"recipe_id,omitempty"`
	SourceURL      string            `json:"source_url,omitempty"`
	SourceAssetIDs []string          `json:"source_asset_ids,omitempty"`
	Operations     []string          `json:"operations,omitempty"`
	Labels         []string          `json:"labels,omitempty"`
	PlatformTags   []string          `json:"platform_tags,omitempty"`
	Width          int               `json:"width,omitempty"`
	Height         int               `json:"height,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

type Selection struct {
	MainAssetID          string   `json:"main_asset_id,omitempty"`
	WhiteBgAssetID       string   `json:"white_bg_asset_id,omitempty"`
	SubjectCutoutAssetID string   `json:"subject_cutout_asset_id,omitempty"`
	GalleryAssetIDs      []string `json:"gallery_asset_ids,omitempty"`
	SourceAssetIDs       []string `json:"source_asset_ids,omitempty"`
}

type Stats struct {
	TotalAssets     int `json:"total_assets"`
	SourceAssets    int `json:"source_assets"`
	DerivedAssets   int `json:"derived_assets"`
	GeneratedAssets int `json:"generated_assets"`
}

type ReviewSummary struct {
	NeedsReview bool     `json:"needs_review"`
	Reasons     []string `json:"reasons,omitempty"`
}

type ComplianceSummary struct {
	Marketplace string        `json:"marketplace,omitempty"`
	Passed      bool          `json:"passed"`
	Issues      []IssueDigest `json:"issues,omitempty"`
}

type QualitySummary struct {
	OverallScore float64  `json:"overall_score,omitempty"`
	MainScore    float64  `json:"main_score,omitempty"`
	WhiteBgScore float64  `json:"white_bg_score,omitempty"`
	Issues       []string `json:"issues,omitempty"`
}

type IPRiskSummary struct {
	Level   string   `json:"level,omitempty"`
	Score   float64  `json:"score,omitempty"`
	Reasons []string `json:"reasons,omitempty"`
}

type IssueDigest struct {
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
	Severity string `json:"severity,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}
