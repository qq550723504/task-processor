package shein

import (
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestBuildCategoryFamilyConflictSummaryDoesNotUsePathRules(t *testing.T) {
	canonical := &canonical.Product{
		Title:        "420ml 304 Stainless Steel Insulated Water Bottle with Dual-Drink Lid",
		CategoryPath: []string{"Home & Kitchen", "Drinkware", "Water Bottles"},
		Attributes: map[string]canonical.Attribute{
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
	if recommend {
		t.Fatalf("expected no rule-based category conflict review, got reason %q", reason)
	}
	if reason != "" {
		t.Fatalf("expected empty category review reason, got %q", reason)
	}
}

func TestBuildCategoryFamilyConflictSummaryDoesNotUseResolvedPathRules(t *testing.T) {
	canonical := &canonical.Product{
		Title:        "420ml Stainless Steel Insulated Tumbler with Dual Drink Lid - Modern Minimalist Design",
		Description:  "Drinkware tumbler water bottle with multiple colors and stainless steel body",
		CategoryPath: []string{"Drinkware", "Tumblers & Water Bottles"},
		Attributes: map[string]canonical.Attribute{
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
	if recommend {
		t.Fatalf("expected no rule-based category conflict review, got reason %q", reason)
	}
	if reason != "" {
		t.Fatalf("expected empty category review reason, got %q", reason)
	}
}

func TestBuildCategoryFamilyConflictSummaryAcceptsDenimHatWorkCap(t *testing.T) {
	canonical := &canonical.Product{
		Title:       "水洗牛仔帽",
		Description: "Washed denim hat",
		Attributes: map[string]canonical.Attribute{
			"材质": {Value: "100%纯棉"},
			"工艺": {Value: "烫画"},
		},
	}
	pkg := &Package{
		SpuName:      "水洗牛仔帽",
		CategoryPath: []string{"服饰装饰品", "男士配饰", "男士帽子", "男士工作帽"},
		Attributes: map[string]string{
			"材质": "100%纯棉",
			"工艺": "烫画",
		},
	}

	recommend, reason := buildCategoryFamilyConflictSummary(canonical, pkg)
	if recommend {
		t.Fatalf("expected denim hat and work cap to be treated as same headwear family, got reason %q", reason)
	}
}
