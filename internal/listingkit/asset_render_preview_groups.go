package listingkit

import (
	previewdomain "task-processor/internal/listing/preview"
)

type PlatformAssetRenderPreviews struct {
	Platform  string                             `json:"platform,omitempty"`
	Summary   *PlatformAssetRenderPreviewSummary `json:"summary,omitempty"`
	Main      *AssetRenderPreviewSlot            `json:"main,omitempty"`
	Gallery   []AssetRenderPreviewSlot           `json:"gallery,omitempty"`
	Auxiliary []AssetRenderPreviewSlot           `json:"auxiliary,omitempty"`
}

type PlatformAssetRenderPreviewSummary struct {
	TotalPreviews    int            `json:"total_previews"`
	MainAvailable    bool           `json:"main_available"`
	GalleryCount     int            `json:"gallery_count"`
	AuxiliaryCount   int            `json:"auxiliary_count"`
	CapabilityCounts map[string]int `json:"capability_counts,omitempty"`
	VisualModes      []string       `json:"visual_modes,omitempty"`
}

func buildPlatformAssetRenderPreviewSummary(group PlatformAssetRenderPreviews) *PlatformAssetRenderPreviewSummary {
	summary := previewdomain.SummarizePlatformRenderPreviews(previewdomain.PlatformRenderPreviewSummaryInput[AssetRenderPreviewSlot]{
		Main:      group.Main,
		Gallery:   group.Gallery,
		Auxiliary: group.Auxiliary,
		VisualMode: func(slot AssetRenderPreviewSlot) string {
			return slot.VisualMode
		},
		Capabilities: buildRenderPreviewCapabilitiesForSlot,
	})
	if summary == nil {
		return nil
	}
	return &PlatformAssetRenderPreviewSummary{
		TotalPreviews:    summary.TotalPreviews,
		MainAvailable:    summary.MainAvailable,
		GalleryCount:     summary.GalleryCount,
		AuxiliaryCount:   summary.AuxiliaryCount,
		CapabilityCounts: cloneStringIntMap(summary.CapabilityCounts),
		VisualModes:      append([]string(nil), summary.VisualModes...),
	}
}

type AssetRenderPreviewSlot struct {
	Slot                string   `json:"slot,omitempty"`
	Purpose             string   `json:"purpose,omitempty"`
	AssetID             string   `json:"asset_id,omitempty"`
	AssetURL            string   `json:"asset_url,omitempty"`
	AssetRevision       string   `json:"asset_revision,omitempty"`
	PreviewRevision     string   `json:"preview_revision,omitempty"`
	TaskRevision        string   `json:"task_revision,omitempty"`
	Kind                string   `json:"kind,omitempty"`
	Role                string   `json:"role,omitempty"`
	StateLabel          string   `json:"state_label,omitempty"`
	RetryHint           string   `json:"retry_hint,omitempty"`
	TemplateLabel       string   `json:"template_label,omitempty"`
	RenderProfile       string   `json:"render_profile,omitempty"`
	PreviewFormat       string   `json:"preview_format,omitempty"`
	PreviewSVG          string   `json:"preview_svg,omitempty"`
	SourceKind          string   `json:"source_kind,omitempty"`
	GenerationMode      string   `json:"generation_mode,omitempty"`
	VisualMode          string   `json:"visual_mode,omitempty"`
	LayoutEngine        string   `json:"layout_engine,omitempty"`
	RenderOutputVersion string   `json:"render_output_version,omitempty"`
	DrawOutputVersion   string   `json:"draw_output_version,omitempty"`
	DrawPreviewVersion  string   `json:"draw_preview_version,omitempty"`
	LayerTypes          []string `json:"layer_types,omitempty"`
	Regions             []string `json:"regions,omitempty"`
	StyleTokens         []string `json:"style_tokens,omitempty"`
}

func buildPlatformAssetRenderPreviews(result *ListingKitResult) []PlatformAssetRenderPreviews {
	if result == nil {
		return nil
	}
	previews := result.AssetRenderPreviews
	if len(previews) == 0 {
		previews = attachTaskRevisionToAssetRenderPreviews(buildAssetRenderPreviews(result.AssetBundle), buildTaskRevision(result))
	}
	previewByAssetID := make(map[string]AssetRenderPreview, len(previews))
	for _, preview := range previews {
		if preview.AssetID == "" {
			continue
		}
		previewByAssetID[preview.AssetID] = preview
	}
	if len(previewByAssetID) == 0 {
		previewByAssetID = map[string]AssetRenderPreview{}
	}
	assetURLByID := buildAssetURLLookup(result.AssetBundle)
	out := make([]PlatformAssetRenderPreviews, 0, 4)
	if group, ok := buildPlatformAssetRenderPreviewGroup("amazon", previewByAssetID, assetURLByID, result.AssetBundle, imageBundleFromAmazon(result.Amazon)); ok {
		out = append(out, group)
	}
	if group, ok := buildPlatformAssetRenderPreviewGroup("shein", previewByAssetID, assetURLByID, result.AssetBundle, imageBundleFromShein(result.Shein)); ok {
		out = append(out, group)
	}
	if group, ok := buildPlatformAssetRenderPreviewGroup("temu", previewByAssetID, assetURLByID, result.AssetBundle, imageBundleFromTemu(result.Temu)); ok {
		out = append(out, group)
	}
	if group, ok := buildPlatformAssetRenderPreviewGroup("walmart", previewByAssetID, assetURLByID, result.AssetBundle, imageBundleFromWalmart(result.Walmart)); ok {
		out = append(out, group)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func syncAssetRenderPreviews(result *ListingKitResult) {
	if result == nil {
		return
	}
	previews := append([]AssetRenderPreview(nil), result.AssetRenderPreviews...)
	if len(previews) == 0 {
		previews = buildAssetRenderPreviews(result.AssetBundle)
	}
	result.AssetRenderPreviews = attachTaskRevisionToAssetRenderPreviews(previews, buildTaskRevision(result))
	platformPreviews := buildPlatformAssetRenderPreviews(result)
	if len(platformPreviews) > 0 || len(result.PlatformAssetRenderPreviews) == 0 {
		result.PlatformAssetRenderPreviews = platformPreviews
	}
}

func filterPlatformAssetRenderPreviews(groups []PlatformAssetRenderPreviews, platform string) []PlatformAssetRenderPreviews {
	if platform == "" || len(groups) == 0 {
		return groups
	}
	out := make([]PlatformAssetRenderPreviews, 0, len(groups))
	for _, group := range groups {
		if group.Platform == platform {
			out = append(out, group)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func platformAssetRenderPreviewsByPlatform(groups []PlatformAssetRenderPreviews, platform string) *PlatformAssetRenderPreviews {
	if len(groups) == 0 || platform == "" {
		return nil
	}
	for _, group := range groups {
		if group.Platform == platform {
			copyGroup := group
			return &copyGroup
		}
	}
	return nil
}
