package size

import (
	"regexp"
	"strconv"
	"strings"
)

type System string

const (
	SystemUnknown System = "unknown"
	SystemUS      System = "us"
	SystemUK      System = "uk"
	SystemEU      System = "eu"
	SystemBR      System = "br"
	SystemCN      System = "cn"
	SystemMM      System = "mm"
)

type Width string

const (
	WidthUnknown Width = "unknown"
	WidthRegular Width = "regular"
	WidthWide    Width = "wide"
	WidthXWide   Width = "xwide"
	WidthNarrow  Width = "narrow"
)

type ShoeSize struct {
	Raw        string
	System     System
	BaseSize   string
	Width      Width
	IsShoeSize bool
}

var (
	numericTokenPattern   = regexp.MustCompile(`\d+(?:\.\d+)?`)
	mmOnlyPattern         = regexp.MustCompile(`^\d{3}(?:\.\d+)?$`)
	sizeTokenOnlyPattern  = regexp.MustCompile(`^\s*(?:us|uk|eu|eur|br|cn)?\s*\d+(?:\.\d+)?(?:\s*(?:w|ww|xw|x[\s-]*wide|wide|narrow|medium|m))?(?:\s*us)?\s*$`)
	widthCompactWPattern  = regexp.MustCompile(`\d(?:\.\d+)?\s*w$`)
	widthCompactWWPattern = regexp.MustCompile(`\d(?:\.\d+)?\s*ww$`)
	sizeRangePattern      = regexp.MustCompile(`\d+(?:\.\d+)?\s*-\s*\d+(?:\.\d+)?`)
)

func ParseShoeSize(raw string) ShoeSize {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ShoeSize{Raw: raw, System: SystemUnknown, Width: WidthUnknown}
	}

	normalized := strings.ToLower(strings.Join(strings.Fields(trimmed), " "))
	if sizeRangePattern.MatchString(normalized) {
		return ShoeSize{Raw: raw, System: SystemUnknown, Width: WidthUnknown}
	}
	base := extractBaseSize(normalized)
	if base == "" {
		return ShoeSize{Raw: raw, System: SystemUnknown, Width: WidthUnknown}
	}

	system := inferSystem(normalized, base)
	width := inferWidth(normalized)
	if width == WidthUnknown {
		width = WidthRegular
	}

	if !looksLikeShoeSize(normalized, system, base, width) {
		return ShoeSize{Raw: raw, System: SystemUnknown, Width: WidthUnknown}
	}

	return ShoeSize{
		Raw:        raw,
		System:     system,
		BaseSize:   base,
		Width:      width,
		IsShoeSize: true,
	}
}

func AreShoeSizesFuzzyCompatible(a, b string) bool {
	left := ParseShoeSize(a)
	right := ParseShoeSize(b)
	if !left.IsShoeSize || !right.IsShoeSize {
		return false
	}
	if left.BaseSize != right.BaseSize {
		return false
	}
	return widthsCompatible(left.Width, right.Width)
}

func widthsCompatible(a, b Width) bool {
	if a == b {
		return true
	}
	if a == WidthRegular || b == WidthRegular {
		return false
	}
	if (a == WidthWide || a == WidthXWide) && (b == WidthWide || b == WidthXWide) {
		return true
	}
	return false
}

func extractBaseSize(value string) string {
	token := numericTokenPattern.FindString(value)
	if token == "" {
		return ""
	}
	parsed, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return ""
	}
	if parsed == float64(int64(parsed)) {
		return strconv.FormatInt(int64(parsed), 10)
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(parsed, 'f', 1, 64), "0"), ".")
}

func inferSystem(value, base string) System {
	switch {
	case strings.Contains(value, "us"):
		return SystemUS
	case strings.Contains(value, "uk"):
		return SystemUK
	case strings.Contains(value, "eu") || strings.Contains(value, "eur"):
		return SystemEU
	case strings.Contains(value, "br"):
		return SystemBR
	case strings.Contains(value, "cn"):
		return SystemCN
	case mmOnlyPattern.MatchString(base):
		return SystemMM
	default:
		return SystemUnknown
	}
}

func inferWidth(value string) Width {
	switch {
	case strings.Contains(value, "x-wide"), strings.Contains(value, "x wide"), strings.Contains(value, "xw"), widthCompactWWPattern.MatchString(value):
		return WidthXWide
	case strings.Contains(value, "narrow"):
		return WidthNarrow
	case strings.Contains(value, "wide"), widthCompactWPattern.MatchString(value):
		return WidthWide
	default:
		return WidthUnknown
	}
}

func looksLikeShoeSize(value string, system System, base string, width Width) bool {
	if system != SystemUnknown {
		return true
	}
	if width != WidthRegular && width != WidthUnknown {
		return true
	}
	if mmOnlyPattern.MatchString(base) {
		return true
	}
	return sizeTokenOnlyPattern.MatchString(value)
}
