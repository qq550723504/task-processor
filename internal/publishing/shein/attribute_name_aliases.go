package shein

func attributeAliasesForName(name string) []string {
	switch normalizeText(name) {
	case "colour":
		return []string{"color", "颜色", "颜色分类"}
	case "color", "颜色", "颜色分类":
		return []string{"colour", "color", "颜色", "颜色分类"}
	case "size", "尺码", "尺寸":
		return []string{"size", "dimension", "尺码", "尺寸"}
	case "dimension":
		return []string{"size", "尺码", "尺寸"}
	case "material", "材质":
		return []string{"fabric", "material", "材质"}
	case "pattern", "图案":
		return []string{"print", "pattern", "图案"}
	case "capacity", "容量":
		return []string{"capacity", "容量"}
	case "style", "款式":
		return []string{"style", "款式"}
	case "type", "类型":
		return []string{"type", "类型", "style", "款式"}
	case "model", "型号":
		return []string{"model", "型号", "款式"}
	case "set", "套装":
		return []string{"set", "套装", "组合"}
	}
	return nil
}
