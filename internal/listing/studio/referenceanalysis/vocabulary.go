package referenceanalysis

import "regexp"

var (
	studioWordPattern               = regexp.MustCompile(`[a-z0-9]+`)
	studioCapitalizedPhrasePattern  = regexp.MustCompile(`\b(?:[A-Z][a-z0-9]*)(?:\s+[A-Z][a-z0-9]*){0,2}\b`)
	studioQuotedTextPattern         = regexp.MustCompile(`["'][^"']+["']`)
	studioProtectedIdentityTerms    = []string{"hello kitty", "adidas", "nike", "mickey mouse", "taylor swift", "elsa", "old navy"}
	studioProtectedIdentityPattern  = regexp.MustCompile(`(?i)\b(?:hello\s+kitty|adidas|nike|mickey\s+mouse|taylor\s+swift|elsa|old\s+navy)\b`)
	studioBrandMarkPattern          = regexp.MustCompile(`\b(?:logo|logos|brand mark|brand marks|wordmark|wordmarks|emblem|emblems|trefoil|swoosh)\b`)
	studioExactTextPattern          = regexp.MustCompile(`\b(?:exact text|exact slogan|same wording|copy this exact|quote|quoted|slogan|tagline|catchphrase)\b`)
	studioExactArtworkPattern       = regexp.MustCompile(`\b(?:exact artwork|same artwork|source artwork|original artwork)\b`)
	studioCharacterIdentityPattern  = regexp.MustCompile(`\b(?:characters?|face|faces|portrait|portraits|person|people|identity|identities|celebrity|celebrities|likeness)\b`)
	studioUniqueLayoutPattern       = regexp.MustCompile(`\b(?:same|exact|identical|signature|unique|distinctive)\s+[a-z0-9\s-]{0,40}?(?:layout|composition|arrangement|split|frame|badge)\b`)
	studioWatermarkPattern          = regexp.MustCompile(`\b(?:watermark|watermarks)\b`)
	studioUnsafeSpacerPattern       = regexp.MustCompile(`[\(\)\[\]\{\}:,;|]+`)
	studioRepeatedWhitespacePattern = regexp.MustCompile(`\s+`)
)

var (
	studioSafeDescriptorWords = map[string]struct{}{
		"abstract": {}, "airy": {}, "allover": {}, "arched": {}, "art": {}, "artwork": {}, "balanced": {}, "badge": {}, "beach": {},
		"black": {}, "block": {}, "blue": {}, "bold": {}, "border": {}, "botanical": {}, "brown": {}, "brush": {}, "center": {}, "centered": {}, "cherry": {},
		"clean": {}, "coastal": {}, "coral": {}, "cream": {}, "gray": {}, "green": {}, "grey": {}, "navy": {}, "old": {}, "orange": {},
		"left": {}, "right": {}, "front": {}, "back": {}, "chest": {}, "large": {}, "sleeve": {}, "placement": {}, "mood": {}, "nostalgic": {},
		"pink": {}, "red": {}, "silver": {}, "white": {}, "gold": {},
		"collegiate": {}, "composition": {}, "crest": {}, "dense": {}, "distressed": {}, "drawn": {}, "dynamic": {},
		"english": {}, "floral": {}, "flower": {}, "flowers": {}, "frame": {}, "framed": {}, "geometric": {}, "glass": {}, "gothic": {},
		"gradient": {}, "hand": {}, "heritage": {}, "illustration": {}, "koi": {}, "layered": {}, "layering": {}, "layout": {},
		"lettering": {}, "linework": {}, "mascot": {}, "medallion": {}, "minimal": {}, "minimalist": {}, "modern": {}, "ombre": {},
		"ornamental": {}, "palette": {}, "pattern": {}, "playful": {}, "repeat": {}, "resort": {}, "retro": {}, "rounded": {},
		"sans": {}, "sea": {}, "seal": {}, "serif": {}, "sky": {}, "sports": {}, "streetwear": {}, "sunset": {}, "teal": {},
		"tan": {}, "texture": {}, "tropical": {}, "vintage": {}, "watercolor": {}, "wave": {}, "wear": {}, "western": {},
	}
	studioMotifPhraseVocabulary = []string{
		"koi wave",
		"retro flowers",
		"western floral",
		"sports mascot",
		"floral border",
		"retro cherry",
	}
	studioMotifWordVocabulary = map[string]string{
		"retro":      "retro",
		"vintage":    "vintage",
		"western":    "western",
		"floral":     "floral",
		"flower":     "flower",
		"flowers":    "flowers",
		"botanical":  "botanical",
		"koi":        "koi",
		"wave":       "wave",
		"cherry":     "cherry",
		"sports":     "sports",
		"mascot":     "mascot",
		"border":     "border",
		"crest":      "crest",
		"badge":      "badge",
		"ornamental": "ornamental",
		"geometric":  "geometric",
		"abstract":   "abstract",
	}
	studioPalettePhraseVocabulary = []string{
		"cherry red",
		"off white",
		"forest green",
		"sky blue",
	}
	studioPaletteWordVocabulary = map[string]string{
		"cream":  "cream",
		"red":    "red",
		"navy":   "navy",
		"tan":    "tan",
		"teal":   "teal",
		"orange": "orange",
		"black":  "black",
		"white":  "white",
		"blue":   "blue",
		"green":  "green",
		"pink":   "pink",
		"gold":   "gold",
		"silver": "silver",
		"brown":  "brown",
		"gray":   "gray",
		"grey":   "gray",
		"cherry": "cherry",
	}
	studioTypographyPhraseVocabulary = []string{
		"brush lettering",
		"old english",
		"sans serif",
	}
	studioTypographyWordVocabulary = map[string]string{
		"bold":       "bold",
		"brush":      "brush",
		"lettering":  "lettering",
		"collegiate": "collegiate",
		"distressed": "distressed",
		"serif":      "serif",
		"script":     "script",
		"gothic":     "gothic",
		"vintage":    "vintage",
		"western":    "western",
		"block":      "block",
		"clean":      "clean",
	}
	studioDensityPhraseVocabulary = []string{
		"clean layering",
	}
	studioDensityWordVocabulary = map[string]string{
		"clean":    "clean",
		"layering": "layering",
		"layered":  "layered",
		"airy":     "airy",
		"dense":    "dense",
		"balanced": "balanced",
		"minimal":  "minimal",
		"bold":     "bold",
	}
	studioProductFitPhraseVocabulary = []string{
		"resort wear",
		"vintage streetwear",
	}
	studioProductFitWordVocabulary = map[string]string{
		"resort":     "resort",
		"wear":       "wear",
		"vintage":    "vintage",
		"streetwear": "streetwear",
		"casual":     "casual",
		"heritage":   "heritage",
		"classic":    "classic",
		"oversized":  "oversized",
		"unisex":     "unisex",
		"athletic":   "athletic",
	}
	studioSafeTitleCasePhraseSet = buildStudioSafeTitleCasePhraseSet(
		studioMotifPhraseVocabulary,
		studioPalettePhraseVocabulary,
		studioTypographyPhraseVocabulary,
		studioDensityPhraseVocabulary,
		studioProductFitPhraseVocabulary,
	)
)
