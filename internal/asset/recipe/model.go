package recipe

import "task-processor/internal/asset"

type ResolveRequest struct {
	Platform     string
	CategoryPath []string
}

type Template struct {
	BundleSlot     string       `json:"bundle_slot,omitempty"`
	Purpose        string       `json:"purpose,omitempty"`
	TemplateLabel  string       `json:"template_label,omitempty"`
	RenderProfile  string       `json:"render_profile,omitempty"`
	PreferredKinds []asset.Kind `json:"preferred_kinds,omitempty"`
	AllowedKinds   []asset.Kind `json:"allowed_kinds,omitempty"`
	Optional       bool         `json:"optional,omitempty"`
	MaxItems       int          `json:"max_items,omitempty"`
}

type AssetRecipe struct {
	ID          string     `json:"id,omitempty"`
	Platform    string     `json:"platform,omitempty"`
	Name        string     `json:"name,omitempty"`
	AssetKind   asset.Kind `json:"asset_kind,omitempty"`
	Generated   bool       `json:"generated,omitempty"`
	Description string     `json:"description,omitempty"`
	Template    *Template  `json:"template,omitempty"`
}

type Resolver interface {
	Resolve(req ResolveRequest) []AssetRecipe
}
