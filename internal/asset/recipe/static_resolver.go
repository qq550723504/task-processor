package recipe

import "task-processor/internal/asset"

type StaticResolver struct{}

func NewStaticResolver() *StaticResolver { return &StaticResolver{} }

func (r *StaticResolver) Resolve(req ResolveRequest) []AssetRecipe {
	switch req.Platform {
	case "shein":
		return []AssetRecipe{
			newRecipe("shein-main-model", "shein", "SHEIN 主图", asset.KindModelImage, "main", "main", "SHEIN Editorial Main", "shein_model_editorial", []asset.Kind{asset.KindModelImage, asset.KindMainImage, asset.KindCleanImage, asset.KindSourceImage}, false, 1),
			newRecipe("shein-gallery-scene", "shein", "SHEIN 场景图库", asset.KindSceneImage, "gallery", "gallery", "SHEIN Lifestyle Gallery", "shein_lifestyle_gallery", []asset.Kind{asset.KindSceneImage, asset.KindGalleryImage, asset.KindMainImage, asset.KindSourceImage}, true, 4),
			newRecipe("shein-size-scene", "shein", "SHEIN 尺码图", asset.KindSizeSceneImage, "auxiliary", "size_scene", "SHEIN Size Scene", "shein_size_infographic", []asset.Kind{asset.KindSizeSceneImage, asset.KindDetailCrop, asset.KindGalleryImage}, true, 2),
			newRecipe("shein-selling-point", "shein", "SHEIN 卖点图", asset.KindSellingPointImage, "auxiliary", "selling_point", "SHEIN Selling Point", "shein_selling_point", []asset.Kind{asset.KindSellingPointImage, asset.KindDetailCrop, asset.KindGalleryImage}, true, 2),
		}
	case "amazon":
		return []AssetRecipe{
			newRecipe("amazon-main-white-bg", "amazon", "Amazon 白底主图", asset.KindWhiteBgImage, "main", "main", "Amazon White Background Main", "amazon_white_bg_main", []asset.Kind{asset.KindWhiteBgImage, asset.KindMainImage, asset.KindCleanImage, asset.KindSourceImage}, false, 1),
			newRecipe("amazon-gallery", "amazon", "Amazon 图库", asset.KindGalleryImage, "gallery", "gallery", "Amazon Gallery", "amazon_gallery_standard", []asset.Kind{asset.KindGalleryImage, asset.KindMainImage, asset.KindSourceImage}, true, 5),
			newRecipe("amazon-size-scene", "amazon", "Amazon 尺寸图", asset.KindSizeSceneImage, "auxiliary", "size_scene", "Amazon Size Infographic", "amazon_size_infographic", []asset.Kind{asset.KindSizeSceneImage, asset.KindDetailCrop, asset.KindGalleryImage}, true, 2),
			newRecipe("amazon-lifestyle", "amazon", "Amazon 场景图", asset.KindSceneImage, "auxiliary", "scene", "Amazon Lifestyle Scene", "amazon_lifestyle_scene", []asset.Kind{asset.KindSceneImage, asset.KindGalleryImage, asset.KindSourceImage}, true, 2),
		}
	case "temu":
		return []AssetRecipe{
			newRecipe("temu-main", "temu", "TEMU 主图", asset.KindMainImage, "main", "main", "TEMU Conversion Main", "temu_main_conversion", []asset.Kind{asset.KindMainImage, asset.KindCleanImage, asset.KindSourceImage}, false, 1),
			newRecipe("temu-gallery", "temu", "TEMU 图库", asset.KindGalleryImage, "gallery", "gallery", "TEMU Conversion Gallery", "temu_conversion_scene", []asset.Kind{asset.KindSellingPointImage, asset.KindSceneImage, asset.KindGalleryImage, asset.KindSourceImage}, true, 4),
			newRecipe("temu-detail", "temu", "TEMU 细节图", asset.KindDetailCrop, "auxiliary", "detail", "TEMU Detail Focus", "temu_detail_focus", []asset.Kind{asset.KindDetailCrop, asset.KindGalleryImage}, true, 2),
		}
	case "walmart":
		return []AssetRecipe{
			newRecipe("walmart-main", "walmart", "Walmart 主图", asset.KindMainImage, "main", "main", "Walmart Catalog Main", "walmart_catalog_main", []asset.Kind{asset.KindWhiteBgImage, asset.KindMainImage, asset.KindCleanImage, asset.KindSourceImage}, false, 1),
			newRecipe("walmart-gallery", "walmart", "Walmart 图库", asset.KindGalleryImage, "gallery", "gallery", "Walmart Catalog Scene", "walmart_catalog_scene", []asset.Kind{asset.KindGalleryImage, asset.KindSceneImage, asset.KindSourceImage}, true, 4),
			newRecipe("walmart-spec", "walmart", "Walmart 规格图", asset.KindSizeSceneImage, "auxiliary", "spec", "Walmart Spec Support", "walmart_spec_support", []asset.Kind{asset.KindSizeSceneImage, asset.KindDetailCrop, asset.KindGalleryImage}, true, 2),
		}
	default:
		return nil
	}
}

func newRecipe(id, platform, name string, kind asset.Kind, slot, purpose, templateLabel, renderProfile string, preferred []asset.Kind, optional bool, maxItems int) AssetRecipe {
	return AssetRecipe{
		ID:        id,
		Platform:  platform,
		Name:      name,
		AssetKind: kind,
		Generated: kind == asset.KindSceneImage || kind == asset.KindSellingPointImage || kind == asset.KindSizeSceneImage || kind == asset.KindModelImage,
		Template: &Template{
			BundleSlot:     slot,
			Purpose:        purpose,
			TemplateLabel:  templateLabel,
			RenderProfile:  renderProfile,
			PreferredKinds: append([]asset.Kind(nil), preferred...),
			Optional:       optional,
			MaxItems:       maxItems,
		},
	}
}
