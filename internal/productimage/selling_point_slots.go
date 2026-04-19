package productimage

import "strings"

type sellingPointSlotPlan struct {
	CopySlots        []string
	BadgeSlots       []string
	MeasurementSlots []string
	DetailAnchors    []string
	MaxCopyLines     int
	MaxBadges        int
	MeasurementMode  string
	DetailAnchorMode string
}

func buildSellingPointSlotPlan(profile sceneProfile) *sellingPointSlotPlan {
	if strings.TrimSpace(profile.visualMode) != "selling_point" {
		return nil
	}
	return &sellingPointSlotPlan{
		CopySlots:        append([]string(nil), profile.copySlots...),
		BadgeSlots:       append([]string(nil), profile.badgeSlots...),
		MeasurementSlots: append([]string(nil), profile.measurementSlots...),
		DetailAnchors:    append([]string(nil), profile.detailAnchorSlots...),
		MaxCopyLines:     profile.maxCopyLines,
		MaxBadges:        profile.maxBadges,
		MeasurementMode:  profile.measurementMode,
		DetailAnchorMode: profile.detailAnchorMode,
	}
}
