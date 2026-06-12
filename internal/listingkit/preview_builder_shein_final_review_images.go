package listingkit

import (
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func buildSheinFinalReviewImages(draft *sheinpub.RequestDraft, finalDraft *sheinpub.FinalDraft, product *sheinproduct.Product) []SheinFinalReviewImage {
	if draft == nil || draft.ImageInfo == nil {
		return nil
	}
	sizeMapURLs := sheinproduct.CollectSizeMapImageURLs(product)
	out := []SheinFinalReviewImage{}
	seen := map[string]int{}
	add := func(url, role string, sort int, main bool) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		role, main = resolveSheinFinalReviewImageRole(url, role, main, finalDraft, sizeMapURLs)
		if existingIndex, ok := seen[url]; ok {
			mergeSheinFinalReviewImage(&out[existingIndex], role, main)
			return
		}
		seen[url] = len(out)
		out = append(out, SheinFinalReviewImage{
			URL:     url,
			Role:    role,
			Sort:    sort,
			Final:   true,
			Main:    main || role == "main",
			Swatch:  isSheinFinalReviewSwatchRole(role),
			SizeMap: role == "size_map",
		})
	}
	add(draft.ImageInfo.MainImage, "main", 1, true)
	for i, image := range draft.ImageInfo.Gallery {
		add(image, "gallery", i+2, false)
	}
	if draft.ImageInfo.WhiteBg != "" {
		add(draft.ImageInfo.WhiteBg, "white_bg", len(out)+1, false)
	}
	for _, skc := range draft.SKCList {
		if skc.ImageInfo != nil {
			add(skc.ImageInfo.MainImage, "skc", len(out)+1, false)
		}
	}
	return out
}

func resolveSheinFinalReviewImageRole(url, role string, main bool, finalDraft *sheinpub.FinalDraft, sizeMapURLs map[string]struct{}) (string, bool) {
	if finalDraft != nil {
		if override := strings.TrimSpace(finalDraft.ImageRoleOverrides[url]); override != "" {
			role = override
		}
		if strings.TrimSpace(finalDraft.MainImageURL) == url && role != "skc" && role != "swatch" && role != "size_map" {
			main = true
			role = "main"
		}
	}
	if _, ok := sizeMapURLs[url]; ok && role == "gallery" {
		role = "size_map"
	}
	return role, main
}

func isSheinFinalReviewSwatchRole(role string) bool {
	return role == "swatch" || role == "skc"
}

func mergeSheinFinalReviewImage(existing *SheinFinalReviewImage, role string, main bool) {
	if existing == nil {
		return
	}
	switch {
	case main || role == "main":
		existing.Role = "main"
		existing.Main = true
		existing.SizeMap = false
		existing.Swatch = false
	case role == "size_map" && existing.Role != "main":
		existing.Role = "size_map"
		existing.SizeMap = true
		existing.Swatch = false
	case isSheinFinalReviewSwatchRole(role) && existing.Role != "main" && existing.Role != "size_map":
		existing.Role = role
		existing.Swatch = true
	}
}
