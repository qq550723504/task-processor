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
	maxTokens int
	maxTokensSet bool
}

func (s *captureAttributeLLM) CreateChatCompletion(_ context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	prompt := ""
	if req != nil && len(req.Messages) > 0 {
		prompt = req.Messages[0].Content
	}
	if req != nil && req.MaxTokens != nil {
		s.maxTokens = *req.MaxTokens
		s.maxTokensSet = true
	}
	content, err := s.Generate(context.Background(), prompt)
	if err != nil {
		return nil, err
	}
	return &openaiclient.ChatCompletionResponse{
		Choices: []openaiclient.ChatCompletionChoice{{
			Message: openaiclient.ChatCompletionMessage{Role: "assistant", Content: content},
		}},
	}, nil
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

func (s *scriptedAttributeLLM) CreateChatCompletion(_ context.Context, req *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	prompt := ""
	if req != nil && len(req.Messages) > 0 {
		prompt = req.Messages[0].Content
	}
	content, err := s.Generate(context.Background(), prompt)
	if err != nil {
		return nil, err
	}
	return &openaiclient.ChatCompletionResponse{
		Choices: []openaiclient.ChatCompletionChoice{{
			Message: openaiclient.ChatCompletionMessage{Role: "assistant", Content: content},
		}},
	}, nil
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

func findResolvedAttributeForTest(attributes []ResolvedAttribute, attributeID int) *ResolvedAttribute {
	for i := range attributes {
		if attributes[i].AttributeID == attributeID {
			return &attributes[i]
		}
	}
	return nil
}

func TestDisplayAttributeTemplateBatchDoesNotLimitOutputBudget(t *testing.T) {
	llm := &captureAttributeLLM{responses: []string{`{"selections":[]}`}}

	if _, err := generateDisplayAttributeTemplateBatch(context.Background(), llm, "prompt"); err != nil {
		t.Fatalf("generate batch = %v", err)
	}
	if llm.maxTokensSet {
		t.Fatalf("max tokens = %d, want unset for maximum provider output budget", llm.maxTokens)
	}

	promptText := buildDisplayAttributeTemplateBatchPrompt([]sheinattribute.AttributeInfo{{
		AttributeID:       1000546,
		AttributeNameEn:   "Product Model",
		AttributeInputNum: 1,
	}}, []common.Attribute{{Name: "variant_sku", Value: "MG17701061001"}})
	if strings.Contains(promptText, "compact JSON") || strings.Contains(promptText, "reason short") {
		t.Fatalf("prompt = %q, want no compact/short output instruction", promptText)
	}
	if !strings.Contains(promptText, "complete JSON") {
		t.Fatalf("prompt = %q, want complete JSON instruction", promptText)
	}
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
			`{"selections":[{"attribute_id":112,"attribute_value_id":5930427,"reasons":["mesh fabric is the closest match"]}]}`,
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
	if resolution.ResolvedAttributes[0].MatchedBy != "llm_attribute_template_batch" {
		t.Fatalf("matched by = %q, want llm_attribute_template_batch", resolution.ResolvedAttributes[0].MatchedBy)
	}
}

func TestAttributeResolverAcceptsBatchTextValueAlias(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:       1000546,
						AttributeName:     "产品型号",
						AttributeNameEn:   "Product Model",
						AttributeType:     4,
						AttributeInputNum: 1,
					},
				},
			}},
		},
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"selections":[{"attribute_id":1000546,"attribute_value_id":0,"text_value":"MG17701061001","reasons":["model comes from variant sku"]}]}`,
		},
	})

	pkg := &Package{
		CategoryID: 3105,
		ProductAttributes: []common.Attribute{
			{Name: "variant_sku", Value: "MG17701061001"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.Status != "resolved" {
		t.Fatalf("status = %q, want resolved; notes=%v", resolution.Status, resolution.ReviewNotes)
	}
	if resolution.ResolvedCount != 1 {
		t.Fatalf("resolved count = %d, want 1", resolution.ResolvedCount)
	}
	got := findResolvedAttributeForTest(resolution.ResolvedAttributes, 1000546)
	if got == nil {
		t.Fatalf("resolved attributes = %#v, want Product Model", resolution.ResolvedAttributes)
	}
	if got.Value != "MG17701061001" || got.AttributeExtraValue != "MG17701061001" {
		t.Fatalf("resolved text = value %q extra %q, want MG17701061001", got.Value, got.AttributeExtraValue)
	}
	if got.MatchedBy != "llm_attribute_template_batch" {
		t.Fatalf("matched by = %q, want llm_attribute_template_batch", got.MatchedBy)
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
			`{"selections":[{"attribute_id":112,"attribute_value_id":5930427,"reasons":["flyknit best aligns with mesh fabric template value"]},{"attribute_id":1000003,"attribute_value_id":6001001,"reasons":["mesh aligns with mesh fabric template value"]}]}`,
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
	if upper.MatchedBy != "llm_attribute_template_batch" {
		t.Fatalf("Upper Material matched by = %q, want llm_attribute_template_batch", upper.MatchedBy)
	}

	lining, ok := got["Lining Material"]
	if !ok {
		t.Fatalf("missing Lining Material resolution: %#v", resolution.ResolvedAttributes)
	}
	if lining.AttributeValueID == nil || *lining.AttributeValueID != 6001001 {
		t.Fatalf("Lining Material value id = %v, want 6001001", lining.AttributeValueID)
	}
	if lining.MatchedBy != "llm_attribute_template_batch" {
		t.Fatalf("Lining Material matched by = %q, want llm_attribute_template_batch", lining.MatchedBy)
	}
}

func TestAttributeResolverMarksMissingRequiredDisplayAttributesAsPending(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:     118,
						AttributeName:   "宽度",
						AttributeNameEn: "Width (cm)",
						AttributeType:   2,
						AttributeStatus: 3,
					},
					{
						AttributeID:     160,
						AttributeName:   "材质",
						AttributeNameEn: "Material",
						AttributeType:   4,
						AttributeStatus: 3,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 1001, AttributeValue: "棉", AttributeValueEn: "Cotton"},
						},
					},
				},
			}},
		},
	}, &scriptedAttributeLLM{
		responses: []string{
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
						AttributeID:     160,
						AttributeNameEn: "Material",
						AttributeType:   4,
						AttributeStatus: 3,
						AttributeValueInfoList: []sheinattribute.AttributeValue{
							{AttributeValueID: 11, AttributeValue: "棉", AttributeValueEn: "Cotton"},
							{AttributeValueID: 13, AttributeValue: "麻", AttributeValueEn: "Linen"},
						},
					},
					{
						AttributeID:                 1000547,
						AttributeNameEn:             "Other Material",
						AttributeType:               4,
						AttributeStatus:             3,
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
							AttributeID:     160,
							AttributeNameEn: "Material",
							AttributeType:   4,
							AttributeStatus: 3,
							AttributeValueInfoList: []sheinattribute.AttributeValue{
								{AttributeValueID: 11, AttributeValue: "棉", AttributeValueEn: "Cotton"},
								{AttributeValueID: 13, AttributeValue: "麻", AttributeValueEn: "Linen"},
							},
						},
						{
							AttributeID:                 1000547,
							AttributeNameEn:             "Other Material",
							AttributeType:               4,
							AttributeStatus:             3,
							CascadeAttributeID:          160,
							CascadeAttributeValueIDList: &cascadeValues,
						},
					},
				}},
			},
		}, &scriptedAttributeLLM{
			responses: []string{
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
			`{"selections":[{"attribute_id":160,"attribute_value_id":11,"reasons":["棉 matches cotton"]},{"attribute_id":1000547,"attribute_value_id":22,"reasons":["麻 matches linen"]}]}`,
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
			`{"selections":[{"attribute_id":160,"attribute_value_id":2001,"reasons":["涤纶 matches polyester"]}]}`,
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

func TestAttributeResolverDoesNotMatchPolyesterAliasesWithoutLLM(t *testing.T) {
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
	if resolution.ResolvedCount != 0 {
		t.Fatalf("resolved count = %d, want 0; notes=%#v", resolution.ResolvedCount, resolution.ReviewNotes)
	}
}

func TestAttributeResolverSkipsOptionalOccasionAliasFromSceneWithoutExactMatch(t *testing.T) {
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
	}, &scriptedAttributeLLM{})

	pkg := &Package{
		CategoryID: 11814,
		ProductAttributes: []common.Attribute{
			{Name: "场景", Value: "室外"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 0 {
		t.Fatalf("resolved count = %d, want 0 for optional alias-only field", resolution.ResolvedCount)
	}
}

func TestAttributeResolverSkipsOptionalRoomAliasFromSpaceWithoutExactMatch(t *testing.T) {
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
	}, &scriptedAttributeLLM{})

	pkg := &Package{
		CategoryID: 11814,
		ProductAttributes: []common.Attribute{
			{Name: "空间", Value: "室外"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 0 {
		t.Fatalf("resolved count = %d, want 0 for optional alias-only field", resolution.ResolvedCount)
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

func TestAttributeResolverSkipsOptionalAliasPromptExpansion(t *testing.T) {
	llm := &captureAttributeLLM{}
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
	if resolution.ResolvedCount != 0 {
		t.Fatalf("resolved count = %d, want 0 for optional alias-only field", resolution.ResolvedCount)
	}
	if strings.TrimSpace(llm.prompt) != "" {
		t.Fatalf("prompt = %q, want empty because optional alias-only field should not call LLM", llm.prompt)
	}
}

func TestAttributeResolverInfersMissingRequiredDisplayAttributeFromContext(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:     3002,
						AttributeNameEn: "Hazard Category",
						AttributeStatus: 3,
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
			`{"selections":[{"attribute_id":3002,"attribute_value_id":701,"reasons":["textile cushion signals support non-hazardous classification"]}]}`,
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
	if resolution.ResolvedAttributes[0].MatchedBy != "llm_attribute_template_batch" {
		t.Fatalf("matched by = %q, want llm_attribute_template_batch", resolution.ResolvedAttributes[0].MatchedBy)
	}
	if got := resolution.ResolvedAttributes[0].AttributeValueID; got == nil || *got != 701 {
		t.Fatalf("attribute value id = %v, want 701", got)
	}
}

func TestMissingDisplayAttributeValuePromptAllowsNeutralRequiredInference(t *testing.T) {
	attr := sheinattribute.AttributeInfo{
		AttributeID:     77,
		AttributeNameEn: "Season",
		AttributeStatus: 3,
		AttributeValueInfoList: []sheinattribute.AttributeValue{
			{AttributeValueID: 654, AttributeValue: "夏", AttributeValueEn: "Summer"},
			{AttributeValueID: 1601, AttributeValue: "ALL/全球/所有", AttributeValueEn: "All"},
		},
	}
	inputs := []common.Attribute{
		{Name: "product_english_name", Value: "Washed denim hat"},
		{Name: "material", Value: "100% cotton"},
		{Name: "sku", Value: "MG8012002"},
		{Name: "picture_request", Value: "1000 px * 562 px"},
		{Name: "is_electricity", Value: "0"},
		{Name: "production_process", Value: "烫画"},
		{Name: "design_area", Value: "区域印制"},
		{Name: "applicable_scenarios", Value: "户外,运动,棒球"},
	}

	prompt := buildMissingDisplayAttributeInferencePrompt(attr, inputs)
	for _, expected := range []string{
		"product semantics",
		"required=true",
		`product_english_name="Washed denim hat"`,
		`production_process="烫画"`,
		`applicable_scenarios="户外,运动,棒球"`,
		`attribute_value_id=1601 value="ALL/全球/所有" value_en="All"`,
	} {
		if !strings.Contains(prompt, expected) {
			t.Fatalf("prompt = %q, want substring %q", prompt, expected)
		}
	}
}

func TestAttributeResolverTemplateBatchInfersRemainingAttributes(t *testing.T) {
	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     1001519,
			AttributeNameEn: "Element",
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 8790846, AttributeValue: "印刷", AttributeValueEn: "Printing"},
			},
		},
		{
			AttributeID:     77,
			AttributeNameEn: "Season",
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 284, AttributeValue: "秋", AttributeValueEn: "Fall"},
				{AttributeValueID: 654, AttributeValue: "夏", AttributeValueEn: "Summer"},
				{AttributeValueID: 1601, AttributeValue: "ALL/全球/所有", AttributeValueEn: "All"},
			},
		},
	}
	inputs := []common.Attribute{
		{Name: "material", Value: "100%纯棉"},
		{Name: "production_process", Value: "烫画"},
		{Name: "design_area", Value: "区域印制"},
		{Name: "product_english_name", Value: "Washed denim hat"},
	}
	llm := &scriptedAttributeLLM{
		responses: []string{
			`{"selections":[{"attribute_id":1001519,"attribute_value_id":8790846,"reasons":["production process and design area support Printing"]},{"attribute_id":77,"attribute_value_id":1601,"reasons":["hat is not season-limited, All is safest"]}]}`,
		},
	}
	resolvedByID := map[int]ResolvedAttribute{}

	resolved, notes := inferDisplayAttributesTemplateBatch(attributes, inputs, resolvedByID, llm)
	if len(resolved) != 2 {
		t.Fatalf("resolved = %+v, notes=%+v, want 2", resolved, notes)
	}
	if got := findResolvedAttributeForTest(resolved, 1001519); got == nil || got.AttributeValueID == nil || *got.AttributeValueID != 8790846 || got.MatchedBy != "llm_attribute_template_batch" {
		t.Fatalf("element resolution = %+v, want template batch Printing", got)
	}
	if got := findResolvedAttributeForTest(resolved, 77); got == nil || got.AttributeValueID == nil || *got.AttributeValueID != 1601 || got.MatchedBy != "llm_attribute_template_batch" {
		t.Fatalf("season resolution = %+v, want template batch All", got)
	}
	if !strings.Contains(llm.prompts[0], `production_process="烫画"`) {
		t.Fatalf("prompt = %q, want production process context", llm.prompts[0])
	}
}

func TestAttributeResolverRepairsRemainingRequiredAttributes(t *testing.T) {
	attributes := []sheinattribute.AttributeInfo{
		{
			AttributeID:     101,
			AttributeNameEn: "Style",
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 167, AttributeValue: "Casual休闲", AttributeValueEn: "Casual"},
				{AttributeValueID: 2491, AttributeValue: "派对", AttributeValueEn: "Party"},
			},
		},
		{
			AttributeID:     77,
			AttributeNameEn: "Season",
			AttributeStatus: 3,
			AttributeValueInfoList: []sheinattribute.AttributeValue{
				{AttributeValueID: 654, AttributeValue: "夏", AttributeValueEn: "Summer"},
				{AttributeValueID: 1601, AttributeValue: "ALL/全球/所有", AttributeValueEn: "All"},
			},
		},
	}
	inputs := []common.Attribute{
		{Name: "product_english_name", Value: "Washed denim hat"},
		{Name: "material", Value: "100% cotton"},
		{Name: "applicable_scenarios", Value: "outdoor, sport, baseball, cycling"},
	}
	llm := &scriptedAttributeLLM{
		responses: []string{
			`{"attribute_value_id":167,"reasons":["Casual is the broad neutral style candidate"]}`,
			`{"attribute_value_id":1601,"reasons":["All is the broad all-season candidate"]}`,
		},
	}
	resolvedByID := map[int]ResolvedAttribute{}

	resolved, notes := inferMissingRequiredDisplayAttributesRepair(attributes, inputs, resolvedByID, llm)
	if len(resolved) != 2 {
		t.Fatalf("resolved = %+v, notes=%+v, want 2", resolved, notes)
	}
	if got := findResolvedAttributeForTest(resolved, 101); got == nil || got.AttributeValueID == nil || *got.AttributeValueID != 167 || got.MatchedBy != "llm_required_attribute_repair" {
		t.Fatalf("style resolution = %+v, want required repair Casual", got)
	}
	if got := findResolvedAttributeForTest(resolved, 77); got == nil || got.AttributeValueID == nil || *got.AttributeValueID != 1601 || got.MatchedBy != "llm_required_attribute_repair" {
		t.Fatalf("season resolution = %+v, want required repair All", got)
	}
	if len(llm.prompts) != 2 {
		t.Fatalf("repair prompt count = %d, want 2", len(llm.prompts))
	}
	if !strings.Contains(llm.prompts[0], "required by the live SHEIN template") {
		t.Fatalf("prompt = %q, want required repair framing", llm.prompts[0])
	}
}

func TestAttributeResolverInfersImportantTextAttributeFromContext(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:     1000546,
						AttributeName:   "产品型号",
						AttributeNameEn: "Product Model",
						AttributeLabel:  1,
					},
				},
			}},
		},
	}, &scriptedAttributeLLM{
		responses: []string{
			`{"selections":[{"attribute_id":1000546,"attribute_extra_value":"MG17701062","reasons":["source sku is an explicit product identifier"]}]}`,
		},
	})

	pkg := &Package{
		CategoryID: 3105,
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "MG17701062"},
		},
	}

	resolution := resolver.Resolve(&BuildRequest{}, &productenrich.CanonicalProduct{}, pkg)
	if resolution.ResolvedCount != 1 {
		t.Fatalf("resolved count = %d, want 1; resolution=%+v", resolution.ResolvedCount, resolution)
	}
	got := resolution.ResolvedAttributes[0]
	if got.AttributeID != 1000546 || got.AttributeExtraValue != "MG17701062" {
		t.Fatalf("resolved attribute = %+v, want product model text value", got)
	}
	if got.MatchedBy != "llm_attribute_template_batch" {
		t.Fatalf("matched by = %q, want llm_attribute_template_batch", got.MatchedBy)
	}
}

func TestMissingDisplayAttributeTextPromptIncludesAllSourceAttributes(t *testing.T) {
	attr := sheinattribute.AttributeInfo{
		AttributeID:     1000546,
		AttributeNameEn: "Product Model",
		AttributeLabel:  1,
	}
	inputs := []common.Attribute{
		{Name: "material", Value: "wood"},
		{Name: "production_process", Value: "UV"},
		{Name: "design_area", Value: "full"},
		{Name: "picture_request", Value: "1500 px * 1500 px"},
		{Name: "applicable_scenarios", Value: "office"},
		{Name: "washing_instructions", Value: "wipe clean"},
		{Name: "is_electricity", Value: "0"},
		{Name: "sku", Value: "MG17701062"},
	}

	prompt := buildMissingDisplayAttributeTextPrompt(attr, inputs)
	if !strings.Contains(prompt, `sku="MG17701062"`) {
		t.Fatalf("prompt = %q, want sku context", prompt)
	}
}

func TestAttributeResolverDoesNotCountEnumeratedAttributeWithoutValueIDAsResolved(t *testing.T) {
	resolver := NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{
					{
						AttributeID:     3001,
						AttributeNameEn: "Occasion",
						AttributeStatus: 3,
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
