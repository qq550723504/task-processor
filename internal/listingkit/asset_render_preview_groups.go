package listingkit

import common "task-processor/internal/publishing/common"

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

type AssetRenderPreviewSlot struct {
	Slot                string   `json:"slot,omitempty"`
	Purpose             string   `json:"purpose,omitempty"`
	AssetID             string   `json:"asset_id,omitempty"`
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
	if len(previews) == 0 {
		return nil
	}
	previewByAssetID := make(map[string]AssetRenderPreview, len(previews))
	for _, preview := range previews {
		if preview.AssetID == "" {
			continue
		}
		previewByAssetID[preview.AssetID] = preview
	}
	if len(previewByAssetID) == 0 {
		return nil
	}
	out := make([]PlatformAssetRenderPreviews, 0, 4)
	if group, ok := buildPlatformAssetRenderPreviewGroup("amazon", previewByAssetID, imageBundleFromAmazon(result.Amazon)); ok {
		out = append(out, group)
	}
	if group, ok := buildPlatformAssetRenderPreviewGroup("shein", previewByAssetID, imageBundleFromShein(result.Shein)); ok {
		out = append(out, group)
	}
	if group, ok := buildPlatformAssetRenderPreviewGroup("temu", previewByAssetID, imageBundleFromTemu(result.Temu)); ok {
		out = append(out, group)
	}
	if group, ok := buildPlatformAssetRenderPreviewGroup("walmart", previewByAssetID, imageBundleFromWalmart(result.Walmart)); ok {
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
	result.AssetRenderPreviews = attachTaskRevisionToAssetRenderPreviews(buildAssetRenderPreviews(result.AssetBundle), buildTaskRevision(result))
	result.PlatformAssetRenderPreviews = buildPlatformAssetRenderPreviews(result)
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

func buildPlatformAssetRenderPreviewGroup(platform string, previewByAssetID map[string]AssetRenderPreview, bundle *common.PublishImageBundle) (PlatformAssetRenderPreviews, bool) {
	if bundle == nil {
		return PlatformAssetRenderPreviews{}, false
	}
	group := PlatformAssetRenderPreviews{Platform: platform}
	group.Main = buildAssetRenderPreviewSlot("main", bundle.Main, previewByAssetID)
	group.Gallery = buildAssetRenderPreviewSlots("gallery", bundle.Gallery, previewByAssetID)
	group.Auxiliary = buildAssetRenderPreviewSlots("auxiliary", bundle.Auxiliary, previewByAssetID)
	if group.Main == nil && len(group.Gallery) == 0 && len(group.Auxiliary) == 0 {
		return PlatformAssetRenderPreviews{}, false
	}
	group.Summary = buildPlatformAssetRenderPreviewSummary(group)
	return group, true
}

func buildAssetRenderPreviewSlots(defaultSlot string, slots []common.BundleSlot, previewByAssetID map[string]AssetRenderPreview) []AssetRenderPreviewSlot {
	if len(slots) == 0 {
		return nil
	}
	out := make([]AssetRenderPreviewSlot, 0, len(slots))
	for _, slot := range slots {
		built := buildAssetRenderPreviewSlot(defaultSlot, &slot, previewByAssetID)
		if built != nil {
			out = append(out, *built)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildAssetRenderPreviewSlot(defaultSlot string, slot *common.BundleSlot, previewByAssetID map[string]AssetRenderPreview) *AssetRenderPreviewSlot {
	if slot == nil || slot.AssetID == "" {
		return nil
	}
	preview, ok := previewByAssetID[slot.AssetID]
	if !ok || preview.PreviewSVG == "" {
		return nil
	}
	slotKey := slot.Key
	if slotKey == "" {
		slotKey = defaultSlot
	}
	return &AssetRenderPreviewSlot{
		Slot:                slotKey,
		Purpose:             slot.Purpose,
		AssetID:             slot.AssetID,
		AssetRevision:       preview.AssetRevision,
		PreviewRevision:     preview.PreviewRevision,
		TaskRevision:        preview.TaskRevision,
		Kind:                slot.Kind,
		Role:                preview.Role,
		StateLabel:          slot.StateLabel,
		RetryHint:           slot.RetryHint,
		TemplateLabel:       firstNonEmpty(slot.TemplateLabel, preview.TemplateLabel),
		RenderProfile:       preview.RenderProfile,
		PreviewFormat:       preview.PreviewFormat,
		PreviewSVG:          preview.PreviewSVG,
		SourceKind:          preview.SourceKind,
		GenerationMode:      preview.GenerationMode,
		VisualMode:          preview.VisualMode,
		LayoutEngine:        preview.LayoutEngine,
		RenderOutputVersion: preview.RenderOutputVersion,
		DrawOutputVersion:   preview.DrawOutputVersion,
		DrawPreviewVersion:  preview.DrawPreviewVersion,
		LayerTypes:          append([]string(nil), preview.LayerTypes...),
		Regions:             append([]string(nil), preview.Regions...),
		StyleTokens:         append([]string(nil), preview.StyleTokens...),
	}
}

func imageBundleFromAmazon(pkg *AmazonPackage) *common.PublishImageBundle {
	if pkg == nil {
		return nil
	}
	return pkg.ImageBundle
}

func imageBundleFromShein(pkg *SheinPackage) *common.PublishImageBundle {
	if pkg == nil {
		return nil
	}
	return pkg.ImageBundle
}

func imageBundleFromTemu(pkg *TemuPackage) *common.PublishImageBundle {
	if pkg == nil {
		return nil
	}
	return pkg.ImageBundle
}

func imageBundleFromWalmart(pkg *WalmartPackage) *common.PublishImageBundle {
	if pkg == nil {
		return nil
	}
	return pkg.ImageBundle
}
