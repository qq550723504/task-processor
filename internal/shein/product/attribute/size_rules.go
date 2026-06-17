package attribute

import "strings"

var apparelMeasurementHeaderKeywords = []string{
	"bust",
	"waist",
	"hip",
	"length",
	"shoulder",
	"sleeve",
	"inseam",
	"thigh",
	"hem width",
}

var shoeSizeHeaderScores = []struct {
	keyword string
	score   int
}{
	{keyword: "us size", score: 3},
	{keyword: "uk size", score: 2},
	{keyword: "eu size", score: 2},
	{keyword: "cn size", score: 2},
	{keyword: "jp size", score: 2},
	{keyword: "cm", score: 2},
	{keyword: "brand size", score: 1},
}

var alphaSizeAliases = map[string]string{
	"xxs":               "XXS",
	"extra extra small": "XXS",
	"xs":                "XS",
	"extra small":       "XS",
	"s":                 "S",
	"small":             "S",
	"m":                 "M",
	"medium":            "M",
	"l":                 "L",
	"large":             "L",
	"xl":                "XL",
	"xl/1x":             "XL",
	"x-large":           "XL",
	"x large":           "XL",
	"extra large":       "XL",
	"xxl":               "XXL",
	"2xl":               "XXL",
	"xx-large":          "XXL",
	"xx large":          "XXL",
	"extra extra large": "XXL",
	"xxxl":              "XXXL",
	"3xl":               "XXXL",
	"xxx-large":         "XXXL",
}

func normalizeAlphaSizeLabel(value string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "_", " ")
	normalized = strings.Join(strings.Fields(normalized), " ")
	if normalized == "" {
		return "", false
	}
	if mapped, ok := alphaSizeAliases[normalized]; ok {
		return mapped, true
	}
	return "", false
}
