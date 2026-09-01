package imagepolicy

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	productimage "task-processor/internal/product/image"
)

const (
	maxProfileInputFieldBytes     = 8 << 10
	maxProfileInputAggregateBytes = 16 << 10
	maxMarketplaceIdentifierBytes = 64
)

var ErrInvalidProfileInput = errors.New("product image profile input is invalid")

type ProfileInput struct {
	Marketplace   string
	Country       string
	ProductType   string
	SceneCategory string
}

type ProductImageProfile struct {
	Family                         string
	Marketplace                    string
	Country                        string
	MainReviewThreshold            float64
	WhiteBackgroundReviewThreshold float64
	WhiteCanvasPenalty             float64
	SceneDefaults                  productimage.SceneOptions
	SceneDefaultsSource            string
}

type familyRule struct {
	family   string
	keywords string
	main     float64
	white    float64
	penalty  float64
}

func ResolveProductImageProfile(input ProfileInput) (ProductImageProfile, error) {
	if !validRawProfileInput(input) {
		return ProductImageProfile{}, ErrInvalidProfileInput
	}

	marketplace := normalize(input.Marketplace)
	country := normalize(input.Country)
	if !validMarketplace(marketplace) || !validCountry(country) {
		return ProductImageProfile{}, ErrInvalidProfileInput
	}

	productType := normalize(input.ProductType)
	category := normalize(input.SceneCategory)
	profile := ProductImageProfile{
		Family:                         "default",
		Marketplace:                    marketplace,
		Country:                        country,
		MainReviewThreshold:            0.65,
		WhiteBackgroundReviewThreshold: 0.70,
		WhiteCanvasPenalty:             0.10,
		SceneDefaultsSource:            "none",
	}
	if marketplace == "amazon" && country == "us" {
		if rule, ok := resolveAmazonUSFamily(productType); ok {
			profile.Family = rule.family
			profile.MainReviewThreshold = rule.main
			profile.WhiteBackgroundReviewThreshold = rule.white
			profile.WhiteCanvasPenalty = rule.penalty
		}
	}

	if category == "" {
		category = inferSceneCategory(productType)
	}
	if defaults, ok := categorySceneDefaults(marketplace, category); ok {
		profile.SceneDefaults = cloneOptions(defaults)
		profile.SceneDefaultsSource = "platform_category"
		return profile, nil
	}
	if defaults, ok := platformSceneDefaults(marketplace); ok {
		defaults.SceneCategory = category
		profile.SceneDefaults = cloneOptions(defaults)
		profile.SceneDefaultsSource = "platform"
	}
	return profile, nil
}

func validRawProfileInput(input ProfileInput) bool {
	fields := [...]string{input.Marketplace, input.Country, input.ProductType, input.SceneCategory}
	total := 0
	for _, field := range fields {
		if len(field) > maxProfileInputFieldBytes ||
			!utf8.ValidString(field) ||
			total > maxProfileInputAggregateBytes-len(field) {
			return false
		}
		total += len(field)
	}
	return true
}

func validMarketplace(value string) bool {
	if value == "" || len(value) > maxMarketplaceIdentifierBytes || !isASCIILetter(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if !isASCIILetter(character) && !isASCIIDigit(character) && character != '-' && character != '_' {
			return false
		}
	}
	last := value[len(value)-1]
	return isASCIILetter(last) || isASCIIDigit(last)
}

func validCountry(value string) bool {
	return len(value) == 2 && isASCIILetter(value[0]) && isASCIILetter(value[1])
}

func isASCIILetter(value byte) bool {
	return value >= 'a' && value <= 'z'
}

func isASCIIDigit(value byte) bool {
	return value >= '0' && value <= '9'
}

func resolveAmazonUSFamily(productType string) (familyRule, bool) {
	for _, rule := range amazonUSFamilyRuleSet() {
		if containsProductKeyword(productType, rule.keywords) {
			return rule, true
		}
	}
	return familyRule{}, false
}

func amazonUSFamilyRuleSet() [7]familyRule {
	return [7]familyRule{
		{family: "footwear", keywords: "slipper|shoe|sandal|boot|sneaker", main: 0.61, white: 0.68, penalty: 0.04},
		{family: "apparel", keywords: "apparel|clothing|shirt|dress|hoodie|jacket|pants|sock|glove", main: 0.62, white: 0.68, penalty: 0.05},
		{family: "bags_accessories", keywords: "bag|handbag|backpack|wallet|purse|hat|cap|belt|tote|satchel|crossbody", main: 0.63, white: 0.69, penalty: 0.06},
		{family: "home_textiles", keywords: "pillow|blanket|textile|curtain|cushion|sheet|towel", main: 0.63, white: 0.69, penalty: 0.06},
		{family: "electronics", keywords: "electronic|phone|smartphone|tablet|laptop|camera|charger|headphone|speaker", main: 0.69, white: 0.75, penalty: 0.12},
		{family: "jewelry_watch", keywords: "jewelry|jewellery|watch|ring|bracelet|necklace|earring", main: 0.70, white: 0.76, penalty: 0.14},
		{family: "beauty_bottle", keywords: "bottle|cosmetic|serum|cream|lotion|perfume", main: 0.68, white: 0.74, penalty: 0.12},
	}
}

