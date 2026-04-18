package productimage

import "encoding/json"

type sellingPointDrawInstruction struct {
	ID          string                    `json:"id,omitempty"`
	LayerType   string                    `json:"layer_type,omitempty"`
	Region      string                    `json:"region,omitempty"`
	VisualRole  string                    `json:"visual_role,omitempty"`
	Alignment   string                    `json:"alignment,omitempty"`
	StyleToken  string                    `json:"style_token,omitempty"`
	TextStyle   string                    `json:"text_style,omitempty"`
	BadgeStyle  string                    `json:"badge_style,omitempty"`
	RenderOrder int                       `json:"render_order,omitempty"`
	ZIndex      int                       `json:"z_index,omitempty"`
	Text        string                    `json:"text,omitempty"`
	Bounds      *sellingPointRenderBounds `json:"bounds,omitempty"`
	Opacity     float64                   `json:"opacity,omitempty"`
	Shape       string                    `json:"shape,omitempty"`
}

type sellingPointDrawOutput struct {
	Renderer     string                        `json:"renderer,omitempty"`
	VisualMode   string                        `json:"visual_mode,omitempty"`
	Instructions []sellingPointDrawInstruction `json:"instructions,omitempty"`
}

func buildSellingPointDrawOutput(output *sellingPointRenderOutput) *sellingPointDrawOutput {
	if output == nil || len(output.Layers) == 0 {
		return nil
	}
	instructions := make([]sellingPointDrawInstruction, 0, len(output.Layers))
	for _, layer := range output.Layers {
		instructions = append(instructions, sellingPointDrawInstruction{
			ID:          layer.ID,
			LayerType:   layer.LayerType,
			Region:      layer.Region,
			VisualRole:  layer.VisualRole,
			Alignment:   layer.Alignment,
			StyleToken:  layer.StyleToken,
			TextStyle:   layer.TextStyle,
			BadgeStyle:  layer.BadgeStyle,
			RenderOrder: layer.RenderOrder,
			ZIndex:      layer.ZIndex,
			Text:        layer.Text,
			Bounds:      layer.Bounds,
			Opacity:     sellingPointInstructionOpacity(layer.LayerType),
			Shape:       sellingPointInstructionShape(layer.LayerType),
		})
	}
	return &sellingPointDrawOutput{
		Renderer:     "selling_point_draw_v1",
		VisualMode:   output.VisualMode,
		Instructions: instructions,
	}
}

func sellingPointInstructionOpacity(layerType string) float64 {
	switch layerType {
	case "background":
		return 1
	case "card":
		return 0.93
	case "subject":
		return 1
	case "badge":
		return 0.98
	case "text", "spec", "detail":
		return 1
	default:
		return 1
	}
}

func sellingPointInstructionShape(layerType string) string {
	switch layerType {
	case "background":
		return "canvas"
	case "card":
		return "rounded_rect"
	case "subject":
		return "image_mask"
	case "badge":
		return "pill"
	case "text":
		return "text_block"
	case "spec":
		return "spec_block"
	case "detail":
		return "callout"
	default:
		return "content_block"
	}
}

func applySellingPointDrawOutputMetadata(metadata map[string]string, profile sceneProfile, productContext *ProductContext) {
	if metadata == nil {
		return
	}
	input := buildSellingPointFillInput(profile, productContext)
	if input == nil {
		return
	}
	plan := buildSellingPointRenderPlan(input, buildSellingPointRenderBlocks(input))
	renderOutput := buildSellingPointRenderOutput(profile, input, plan)
	drawOutput := buildSellingPointDrawOutput(renderOutput)
	if drawOutput == nil {
		return
	}
	data, err := json.Marshal(drawOutput)
	if err != nil {
		return
	}
	setMetadataDefault(metadata, "layout_draw_output", string(data))
	setMetadataDefault(metadata, "draw_output_version", "v1")
}

func ApplySellingPointDrawOutputMetadata(metadata map[string]string, profileName string, productContext *ProductContext) map[string]string {
	if metadata == nil {
		metadata = map[string]string{}
	}
	profile := defaultSceneProfile(profileName)
	if registry, err := loadRendererPresetRegistry(); err == nil && registry != nil {
		profile = registry.Resolve(profileName)
	}
	applySellingPointDrawOutputMetadata(metadata, profile, productContext)
	return metadata
}
