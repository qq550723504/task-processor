package listingkit

import (
	"context"
	"errors"
	"strings"
	"testing"

	openaiclient "task-processor/internal/infra/clients/openai"
)

type stubReferenceAnalysisCompleter struct {
	responses []string
	errAt     int
	calls     []string
}

func (s *stubReferenceAnalysisCompleter) CreateChatCompletion(context.Context, *openaiclient.ChatCompletionRequest) (*openaiclient.ChatCompletionResponse, error) {
	return nil, errors.New("not used")
}

func (s *stubReferenceAnalysisCompleter) Generate(context.Context, string) (string, error) {
	return "", errors.New("not used")
}

func (s *stubReferenceAnalysisCompleter) AnalyzeImage(_ context.Context, imageURL string, prompt string) (string, error) {
	s.calls = append(s.calls, imageURL+"|"+prompt)
	if s.errAt > 0 && len(s.calls) == s.errAt {
		return "", errors.New("vision failed")
	}
	idx := len(s.calls) - 1
	if idx < len(s.responses) {
		return s.responses[idx], nil
	}
	return `{"motif":"retro flowers","palette":["cream","red"],"composition":"large centered badge","avoid":["logos","exact text"]}`, nil
}

func (s *stubReferenceAnalysisCompleter) GetDefaultModel() string {
	return "vision-test"
}

func TestAnalyzeStudioReferenceStyleRejectsEmptyReferences(t *testing.T) {
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: &stubReferenceAnalysisCompleter{}})

	_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{})
	if err == nil || !strings.Contains(err.Error(), "reference_image_urls is required") {
		t.Fatalf("error = %v, want reference_image_urls validation", err)
	}
}

func TestAnalyzeStudioReferenceStyleLimitsReferencesAndSanitizesPrompt(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"sports mascot","palette":["navy","cream"],"composition":"varsity badge","typography":"bold collegiate","avoid":["Adidas trefoil logo","exact slogan \"Just Do It\"","Mickey Mouse character","Taylor Swift face portrait","same diagonal split layout"]}`,
		`{"motif":"floral border","palette":["red","cream"],"composition":"arched frame","typography":"distressed serif","avoid":["brand mark"]}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png", "https://example.com/b.png", "https://example.com/c.png", "https://example.com/d.png", "https://example.com/e.png", "https://example.com/f.png"},
		ProductName:        "T-shirt",
		CategoryPath:       []string{"Apparel", "Tops"},
		BasePrompt:         "summer",
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}
	if len(completer.calls) != 5 {
		t.Fatalf("calls = %d, want 5", len(completer.calls))
	}
	for _, unsafeToken := range []string{
		"adidas", "trefoil", "just do it", "mickey", "taylor swift", "face portrait", "same diagonal split layout",
	} {
		if strings.Contains(strings.ToLower(resp.SanitizedPrompt), unsafeToken) {
			t.Fatalf("sanitized prompt contains unsafe token %q: %q", unsafeToken, resp.SanitizedPrompt)
		}
	}
	if !strings.Contains(strings.ToLower(resp.SanitizedPrompt), "original") {
		t.Fatalf("sanitized prompt = %q, want originality instruction", resp.SanitizedPrompt)
	}
	if len(resp.Warnings) == 0 {
		t.Fatalf("warnings = nil, want warning for truncated reference list")
	}
}

func TestAnalyzeStudioReferenceStyleFallsBackForMalformedJSON(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{`Use the Adidas trefoil logo, the quote "Just Do It", Elsa's face portrait, and the same split poster layout with retro cherry accents.`}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}
	if !strings.Contains(resp.ReferenceStyleBrief, "Adidas trefoil logo") {
		t.Fatalf("brief = %q, want malformed text retained for diagnostics", resp.ReferenceStyleBrief)
	}
	for _, unsafeToken := range []string{
		"adidas", "trefoil", "just do it", "elsa", "face portrait", "same split poster layout",
	} {
		if strings.Contains(strings.ToLower(resp.SanitizedPrompt), unsafeToken) {
			t.Fatalf("sanitized prompt contains unsafe malformed detail %q: %q", unsafeToken, resp.SanitizedPrompt)
		}
	}
}

func TestAnalyzeStudioReferenceStyleUsesPartialSuccess(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{
		responses: []string{`{"motif":"western floral","palette":["tan","red"],"composition":"center badge"}`},
		errAt:     2,
	}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png", "https://example.com/b.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}
	if len(resp.Warnings) == 0 {
		t.Fatalf("warnings = nil, want partial failure warning")
	}
	if !strings.Contains(resp.SanitizedPrompt, "western floral") {
		t.Fatalf("sanitized prompt = %q, want successful image analysis used", resp.SanitizedPrompt)
	}
}

func TestAnalyzeStudioReferenceStyleKeepsSafeTitleCaseStyleSignals(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"Retro Flowers","palette":["Cream","Cherry Red"],"composition":"Centered Badge","typography":"Old English","density":"Clean Layering","product_fit":"Vintage Streetwear"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	lowerPrompt := strings.ToLower(resp.SanitizedPrompt)
	for _, safeSignal := range []string{
		"retro flowers",
		"cream",
		"centered badge",
		"old english",
		"clean layering",
		"vintage streetwear",
	} {
		if !strings.Contains(lowerPrompt, safeSignal) {
			t.Fatalf("sanitized prompt = %q, want safe style signal %q preserved", resp.SanitizedPrompt, safeSignal)
		}
	}
}
