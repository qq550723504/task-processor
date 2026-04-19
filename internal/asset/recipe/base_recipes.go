package recipe

import "task-processor/internal/asset"

func BaseAssetRecipes() []AssetRecipe {
	return []AssetRecipe{
		{
			ID:        "base-clean-image",
			Platform:  "common",
			Name:      "基础净图",
			AssetKind: asset.KindCleanImage,
			Generated: true,
			Template: &Template{
				Purpose:        "base_clean",
				PreferredKinds: []asset.Kind{asset.KindCleanImage},
				Optional:       true,
				MaxItems:       1,
			},
		},
		{
			ID:        "base-white-bg-image",
			Platform:  "common",
			Name:      "基础白底图",
			AssetKind: asset.KindWhiteBgImage,
			Generated: true,
			Template: &Template{
				Purpose:        "base_white_bg",
				PreferredKinds: []asset.Kind{asset.KindWhiteBgImage},
				Optional:       true,
				MaxItems:       1,
			},
		},
		{
			ID:        "base-subject-cutout",
			Platform:  "common",
			Name:      "基础主体抠图",
			AssetKind: asset.KindSubjectCutout,
			Generated: true,
			Template: &Template{
				Purpose:        "base_cutout",
				PreferredKinds: []asset.Kind{asset.KindSubjectCutout},
				Optional:       true,
				MaxItems:       1,
			},
		},
	}
}
