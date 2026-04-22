package shein

import (
	"testing"

	"task-processor/internal/productenrich"
)

func TestBuildCategoryFamilyConflictSummaryDetectsDrinkwareVsFootwear(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		Title:        "420ml 304 Stainless Steel Insulated Water Bottle with Dual-Drink Lid",
		CategoryPath: []string{"Home & Kitchen", "Drinkware", "Water Bottles"},
		Attributes: map[string]productenrich.CanonicalAttribute{
			"材质": {Value: "不锈钢"},
			"容量": {Value: "420ml"},
		},
	}
	pkg := &Package{
		SpuName:      canonical.Title,
		CategoryPath: []string{"家居&生活", "家庭用品", "鞋用品", "鞋配饰"},
		Attributes: map[string]string{
			"材质": "不锈钢",
			"容量": "420ml",
		},
	}

	recommend, reason := buildCategoryFamilyConflictSummary(canonical, pkg)
	if !recommend {
		t.Fatal("expected drinkware vs footwear category conflict to require review")
	}
	if reason == "" {
		t.Fatal("expected non-empty category review reason")
	}
}

func TestBuildCategoryFamilyConflictSummaryPrefersResolvedSheinPath(t *testing.T) {
	canonical := &productenrich.CanonicalProduct{
		Title:        "420ml Stainless Steel Insulated Tumbler with Dual Drink Lid - Modern Minimalist Design",
		Description:  "Drinkware tumbler water bottle with multiple colors and stainless steel body",
		CategoryPath: []string{"Drinkware", "Tumblers & Water Bottles"},
		Attributes: map[string]productenrich.CanonicalAttribute{
			"材质": {Value: "不锈钢"},
			"容量": {Value: "420ml"},
			"颜色": {Value: "裸粉,抹茶绿,米色,黑色,奶油黄"},
		},
	}
	pkg := &Package{
		SpuName:      canonical.Title,
		CategoryPath: []string{"Drinkware", "Tumblers & Water Bottles"},
		CategoryResolution: &CategoryResolution{
			MatchedPath: []string{"家居&生活", "家庭用品", "鞋用品", "鞋配饰"},
		},
		Attributes: map[string]string{
			"材质": "不锈钢",
			"容量": "420ml",
			"颜色": "裸粉,抹茶绿,米色,黑色,奶油黄",
		},
	}

	recommend, reason := buildCategoryFamilyConflictSummary(canonical, pkg)
	if !recommend {
		t.Fatal("expected drinkware product to conflict with resolved footwear path")
	}
	if reason == "" {
		t.Fatal("expected non-empty category review reason")
	}
}
