package shein

import "testing"

func TestShouldExtractSaleAttributeSourceValueUsesLLMForNonCompactAIStyle(t *testing.T) {
	value := "帮我设计一个适合印在防滑地垫的图案，要有英文跟图案来表达懒惰型人格标签，各种人物或动物，释放想躺平没动力的情绪，3D视觉效果，背"
	if !shouldExtractSaleAttributeSourceValue("ai_style", value) {
		t.Fatalf("shouldExtractSaleAttributeSourceValue(ai_style, %q) = false, want true", value)
	}
}

func TestShouldExtractSaleAttributeSourceValueKeepsCompactEnglishStyle(t *testing.T) {
	value := "Lazy Cat Graphic"
	if shouldExtractSaleAttributeSourceValue("ai_style", value) {
		t.Fatalf("shouldExtractSaleAttributeSourceValue(ai_style, %q) = true, want false", value)
	}
}
