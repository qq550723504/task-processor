package listingkit

import (
	"encoding/json"

	"task-processor/internal/asset"
)

type AssetRenderPreview struct {
	AssetID             string     `json:"asset_id,omitempty"`
	AssetRevision       string     `json:"asset_revision,omitempty"`
	PreviewRevision     string     `json:"preview_revision,omitempty"`
	TaskRevision        string     `json:"task_revision,omitempty"`
	Kind                asset.Kind `json:"kind,omitempty"`
	Role                string     `json:"role,omitempty"`
	RenderProfile       string     `json:"render_profile,omitempty"`
	TemplateLabel       string     `json:"template_label,omitempty"`
	PreviewFormat       string     `json:"preview_format,omitempty"`
	PreviewSVG          string     `json:"preview_svg,omitempty"`
	SourceKind          string     `json:"source_kind,omitempty"`
	GenerationMode      string     `json:"generation_mode,omitempty"`
	VisualMode          string     `json:"visual_mode,omitempty"`
	LayoutEngine        string     `json:"layout_engine,omitempty"`
	RenderOutputVersion string     `json:"render_output_version,omitempty"`
	DrawOutputVersion   string     `json:"draw_output_version,omitempty"`
	DrawPreviewVersion  string     `json:"draw_preview_version,omitempty"`
	LayerTypes          []string   `json:"layer_types,omitempty"`
	Regions             []string   `json:"regions,omitempty"`
	StyleTokens         []string   `json:"style_tokens,omitempty"`
}

func buildAssetRenderPreviews(bundle *asset.Bundle) []AssetRenderPreview {
	if bundle == nil || len(bundle.Assets) == 0 {
		return nil
	}
	out := make([]AssetRenderPreview, 0, len(bundle.Assets))
	for _, item := range bundle.Assets {
		if item.Metadata == nil || item.Metadata["layout_draw_preview_svg"] == "" {
			continue
		}
		summary := buildAssetRenderPreviewSummary(item.Metadata)
		out = append(out, AssetRenderPreview{
			AssetID:             item.ID,
			AssetRevision:       buildAssetRenderAssetRevision(item),
			PreviewRevision:     buildAssetRenderPreviewRevision(item),
			Kind:                item.Kind,
			Role:                item.Role,
			RenderProfile:       item.Metadata["render_profile"],
			TemplateLabel:       item.Metadata["template_label"],
			PreviewFormat:       item.Metadata["draw_preview_format"],
			PreviewSVG:          item.Metadata["layout_draw_preview_svg"],
			SourceKind:          item.Metadata["source_kind"],
			GenerationMode:      item.Metadata["execution_mode"],
			VisualMode:          summary.VisualMode,
			LayoutEngine:        summary.LayoutEngine,
			RenderOutputVersion: summary.RenderOutputVersion,
			DrawOutputVersion:   summary.DrawOutputVersion,
			DrawPreviewVersion:  summary.DrawPreviewVersion,
			LayerTypes:          summary.LayerTypes,
			Regions:             summary.Regions,
			StyleTokens:         summary.StyleTokens,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func attachTaskRevisionToAssetRenderPreviews(previews []AssetRenderPreview, taskRevision string) []AssetRenderPreview {
	if len(previews) == 0 {
		return nil
	}
	out := make([]AssetRenderPreview, len(previews))
	copy(out, previews)
	for i := range out {
		out[i].TaskRevision = taskRevision
	}
	return out
}

type assetRenderOutputSummary struct {
	LayoutEngine string `json:"layout_engine,omitempty"`
	VisualMode   string `json:"visual_mode,omitempty"`
	Layers       []struct {
		LayerType  string `json:"layer_type,omitempty"`
		Region     string `json:"region,omitempty"`
		StyleToken string `json:"style_token,omitempty"`
	} `json:"layers,omitempty"`
}

type assetRenderPreviewSummary struct {
	VisualMode          string
	LayoutEngine        string
	RenderOutputVersion string
	DrawOutputVersion   string
	DrawPreviewVersion  string
	LayerTypes          []string
	Regions             []string
	StyleTokens         []string
}

func buildAssetRenderPreviewSummary(metadata map[string]string) assetRenderPreviewSummary {
	if len(metadata) == 0 {
		return assetRenderPreviewSummary{}
	}
	summary := assetRenderPreviewSummary{
		RenderOutputVersion: metadata["render_output_version"],
		DrawOutputVersion:   metadata["draw_output_version"],
		DrawPreviewVersion:  metadata["draw_preview_version"],
	}
	raw := metadata["layout_render_output"]
	if raw == "" {
		return summary
	}
	var output assetRenderOutputSummary
	if err := json.Unmarshal([]byte(raw), &output); err != nil {
		return summary
	}
	summary.VisualMode = output.VisualMode
	summary.LayoutEngine = output.LayoutEngine
	summary.LayerTypes = uniqueNonEmptyStringsFrom(output.Layers, func(layer struct {
		LayerType  string `json:"layer_type,omitempty"`
		Region     string `json:"region,omitempty"`
		StyleToken string `json:"style_token,omitempty"`
	}) string {
		return layer.LayerType
	})
	summary.Regions = uniqueNonEmptyStringsFrom(output.Layers, func(layer struct {
		LayerType  string `json:"layer_type,omitempty"`
		Region     string `json:"region,omitempty"`
		StyleToken string `json:"style_token,omitempty"`
	}) string {
		return layer.Region
	})
	summary.StyleTokens = uniqueNonEmptyStringsFrom(output.Layers, func(layer struct {
		LayerType  string `json:"layer_type,omitempty"`
		Region     string `json:"region,omitempty"`
		StyleToken string `json:"style_token,omitempty"`
	}) string {
		return layer.StyleToken
	})
	return summary
}

func uniqueNonEmptyStringsFrom[T any](items []T, pick func(T) string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		value := pick(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
