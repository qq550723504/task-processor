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
	family  string
	lexemes string
	main    float64
	white   float64
	penalty float64
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
	if category != "" && !supportedSceneCategory(category) {
		return ProductImageProfile{}, ErrInvalidProfileInput
	}
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

func supportedSceneCategory(category string) bool {
	return category == "shoes" || category == "jewelry" || category == "bags"
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
		if containsAcceptedLexeme(productType, rule.lexemes) {
			return rule, true
		}
	}
	return familyRule{}, false
}

func amazonUSFamilyRuleSet() [7]familyRule {
	return [7]familyRule{
		{family: "footwear", lexemes: "slipper|slippers|shoe|shoes|sandal|sandals|boot|boots|sneaker|sneakers", main: 0.61, white: 0.68, penalty: 0.04},
		{family: "apparel", lexemes: "apparel|clothing|shirt|shirts|dress|dresses|hoodie|hoodies|jacket|jackets|pants|sock|socks|glove|gloves", main: 0.62, white: 0.68, penalty: 0.05},
		{family: "bags_accessories", lexemes: "bag|bags|handbag|handbags|backpack|backpacks|wallet|wallets|purse|purses|hat|hats|cap|caps|belt|belts|tote|totes|satchel|satchels|crossbody|crossbodies", main: 0.63, white: 0.69, penalty: 0.06},
		{family: "home_textiles", lexemes: "pillow|pillows|blanket|blankets|textile|textiles|curtain|curtains|cushion|cushions|sheet|sheets|towel|towels", main: 0.63, white: 0.69, penalty: 0.06},
		{family: "electronics", lexemes: "electronic|electronics|phone|phones|smartphone|smartphones|tablet|tablets|laptop|laptops|camera|cameras|charger|chargers|headphone|headphones|speaker|speakers", main: 0.69, white: 0.75, penalty: 0.12},
		{family: "jewelry_watch", lexemes: "jewelry|jewellery|watch|watches|ring|rings|bracelet|bracelets|necklace|necklaces|earring|earrings", main: 0.70, white: 0.76, penalty: 0.14},
		{family: "beauty_bottle", lexemes: "bottle|bottles|cosmetic|cosmetics|serum|serums|cream|creams|lotion|lotions|perfume|perfumes", main: 0.68, white: 0.74, penalty: 0.12},
	}
}

func inferSceneCategory(productType string) string {
	switch {
	case containsAcceptedLexeme(productType, "sneaker|sneakers|shoe|shoes|boot|boots|sandal|sandals|slipper|slippers|heel|heels|loafer|loafers"):
		return "shoes"
	case containsAcceptedLexeme(productType, "necklace|necklaces|ring|rings|earring|earrings|bracelet|bracelets|jewelry|jewellery|pendant|pendants|brooch|brooches"):
		return "jewelry"
	case containsAcceptedLexeme(productType, "handbag|handbags|backpack|backpacks|bag|bags|purse|purses|tote|totes|satchel|satchels|crossbody|crossbodies"):
		return "bags"
	default:
		return ""
	}
}

func containsAcceptedLexeme(value, acceptedLexemes string) bool {
	for tokenStart := -1; ; {
		tokenEnd := len(value)
		for index, character := range value {
			if tokenStart < 0 {
				if isProductTokenCharacter(character) {
					tokenStart = index
				}
				continue
			}
			if !isProductTokenCharacter(character) {
				tokenEnd = index
				break
			}
		}
		if tokenStart < 0 {
			return false
		}
		if tokenMatchesAcceptedLexeme(value[tokenStart:tokenEnd], acceptedLexemes) {
			return true
		}
		if tokenEnd == len(value) {
			return false
		}
		value = value[tokenEnd:]
		tokenStart = -1
	}
}

func isProductTokenCharacter(character rune) bool {
	return unicode.IsLetter(character) || unicode.IsNumber(character) || unicode.IsMark(character)
}

func tokenMatchesAcceptedLexeme(token, acceptedLexemes string) bool {
	for acceptedLexemes != "" {
		lexeme := acceptedLexemes
		if separator := strings.IndexByte(acceptedLexemes, '|'); separator >= 0 {
			lexeme = acceptedLexemes[:separator]
			acceptedLexemes = acceptedLexemes[separator+1:]
		} else {
			acceptedLexemes = ""
		}
		if token == lexeme {
			return true
		}
	}
	return false
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
