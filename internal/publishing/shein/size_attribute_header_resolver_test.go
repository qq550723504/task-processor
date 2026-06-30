package shein

import (
	"context"
	"strings"
	"testing"

	sheinpublishing "task-processor/internal/marketplace/shein/publishing"
)

func TestSizeAttributeHeaderResolverMapsUnknownHeaderFromCandidateList(t *testing.T) {
	t.Parallel()

	llm := &stubSizeAttributeHeaderLLM{
		response: `{"selections":[{"header":"下摆围(cm/in)","attribute_id":58,"confidence":0.92,"reasons":["下摆围 matches Hem measurement"]}]}`,
	}
	resolver := NewSizeAttributeHeaderResolver(llm)

	got := resolver.ResolveSizeAttributeHeaders(SizeAttributeHeaderResolutionInput{
		Context: context.Background(),
		Headers: []string{"下摆围(cm/in)"},
		TemplateAttributes: []sheinpublishing.SizeChartTemplateAttribute{{
			AttributeID:     58,
			AttributeName:   "摆围 (cm)",
			AttributeNameEn: "Hem (cm)",
		}},
	})

	if got.AttributeIDsByHeader["下摆围(cm/in)"] != 58 {
		t.Fatalf("AttributeIDsByHeader = %#v, want header mapped to Hem", got.AttributeIDsByHeader)
	}
	if len(got.ReviewNotes) == 0 || !strings.Contains(got.ReviewNotes[0], "下摆围") {
		t.Fatalf("ReviewNotes = %#v, want LLM mapping note", got.ReviewNotes)
	}
	if !strings.Contains(llm.prompt, `attribute_id=58`) || !strings.Contains(llm.prompt, `下摆围(cm/in)`) {
		t.Fatalf("prompt = %q, want header and candidate id", llm.prompt)
	}
}

func TestSizeAttributeHeaderResolverRejectsInvalidOrLowConfidenceSelection(t *testing.T) {
	t.Parallel()

	resolver := NewSizeAttributeHeaderResolver(&stubSizeAttributeHeaderLLM{
		response: `{"selections":[{"header":"下摆围(cm/in)","attribute_id":999,"confidence":0.99},{"header":"袖肥(cm/in)","attribute_id":58,"confidence":0.42}]}`,
	})

	got := resolver.ResolveSizeAttributeHeaders(SizeAttributeHeaderResolutionInput{
		Context: context.Background(),
		Headers: []string{"下摆围(cm/in)", "袖肥(cm/in)"},
		TemplateAttributes: []sheinpublishing.SizeChartTemplateAttribute{{
			AttributeID:     58,
			AttributeName:   "摆围 (cm)",
			AttributeNameEn: "Hem (cm)",
		}},
	})

	if len(got.AttributeIDsByHeader) != 0 {
		t.Fatalf("AttributeIDsByHeader = %#v, want invalid and low confidence selections rejected", got.AttributeIDsByHeader)
	}
}

type stubSizeAttributeHeaderLLM struct {
	prompt   string
	response string
}

func (s *stubSizeAttributeHeaderLLM) Generate(_ context.Context, prompt string) (string, error) {
	s.prompt = prompt
	return s.response, nil
}
