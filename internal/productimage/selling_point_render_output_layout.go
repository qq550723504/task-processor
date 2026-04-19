package productimage

type sellingPointRenderBounds struct {
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

func sellingPointLayerBounds(layoutVariant, region string) *sellingPointRenderBounds {
	switch region {
	case "full_canvas":
		return &sellingPointRenderBounds{X: 0, Y: 0, Width: 1, Height: 1}
	case "content_frame":
		return &sellingPointRenderBounds{X: 0.08, Y: 0.08, Width: 0.84, Height: 0.84}
	case "product_focus":
		if layoutVariant == "info_card_right" || layoutVariant == "selling_point_focus" {
			return &sellingPointRenderBounds{X: 0.10, Y: 0.16, Width: 0.42, Height: 0.66}
		}
		return &sellingPointRenderBounds{X: 0.14, Y: 0.18, Width: 0.48, Height: 0.60}
	case "top_band":
		return &sellingPointRenderBounds{X: 0.12, Y: 0.08, Width: 0.72, Height: 0.10}
	case "headline_panel":
		return &sellingPointRenderBounds{X: 0.57, Y: 0.20, Width: 0.25, Height: 0.22}
	case "right_panel":
		return &sellingPointRenderBounds{X: 0.56, Y: 0.20, Width: 0.26, Height: 0.34}
	case "bottom_band":
		return &sellingPointRenderBounds{X: 0.18, Y: 0.78, Width: 0.60, Height: 0.10}
	case "side_panel":
		return &sellingPointRenderBounds{X: 0.82, Y: 0.22, Width: 0.10, Height: 0.40}
	default:
		return &sellingPointRenderBounds{X: 0.14, Y: 0.14, Width: 0.72, Height: 0.72}
	}
}

func sellingPointLayerAlignment(region, kind, contentType string) string {
	switch region {
	case "top_band":
		return "top-left"
	case "bottom_band":
		return "bottom-center"
	case "side_panel":
		return "right-center"
	}
	switch kind {
	case "badge":
		return "top-left"
	case "measurement":
		return "bottom-center"
	case "detail_anchor":
		return "right-center"
	case "copy":
		if contentType == "headline" {
			return "top-left"
		}
		return "left-stack"
	default:
		return "center"
	}
}

func sellingPointLayerType(kind string) string {
	switch kind {
	case "copy":
		return "text"
	case "badge":
		return "badge"
	case "measurement":
		return "spec"
	case "detail_anchor":
		return "detail"
	default:
		return kind
	}
}

func sellingPointStyleToken(profile sceneProfile, kind, contentType string) string {
	switch kind {
	case "background":
		return "background:" + profile.backgroundTemplate
	case "card":
		return "overlay:" + profile.overlayTemplate
	case "subject":
		return "subject:" + profile.layoutVariant
	case "badge":
		return "badge:" + profile.visualMode
	case "measurement":
		return "spec:" + profile.measurementMode
	case "detail_anchor":
		return "detail:" + profile.detailAnchorMode
	case "copy":
		if contentType == "headline" {
			return "text:headline"
		}
		return "text:supporting"
	default:
		return "content:default"
	}
}

func sellingPointTextStyle(contentType string) string {
	switch contentType {
	case "headline":
		return "headline-strong"
	case "supporting_copy":
		return "body-compact"
	default:
		return ""
	}
}

func sellingPointBadgeStyle(profile sceneProfile) string {
	if profile.visualMode == "selling_point" {
		return "pill-emphasis"
	}
	return "pill-neutral"
}

