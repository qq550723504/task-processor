package productimage

import "encoding/json"

type sellingPointRenderOutputLayer struct {
	ID          string                    `json:"id,omitempty"`
	Layer       string                    `json:"layer,omitempty"`
	LayerType   string                    `json:"layer_type,omitempty"`
	Region      string                    `json:"region,omitempty"`
	VisualRole  string                    `json:"visual_role,omitempty"`
	Alignment   string                    `json:"alignment,omitempty"`
	StyleToken  string                    `json:"style_token,omitempty"`
	TextStyle   string                    `json:"text_style,omitempty"`
	BadgeStyle  string                    `json:"badge_style,omitempty"`
	RenderOrder int                       `json:"render_order,omitempty"`
	ZIndex      int                       `json:"z_index,omitempty"`
	SourceBlock string                    `json:"source_block,omitempty"`
	Text        string                    `json:"text,omitempty"`
	Bounds      *sellingPointRenderBounds `json:"bounds,omitempty"`
}

type sellingPointRenderOutput struct {
	LayoutEngine string                          `json:"layout_engine,omitempty"`
	VisualMode   string                          `json:"visual_mode,omitempty"`
	Layers       []sellingPointRenderOutputLayer `json:"layers,omitempty"`
}

func buildSellingPointRenderOutput(profile sceneProfile, input *sellingPointFillInput, plan *sellingPointRenderPlan) *sellingPointRenderOutput {
	if input == nil || plan == nil {
		return nil
	}
	layers := []sellingPointRenderOutputLayer{
		{
			ID:          "background",
			Layer:       "background",
			LayerType:   "background",
			Region:      "full_canvas",
			VisualRole:  "background",
			Alignment:   "center",
			StyleToken:  sellingPointStyleToken(profile, "background", ""),
			RenderOrder: 1,
			ZIndex:      1,
			Text:        profile.backgroundTemplate,
			Bounds:      sellingPointLayerBounds(input.LayoutVariant, "full_canvas"),
		},
		{
			ID:          "card",
			Layer:       "card",
			LayerType:   "card",
			Region:      "content_frame",
			VisualRole:  "card",
			Alignment:   "center",
			StyleToken:  sellingPointStyleToken(profile, "card", ""),
			RenderOrder: 2,
			ZIndex:      2,
			Text:        profile.overlayTemplate,
			Bounds:      sellingPointLayerBounds(input.LayoutVariant, "content_frame"),
		},
		{
			ID:          "subject",
			Layer:       "subject",
			LayerType:   "subject",
			Region:      "product_focus",
			VisualRole:  "subject",
			Alignment:   "center-left",
			StyleToken:  sellingPointStyleToken(profile, "subject", ""),
			RenderOrder: 3,
			ZIndex:      3,
			Text:        profile.layoutVariant,
			Bounds:      sellingPointLayerBounds(input.LayoutVariant, "product_focus"),
		},
	}
	for _, item := range plan.Items {
		contentType := sellingPointContentTypeForBlock(input, item.BlockID)
		layerType := sellingPointLayerType(item.Kind)
		layers = append(layers, sellingPointRenderOutputLayer{
			ID:          "content:" + item.BlockID,
			Layer:       "content",
			LayerType:   layerType,
			Region:      item.Region,
			VisualRole:  item.VisualRole,
			Alignment:   sellingPointLayerAlignment(item.Region, item.Kind, contentType),
			StyleToken:  sellingPointStyleToken(profile, item.Kind, contentType),
			TextStyle:   sellingPointTextStyle(contentType),
			BadgeStyle:  sellingPointBadgeStyle(profile),
			RenderOrder: 100 + item.RenderOrder,
			ZIndex:      100 + item.RenderOrder,
			SourceBlock: item.BlockID,
			Text:        sellingPointTextForBlock(input, item.BlockID),
			Bounds:      sellingPointLayerBounds(input.LayoutVariant, item.Region),
		})
	}
	return &sellingPointRenderOutput{
		LayoutEngine: "selling_point_output_v2",
		VisualMode:   input.VisualMode,
		Layers:       layers,
	}
}

func sellingPointTextForBlock(input *sellingPointFillInput, blockID string) string {
	if input == nil || input.Content == nil {
		return ""
	}
	for _, entry := range input.Content.Copy {
		if "copy:"+entry.Slot == blockID {
			return entry.Text
		}
	}
	for _, entry := range input.Content.Badges {
		if "badge:"+entry.Slot == blockID {
			return entry.Text
		}
	}
	for _, entry := range input.Content.Measurements {
		if "measurement:"+entry.Slot == blockID {
			return entry.Text
		}
	}
	for _, entry := range input.Content.DetailAnchors {
		if "detail_anchor:"+entry.Slot == blockID {
			return entry.Text
		}
	}
	return ""
}

func sellingPointContentTypeForBlock(input *sellingPointFillInput, blockID string) string {
	if input == nil || input.Content == nil {
		return ""
	}
	for _, entry := range input.Content.Copy {
		if "copy:"+entry.Slot == blockID {
			return entry.ContentType
		}
	}
	for _, entry := range input.Content.Badges {
		if "badge:"+entry.Slot == blockID {
			return entry.ContentType
		}
	}
	for _, entry := range input.Content.Measurements {
		if "measurement:"+entry.Slot == blockID {
			return entry.ContentType
		}
	}
	for _, entry := range input.Content.DetailAnchors {
		if "detail_anchor:"+entry.Slot == blockID {
			return entry.ContentType
		}
	}
	return ""
}

func applySellingPointRenderOutputMetadata(metadata map[string]string, profile sceneProfile, productContext *ProductContext) {
	if metadata == nil {
		return
	}
	input := buildSellingPointFillInput(profile, productContext)
	if input == nil {
		return
	}
	plan := buildSellingPointRenderPlan(input, buildSellingPointRenderBlocks(input))
	output := buildSellingPointRenderOutput(profile, input, plan)
	if output == nil {
		return
	}
	data, err := json.Marshal(output)
	if err != nil {
		return
	}
	setMetadataDefault(metadata, "layout_render_output", string(data))
	setMetadataDefault(metadata, "render_output_version", "v2")
}

func ApplySellingPointRenderOutputMetadata(metadata map[string]string, profileName string, productContext *ProductContext) map[string]string {
	if metadata == nil {
		metadata = map[string]string{}
	}
	profile := defaultSceneProfile(profileName)
	if registry, err := loadRendererPresetRegistry(); err == nil && registry != nil {
		profile = registry.Resolve(profileName)
	}
	applySellingPointRenderOutputMetadata(metadata, profile, productContext)
	return metadata
}
