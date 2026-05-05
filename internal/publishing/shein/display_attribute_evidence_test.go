package shein

import (
	"testing"

	common "task-processor/internal/publishing/common"
)

func TestBuildDisplayAttributeEvidencePoolIncludesRichSDSFields(t *testing.T) {
	t.Parallel()

	pool := buildDisplayAttributeEvidencePool(&Package{
		SpuName:       "带刻度方形挂钟25*25",
		CategoryPath:  []string{"家居&生活", "家居装饰", "时钟", "挂钟"},
		Description:   "Battery powered wall clock",
		SellingPoints: []string{"静音无声", "轻奢质地"},
		ProductAttributes: []common.Attribute{
			{Name: "material_description", Value: "优选复合板材质"},
			{Name: "production_process", Value: "UV打印"},
			{Name: "product_performance", Value: "静音无声，轻奢质地"},
			{Name: "applicable_scenarios", Value: "办公室、卧室、客厅"},
			{Name: "special_description", Value: "请使用碳性电池"},
			{Name: "product_size", Value: "25*25cm；25cm/9.8inch"},
			{Name: "packaging_specification", Value: "30*30*5cm，0.45kg"},
			{Name: "variant_sku", Value: "MG17701061001"},
			{Name: "variant_size", Value: "25cm/9.8inch"},
			{Name: "variant_color", Value: "White"},
		},
	})

	if pool == nil {
		t.Fatal("evidence pool = nil")
	}
	if !pool.HasField("material_description") {
		t.Fatalf("pool fields = %#v, want material_description", pool.FieldNames())
	}
	if !pool.HasField("product_performance") || !pool.HasField("product_size") || !pool.HasField("packaging_specification") {
		t.Fatalf("pool fields = %#v, want rich SDS fields", pool.FieldNames())
	}
	if !pool.HasField("variant_sku") || !pool.HasField("variant_size") || !pool.HasField("variant_color") {
		t.Fatalf("pool fields = %#v, want variant fields", pool.FieldNames())
	}
	if len(pool.StructuredItems()) == 0 {
		t.Fatalf("structured items = %#v, want parsed structured evidence", pool.Items)
	}
	inputs := pool.AttributeInputs()
	got := map[string]string{}
	for _, input := range inputs {
		got[input.Name] = input.Value
	}
	if got["Product Size"] == "" || got["Packaging Specification"] == "" || got["Product Model"] == "" {
		t.Fatalf("derived inputs = %#v, want size/packaging/product model", got)
	}
}
