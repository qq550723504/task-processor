package variations

import (
	"testing"
)

// TestVariationsDuplicateASINFix 测试修复重复ASIN的问题
func TestVariationsDuplicateASINFix(t *testing.T) {
	matcher := NewMatcher(GetDefaultConfig())

	// 模拟ASIN映射（简化版本，基于产品 B08937KYGJ）
	asinMapping := map[string]map[string]string{
		"B0DHXZD7RD": {
			"size":  "L=17.4\" (1pcs)",
			"color": "Black",
		},
		"B082D965XB": {
			"size":  "L=17.4\" (1pcs)",
			"color": "Light Brown & Black",
		},
		"B0DHXW5MKC": {
			"size":  "L=17.4\" (2pcs)",
			"color": "Black",
		},
		"B089373LC7": {
			"size":  "L=38.3\"(2pcs)",
			"color": "Black",
		},
		"B08937KYGJ": {
			"size":  "L=38.3\"(2pcs)",
			"color": "Light Brown & Black",
		},
	}

	// 测试用例1: Black + L=38.3"(2pcs) 应该只匹配 B089373LC7
	combo1 := map[string]interface{}{
		"size":  "L=38.3\"(2pcs)",
		"color": "Black",
	}
	matchedASIN1 := matcher.FindMatchingASIN(combo1, asinMapping)
	if matchedASIN1 != "B089373LC7" {
		t.Errorf("组合1应该匹配B089373LC7，实际匹配: %s", matchedASIN1)
	}
	t.Logf("✅ 组合1正确匹配: %s -> %s", "Black + L=38.3\"(2pcs)", matchedASIN1)

	// 测试用例2: Light Brown & Black + L=38.3"(2pcs) 应该只匹配 B08937KYGJ
	combo2 := map[string]interface{}{
		"size":  "L=38.3\"(2pcs)",
		"color": "Light Brown & Black",
	}
	matchedASIN2 := matcher.FindMatchingASIN(combo2, asinMapping)
	if matchedASIN2 != "B08937KYGJ" {
		t.Errorf("组合2应该匹配B08937KYGJ，实际匹配: %s", matchedASIN2)
	}
	t.Logf("✅ 组合2正确匹配: %s -> %s", "Light Brown & Black + L=38.3\"(2pcs)", matchedASIN2)

	// 测试用例3: Black + L=17.4" (1pcs) 应该只匹配 B0DHXZD7RD
	combo3 := map[string]interface{}{
		"size":  "L=17.4\" (1pcs)",
		"color": "Black",
	}
	matchedASIN3 := matcher.FindMatchingASIN(combo3, asinMapping)
	if matchedASIN3 != "B0DHXZD7RD" {
		t.Errorf("组合3应该匹配B0DHXZD7RD，实际匹配: %s", matchedASIN3)
	}
	t.Logf("✅ 组合3正确匹配: %s -> %s", "Black + L=17.4\" (1pcs)", matchedASIN3)

	// 验证没有重复匹配
	allCombos := []map[string]interface{}{combo1, combo2, combo3}
	matchedASINs := make(map[string]int)

	for _, combo := range allCombos {
		asin := matcher.FindMatchingASIN(combo, asinMapping)
		if asin != "" {
			matchedASINs[asin]++
		}
	}

	for asin, count := range matchedASINs {
		if count > 1 {
			t.Errorf("❌ ASIN %s 被匹配了 %d 次，应该只匹配1次", asin, count)
		}
	}
	t.Logf("✅ 所有ASIN都是唯一匹配，没有重复")
}

// TestAttributesMatchPrecision 测试精确匹配逻辑
func TestAttributesMatchPrecision(t *testing.T) {
	matcher := NewMatcher(GetDefaultConfig())

	// 测试用例1: 完全匹配
	combo1 := map[string]interface{}{
		"size":  "L=38.3\"(2pcs)",
		"color": "Black",
	}
	asinAttrs1 := map[string]string{
		"size":  "L=38.3\"(2pcs)",
		"color": "Black",
	}
	if !matcher.AttributesMatch(combo1, asinAttrs1) {
		t.Error("❌ 完全匹配的情况应该返回true")
	} else {
		t.Log("✅ 完全匹配测试通过")
	}

	// 测试用例2: 部分匹配（应该返回false）
	combo2 := map[string]interface{}{
		"size":  "L=38.3\"(2pcs)",
		"color": "Black",
	}
	asinAttrs2 := map[string]string{
		"size":  "L=38.3\"(2pcs)",
		"color": "Light Brown & Black", // 颜色不匹配
	}
	if matcher.AttributesMatch(combo2, asinAttrs2) {
		t.Error("❌ 部分匹配的情况应该返回false")
	} else {
		t.Log("✅ 部分匹配测试通过（正确返回false）")
	}

	// 测试用例3: ASIN属性多于combo（应该返回true，只要combo的所有属性都匹配）
	combo3 := map[string]interface{}{
		"size":  "L=38.3\"(2pcs)",
		"color": "Black",
	}
	asinAttrs3 := map[string]string{
		"size":  "L=38.3\"(2pcs)",
		"color": "Black",
		"style": "Modern", // 额外的属性
	}
	if !matcher.AttributesMatch(combo3, asinAttrs3) {
		t.Error("❌ ASIN有额外属性但combo的所有属性都匹配时应该返回true")
	} else {
		t.Log("✅ 额外属性测试通过")
	}
}

// TestValuesMatchPrecision 测试值匹配的精确性
func TestValuesMatchPrecision(t *testing.T) {
	matcher := NewMatcher(GetDefaultConfig())

	// 测试用例1: 精确匹配
	if !matcher.ValuesMatch("Black", "Black") {
		t.Error("❌ 精确匹配应该返回true")
	} else {
		t.Log("✅ 精确匹配测试通过")
	}

	// 测试用例2: 大小写不敏感
	if !matcher.ValuesMatch("Black", "black") {
		t.Error("❌ 大小写不敏感匹配应该返回true")
	} else {
		t.Log("✅ 大小写不敏感测试通过")
	}

	// 测试用例3: 包含匹配应该返回false（关键修复）
	if matcher.ValuesMatch("Black", "Light Brown & Black") {
		t.Error("❌ 'Black' 不应该匹配 'Light Brown & Black'")
	} else {
		t.Log("✅ 包含匹配正确返回false（关键修复验证）")
	}

	// 测试用例4: 移除特殊字符后的精确匹配
	if !matcher.ValuesMatch("L=17.4\" (1pcs)", "L=17.4\"(1pcs)") {
		t.Error("❌ 移除空格后应该匹配")
	} else {
		t.Log("✅ 特殊字符处理测试通过")
	}

	// 测试用例5: 不同的值应该返回false
	if matcher.ValuesMatch("L=17.4\" (1pcs)", "L=38.3\"(2pcs)") {
		t.Error("❌ 不同的值应该返回false")
	} else {
		t.Log("✅ 不同值测试通过")
	}
}