func inferSceneCategory(productType string) string {
	switch {
	case containsProductKeyword(productType, "sneaker|shoe|boot|sandal|slipper|heel|loafer"):
		return "shoes"
	case containsProductKeyword(productType, "necklace|ring|earring|bracelet|jewelry|jewellery|pendant|brooch"):
		return "jewelry"
	case containsProductKeyword(productType, "handbag|backpack|bag|purse|tote|satchel|crossbody"):
		return "bags"
	default:
		return ""
	}
}

func containsProductKeyword(value, keywordList string) bool {
	for tokenStart := -1; ; {
		tokenEnd := len(value)
		for index, character := range value {
			if tokenStart < 0 {
				if unicode.IsLetter(character) || unicode.IsDigit(character) {
					tokenStart = index
				}
				continue
			}
			if !unicode.IsLetter(character) && !unicode.IsDigit(character) {
				tokenEnd = index
				break
			}
		}
		if tokenStart < 0 {
			return false
		}
		if tokenMatchesKeywordList(value[tokenStart:tokenEnd], keywordList) {
			return true
		}
		if tokenEnd == len(value) {
			return false
		}
		value = value[tokenEnd:]
		tokenStart = -1
	}
}

func tokenMatchesKeywordList(token, keywordList string) bool {
	for keywordList != "" {
		keyword := keywordList
		if separator := strings.IndexByte(keywordList, '|'); separator >= 0 {
			keyword = keywordList[:separator]
			keywordList = keywordList[separator+1:]
		} else {
			keywordList = ""
		}
		if tokenMatchesKeyword(token, keyword) {
			return true
		}
	}
	return false
}

func tokenMatchesKeyword(token, keyword string) bool {
	if !strings.HasPrefix(token, keyword) {
		return false
	}
	suffix := token[len(keyword):]
	return suffix == "" || suffix == "s" || suffix == "es"
}

func platformSceneDefaults(marketplace string) (productimage.SceneOptions, bool) {
	switch marketplace {
	case "amazon":
		return sceneOptions("", "studio", "bright", "centered", "none", "premium"), true
	case "shein":
		return sceneOptions("", "lifestyle", "warm", "close_up", "light", "youthful"), true
	case "temu":
		return sceneOptions("", "lifestyle", "bright", "multi_angle", "moderate", "sporty"), true
	case "walmart":
		return sceneOptions("", "lifestyle", "neutral", "centered", "light", "homey"), true
	default:
		return productimage.SceneOptions{}, false
	}
}

func categorySceneDefaults(marketplace, category string) (productimage.SceneOptions, bool) {
	switch marketplace {
	case "amazon":
		switch category {
		case "shoes":
			return sceneOptions(category, "studio", "bright", "centered", "none", "premium"), true
		case "jewelry":
			return sceneOptions(category, "studio", "cool", "close_up", "none", "premium"), true
		case "bags":
			return sceneOptions(category, "studio", "neutral", "centered", "none", "premium"), true
		}
	case "shein":
		switch category {
		case "shoes", "jewelry":
			return sceneOptions(category, "lifestyle", "warm", "close_up", "light", "youthful"), true
		case "bags":
			return sceneOptions(category, "lifestyle", "warm", "multi_angle", "light", "youthful"), true
		}
	case "temu":
		switch category {
		case "shoes", "bags":
			return sceneOptions(category, "lifestyle", "bright", "multi_angle", "moderate", "sporty"), true
		case "jewelry":
			return sceneOptions(category, "lifestyle", "bright", "close_up", "light", "youthful"), true
		}
	case "walmart":
		switch category {
		case "shoes", "bags":
			return sceneOptions(category, "lifestyle", "neutral", "centered", "light", "homey"), true
		case "jewelry":
			return sceneOptions(category, "studio", "neutral", "close_up", "none", "premium"), true
		}
	}
	return productimage.SceneOptions{}, false
}

func sceneOptions(category, style, tone, composition, props, audience string) productimage.SceneOptions {
	return productimage.SceneOptions{
		SceneCategory:  category,
		SceneStyle:     style,
		BackgroundTone: tone,
		Composition:    composition,
		PropsLevel:     props,
		AudienceHint:   audience,
	}
}

func cloneOptions(options productimage.SceneOptions) productimage.SceneOptions {
	options.StyleReferenceIDs = append([]string(nil), options.StyleReferenceIDs...)
	return options
}

func normalize(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
