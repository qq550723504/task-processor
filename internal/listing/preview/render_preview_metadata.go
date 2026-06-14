package preview

import "encoding/json"

type RenderPreviewMetadataSummary struct {
	VisualMode          string
	LayoutEngine        string
	RenderOutputVersion string
	DrawOutputVersion   string
	DrawPreviewVersion  string
	LayerTypes          []string
	Regions             []string
	StyleTokens         []string
}

type renderOutputMetadata struct {
	LayoutEngine string `json:"layout_engine,omitempty"`
	VisualMode   string `json:"visual_mode,omitempty"`
	Layers       []struct {
		LayerType  string `json:"layer_type,omitempty"`
		Region     string `json:"region,omitempty"`
		StyleToken string `json:"style_token,omitempty"`
	} `json:"layers,omitempty"`
}

// SummarizeRenderPreviewMetadata extracts preview-facing render metadata from
// renderer output stored on an asset.
func SummarizeRenderPreviewMetadata(metadata map[string]string) RenderPreviewMetadataSummary {
	if len(metadata) == 0 {
		return RenderPreviewMetadataSummary{}
	}
	summary := RenderPreviewMetadataSummary{
		RenderOutputVersion: metadata["render_output_version"],
		DrawOutputVersion:   metadata["draw_output_version"],
		DrawPreviewVersion:  metadata["draw_preview_version"],
	}
	raw := metadata["layout_render_output"]
	if raw == "" {
		return summary
	}
	var output renderOutputMetadata
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
