package productimage

import "image"

func buildSellingPointLayoutMetrics(profile sceneProfile, canvasSize int, subject image.Image) sceneLayoutMetrics {
	subjectBounds := subject.Bounds()
	subjectWidth := subjectBounds.Dx()
	subjectHeight := subjectBounds.Dy()

	baseReserve := maxInt(canvasSize/18, 56)
	copyReserve := maxInt(profile.maxCopyLines, 1) * canvasSize / 18
	badgeReserve := maxInt(profile.maxBadges, 1) * canvasSize / 28

	leftReserve := baseReserve + canvasSize/16
	rightReserve := baseReserve + copyReserve
	topReserve := baseReserve + badgeReserve
	bottomReserve := baseReserve

	switch profile.measurementMode {
	case "dual_axis":
		bottomReserve += canvasSize / 10
	case "callout":
		bottomReserve += canvasSize / 14
	}
	switch profile.detailAnchorMode {
	case "dual_anchor":
		leftReserve += canvasSize / 20
		rightReserve += canvasSize / 20
	case "side_stack":
		rightReserve += canvasSize / 16
	}
	switch profile.layoutVariant {
	case "selling_point_grid":
		rightReserve += canvasSize / 18
	case "selling_point_stack":
		topReserve += canvasSize / 18
	case "selling_point_focus":
		leftReserve += canvasSize / 12
	}

	cardWidth := minInt(canvasSize-(leftReserve+rightReserve), subjectWidth+leftReserve+rightReserve)
	cardHeight := minInt(canvasSize-(topReserve+bottomReserve), subjectHeight+topReserve+bottomReserve)
	cardWidth = maxInt(cardWidth, subjectWidth+baseReserve+canvasSize/10)
	cardHeight = maxInt(cardHeight, subjectHeight+baseReserve+canvasSize/12)
	cardWidth = minInt(cardWidth, canvasSize-baseReserve)
	cardHeight = minInt(cardHeight, canvasSize-baseReserve)

	cardX := clampInt(canvasSize/14, baseReserve/2, canvasSize-cardWidth-baseReserve/2)
	cardY := clampInt((canvasSize-cardHeight)/2, baseReserve/2, canvasSize-cardHeight-baseReserve/2)

	subjectX := clampInt(cardX+leftReserve/2, cardX+baseReserve/3, cardX+cardWidth-subjectWidth-baseReserve/2)
	subjectY := clampInt(cardY+topReserve+(cardHeight-topReserve-bottomReserve-subjectHeight)/2, cardY+baseReserve/3, cardY+cardHeight-subjectHeight-baseReserve/2)

	return sceneLayoutMetrics{
		cardWidth:             cardWidth,
		cardHeight:            cardHeight,
		cardPoint:             image.Pt(cardX, cardY),
		subjectPoint:          image.Pt(subjectX, subjectY),
		cardOpacity:           0.93,
		layoutEngine:          "selling_point_layout_v1",
		qualityGradeCandidate: "ideal",
	}
}
