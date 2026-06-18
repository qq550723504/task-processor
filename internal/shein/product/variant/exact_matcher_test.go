package variant

import (
	"context"
	"testing"

	shein "task-processor/internal/shein"
	"task-processor/internal/shein/api/attribute"
	sheinattr "task-processor/internal/shein/product/attribute"
)

func makeVariant(asin string, attrs map[string]string) sheinattr.Variant {
	return sheinattr.Variant{ASIN: asin, Attributes: attrs}
}

func TestVariantExactMatcher_FindExactMatches(t *testing.T) {
	m := NewVariantExactMatcher()

	variants := []sheinattr.Variant{
		makeVariant("B001", map[string]string{"Color": "Red", "Size": "M"}),
		makeVariant("B002", map[string]string{"Color": "Blue", "Size": "L"}),
		makeVariant("B003", map[string]string{"Color": "red", "Size": "S"}),   // 小写，测试大小写不敏感
		makeVariant("B004", map[string]string{"colour": "Red", "Size": "XL"}), // 属性名大小写不敏感
	}

	tests := []struct {
		name            string
		attrNames       []string
		targetValueNorm string
		wantASINs       []string
	}{
		{
			name:            "精确匹配颜色 Red（大小写不敏感）",
			attrNames:       []string{"Color"},
			targetValueNorm: "red",
			wantASINs:       []string{"B001", "B003"},
		},
		{
			name:            "精确匹配尺寸 L",
			attrNames:       []string{"Size"},
			targetValueNorm: "l",
			wantASINs:       []string{"B002"},
		},
		{
			name:            "属性名大小写不敏感匹配 colour",
			attrNames:       []string{"Colour"},
			targetValueNorm: "red",
			wantASINs:       []string{"B004"},
		},
		{
			name:            "多个属性名候选，匹配第一个命中",
			attrNames:       []string{"Color", "Colour"},
			targetValueNorm: "red",
			wantASINs:       []string{"B001", "B003", "B004"},
		},
		{
			name:            "无匹配返回空",
			attrNames:       []string{"Color"},
			targetValueNorm: "green",
			wantASINs:       []string{},
		},
		{
			name:            "空变体列表返回空",
			attrNames:       []string{"Color"},
			targetValueNorm: "red",
			wantASINs:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := variants
			if tt.name == "空变体列表返回空" {
				input = []sheinattr.Variant{}
			}

			got := m.FindExactMatches(input, tt.attrNames, tt.targetValueNorm)

			if len(got) != len(tt.wantASINs) {
				t.Errorf("len(matches) = %d, want %d", len(got), len(tt.wantASINs))
				return
			}

			gotASINs := make(map[string]bool)
			for _, v := range got {
				gotASINs[v.ASIN] = true
			}
			for _, asin := range tt.wantASINs {
				if !gotASINs[asin] {
					t.Errorf("expected ASIN %q in results, got %v", asin, got)
				}
			}
		})
	}
}

