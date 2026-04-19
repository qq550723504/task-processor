package productimage

import (
	"html"
	"strconv"
	"strings"
)

func buildSellingPointDrawPreviewSVG(output *sellingPointDrawOutput) string {
	if output == nil || len(output.Instructions) == 0 {
		return ""
	}

	const canvas = 1000

	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1000 1000" role="img" data-renderer="`)
	b.WriteString(html.EscapeString(output.Renderer))
	b.WriteString(`" data-visual-mode="`)
	b.WriteString(html.EscapeString(output.VisualMode))
	b.WriteString(`">`)

	for _, instruction := range output.Instructions {
		if instruction.Bounds == nil {
			continue
		}
		x := int(instruction.Bounds.X * canvas)
		y := int(instruction.Bounds.Y * canvas)
		w := int(instruction.Bounds.Width * canvas)
		h := int(instruction.Bounds.Height * canvas)

		switch instruction.LayerType {
		case "background":
			b.WriteString(`<rect data-layer="background" x="0" y="0" width="1000" height="1000" fill="#eef2f6"/>`)
		case "card":
			b.WriteString(`<rect data-layer="card" x="`)
			b.WriteString(strconv.Itoa(x))
			b.WriteString(`" y="`)
			b.WriteString(strconv.Itoa(y))
			b.WriteString(`" width="`)
			b.WriteString(strconv.Itoa(w))
			b.WriteString(`" height="`)
			b.WriteString(strconv.Itoa(h))
			b.WriteString(`" rx="24" fill="#ffffff" fill-opacity="0.93" data-style="`)
			b.WriteString(html.EscapeString(instruction.StyleToken))
			b.WriteString(`"/>`)
		case "subject":
			b.WriteString(`<rect data-layer="subject" x="`)
			b.WriteString(strconv.Itoa(x))
			b.WriteString(`" y="`)
			b.WriteString(strconv.Itoa(y))
			b.WriteString(`" width="`)
			b.WriteString(strconv.Itoa(w))
			b.WriteString(`" height="`)
			b.WriteString(strconv.Itoa(h))
			b.WriteString(`" rx="18" fill="#d7dde6" data-style="`)
			b.WriteString(html.EscapeString(instruction.StyleToken))
			b.WriteString(`"/>`)
		case "badge":
			b.WriteString(`<rect data-layer="badge" x="`)
			b.WriteString(strconv.Itoa(x))
			b.WriteString(`" y="`)
			b.WriteString(strconv.Itoa(y))
			b.WriteString(`" width="`)
			b.WriteString(strconv.Itoa(w))
			b.WriteString(`" height="`)
			b.WriteString(strconv.Itoa(h))
			b.WriteString(`" rx="18" fill="#111827"/>`)
			b.WriteString(`<text data-layer="badge-text" x="`)
			b.WriteString(strconv.Itoa(x + 16))
			b.WriteString(`" y="`)
			b.WriteString(strconv.Itoa(y + minInt(h/2, 28)))
			b.WriteString(`" font-size="24" fill="#ffffff">`)
			b.WriteString(html.EscapeString(instruction.Text))
			b.WriteString(`</text>`)
		case "text":
			b.WriteString(`<text data-layer="`)
			b.WriteString(html.EscapeString(instruction.LayerType))
			b.WriteString(`" x="`)
			b.WriteString(strconv.Itoa(x))
			b.WriteString(`" y="`)
			b.WriteString(strconv.Itoa(y + 32))
			b.WriteString(`" font-size="`)
			b.WriteString(svgFontSize(instruction.TextStyle))
			b.WriteString(`" fill="#111827" data-style="`)
			b.WriteString(html.EscapeString(instruction.StyleToken))
			b.WriteString(`">`)
			b.WriteString(html.EscapeString(instruction.Text))
			b.WriteString(`</text>`)
		case "spec":
			b.WriteString(`<rect data-layer="measurement-chip" x="`)
			b.WriteString(strconv.Itoa(x))
			b.WriteString(`" y="`)
			b.WriteString(strconv.Itoa(y))
			b.WriteString(`" width="`)
			b.WriteString(strconv.Itoa(w))
			b.WriteString(`" height="`)
			b.WriteString(strconv.Itoa(h))
			b.WriteString(`" rx="16" fill="#f3f4f6" stroke="#9ca3af" data-style="`)
			b.WriteString(html.EscapeString(instruction.StyleToken))
			b.WriteString(`"/>`)
			b.WriteString(`<text data-layer="measurement-text" x="`)
			b.WriteString(strconv.Itoa(x + 16))
			b.WriteString(`" y="`)
			b.WriteString(strconv.Itoa(y + minInt(h/2, 28)))
			b.WriteString(`" font-size="22" fill="#111827">`)
			b.WriteString(html.EscapeString(instruction.Text))
			b.WriteString(`</text>`)
		case "detail":
			lineX := x - 18
			if lineX < 0 {
				lineX = x
			}
			b.WriteString(`<line data-layer="detail-callout" x1="`)
			b.WriteString(strconv.Itoa(lineX))
			b.WriteString(`" y1="`)
			b.WriteString(strconv.Itoa(y + 20))
			b.WriteString(`" x2="`)
			b.WriteString(strconv.Itoa(x))
			b.WriteString(`" y2="`)
			b.WriteString(strconv.Itoa(y + 20))
			b.WriteString(`" stroke="#111827" stroke-width="2"/>`)
			b.WriteString(`<text data-layer="detail-callout" x="`)
			b.WriteString(strconv.Itoa(x))
			b.WriteString(`" y="`)
			b.WriteString(strconv.Itoa(y + 32))
			b.WriteString(`" font-size="22" fill="#111827" data-style="`)
			b.WriteString(html.EscapeString(instruction.StyleToken))
			b.WriteString(`">`)
			b.WriteString(html.EscapeString(instruction.Text))
			b.WriteString(`</text>`)
		}
	}

	b.WriteString(`</svg>`)
	return b.String()
}

func svgFontSize(textStyle string) string {
	switch textStyle {
	case "headline-strong":
		return "34"
	case "body-compact":
		return "24"
	default:
		return "22"
	}
}

func applySellingPointDrawPreviewMetadata(metadata map[string]string, profile sceneProfile, productContext *ProductContext) {
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
	svg := buildSellingPointDrawPreviewSVG(drawOutput)
	if strings.TrimSpace(svg) == "" {
		return
	}
	setMetadataDefault(metadata, "layout_draw_preview_svg", svg)
	setMetadataDefault(metadata, "draw_preview_version", "v1")
	setMetadataDefault(metadata, "draw_preview_format", "svg")
}

func ApplySellingPointDrawPreviewMetadata(metadata map[string]string, profileName string, productContext *ProductContext) map[string]string {
	if metadata == nil {
		metadata = map[string]string{}
	}
	profile := defaultSceneProfile(profileName)
	if registry, err := loadRendererPresetRegistry(); err == nil && registry != nil {
		profile = registry.Resolve(profileName)
	}
	applySellingPointDrawPreviewMetadata(metadata, profile, productContext)
	return metadata
}
