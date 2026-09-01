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

type Role string

const (
	RoleDesign          Role = "design"
	RoleMain            Role = "main"
	RoleWhiteBackground Role = "white_background"
	RoleGallery         Role = "gallery"
)

func (r Role) valid() bool {
	switch r {
	case RoleDesign, RoleMain, RoleWhiteBackground, RoleGallery:
		return true
	default:
		return false
	}
}

// ApprovedAsset is an immutable approved product-asset fact. Repositories
// defensively copy its slice fields at their input and output boundaries.
type ApprovedAsset struct {
	ID            string   `json:"id"`
	RunID         string   `json:"run_id"`
	PlanRevision  int64    `json:"plan_revision"`
	SlotID        string   `json:"slot_id"`
	Attempt       int      `json:"attempt"`
	Role          Role     `json:"role"`
	URL           string   `json:"url"`
	SourceAssetID string   `json:"source_asset_id,omitempty"`
	Width         int      `json:"width,omitempty"`
	Height        int      `json:"height,omitempty"`
	Operations    []string `json:"operations,omitempty"`
}
