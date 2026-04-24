package shein

import (
	"context"
	"strings"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type captureAttributeLLM struct {
	responses []string
	err       error
	prompt    string
}

func (s *captureAttributeLLM) CreateChatCompletion(context.Context, *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return nil, s.err
}

func (s *captureAttributeLLM) Generate(_ context.Context, prompt string) (string, error) {
	s.prompt = prompt
	if len(s.responses) == 0 {
		return "", s.err
	}
	response := s.responses[0]
	s.responses = s.responses[1:]
	return response, s.err
}

func (s *captureAttributeLLM) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", s.err
}

func (s *captureAttributeLLM) GetDefaultModel() string {
	return "test"
}

type scriptedAttributeLLM struct {
	responses []string
	err       error
	prompts   []string
}

func (s *scriptedAttributeLLM) CreateChatCompletion(context.Context, *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return nil, s.err
}

func (s *scriptedAttributeLLM) Generate(_ context.Context, prompt string) (string, error) {
	s.prompts = append(s.prompts, prompt)
	if len(s.responses) == 0 {
		return "", s.err
	}
	response := s.responses[0]
	s.responses = s.responses[1:]
	return response, s.err
}

func (s *scriptedAttributeLLM) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", s.err
}

func (s *scriptedAttributeLLM) GetDefaultModel() string {
	return "test"
}

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
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"attribute_id":0,"reasons":["sale attribute should not map to display template"]}`,
			`{"attribute_id":112,"reasons":["upper material matches template field"]}`,
			`{"attribute_value_id":5930427,"reasons":["mesh fabric is the template value"]}`,
		},
	})

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
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"attribute_id":112,"reasons":["upper material matches template field"]}`,
			`{"attribute_value_id":5930427,"reasons":["mesh fabric is the closest match"]}`,
		},
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
	if resolution.ResolvedAttributes[0].MatchedBy != "llm_attribute_value" {
		t.Fatalf("matched by = %q, want llm_attribute_value", resolution.ResolvedAttributes[0].MatchedBy)
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
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"attribute_id":112,"reasons":["flyknit is an upper material source field"]}`,
			`{"attribute_id":1000003,"reasons":["mesh is a lining material source field"]}`,
			`{"attribute_value_id":5930427,"reasons":["flyknit best aligns with mesh fabric template value"]}`,
			`{"attribute_value_id":6001001,"reasons":["mesh aligns with mesh fabric template value"]}`,
		},
	})

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
	if upper.MatchedBy != "llm_attribute_value" {
		t.Fatalf("Upper Material matched by = %q, want llm_attribute_value", upper.MatchedBy)
	}

	lining, ok := got["Lining Material"]
	if !ok {
		t.Fatalf("missing Lining Material resolution: %#v", resolution.ResolvedAttributes)
	}
	if lining.AttributeValueID == nil || *lining.AttributeValueID != 6001001 {
		t.Fatalf("Lining Material value id = %v, want 6001001", lining.AttributeValueID)
	}
	if lining.MatchedBy != "llm_attribute_value" {
		t.Fatalf("Lining Material matched by = %q, want llm_attribute_value", lining.MatchedBy)
	}
}

func TestAttributeResolverMarksMissingRequiredDisplayAttributesAsPending(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       118,
						AttributeName:     "宽度",
						AttributeNameEn:   "Width (cm)",
						AttributeType:     2,
						AttributeInputNum: 1,
					},
					{
						AttributeID:       160,
						AttributeName:     "材质",
						AttributeNameEn:   "Material",
						AttributeType:     4,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 1001, AttributeValue: "棉", AttributeValueEn: "Cotton"},
						},
					},
				},
			}},
		},
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"attribute_id":160,"reasons":["material matches template field"]}`,
			`{"attribute_value_id":1001,"reasons":["棉 matches cotton"]}`,
		},
	})

	pkg := &Package{
		CategoryID: 5229,
		ProductAttributes: []common.Attribute{
			{Name: "Material", Value: "棉"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.Status != "partial" {
		t.Fatalf("status = %q, want partial", resolution.Status)
	}
	if len(resolution.PendingAttributes) != 1 {
		t.Fatalf("pending attributes = %#v, want 1", resolution.PendingAttributes)
	}
	if resolution.PendingAttributes[0].Name != "Width (cm)" {
		t.Fatalf("pending attribute name = %q, want Width (cm)", resolution.PendingAttributes[0].Name)
	}
	if resolution.UnresolvedCount != 1 {
		t.Fatalf("unresolved count = %d, want 1", resolution.UnresolvedCount)
	}
}

func TestAttributeResolverOnlyMarksCascadeAttributePendingWhenDependencyIsActive(t *testing.T) {
	cascadeValues := "11,12"
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       160,
						AttributeNameEn:   "Material",
						AttributeType:     4,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 11, AttributeValue: "棉", AttributeValueEn: "Cotton"},
							{AttributeValueID: 13, AttributeValue: "麻", AttributeValueEn: "Linen"},
						},
					},
					{
						AttributeID:                 1000547,
						AttributeNameEn:             "Other Material",
						AttributeType:               4,
						AttributeInputNum:           1,
						CascadeAttributeID:          160,
						CascadeAttributeValueIDList: &cascadeValues,
					},
				},
			}},
		},
	}, nil)

	t.Run("inactive without parent", func(t *testing.T) {
		pkg := &Package{
			CategoryID:        5229,
			ProductAttributes: []common.Attribute{{Name: "Width (cm)", Value: "120"}},
		}
		resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
		if len(resolution.PendingAttributes) != 1 || resolution.PendingAttributes[0].Name != "Material" {
			t.Fatalf("pending attributes = %#v, want only Material", resolution.PendingAttributes)
		}
	})

	t.Run("active with parent value", func(t *testing.T) {
		resolver := NewAttributeResolver(stubAttributeAPI{
			templates: &sheinattribute.AttributeTemplateInfo{
				Data: []sheinattribute.AttributeTemplate{{
					AttributeInfos: []sheinattribute.AttributeInfo{
						{
							AttributeID:       160,
							AttributeNameEn:   "Material",
							AttributeType:     4,
							AttributeInputNum: 1,
							AttributeValueInfoList: []sheinattribute.AttributeValue{
								{AttributeValueID: 11, AttributeValue: "棉", AttributeValueEn: "Cotton"},
								{AttributeValueID: 13, AttributeValue: "麻", AttributeValueEn: "Linen"},
							},
						},
						{
							AttributeID:                 1000547,
							AttributeNameEn:             "Other Material",
							AttributeType:               4,
							AttributeInputNum:           1,
							CascadeAttributeID:          160,
							CascadeAttributeValueIDList: &cascadeValues,
						},
					},
				}},
			},
		}, &scriptedAttributeLLM{
			responses: []string{
				`{"attribute_id":160,"reasons":["material matches template field"]}`,
				`{"attribute_value_id":11,"reasons":["棉 matches cotton"]}`,
			},
		})
		pkg := &Package{
			CategoryID:        5229,
			ProductAttributes: []common.Attribute{{Name: "Material", Value: "棉"}},
		}
		resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
		if len(resolution.PendingAttributes) != 1 || resolution.PendingAttributes[0].Name != "Other Material" {
			t.Fatalf("pending attributes = %#v, want only Other Material", resolution.PendingAttributes)
		}
	})

	t.Run("inactive with non-triggering parent value", func(t *testing.T) {
		resolver := NewAttributeResolver(stubAttributeAPI{
			templates: &sheinattribute.AttributeTemplateInfo{
				Data: []sheinattribute.AttributeTemplate{{
					AttributeInfos: []sheinattribute.AttributeInfo{
						{
							AttributeID:       160,
							AttributeNameEn:   "Material",
							AttributeType:     4,
							AttributeInputNum: 1,
							AttributeValueInfoList: []sheinattribute.AttributeValue{
								{AttributeValueID: 11, AttributeValue: "棉", AttributeValueEn: "Cotton"},
								{AttributeValueID: 13, AttributeValue: "麻", AttributeValueEn: "Linen"},
							},
						},
						{
							AttributeID:                 1000547,
							AttributeNameEn:             "Other Material",
							AttributeType:               4,
							AttributeInputNum:           1,
							CascadeAttributeID:          160,
							CascadeAttributeValueIDList: &cascadeValues,
						},
					},
				}},
			},
		}, &scriptedAttributeLLM{
			responses: []string{
				`{"attribute_id":160,"reasons":["material matches template field"]}`,
				`{"attribute_value_id":13,"reasons":["麻 matches linen"]}`,
			},
		})
		pkg := &Package{
			CategoryID:        5229,
			ProductAttributes: []common.Attribute{{Name: "Material", Value: "麻"}},
		}
		resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
		if len(resolution.PendingAttributes) != 0 {
			t.Fatalf("pending attributes = %#v, want none", resolution.PendingAttributes)
		}
	})
}

