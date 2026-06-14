package listingkit

import (
	"strings"

	"task-processor/internal/asset"
	previewdomain "task-processor/internal/listing/preview"
	common "task-processor/internal/publishing/common"
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

func buildPlatformAssetRenderPreviewGroup(platform string, previewByAssetID map[string]AssetRenderPreview, assetURLByID map[string]string, assetBundle *asset.Bundle, bundle *common.PublishImageBundle) (PlatformAssetRenderPreviews, bool) {
	if bundle == nil {
		return PlatformAssetRenderPreviews{}, false
	}
	group := PlatformAssetRenderPreviews{Platform: platform}
	group.Main = buildAssetRenderPreviewSlot("main", bundle.Main, previewByAssetID, assetURLByID, assetBundle)
	group.Gallery = buildAssetRenderPreviewSlots("gallery", bundle.Gallery, previewByAssetID, assetURLByID, assetBundle)
	group.Auxiliary = buildAssetRenderPreviewSlots("auxiliary", bundle.Auxiliary, previewByAssetID, assetURLByID, assetBundle)
	if group.Main == nil && len(group.Gallery) == 0 && len(group.Auxiliary) == 0 {
		return PlatformAssetRenderPreviews{}, false
	}
	group.Summary = buildPlatformAssetRenderPreviewSummary(group)
	return group, true
}

func buildAssetRenderPreviewSlots(defaultSlot string, slots []common.BundleSlot, previewByAssetID map[string]AssetRenderPreview, assetURLByID map[string]string, assetBundle *asset.Bundle) []AssetRenderPreviewSlot {
	if len(slots) == 0 {
		return nil
	}
	out := make([]AssetRenderPreviewSlot, 0, len(slots))
	for _, slot := range slots {
		built := buildAssetRenderPreviewSlot(defaultSlot, &slot, previewByAssetID, assetURLByID, assetBundle)
		if built != nil {
			out = append(out, *built)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildAssetRenderPreviewSlot(defaultSlot string, slot *common.BundleSlot, previewByAssetID map[string]AssetRenderPreview, assetURLByID map[string]string, assetBundle *asset.Bundle) *AssetRenderPreviewSlot {
	if slot == nil || slot.AssetID == "" {
		return nil
	}
	preview, hasPreview := previewByAssetID[slot.AssetID]
	assetURL := ""
	if assetURLByID != nil {
		assetURL = assetURLByID[slot.AssetID]
		if assetURL == "" {
			assetURL = assetURLByID[strings.TrimSpace(slot.URL)]
		}
	}
	resolvedPublishedURL := publishedAssetURLForBundleSlot(slot, assetURLByID, assetBundle)
	if resolvedPublishedURL != "" && shouldReplaceAssetURL(assetURL, resolvedPublishedURL) {
		assetURL = resolvedPublishedURL
	}
	if !hasPreview && assetURL == "" {
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
		AssetURL:            assetURL,
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

func buildAssetURLLookup(bundle *asset.Bundle) map[string]string {
	if bundle == nil || len(bundle.Assets) == 0 {
		return nil
	}
	lookup := make(map[string]string, len(bundle.Assets)*4)
	for _, item := range bundle.Assets {
		publishedURL := preferredAssetURL(item)
		if publishedURL == "" {
			continue
		}
		indexAssetURLLookup(lookup, item.ID, publishedURL)
		indexAssetURLLookup(lookup, item.URL, publishedURL)
		if item.Metadata != nil {
			indexAssetURLLookup(lookup, item.Metadata["published_url"], publishedURL)
			indexAssetURLLookup(lookup, item.Metadata["published_path"], publishedURL)
			indexAssetURLLookup(lookup, item.Metadata["local_path"], publishedURL)
		}
	}
	if len(lookup) == 0 {
		return nil
	}
	return lookup
}

func indexAssetURLLookup(lookup map[string]string, key, value string) {
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if key == "" || value == "" {
		return
	}
	if existing := strings.TrimSpace(lookup[key]); existing != "" && !shouldReplaceAssetURL(existing, value) {
		return
	}
	lookup[key] = value
}

func publishedAssetURLForBundleSlot(slot *common.BundleSlot, assetURLByID map[string]string, assetBundle *asset.Bundle) string {
	if slot == nil || len(assetURLByID) == 0 {
		if slot == nil {
			return ""
		}
	}
	fallbackURL := ""
	candidates := []string{slot.URL, slot.RecipeID}
	for _, candidate := range candidates {
		if url := assetURLByID[strings.TrimSpace(candidate)]; url != "" {
			if isPublishedAssetURL(url) {
				return url
			}
			if fallbackURL == "" {
				fallbackURL = url
			}
		}
	}
	if assetBundle == nil {
		return fallbackURL
	}
	for _, item := range assetBundle.Assets {
		url := preferredAssetURL(item)
		if !isPublishedAssetURL(url) {
			continue
		}
		if matchesPublishedBundleAssetForSlot(item, slot) {
			return url
		}
	}
	return fallbackURL
}

func matchesPublishedBundleAssetForSlot(item asset.Asset, slot *common.BundleSlot) bool {
	if slot == nil {
		return false
	}
	if strings.TrimSpace(item.RecipeID) != "" && strings.EqualFold(strings.TrimSpace(item.RecipeID), strings.TrimSpace(slot.RecipeID)) {
		return true
	}
	if !shareSourceAssetID(item.SourceAssetIDs, slot.SourceAssetIDs) {
		return false
	}
	if hasLabel(item.Labels, slot.Key) || strings.EqualFold(strings.TrimSpace(item.Role), strings.TrimSpace(slot.Purpose)) || strings.EqualFold(strings.TrimSpace(item.Role), strings.TrimSpace(slot.Key)) {
		return true
	}
	return false
}

func shareSourceAssetID(left, right []string) bool {
	if len(left) == 0 || len(right) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(left))
	for _, item := range left {
		key := strings.TrimSpace(strings.ToLower(item))
		if key == "" {
			continue
		}
		seen[key] = struct{}{}
	}
	for _, item := range right {
		key := strings.TrimSpace(strings.ToLower(item))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			return true
		}
	}
	return false
}

func hasLabel(labels []string, target string) bool {
	target = strings.TrimSpace(strings.ToLower(target))
	if target == "" {
		return false
	}
	for _, label := range labels {
		if strings.TrimSpace(strings.ToLower(label)) == target {
			return true
		}
	}
	return false
}

func preferredAssetURL(item asset.Asset) string {
	if item.Metadata != nil {
		if published := strings.TrimSpace(item.Metadata["published_url"]); published != "" {
			return published
		}
	}
	return strings.TrimSpace(item.URL)
}

func shouldReplaceAssetURL(existing, candidate string) bool {
	existingRemote := isPublishedAssetURL(existing)
	candidateRemote := isPublishedAssetURL(candidate)
	switch {
	case existingRemote && !candidateRemote:
		return false
	case !existingRemote && candidateRemote:
		return true
	default:
		return existing == ""
	}
}

func isPublishedAssetURL(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
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
