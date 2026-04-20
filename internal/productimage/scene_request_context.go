package productimage

import "strings"

func applySceneOptionsToProductContext(context *ProductContext, req *ImageProcessRequest) *ProductContext {
	if req == nil {
		return context
	}
	if context == nil {
		context = &ProductContext{}
	}
	if context.Attributes == nil {
		context.Attributes = map[string]string{}
	}
	setSceneContextAttribute(context.Attributes, "marketplace", req.Marketplace)
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
