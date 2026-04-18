package listingkit

import "task-processor/internal/asset"

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
