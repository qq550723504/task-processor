package shein

import (
	"testing"

	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

func TestAttributeResolverSkipsSaleScopeAttributes(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       27,
						AttributeName:     "颜色",
						AttributeNameEn:   "Color",
						AttributeType:     1,
						SKCScope:          boolPointer(true),
						AttributeInputNum: 1,
					},
					{
						AttributeID:       112,
						AttributeName:     "鞋面材质",
						AttributeNameEn:   "Upper Material",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 5930427, AttributeValue: "网布", AttributeValueEn: "Mesh Fabric"},
						},
					},
				},
			}},
		},
	}, nil)

	pkg := &Package{
		CategoryID: 8824,
		ProductAttributes: []common.Attribute{
			{Name: "Color", Value: "Black"},
			{Name: "Upper Material", Value: "网布"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 1 {
		t.Fatalf("resolved count = %d, want 1", resolution.ResolvedCount)
	}
	if len(resolution.ResolvedAttributes) != 1 {
		t.Fatalf("resolved attributes = %#v, want 1", resolution.ResolvedAttributes)
	}
	if resolution.ResolvedAttributes[0].AttributeID != 112 {
		t.Fatalf("resolved attribute id = %d, want 112", resolution.ResolvedAttributes[0].AttributeID)
	}
}

func TestAttributeResolverMapsTemplateValueID(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       112,
						AttributeName:     "鞋面材质",
						AttributeNameEn:   "Upper Material",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 5930427, AttributeValue: "网布", AttributeValueEn: "Mesh Fabric"},
						},
					},
				},
			}},
		},
	}, stubSaleAttributeLLM{
		response: `{"attribute_value_id":5930427,"reasons":["mesh fabric is the closest match"]}`,
	})

	pkg := &Package{
		CategoryID: 8824,
		ProductAttributes: []common.Attribute{
			{Name: "Upper Material", Value: "飞织布"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 1 {
		t.Fatalf("resolved count = %d, want 1", resolution.ResolvedCount)
	}
	if got := resolution.ResolvedAttributes[0].AttributeValueID; got == nil || *got != 5930427 {
		t.Fatalf("attribute value id = %v, want 5930427", got)
	}
	if resolution.ResolvedAttributes[0].MatchedBy != "attribute_value_normalized" {
		t.Fatalf("matched by = %q, want attribute_value_normalized", resolution.ResolvedAttributes[0].MatchedBy)
	}
}

func TestAttributeResolverNormalizesMaterialValueSynonyms(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       112,
						AttributeName:     "鞋面材质",
						AttributeNameEn:   "Upper Material",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 5930427, AttributeValue: "网布", AttributeValueEn: "Mesh Fabric"},
						},
					},
					{
						AttributeID:       1000003,
						AttributeName:     "里料材质",
						AttributeNameEn:   "Lining Material",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 6001001, AttributeValue: "网布", AttributeValueEn: "Mesh Fabric"},
						},
					},
				},
			}},
		},
	}, nil)

	pkg := &Package{
		CategoryID: 8824,
		ProductAttributes: []common.Attribute{
			{Name: "Upper Material", Value: "flyknit"},
			{Name: "Lining Material", Value: "mesh"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 2 {
		t.Fatalf("resolved count = %d, want 2", resolution.ResolvedCount)
	}
	if len(resolution.ReviewNotes) != 0 {
		t.Fatalf("review notes = %v, want empty", resolution.ReviewNotes)
	}

	got := map[string]ResolvedAttribute{}
	for _, attr := range resolution.ResolvedAttributes {
		got[attr.Name] = attr
	}

	upper, ok := got["Upper Material"]
	if !ok {
		t.Fatalf("missing Upper Material resolution: %#v", resolution.ResolvedAttributes)
	}
	if upper.AttributeValueID == nil || *upper.AttributeValueID != 5930427 {
		t.Fatalf("Upper Material value id = %v, want 5930427", upper.AttributeValueID)
	}
	if upper.MatchedBy != "attribute_value_normalized" {
		t.Fatalf("Upper Material matched by = %q, want attribute_value_normalized", upper.MatchedBy)
	}

	lining, ok := got["Lining Material"]
	if !ok {
		t.Fatalf("missing Lining Material resolution: %#v", resolution.ResolvedAttributes)
	}
	if lining.AttributeValueID == nil || *lining.AttributeValueID != 6001001 {
		t.Fatalf("Lining Material value id = %v, want 6001001", lining.AttributeValueID)
	}
	if lining.MatchedBy != "attribute_value_normalized" {
		t.Fatalf("Lining Material matched by = %q, want attribute_value_normalized", lining.MatchedBy)
	}
}