func TestAttributeResolverResolvesDependentAttributeAfterParentEvenWhenInputOrderIsReversed(t *testing.T) {
	cascadeValues := "11"
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       160,
						AttributeNameEn:   "Material",
						AttributeType:     4,
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 11, AttributeValue: "棉", AttributeValueEn: "Cotton"},
						},
					},
					{
						AttributeID:                 1000547,
						AttributeNameEn:             "Other Material",
						AttributeType:               4,
						AttributeInputNum:           1,
						CascadeAttributeID:          160,
						CascadeAttributeValueIDList: &cascadeValues,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 22, AttributeValue: "麻", AttributeValueEn: "Linen"},
						},
					},
				},
			}},
		},
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"attribute_id":1000547,"reasons":["other material matches dependent field"]}`,
			`{"attribute_id":160,"reasons":["material matches parent field"]}`,
			`{"attribute_value_id":11,"reasons":["棉 matches cotton"]}`,
			`{"attribute_value_id":22,"reasons":["麻 matches linen"]}`,
		},
	})

	pkg := &Package{
		CategoryID: 5229,
		ProductAttributes: []common.Attribute{
			{Name: "Other Material", Value: "麻"},
			{Name: "Material", Value: "棉"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if len(resolution.PendingAttributes) != 0 {
		t.Fatalf("pending attributes = %#v, want none", resolution.PendingAttributes)
	}
	if len(resolution.ResolvedAttributes) != 2 {
		t.Fatalf("resolved attributes = %#v, want 2", resolution.ResolvedAttributes)
	}
	found := false
	for _, item := range resolution.ResolvedAttributes {
		if item.AttributeID == 1000547 && item.AttributeValueID != nil && *item.AttributeValueID == 22 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("dependent attribute was not resolved correctly: %#v", resolution.ResolvedAttributes)
	}
}

func TestAttributeResolverDoesNotRepeatPendingForMatchedAliasTemplate(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       1000067,
						AttributeName:     "填充物",
						AttributeNameEn:   "Filling",
						AttributeType:     3,
						AttributeInputNum: 1,
					},
				},
			}},
		},
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"attribute_id":1000067,"reasons":["填充物 matches Filling field"]}`,
		},
	})

	pkg := &Package{
		CategoryID: 11814,
		ProductAttributes: []common.Attribute{
			{Name: "填充物", Value: "聚酯纤维"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if len(resolution.PendingAttributes) != 0 {
		t.Fatalf("pending attributes = %#v, want none", resolution.PendingAttributes)
	}
}

func TestAttributeResolverMatchesPolyesterAliases(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       160,
						AttributeName:     "材质",
						AttributeNameEn:   "Material",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 2001, AttributeValue: "聚酯纤维", AttributeValueEn: "Polyester"},
						},
					},
				},
			}},
		},
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"attribute_id":160,"reasons":["材质 matches Material field"]}`,
			`{"attribute_value_id":2001,"reasons":["涤纶 matches polyester"]}`,
		},
	})

	pkg := &Package{
		CategoryID: 11814,
		ProductAttributes: []common.Attribute{
			{Name: "材质", Value: "涤纶"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 1 {
		t.Fatalf("resolved count = %d, want 1", resolution.ResolvedCount)
	}
	if got := resolution.ResolvedAttributes[0].AttributeValueID; got == nil || *got != 2001 {
		t.Fatalf("attribute value id = %v, want 2001", got)
	}
}

func TestAttributeResolverMatchesPolyesterAliasesWithoutLLM(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       160,
						AttributeName:     "材质",
						AttributeNameEn:   "Material",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 2001, AttributeValue: "聚酯纤维", AttributeValueEn: "Polyester"},
						},
					},
				},
			}},
		},
	}, nil)

	pkg := &Package{
		CategoryID: 11814,
		ProductAttributes: []common.Attribute{
			{Name: "material", Value: "涤纶"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 1 {
		t.Fatalf("resolved count = %d, want 1; notes=%#v", resolution.ResolvedCount, resolution.ReviewNotes)
	}
	if got := resolution.ResolvedAttributes[0].AttributeValueID; got == nil || *got != 2001 {
		t.Fatalf("attribute value id = %v, want 2001", got)
	}
	if resolution.ResolvedAttributes[0].MatchedBy != "static_attribute_value" {
		t.Fatalf("matched by = %q, want static_attribute_value", resolution.ResolvedAttributes[0].MatchedBy)
	}
}

func TestAttributeResolverMatchesOccasionAliasFromScene(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       3001,
						AttributeNameEn:   "Occasion",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 901, AttributeValue: "Outdoor", AttributeValueEn: "Outdoor"},
						},
					},
				},
			}},
		},
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"attribute_id":3001,"reasons":["场景 maps to occasion field"]}`,
			`{"attribute_value_id":901,"reasons":["室外 aligns with outdoor occasion"]}`,
		},
	})

	pkg := &Package{
		CategoryID: 11814,
		ProductAttributes: []common.Attribute{
			{Name: "场景", Value: "室外"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 1 {
		t.Fatalf("resolved count = %d, want 1", resolution.ResolvedCount)
	}
	if resolution.ResolvedAttributes[0].AttributeID != 3001 {
		t.Fatalf("attribute id = %d, want 3001", resolution.ResolvedAttributes[0].AttributeID)
	}
}

func TestAttributeResolverMatchesRoomAliasFromSpace(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       3002,
						AttributeNameEn:   "Room",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 902, AttributeValue: "Outdoor", AttributeValueEn: "Outdoor"},
						},
					},
					{
						AttributeID:       3001,
						AttributeNameEn:   "Occasion",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 901, AttributeValue: "Party", AttributeValueEn: "Party"},
						},
					},
				},
			}},
		},
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"attribute_id":3002,"reasons":["空间 maps to room field"]}`,
			`{"attribute_value_id":902,"reasons":["空间 室外 aligns with Outdoor room"]}`,
		},
	})

	pkg := &Package{
		CategoryID: 11814,
		ProductAttributes: []common.Attribute{
			{Name: "空间", Value: "室外"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 1 {
		t.Fatalf("resolved count = %d, want 1", resolution.ResolvedCount)
	}
	if resolution.ResolvedAttributes[0].AttributeID != 3002 {
		t.Fatalf("attribute id = %d, want 3002", resolution.ResolvedAttributes[0].AttributeID)
	}
}

func TestAttributeResolverSkipsOtherMaterialPendingWhenParentUsesStandardValue(t *testing.T) {
	cascadeValues := ""
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       160,
						AttributeNameEn:   "Material",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 2001, AttributeValue: "聚酯纤维", AttributeValueEn: "Polyester"},
						},
					},
					{
						AttributeID:                 1601,
						AttributeNameEn:             "Other Material",
						AttributeInputNum:           1,
						CascadeAttributeID:          160,
						CascadeAttributeValueIDList: &cascadeValues,
					},
				},
			}},
		},
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"attribute_id":160,"reasons":["材质 maps to Material field"]}`,
			`{"attribute_value_id":2001,"reasons":["涤纶 matches polyester"]}`,
		},
	})

	pkg := &Package{
		CategoryID: 11814,
		ProductAttributes: []common.Attribute{
			{Name: "材质", Value: "涤纶"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	for _, attr := range resolution.PendingAttributes {
		if attr.Name == "Other Material" {
			t.Fatalf("pending attributes should skip Other Material when parent uses standard value: %#v", resolution.PendingAttributes)
		}
	}
}

func TestAttributeResolverIncludesSegmentsAndContextInValueMappingPrompt(t *testing.T) {
	llm := &captureAttributeLLM{
		responses: []string{
			`{"attribute_id":3001,"reasons":["场景 maps to occasion field"]}`,
			`{"attribute_id":0,"reasons":["风格 is not the target field"]}`,
			`{"attribute_value_id":901,"reasons":["室外 and 阳台 jointly indicate outdoor use"]}`,
		},
	}
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       3001,
						AttributeNameEn:   "Occasion",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 901, AttributeValue: "Outdoor", AttributeValueEn: "Outdoor"},
						},
					},
				},
			}},
		},
	}, llm)

	pkg := &Package{
		CategoryID: 11814,
		ProductAttributes: []common.Attribute{
			{Name: "场景", Value: "室外,阳台"},
			{Name: "风格", Value: "北欧"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 1 {
		t.Fatalf("resolved count = %d, want 1", resolution.ResolvedCount)
	}
	if resolution.ResolvedAttributes[0].AttributeValueID == nil || *resolution.ResolvedAttributes[0].AttributeValueID != 901 {
		t.Fatalf("attribute value id = %v, want 901", resolution.ResolvedAttributes[0].AttributeValueID)
	}
	for _, expected := range []string{"Source value segments:", "\"室外\"", "\"阳台\"", "Additional source context:", "风格=\"北欧\""} {
		if !strings.Contains(llm.prompt, expected) {
			t.Fatalf("prompt = %q, want substring %q", llm.prompt, expected)
		}
	}
}

func TestAttributeResolverInfersMissingRequiredDisplayAttributeFromContext(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       3002,
						AttributeNameEn:   "Hazard Category",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 701, AttributeValue: "Non-Hazardous", AttributeValueEn: "Non-Hazardous"},
							{AttributeValueID: 702, AttributeValue: "Flammable", AttributeValueEn: "Flammable"},
						},
					},
				},
			}},
		},
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"attribute_id":0,"reasons":["产品类别 is not a display template field"]}`,
			`{"attribute_id":0,"reasons":["材质 alone does not identify the target field here"]}`,
			`{"attribute_id":0,"reasons":["空间 alone does not identify the target field here"]}`,
			`{"attribute_value_id":701,"reasons":["textile cushion signals support non-hazardous classification"]}`,
		},
	})

	pkg := &Package{
		CategoryID: 11814,
		ProductAttributes: []common.Attribute{
			{Name: "产品类别", Value: "椅垫"},
			{Name: "材质", Value: "涤纶"},
			{Name: "空间", Value: "室外,阳台"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 1 {
		t.Fatalf("resolved count = %d, want 1", resolution.ResolvedCount)
	}
	if len(resolution.PendingAttributes) != 0 {
		t.Fatalf("pending attributes = %#v, want none", resolution.PendingAttributes)
	}
	if resolution.ResolvedAttributes[0].MatchedBy != "llm_attribute_inference" {
		t.Fatalf("matched by = %q, want llm_attribute_inference", resolution.ResolvedAttributes[0].MatchedBy)
	}
	if got := resolution.ResolvedAttributes[0].AttributeValueID; got == nil || *got != 701 {
		t.Fatalf("attribute value id = %v, want 701", got)
	}
}

func TestAttributeResolverDoesNotCountEnumeratedAttributeWithoutValueIDAsResolved(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       3001,
						AttributeNameEn:   "Occasion",
						AttributeInputNum: 1,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 901, AttributeValue: "Daily", AttributeValueEn: "Daily"},
							{AttributeValueID: 902, AttributeValue: "Party", AttributeValueEn: "Party"},
						},
					},
				},
			}},
		},
	}, nil)

	pkg := &Package{
		CategoryID: 11814,
		ProductAttributes: []common.Attribute{
			{Name: "空间", Value: "室外,阳台"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 0 {
		t.Fatalf("resolved count = %d, want 0", resolution.ResolvedCount)
	}
	if len(resolution.PendingAttributes) != 1 || resolution.PendingAttributes[0].Name != "Occasion" {
		t.Fatalf("pending attributes = %#v, want Occasion", resolution.PendingAttributes)
	}
}
