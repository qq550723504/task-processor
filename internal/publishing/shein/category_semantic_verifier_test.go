package shein

import (
	"context"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
)

type stubCategorySemanticLLM struct {
	response string
	err      error
	prompt   string
}

func (s *stubCategorySemanticLLM) CreateChatCompletion(context.Context, *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return nil, errors.New("not implemented")
}

func (s *stubCategorySemanticLLM) Generate(_ context.Context, prompt string) (string, error) {
	s.prompt = prompt
	return s.response, s.err
}

func (s *stubCategorySemanticLLM) AnalyzeImage(context.Context, string, string) (string, error) {
	return "", errors.New("not implemented")
}

func (s *stubCategorySemanticLLM) GetDefaultModel() string { return "stub" }

func TestAICategorySemanticVerifierBuildsStructuredPrompt(t *testing.T) {
	llm := &stubCategorySemanticLLM{response: `{"verdict":"compatible","reason":"chair cushion semantics match home furnishing"}`}
	verifier := newAICategorySemanticVerifier(llm)
	validation := verifier.ValidateProductCategory(context.Background(), &canonical.Product{
		Title:       "New Women's Summer Thin Ice Silk Pajamas",
		Description: "Outdoor garden bench cushion for hanging chair and balcony seating",
		Attributes: map[string]canonical.Attribute{
			"产品类别": {Value: "椅垫"},
			"空间":   {Value: "室外,阳台"},
			"材质":   {Value: "涤纶"},
		},
	}, &Package{
		Attributes: map[string]string{
			"产品类别": "椅垫",
			"空间":   "室外,阳台",
		},
	}, []string{"家居&生活", "厨房&餐厅", "餐桌装饰品&餐厨布艺", "椅垫"})

	if validation == nil || validation.Verdict != "compatible" {
		t.Fatalf("validation = %+v, want compatible", validation)
	}
	for _, expected := range []string{"产品类别", "椅垫", "室外,阳台", "椅垫", "New Women's Summer Thin Ice Silk Pajamas"} {
		if !strings.Contains(llm.prompt, expected) {
			t.Fatalf("prompt = %q, want to contain %q", llm.prompt, expected)
		}
	}
}

func TestAICategorySemanticVerifierAvoidsNoisyDescriptionWhenStructuredSignalsExist(t *testing.T) {
	llm := &stubCategorySemanticLLM{response: `{"verdict":"compatible","reason":"structured signals clearly indicate an outdoor cushion"}`}
	verifier := newAICategorySemanticVerifier(llm)
	_ = verifier.ValidateProductCategory(context.Background(), &canonical.Product{
		Title:       "Outdoor Bench Cushion",
		Description: "iCOSS Smart Toilet Seat Bidet Attachment",
		Attributes: map[string]canonical.Attribute{
			"产品类别": {Value: "椅垫"},
			"空间":   {Value: "室外,阳台"},
			"材质":   {Value: "涤纶"},
		},
		VariantDimensions: []canonical.ScrapedVariantDimension{
			{Name: "尺寸", Values: []string{"150*100*10CM"}},
		},
	}, &Package{
		Attributes: map[string]string{
			"产品类别": "椅垫",
			"空间":   "室外,阳台",
		},
	}, []string{"工具&家装", "户外家具&配件", "庭院坐垫"})

	if strings.Contains(llm.prompt, "Smart Toilet Seat Bidet Attachment") {
		t.Fatalf("prompt should avoid noisy free-form description when structured signals exist: %q", llm.prompt)
	}
}

func TestBuildCategoryFamilyConflictSummaryUsesSemanticValidation(t *testing.T) {
	recommend, reason := buildCategoryFamilyConflictSummary(&canonical.Product{
		Title: "Outdoor bench cushion for hanging chair",
	}, &Package{
		CategoryPath: []string{"女士服装", "女士制服&特殊服饰", "女士装扮服饰&角色扮演服饰", "角色扮演服饰"},
		CategoryResolution: &CategoryResolution{
			MatchedPath: []string{"女士服装", "女士制服&特殊服饰", "女士装扮服饰&角色扮演服饰", "角色扮演服饰"},
			SemanticValidation: &CategorySemanticValidation{
				Verdict: "incompatible",
				Reason:  "AI 判断当前类目路径与商品语义不一致",
			},
		},
	})
	if !recommend {
		t.Fatal("expected semantic validation to trigger category review")
	}
	if !strings.Contains(reason, "语义不一致") {
		t.Fatalf("reason = %q, want semantic mismatch hint", reason)
	}
}

func TestBuildCategoryFamilyConflictSummaryTrustsCompatibleSemanticValidation(t *testing.T) {
	recommend, reason := buildCategoryFamilyConflictSummary(&canonical.Product{
		Title: "Washed denim hat",
	}, &Package{
		CategoryPath: []string{"服饰装饰品", "女士配饰", "女士帽子", "女士棒球帽"},
		CategoryResolution: &CategoryResolution{
			MatchedPath: []string{"服饰装饰品", "女士配饰", "女士帽子", "女士棒球帽"},
			SemanticValidation: &CategorySemanticValidation{
				Verdict: "compatible",
				Reason:  "denim hat semantics fit baseball caps",
			},
		},
	})
	if recommend {
		t.Fatalf("recommend = true, reason=%q; compatible semantic validation should not be re-blocked by leaf token mismatch", reason)
	}
}
