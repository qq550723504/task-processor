package productimage

import "encoding/json"

type sellingPointRenderBlock struct {
	ID          string `json:"id,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Slot        string `json:"slot,omitempty"`
	Text        string `json:"text,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	SourceKey   string `json:"source_key,omitempty"`
	SourceType  string `json:"source_type,omitempty"`
	Priority    int    `json:"priority,omitempty"`
}

func buildSellingPointRenderBlocks(input *sellingPointFillInput) []sellingPointRenderBlock {
	if input == nil || input.Content == nil {
		return nil
	}
	blocks := make([]sellingPointRenderBlock, 0, len(input.Content.Copy)+len(input.Content.Badges)+len(input.Content.Measurements)+len(input.Content.DetailAnchors))
	blocks = appendSellingPointRenderBlocks(blocks, "copy", input.Content.Copy)
	blocks = appendSellingPointRenderBlocks(blocks, "badge", input.Content.Badges)
	blocks = appendSellingPointRenderBlocks(blocks, "measurement", input.Content.Measurements)
	blocks = appendSellingPointRenderBlocks(blocks, "detail_anchor", input.Content.DetailAnchors)
	return blocks
}

func appendSellingPointRenderBlocks(dst []sellingPointRenderBlock, kind string, entries []sellingPointContentEntry) []sellingPointRenderBlock {
	for idx, entry := range entries {
		if entry.Text == "" || entry.Slot == "" {
			continue
		}
		dst = append(dst, sellingPointRenderBlock{
			ID:          kind + ":" + entry.Slot,
			Kind:        kind,
			Slot:        entry.Slot,
			Text:        entry.Text,
			ContentType: entry.ContentType,
			SourceKey:   entry.SourceKey,
			SourceType:  entry.SourceType,
			Priority:    idx + 1,
		})
	}
	return dst
}

func applySellingPointRenderBlocksMetadata(metadata map[string]string, profile sceneProfile, productContext *ProductContext) {
	if metadata == nil {
		return
	}
	input := buildSellingPointFillInput(profile, productContext)
	if input == nil {
		return
	}
	blocks := buildSellingPointRenderBlocks(input)
	if len(blocks) == 0 {
		return
	}
	data, err := json.Marshal(blocks)
	if err != nil {
		return
	}
	setMetadataDefault(metadata, "layout_render_blocks", string(data))
	setMetadataDefault(metadata, "render_block_plan_version", "v1")
}

func ApplySellingPointRenderBlocksMetadata(metadata map[string]string, profileName string, productContext *ProductContext) map[string]string {
	if metadata == nil {
		metadata = map[string]string{}
	}
	profile := defaultSceneProfile(profileName)
	if registry, err := loadRendererPresetRegistry(); err == nil && registry != nil {
		profile = registry.Resolve(profileName)
	}
	applySellingPointRenderBlocksMetadata(metadata, profile, productContext)
	return metadata
}
