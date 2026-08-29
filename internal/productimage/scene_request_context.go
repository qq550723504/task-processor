package productimage

import "strings"

func applySceneOptionsToProductContext(context *ProductContext, req *ImageProcessRequest) *ProductContext {
	return ApplySceneOptionsToProductContext(context, req)
}

// ApplySceneOptionsToProductContext adds the typed scene options to the
// renderer context. It is exported for callers that execute one ProductImage
// capability directly instead of the compatibility pipeline.
func ApplySceneOptionsToProductContext(context *ProductContext, req *ImageProcessRequest) *ProductContext {
	if req == nil {
		return context
	}
	if context == nil {
		context = &ProductContext{}
	}
	if context.Attributes == nil {
		context.Attributes = map[string]string{}
	}
	setSceneContextAttribute(context.Attributes, "marketplace", req.TargetPlatform)
	if req.Scene == nil || req.Scene.IsEmpty() {
		return context
	}
	setSceneContextAttribute(context.Attributes, "scene_category", req.Scene.SceneCategory)
	setSceneContextAttribute(context.Attributes, "scene_style", req.Scene.SceneStyle)
	setSceneContextAttribute(context.Attributes, "background_tone", req.Scene.BackgroundTone)
	setSceneContextAttribute(context.Attributes, "composition", req.Scene.Composition)
	setSceneContextAttribute(context.Attributes, "props_level", req.Scene.PropsLevel)
	setSceneContextAttribute(context.Attributes, "audience_hint", req.Scene.AudienceHint)
	setSceneContextAttribute(context.Attributes, "custom_scene_hint", req.Scene.CustomSceneHint)
	setSceneContextAttribute(context.Attributes, "slot_role", req.Scene.SlotRole)
	setSceneContextAttribute(context.Attributes, "slot_brief", req.Scene.SlotBrief)
	setSceneContextAttribute(context.Attributes, "style_reference_ids", strings.Join(normalizedSceneStyleReferenceIDs(req.Scene.StyleReferenceIDs), ","))
	return context
}

func setSceneContextAttribute(attrs map[string]string, key, value string) {
	if attrs == nil {
		return
	}
	if strings.TrimSpace(value) == "" {
		return
	}
	attrs[key] = strings.TrimSpace(value)
}
