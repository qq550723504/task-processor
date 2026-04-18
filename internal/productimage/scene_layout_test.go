package productimage

import (
	"image"
	"testing"
)

func TestBuildSceneLayoutMetricsEditorialVsSpec(t *testing.T) {
	subject := image.NewNRGBA(image.Rect(0, 0, 800, 800))
	editorial := sceneProfile{
		group:            "editorial/model",
		layoutVariant:    "hero_center",
		maxCopyLines:     2,
		maxBadges:        1,
		measurementMode:  "single_axis",
		detailAnchorMode: "single_anchor",
	}
	spec := sceneProfile{
		group:            "selling_point/size/spec/detail",
		layoutVariant:    "right_info_panel",
		maxCopyLines:     4,
		maxBadges:        3,
		measurementMode:  "dual_axis",
		detailAnchorMode: "dual_anchor",
	}

	editorialLayout := buildSceneLayoutMetrics(editorial, 1600, subject)
	specLayout := buildSceneLayoutMetrics(spec, 1600, subject)

	if specLayout.cardWidth <= editorialLayout.cardWidth {
		t.Fatalf("expected spec layout card width to reserve more panel space, got spec=%d editorial=%d", specLayout.cardWidth, editorialLayout.cardWidth)
	}
	if specLayout.subjectPoint.X >= editorialLayout.subjectPoint.X {
		t.Fatalf("expected spec layout subject to shift left for info panel, got spec=%d editorial=%d", specLayout.subjectPoint.X, editorialLayout.subjectPoint.X)
	}
	if specLayout.cardOpacity <= editorialLayout.cardOpacity {
		t.Fatalf("expected spec layout to use denser card opacity, got spec=%f editorial=%f", specLayout.cardOpacity, editorialLayout.cardOpacity)
	}
}

func TestBuildSceneLayoutMetricsDualAxisAddsBottomReserve(t *testing.T) {
	subject := image.NewNRGBA(image.Rect(0, 0, 700, 900))
	singleAxis := sceneProfile{
		group:            "selling_point/size/spec/detail",
		layoutVariant:    "spec_sheet",
		maxCopyLines:     2,
		maxBadges:        1,
		measurementMode:  "single_axis",
		detailAnchorMode: "single_anchor",
	}
	dualAxis := singleAxis
	dualAxis.measurementMode = "dual_axis"

	singleLayout := buildSceneLayoutMetrics(singleAxis, 1600, subject)
	dualLayout := buildSceneLayoutMetrics(dualAxis, 1600, subject)

	if dualLayout.subjectPoint.Y >= singleLayout.subjectPoint.Y {
		t.Fatalf("expected dual-axis layout to move subject upward, got dual=%d single=%d", dualLayout.subjectPoint.Y, singleLayout.subjectPoint.Y)
	}
}

func TestBuildSceneLayoutMetricsUsesSellingPointBranch(t *testing.T) {
	subject := image.NewNRGBA(image.Rect(0, 0, 720, 720))
	sceneCfg := sceneProfile{
		group:            "lifestyle/scene",
		layoutVariant:    "hero_center",
		visualMode:       "scene",
		maxCopyLines:     2,
		maxBadges:        1,
		measurementMode:  "single_axis",
		detailAnchorMode: "single_anchor",
	}
	sellingPointProfile := sceneProfile{
		group:            "selling_point/size/spec/detail",
		layoutVariant:    "selling_point_grid",
		visualMode:       "selling_point",
		maxCopyLines:     4,
		maxBadges:        3,
		measurementMode:  "dual_axis",
		detailAnchorMode: "dual_anchor",
	}

	sceneLayout := buildSceneLayoutMetrics(sceneCfg, 1600, subject)
	sellingPointLayout := buildSceneLayoutMetrics(sellingPointProfile, 1600, subject)

	if sellingPointLayout.layoutEngine != "selling_point_layout_v1" {
		t.Fatalf("selling-point layout engine = %q, want selling_point_layout_v1", sellingPointLayout.layoutEngine)
	}
	if sceneLayout.layoutEngine != "preset_layout_v1" {
		t.Fatalf("scene layout engine = %q, want preset_layout_v1", sceneLayout.layoutEngine)
	}
	if sellingPointLayout.cardPoint.X >= sceneLayout.cardPoint.X {
		t.Fatalf("expected selling-point card to shift left for copy panel, got selling=%d scene=%d", sellingPointLayout.cardPoint.X, sceneLayout.cardPoint.X)
	}
	if sellingPointLayout.subjectPoint.X >= sceneLayout.subjectPoint.X {
		t.Fatalf("expected selling-point subject to move left, got selling=%d scene=%d", sellingPointLayout.subjectPoint.X, sceneLayout.subjectPoint.X)
	}
}
