package shein

var templateAttributeAliases = map[string][]string{
	"size":  {"尺码", "尺寸"},
	"尺寸":    {"尺码", "size"},
	"尺码":    {"尺寸", "size"},
	"color": {"颜色", "colour"},
	"颜色":    {"color", "colour"},
}

func attributeAliasesForName(name string) []string {
	return append([]string(nil), templateAttributeAliases[normalizeText(name)]...)
}
