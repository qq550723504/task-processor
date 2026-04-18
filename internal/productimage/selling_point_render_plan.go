package productimage

import "encoding/json"

type sellingPointRenderPlanItem struct {
	BlockID      string `json:"block_id,omitempty"`
	Kind         string `json:"kind,omitempty"`
	Slot         string `json:"slot,omitempty"`
	Region       string `json:"region,omitempty"`
	VisualRole   string `json:"visual_role,omitempty"`
	RenderOrder  int    `json:"render_order,omitempty"`
	Priority     int    `json:"priority,omitempty"`
}

type sellingPointRenderPlan struct {
	LayoutVariant string                       `json:"layout_variant,omitempty"`
	VisualMode    string                       `json:"visual_mode,omitempty"`
	Items         []sellingPointRenderPlanItem `json:"items,omitempty"`
}

func buildSellingPointRenderPlan(input *sellingPointFillInput, blocks []sellingPointRenderBlock) *sellingPointRenderPlan {
	if input == nil || len(blocks) == 0 {
		return nil
	}
	items := make([]sellingPointRenderPlanItem, 0, len(blocks))
	for idx, block := range blocks {
		items = append(items, sellingPointRenderPlanItem{
			BlockID:     block.ID,
			Kind:        block.Kind,
			Slot:        block.Slot,
			Region:      sellingPointRegionForBlock(input, block),
			VisualRole:  sellingPointVisualRoleForBlock(block),
			RenderOrder: sellingPointRenderOrder(block, idx),
			Priority:    block.Priority,
		})
	}
	return &sellingPointRenderPlan{
		LayoutVariant: input.LayoutVariant,
		VisualMode:    input.VisualMode,
		Items:         items,
	}
}

func sellingPointRegionForBlock(input *sellingPointFillInput, block sellingPointRenderBlock) string {
	switch block.Kind {
	case "badge":
		return "top_band"
	case "copy":
		if input != nil && (input.LayoutVariant == "selling_point_stack" || input.LayoutVariant == "selling_point_focus") {
			return "right_panel"
		}
		return "headline_panel"
	case "measurement":
		return "bottom_band"
	case "detail_anchor":
		return "side_panel"
	default:
		return "content_panel"
	}
}

func sellingPointVisualRoleForBlock(block sellingPointRenderBlock) string {
	switch block.Kind {
	case "badge":
		return "attention"
	case "copy":
		return "message"
	case "measurement":
		return "specification"
	case "detail_anchor":
		return "detail_callout"
	default:
		return "content"
	}
}

func sellingPointRenderOrder(block sellingPointRenderBlock, idx int) int {
	base := idx + 1
	switch block.Kind {
	case "badge":
		return 10 + base
	case "copy":
		return 20 + base
	case "measurement":
		return 30 + base
	case "detail_anchor":
		return 40 + base
	default:
		return 50 + base
	}
}

func applySellingPointRenderPlanMetadata(metadata map[string]string, profile sceneProfile, productContext *ProductContext) {
	if metadata == nil {
		return
	}
	input := buildSellingPointFillInput(profile, productContext)
	if input == nil {
		return
	}
	blocks := buildSellingPointRenderBlocks(input)
	plan := buildSellingPointRenderPlan(input, blocks)
	if plan == nil {
		return
	}
	data, err := json.Marshal(plan)
	if err != nil {
		return
	}
	setMetadataDefault(metadata, "layout_render_plan", string(data))
	setMetadataDefault(metadata, "render_plan_version", "v1")
}

func ApplySellingPointRenderPlanMetadata(metadata map[string]string, profileName string, productContext *ProductContext) map[string]string {
	if metadata == nil {
		metadata = map[string]string{}
	}
	profile := defaultSceneProfile(profileName)
	if registry, err := loadRendererPresetRegistry(); err == nil && registry != nil {
		profile = registry.Resolve(profileName)
	}
	applySellingPointRenderPlanMetadata(metadata, profile, productContext)
	return metadata
}
