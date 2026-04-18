package productimage

import "image"

type sceneLayoutMetrics struct {
	cardWidth             int
	cardHeight            int
	cardPoint             image.Point
	subjectPoint          image.Point
	cardOpacity           float64
	layoutEngine          string
	qualityGradeCandidate string
}

func buildSceneLayoutMetrics(profile sceneProfile, canvasSize int, subject image.Image) sceneLayoutMetrics {
	if profile.visualMode == "selling_point" {
		return buildSellingPointLayoutMetrics(profile, canvasSize, subject)
	}
	subjectBounds := subject.Bounds()
	subjectWidth := subjectBounds.Dx()
	subjectHeight := subjectBounds.Dy()
	baseReserve := maxInt(canvasSize/20, 40)
	reserveTop := baseReserve
	reserveRight := baseReserve
	reserveBottom := baseReserve
	reserveLeft := baseReserve

	if profile.maxBadges > 1 {
		reserveTop += canvasSize / 24
	}
	if profile.maxCopyLines > 2 {
		reserveRight += canvasSize / 14
	}
	if profile.measurementMode == "dual_axis" {
		reserveBottom += canvasSize / 12
	}
	if profile.detailAnchorMode == "dual_anchor" {
		reserveRight += canvasSize / 18
		reserveLeft += canvasSize / 36
	}

	switch profile.layoutVariant {
	case "right_info_panel", "spec_sheet", "detail_grid":
		reserveRight += canvasSize / 10
	case "left_focus_panel":
		reserveLeft += canvasSize / 10
	case "hero_center", "editorial_full":
		reserveTop = maxInt(reserveTop-canvasSize/36, baseReserve/2)
		reserveBottom = maxInt(reserveBottom-canvasSize/36, baseReserve/2)
	}

	cardWidth := minInt(canvasSize-(reserveLeft+reserveRight), subjectWidth+reserveLeft+reserveRight)
	cardHeight := minInt(canvasSize-(reserveTop+reserveBottom), subjectHeight+reserveTop+reserveBottom)
	cardWidth = maxInt(cardWidth, subjectWidth+baseReserve)
	cardHeight = maxInt(cardHeight, subjectHeight+baseReserve)
	cardWidth = minInt(cardWidth, canvasSize-baseReserve)
	cardHeight = minInt(cardHeight, canvasSize-baseReserve)

	cardX := (canvasSize - cardWidth) / 2
	cardY := (canvasSize - cardHeight) / 2
	if reserveRight > reserveLeft {
		cardX -= (reserveRight - reserveLeft) / 2
	}
	if reserveBottom > reserveTop {
		cardY -= (reserveBottom - reserveTop) / 2
	}
	cardX = clampInt(cardX, baseReserve/2, canvasSize-cardWidth-baseReserve/2)
	cardY = clampInt(cardY, baseReserve/2, canvasSize-cardHeight-baseReserve/2)

	subjectX := cardX + reserveLeft + (cardWidth-reserveLeft-reserveRight-subjectWidth)/2
	subjectY := cardY + reserveTop + (cardHeight-reserveTop-reserveBottom-subjectHeight)/2
	subjectX = clampInt(subjectX, cardX+baseReserve/4, cardX+cardWidth-subjectWidth-baseReserve/4)
	subjectY = clampInt(subjectY, cardY+baseReserve/4, cardY+cardHeight-subjectHeight-baseReserve/4)

	cardOpacity := 0.85
	switch profile.group {
	case "editorial/model":
		cardOpacity = 0.82
	case "lifestyle/scene":
		cardOpacity = 0.88
	case "selling_point/size/spec/detail":
		cardOpacity = 0.91
	}

	return sceneLayoutMetrics{
		cardWidth:             cardWidth,
		cardHeight:            cardHeight,
		cardPoint:             image.Pt(cardX, cardY),
		subjectPoint:          image.Pt(subjectX, subjectY),
		cardOpacity:           cardOpacity,
		layoutEngine:          "preset_layout_v1",
		qualityGradeCandidate: "ideal",
	}
}

func clampInt(value, minValue, maxValue int) int {
	if maxValue < minValue {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