// TestVariantMatcherUtils_RemoveDuplicates 验证去重逻辑
func TestVariantMatcherUtils_RemoveDuplicates(t *testing.T) {
	u := NewVariantMatcherUtils()

	tests := []struct {
		name  string
		input []string
		want  int // 期望去重后的长度
	}{
		{"无重复", []string{"a", "b", "c"}, 3},
		{"有重复", []string{"a", "b", "a", "c", "b"}, 3},
		{"全相同", []string{"a", "a", "a"}, 1},
		{"空切片", []string{}, 0},
		{"nil 切片", nil, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := u.RemoveDuplicates(tt.input)
			if len(got) != tt.want {
				t.Errorf("RemoveDuplicates(%v) len = %d, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

// TestVariantMatcherUtils_IsSizeAttribute 验证尺寸属性识别
func TestVariantMatcherUtils_IsSizeAttribute(t *testing.T) {
	u := NewVariantMatcherUtils()

	tests := []struct {
		variantVal string
		targetVal  string
		want       bool
	}{
		{"4x6 inch", "4x6", true},
		{"30cm", "30", true},
		{"10mm", "10", true},
		{"red", "blue", false},
		{"large", "small", false},
		{"mat to wall", "mat", true},
	}

	for _, tt := range tests {
		t.Run(tt.variantVal+"_vs_"+tt.targetVal, func(t *testing.T) {
			got := u.IsSizeAttribute(tt.variantVal, tt.targetVal)
			if got != tt.want {
				t.Errorf("IsSizeAttribute(%q, %q) = %v, want %v", tt.variantVal, tt.targetVal, got, tt.want)
			}
		})
	}
}

// TestVariantMatcherUtils_IsColorAttribute 验证颜色属性识别
func TestVariantMatcherUtils_IsColorAttribute(t *testing.T) {
	u := NewVariantMatcherUtils()

	tests := []struct {
		variantVal string
		targetVal  string
		want       bool
	}{
		{"Red", "red", true},
		{"Dark Blue", "blue", true},
		{"Silver Gray", "gray", true},
		{"Large", "Small", false},
		{"4x6", "4x6 inch", false},
		{"Gold Frame", "gold", true},
	}

	for _, tt := range tests {
		t.Run(tt.variantVal+"_vs_"+tt.targetVal, func(t *testing.T) {
			got := u.IsColorAttribute(tt.variantVal, tt.targetVal)
			if got != tt.want {
				t.Errorf("IsColorAttribute(%q, %q) = %v, want %v", tt.variantVal, tt.targetVal, got, tt.want)
			}
		})
	}
}

func TestVariantMatcher_FindUniqueMatchesForValues(t *testing.T) {
	m := NewVariantMatcher()
	ctx := shein.NewTaskContext(context.Background(), nil)
	ctx.AttributeTemplates = &attribute.AttributeTemplateInfo{
		Data: []attribute.AttributeTemplate{
			{
				AttributeInfos: []attribute.AttributeInfo{
					{AttributeID: 27, AttributeName: "颜色", AttributeNameEn: "Color"},
				},
			},
		},
	}

	variants := []sheinattr.Variant{
		makeVariant("B001", map[string]string{"Color": "Black/White"}),
		makeVariant("B002", map[string]string{"Color": "White"}),
	}

	assignments := m.FindUniqueMatchesForValues(ctx, variants, 27, []string{"Black", "White"})

	if len(assignments["Black"]) != 1 || assignments["Black"][0].ASIN != "B001" {
		t.Fatalf("expected Black to own B001, got %+v", assignments["Black"])
	}
	if len(assignments["White"]) != 1 || assignments["White"][0].ASIN != "B002" {
		t.Fatalf("expected White to own B002 only, got %+v", assignments["White"])
	}
}

func TestVariantMatcher_FindUniqueMatchesForValues_DoesNotFuzzyMatchFarShoeSizes(t *testing.T) {
	m := NewVariantMatcher()
	ctx := shein.NewTaskContext(context.Background(), nil)
	ctx.AttributeTemplates = &attribute.AttributeTemplateInfo{
		Data: []attribute.AttributeTemplate{
			{
				AttributeInfos: []attribute.AttributeInfo{
					{AttributeID: 87, AttributeName: "尺寸", AttributeNameEn: "Size"},
				},
			},
		},
	}

	variants := []sheinattr.Variant{
		makeVariant("B001", map[string]string{"Size": "8.5 Wide"}),
		makeVariant("B002", map[string]string{"Size": "10"}),
	}

	assignments := m.FindUniqueMatchesForValues(ctx, variants, 87, []string{"5 Wide", "10.5"})

	if got := len(assignments["5 Wide"]); got != 0 {
		t.Fatalf("expected 5 Wide to keep no assignments, got %+v", assignments["5 Wide"])
	}
	if got := len(assignments["10.5"]); got != 0 {
		t.Fatalf("expected 10.5 to keep no assignments, got %+v", assignments["10.5"])
	}
}

func TestVariantMatcher_FindUniqueMatchesForValues_AllowsFuzzyMatchForSameBaseShoeSizeWidthVariant(t *testing.T) {
	m := NewVariantMatcher()
	ctx := shein.NewTaskContext(context.Background(), nil)
	ctx.AttributeTemplates = &attribute.AttributeTemplateInfo{
		Data: []attribute.AttributeTemplate{
			{
				AttributeInfos: []attribute.AttributeInfo{
					{AttributeID: 87, AttributeName: "尺寸", AttributeNameEn: "Size"},
				},
			},
		},
	}

	variants := []sheinattr.Variant{
		makeVariant("B001", map[string]string{"Size": "7 X-Wide"}),
	}

	assignments := m.FindUniqueMatchesForValues(ctx, variants, 87, []string{"7 Wide"})

	if len(assignments["7 Wide"]) != 1 || assignments["7 Wide"][0].ASIN != "B001" {
		t.Fatalf("expected 7 Wide to fuzzy-match same base size width variant, got %+v", assignments["7 Wide"])
	}
}
