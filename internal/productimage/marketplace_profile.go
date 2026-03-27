package productimage

import "strings"

type marketplaceProfile struct {
	MainReviewThreshold    float64
	WhiteBgReviewThreshold float64
	WhiteCanvasPenalty     float64
	Family                 string
	Marketplace            string
	Country                string
}

var amazonUSFamilyKeywords = map[string][]string{
	"footwear": {
		"slipper", "shoe", "sandal", "boot", "sneaker",
	},
	"apparel": {
		"apparel", "clothing", "shirt", "dress", "hoodie", "jacket", "pants", "sock", "glove",
	},
	"bags_accessories": {
		"bag", "backpack", "wallet", "purse", "hat", "cap", "belt",
	},
	"home_textiles": {
		"pillow", "blanket", "textile", "curtain", "cushion", "sheet", "towel",
	},
	"electronics": {
		"electronic", "phone", "tablet", "laptop", "camera", "charger", "headphone", "speaker",
	},
	"jewelry_watch": {
		"jewelry", "watch", "ring", "bracelet", "necklace", "earring",
	},
	"beauty_bottle": {
		"bottle", "cosmetic", "serum", "cream", "lotion", "perfume",
	},
}

func resolveMarketplaceProfile(source *SourceBundle) marketplaceProfile {
	profile := marketplaceProfile{
		MainReviewThreshold:    0.65,
		WhiteBgReviewThreshold: 0.70,
		WhiteCanvasPenalty:     0.10,
		Family:                 "default",
		Marketplace:            normalizeProfileToken(sourceMarketplace(source)),
		Country:                normalizeProfileToken(sourceCountry(source)),
	}

	if profile.Marketplace == "" {
		profile.Marketplace = "amazon"
	}
	if profile.Country == "" {
		profile.Country = "us"
	}

	productType := normalizeProfileToken(sourceProductType(source))
	if productType == "" {
		return profile
	}

	if profile.Marketplace == "amazon" {
		switch resolveAmazonUSFamily(productType) {
		case "footwear":
			profile.Family = "footwear"
			profile.MainReviewThreshold = 0.61
			profile.WhiteBgReviewThreshold = 0.68
			profile.WhiteCanvasPenalty = 0.04
		case "apparel":
			profile.Family = "apparel"
			profile.MainReviewThreshold = 0.62
			profile.WhiteBgReviewThreshold = 0.68
			profile.WhiteCanvasPenalty = 0.05
		case "bags_accessories":
			profile.Family = "bags_accessories"
			profile.MainReviewThreshold = 0.63
			profile.WhiteBgReviewThreshold = 0.69
			profile.WhiteCanvasPenalty = 0.06
		case "home_textiles":
			profile.Family = "home_textiles"
			profile.MainReviewThreshold = 0.63
			profile.WhiteBgReviewThreshold = 0.69
			profile.WhiteCanvasPenalty = 0.06
		case "electronics":
			profile.Family = "electronics"
			profile.MainReviewThreshold = 0.69
			profile.WhiteBgReviewThreshold = 0.75
			profile.WhiteCanvasPenalty = 0.12
		case "jewelry_watch":
			profile.Family = "jewelry_watch"
			profile.MainReviewThreshold = 0.70
			profile.WhiteBgReviewThreshold = 0.76
			profile.WhiteCanvasPenalty = 0.14
		case "beauty_bottle":
			profile.Family = "beauty_bottle"
			profile.MainReviewThreshold = 0.68
			profile.WhiteBgReviewThreshold = 0.74
			profile.WhiteCanvasPenalty = 0.12
		}
	}

	return profile
}

func normalizeProfileToken(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func sourceMarketplace(source *SourceBundle) string {
	if source == nil {
		return ""
	}
	return source.Marketplace
}

func sourceCountry(source *SourceBundle) string {
	if source == nil {
		return ""
	}
	return source.Country
}

func sourceProductType(source *SourceBundle) string {
	if source == nil || source.Analysis == nil || source.Analysis.Representation == nil {
		return ""
	}
	return source.Analysis.Representation.ProductType
}

func resolveAmazonUSFamily(productType string) string {
	for family, keywords := range amazonUSFamilyKeywords {
		for _, keyword := range keywords {
			if strings.Contains(productType, keyword) {
				return family
			}
		}
	}
	return "default"
}

func profileKeywordsForTest(family string) []string {
	if keywords, ok := amazonUSFamilyKeywords[family]; ok {
		return append([]string(nil), keywords...)
	}
	return nil
}

func ProfileKeywordsForTest(family string) []string {
	return profileKeywordsForTest(family)
}

func ResolveMarketplaceProfileForTest(source *SourceBundle) (family string, mainThreshold, whiteBgThreshold, whiteCanvasPenalty float64) {
	profile := resolveMarketplaceProfile(source)
	return profile.Family, profile.MainReviewThreshold, profile.WhiteBgReviewThreshold, profile.WhiteCanvasPenalty
}
