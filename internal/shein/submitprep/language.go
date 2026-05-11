package submitprep

import "strings"

func DetectLanguage(title, description string) string {
	text := strings.TrimSpace(title + " " + description)
	if text == "" {
		return "en"
	}

	var japaneseCount, chineseCount, englishCount int
	for _, r := range text {
		switch {
		case (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF):
			japaneseCount++
		case r >= 0x4E00 && r <= 0x9FFF:
			chineseCount++
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			englishCount++
		}
	}

	if japaneseCount > chineseCount && japaneseCount > englishCount {
		return "ja"
	}
	if chineseCount > englishCount && chineseCount > japaneseCount {
		return "zh"
	}
	return "en"
}

func IsJapanese(text string) bool {
	return DetectLanguage(text, "") == "ja"
}

func IsChinese(text string) bool {
	return DetectLanguage(text, "") == "zh"
}

func IsEnglish(text string) bool {
	return DetectLanguage(text, "") == "en"
}

func GetCharacterCounts(text string) (japanese, chinese, english int) {
	for _, r := range text {
		switch {
		case (r >= 0x3040 && r <= 0x309F) || (r >= 0x30A0 && r <= 0x30FF):
			japanese++
		case r >= 0x4E00 && r <= 0x9FFF:
			chinese++
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			english++
		}
	}
	return japanese, chinese, english
}

func GetTargetLanguagesByRegion(region string) []string {
	switch region {
	case "US", "MX":
		return []string{"en", "es"}
	case "FR", "DE", "IT", "ES":
		return []string{"de", "es", "fr", "it", "en"}
	case "JP":
		return []string{"ja", "en"}
	case "SA", "AE":
		return []string{"ar", "en"}
	default:
		return []string{"en"}
	}
}
