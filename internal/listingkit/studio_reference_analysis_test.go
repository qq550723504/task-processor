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

func TestAnalyzeStudioReferenceStyleAcceptsPublicHTTPSReferenceImages(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"retro flowers","palette":["cream"],"composition":"centered badge"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/reference.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}
	if len(resp.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want no validation warning", resp.Warnings)
	}
	if len(completer.calls) != 1 || !strings.HasPrefix(completer.calls[0], "https://example.com/reference.png|") {
		t.Fatalf("AnalyzeImage calls = %#v, want original public https URL", completer.calls)
	}
}

func TestAnalyzeStudioReferenceStyleRejectsInvalidReferenceImageURLs(t *testing.T) {
	testCases := []struct {
		name string
		urls []string
	}{
		{name: "http url", urls: []string{"http://example.com/reference.png"}},
		{name: "non upload relative path", urls: []string{"/images/reference.png"}},
		{name: "malformed absolute url", urls: []string{"https://"}},
		{name: "non absolute text", urls: []string{"example.com/reference.png"}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: &stubReferenceAnalysisCompleter{}})

			_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
				ReferenceImageURLs: tc.urls,
			})
			if err == nil {
				t.Fatal("AnalyzeStudioReferenceStyle() error = nil, want invalid request")
			}
			if !strings.Contains(err.Error(), "invalid request") {
				t.Fatalf("error = %v, want invalid request", err)
			}
		})
	}
}

func TestAnalyzeStudioReferenceStyleResolvesUploadedReferencePathsToPublicHTTPS(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"retro flowers","palette":["cream"],"composition":"centered badge"}`,
		`{"motif":"floral border","palette":["red"],"composition":"arched frame"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{
		promptDiversifier: completer,
		resolveUploadedImagePublicURL: func(_ context.Context, key string) (string, error) {
			switch key {
			case "folder/ref-a.png":
				return "https://cdn.example.com/folder/ref-a.png", nil
			case "folder/ref-b.png":
				return "https://cdn.example.com/folder/ref-b.png", nil
			default:
				return "", ErrUploadedImageNotFound
			}
		},
	})

	_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{
			" /api/v1/listing-kits/uploads/files/folder/ref-a.png ",
			"/api/v1/listing-kits/uploads/files/folder/ref-a.png",
			"/api/v1/listing-kits/uploads/files/folder/ref-b.png",
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	wantURLs := []string{
		"https://cdn.example.com/folder/ref-a.png",
		"https://cdn.example.com/folder/ref-b.png",
	}
	if len(completer.calls) != len(wantURLs) {
		t.Fatalf("calls = %d, want %d", len(completer.calls), len(wantURLs))
	}
	for i, wantURL := range wantURLs {
		gotURL := strings.SplitN(completer.calls[i], "|", 2)[0]
		if gotURL != wantURL {
			t.Fatalf("call[%d] imageURL = %q, want %q", i, gotURL, wantURL)
		}
	}
}

func TestAnalyzeStudioReferenceStyleResolvesAbsoluteUploadedReferenceURLsToPublicHTTPS(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"retro flowers","palette":["cream"],"composition":"centered badge"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{
		promptDiversifier: completer,
		resolveUploadedImagePublicURL: func(_ context.Context, key string) (string, error) {
			if key != "2026/a.png" {
				t.Fatalf("unexpected key = %q", key)
			}
			return "https://cdn.example.com/uploads/2026/a.png", nil
		},
	})

	_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{
			"http://localhost:8080/api/v1/listing-kits/uploads/files/2026/a.png",
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	if len(completer.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(completer.calls))
	}
	gotURL := strings.SplitN(completer.calls[0], "|", 2)[0]
	if gotURL != "https://cdn.example.com/uploads/2026/a.png" {
		t.Fatalf("AnalyzeImage imageURL = %q, want resolved CDN https URL", gotURL)
	}
}

func TestAnalyzeStudioReferenceStyleTreatsRemoteUploadShapedHTTPSURLsAsExternal(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"retro flowers","palette":["cream"],"composition":"centered badge"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{
		promptDiversifier: completer,
		resolveUploadedImagePublicURL: func(_ context.Context, key string) (string, error) {
			t.Fatalf("resolveUploadedImagePublicURL should not be called for remote host key %q", key)
			return "", nil
		},
	})

	rawURL := "https://assets.example.com/api/v1/listing-kits/uploads/files/folder/reference.png"
	_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{rawURL},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	if len(completer.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(completer.calls))
	}
	gotURL := strings.SplitN(completer.calls[0], "|", 2)[0]
	if gotURL != rawURL {
		t.Fatalf("AnalyzeImage imageURL = %q, want unchanged remote HTTPS URL %q", gotURL, rawURL)
	}
}

func TestAnalyzeStudioReferenceStyleRejectsUploadedReferencePathsWithoutPublicHTTPS(t *testing.T) {
	testCases := []struct {
		name       string
		publicURL  string
		resolveErr error
	}{
		{name: "missing public url"},
		{name: "non https public url", publicURL: "http://localhost:3000/uploads/folder/ref-a.png"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			completer := &stubReferenceAnalysisCompleter{}
			svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{
				promptDiversifier: completer,
				resolveUploadedImagePublicURL: func(_ context.Context, key string) (string, error) {
					if key != "folder/ref-a.png" {
						t.Fatalf("unexpected key = %q", key)
					}
					return tc.publicURL, tc.resolveErr
				},
			})

			_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
				ReferenceImageURLs: []string{"/api/v1/listing-kits/uploads/files/folder/ref-a.png"},
			})
			if err == nil {
				t.Fatal("AnalyzeStudioReferenceStyle() error = nil, want invalid request")
			}
			if !strings.Contains(err.Error(), "invalid request") {
				t.Fatalf("error = %v, want invalid request", err)
			}
			if len(completer.calls) != 0 {
				t.Fatalf("AnalyzeImage calls = %#v, want no analysis call", completer.calls)
			}
		})
	}
}

func TestAnalyzeStudioReferenceStyleNormalizesHTTPSReferenceURLs(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"retro flowers","palette":["cream"],"composition":"centered badge"}`,
		`{"motif":"watercolor texture","palette":["coral"],"composition":"arched floral frame"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{
			"  https://example.com/folder/ref-a.png  ",
			"https://example.com/folder/ref-a.png",
			"https://cdn.example.com/folder/ref-b.png",
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	wantURLs := []string{
		"https://example.com/folder/ref-a.png",
		"https://cdn.example.com/folder/ref-b.png",
	}
	if len(completer.calls) != len(wantURLs) {
		t.Fatalf("calls = %d, want %d", len(completer.calls), len(wantURLs))
	}
	for i, wantURL := range wantURLs {
		gotURL := strings.SplitN(completer.calls[i], "|", 2)[0]
		if gotURL != wantURL {
			t.Fatalf("call[%d] imageURL = %q, want %q", i, gotURL, wantURL)
		}
	}
}

func TestAnalyzeStudioReferenceStyleLimitsReferencesAndSanitizesPrompt(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"Hello Kitty bow","palette":["navy","cream"],"composition":"diagonal split badge","typography":"Old English","density":"Clean Layering","product_fit":"Vintage Streetwear","avoid":["Adidas trefoil logo","exact slogan \"Just Do It\"","Mickey Mouse character","Taylor Swift face portrait","same diagonal split layout"]}`,
		`{"motif":"floral border","palette":["red","cream"],"composition":"arched frame","typography":"distressed serif","avoid":["brand mark"]}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{
			"https://example.com/a.png",
			"https://example.com/b.png",
			"https://example.com/c.png",
			"https://example.com/d.png",
			"https://example.com/e.png",
			"https://example.com/f.png",
		},
		ProductName:  "T-shirt",
		CategoryPath: []string{"Apparel", "Tops"},
		BasePrompt:   "summer",
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}
	if len(completer.calls) != 5 {
		t.Fatalf("calls = %d, want 5", len(completer.calls))
	}
	for _, unsafeToken := range []string{
		"hello kitty", "diagonal split badge", "adidas", "trefoil", "just do it", "mickey", "taylor swift", "face portrait", "same diagonal split layout",
	} {
		if strings.Contains(strings.ToLower(resp.SanitizedPrompt), unsafeToken) {
			t.Fatalf("sanitized prompt contains unsafe token %q: %q", unsafeToken, resp.SanitizedPrompt)
		}
		if strings.Contains(strings.ToLower(resp.ReferenceStyleBrief), unsafeToken) {
			t.Fatalf("reference style brief contains unsafe token %q: %q", unsafeToken, resp.ReferenceStyleBrief)
		}
	}
	if !strings.Contains(strings.ToLower(resp.SanitizedPrompt), "original") {
		t.Fatalf("sanitized prompt = %q, want originality instruction", resp.SanitizedPrompt)
	}
	for _, safeSignal := range []string{
		"old english",
		"clean layering",
		"vintage streetwear",
		"cream",
		"framed composition",
	} {
		if !strings.Contains(strings.ToLower(resp.SanitizedPrompt), safeSignal) {
			t.Fatalf("sanitized prompt = %q, want safe signal %q preserved", resp.SanitizedPrompt, safeSignal)
		}
	}
	if len(resp.Warnings) == 0 {
		t.Fatalf("warnings = nil, want warning for truncated reference list")
	}
	if !containsWarningFragment(resp.Warnings, "已移除品牌、Logo、原文案或过于接近原图的描述") {
		t.Fatalf("warnings = %#v, want unsafe-signal warning", resp.Warnings)
	}
}

func TestAnalyzeStudioReferenceStyleFallsBackForMalformedJSON(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{`Use the Adidas trefoil logo, the quote "Just Do It", Elsa's face portrait, and the same split poster layout with Hello Kitty bow accents.`}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}
	for _, unsafeToken := range []string{
		"adidas", "trefoil", "just do it", "elsa", "face portrait", "same split poster layout", "hello kitty", "bow",
	} {
		if strings.Contains(strings.ToLower(resp.SanitizedPrompt), unsafeToken) {
			t.Fatalf("sanitized prompt contains unsafe malformed detail %q: %q", unsafeToken, resp.SanitizedPrompt)
		}
		if strings.Contains(strings.ToLower(resp.ReferenceStyleBrief), unsafeToken) {
			t.Fatalf("reference style brief contains unsafe malformed detail %q: %q", unsafeToken, resp.ReferenceStyleBrief)
		}
	}
	if !containsWarningFragment(resp.Warnings, "已移除品牌、Logo、原文案或过于接近原图的描述") {
		t.Fatalf("warnings = %#v, want unsafe malformed warning", resp.Warnings)
	}
}

func TestAnalyzeStudioReferenceStyleDerivesFallbackForSafeOffVocabularyMalformedText(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		"souvenir keepsake vibe, breezy travel poster energy, softened ink haze",
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v, want safe malformed fallback to degrade instead of failing", err)
	}
	if strings.TrimSpace(resp.ReferenceStyleBrief) == "" {
		t.Fatal("reference style brief is empty, want conservative fallback brief")
	}
	if strings.TrimSpace(resp.SanitizedPrompt) == "" {
		t.Fatal("sanitized prompt is empty, want conservative fallback prompt")
	}
	if !strings.Contains(strings.ToLower(resp.ReferenceStyleBrief), "abstract") {
		t.Fatalf("reference style brief = %q, want conservative derived fallback signal", resp.ReferenceStyleBrief)
	}
	if !strings.Contains(strings.ToLower(resp.SanitizedPrompt), "abstract") {
		t.Fatalf("sanitized prompt = %q, want conservative derived fallback signal", resp.SanitizedPrompt)
	}
	if !strings.Contains(strings.ToLower(resp.SanitizedPrompt), "original") {
		t.Fatalf("sanitized prompt = %q, want originality guardrail preserved", resp.SanitizedPrompt)
	}
	if !containsWarningFragment(resp.Warnings, "部分参考图返回了非结构化分析结果，仅保留可安全复用的风格提示。") {
		t.Fatalf("warnings = %#v, want malformed fallback warning", resp.Warnings)
	}
}

func TestAnalyzeStudioReferenceStyleRecoversStructuredCuesFromSafeMalformedText(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{"distressed serif, clean layering, vintage streetwear"}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	for _, expectedSection := range []string{
		"typography feel: distressed serif.",
		"visual density: clean layering.",
		"product fit: vintage streetwear.",
	} {
		if !strings.Contains(strings.ToLower(resp.ReferenceStyleBrief), expectedSection) {
			t.Fatalf("reference style brief = %q, want malformed safe cue section %q", resp.ReferenceStyleBrief, expectedSection)
		}
	}
	for _, expectedSection := range []string{
		"typography feel: distressed serif.",
		"visual density: clean layering.",
		"product fit: vintage streetwear.",
	} {
		if !strings.Contains(strings.ToLower(resp.SanitizedPrompt), expectedSection) {
			t.Fatalf("sanitized prompt = %q, want malformed safe cue section %q", resp.SanitizedPrompt, expectedSection)
		}
	}
	if !containsWarningFragment(resp.Warnings, "部分参考图返回了非结构化分析结果，仅保留可安全复用的风格提示。") {
		t.Fatalf("warnings = %#v, want malformed fallback warning", resp.Warnings)
	}
}

func TestAnalyzeStudioReferenceStyleKeepsReferenceBriefDerivedOnlyFromReferenceSignals(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"Retro Flowers","palette":["Cream","Cherry Red"],"composition":"Centered Badge","typography":"Old English"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png"},
		ProductName:        "T-shirt",
		CategoryPath:       []string{"Apparel", "Tops"},
		BasePrompt:         "summer resort capsule",
		UserInstruction:    "keep it cheerful",
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	lowerBrief := strings.ToLower(resp.ReferenceStyleBrief)
	for _, forbidden := range []string{
		"base product:",
		"category:",
		"t-shirt",
		"apparel > tops",
		"create a new original design",
		"do not reproduce logos",
	} {
		if strings.Contains(lowerBrief, forbidden) {
			t.Fatalf("reference style brief = %q, want extracted reference brief only without %q", resp.ReferenceStyleBrief, forbidden)
		}
	}

	lowerPrompt := strings.ToLower(resp.SanitizedPrompt)
	for _, required := range []string{
		"original",
		"brand-neutral",
		"fresh custom wording",
		"avoid recognizable characters or people",
		"clearly original layout",
	} {
		if !strings.Contains(lowerPrompt, required) {
			t.Fatalf("sanitized prompt = %q, want guardrail %q preserved", resp.SanitizedPrompt, required)
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
		ReferenceImageURLs: []string{
			"https://example.com/a.png",
			"https://example.com/b.png",
		},
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
		"centered composition",
		"badge composition",
		"old english",
		"clean layering",
		"vintage streetwear",
	} {
		if !strings.Contains(lowerPrompt, safeSignal) {
			t.Fatalf("sanitized prompt = %q, want safe style signal %q preserved", resp.SanitizedPrompt, safeSignal)
		}
	}
}

func TestAnalyzeStudioReferenceStyleDropsUnknownTitleCaseNamedPhrases(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"Old Navy","typography":"Old English"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/title-case.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	lowerPrompt := strings.ToLower(resp.SanitizedPrompt)
	lowerBrief := strings.ToLower(resp.ReferenceStyleBrief)
	if strings.Contains(lowerPrompt, "old navy") {
		t.Fatalf("sanitized prompt contains unsafe title case phrase %q", resp.SanitizedPrompt)
	}
	if strings.Contains(lowerBrief, "old navy") {
		t.Fatalf("reference style brief contains unsafe title case phrase %q", resp.ReferenceStyleBrief)
	}
	for _, safeSignal := range []string{"old english"} {
		if !strings.Contains(lowerPrompt, safeSignal) {
			t.Fatalf("sanitized prompt = %q, want safe title case phrase %q preserved", resp.SanitizedPrompt, safeSignal)
		}
		if !strings.Contains(lowerBrief, safeSignal) {
			t.Fatalf("reference style brief = %q, want safe title case phrase %q preserved", resp.ReferenceStyleBrief, safeSignal)
		}
	}
}

func TestAnalyzeStudioReferenceStyleDropsLowercaseProtectedPhrases(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"old navy","typography":"Old English"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/lowercase-old-navy.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	lowerPrompt := strings.ToLower(resp.SanitizedPrompt)
	lowerBrief := strings.ToLower(resp.ReferenceStyleBrief)
	if strings.Contains(lowerPrompt, "old navy") {
		t.Fatalf("sanitized prompt contains lowercase protected phrase %q", resp.SanitizedPrompt)
	}
	if strings.Contains(lowerBrief, "old navy") {
		t.Fatalf("reference style brief contains lowercase protected phrase %q", resp.ReferenceStyleBrief)
	}
	if !strings.Contains(lowerPrompt, "old english") {
		t.Fatalf("sanitized prompt = %q, want safe phrase old english preserved", resp.SanitizedPrompt)
	}
	if !strings.Contains(lowerBrief, "old english") {
		t.Fatalf("reference style brief = %q, want safe phrase old english preserved", resp.ReferenceStyleBrief)
	}
	if !containsWarningFragment(resp.Warnings, "已移除品牌、Logo、原文案或过于接近原图的描述") {
		t.Fatalf("warnings = %#v, want unsafe-removal warning for lowercase protected phrase", resp.Warnings)
	}
}

func TestAnalyzeStudioReferenceStyleDoesNotWarnForSafeStructuredJSONQuotes(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"Retro Flowers","palette":["Cream","Red"],"composition":"Centered Badge","typography":"Old English"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}
	if containsWarningFragment(resp.Warnings, "已移除品牌、Logo、原文案或过于接近原图的描述") {
		t.Fatalf("warnings = %#v, do not want unsafe-removal warning for safe structured JSON", resp.Warnings)
	}
}

func TestAnalyzeStudioReferenceStyleKeepsBroaderSafeStyleCues(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"Koi Wave","palette":["Teal","Orange"],"composition":"Centered Badge","typography":"Brush Lettering","product_fit":"Resort Wear"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	lowerPrompt := strings.ToLower(resp.SanitizedPrompt)
	lowerBrief := strings.ToLower(resp.ReferenceStyleBrief)
	for _, safeSignal := range []string{
		"koi wave",
		"teal, orange",
		"centered composition",
		"badge composition",
		"brush lettering",
		"resort wear",
	} {
		if !strings.Contains(lowerPrompt, safeSignal) {
			t.Fatalf("sanitized prompt = %q, want safe style cue %q preserved", resp.SanitizedPrompt, safeSignal)
		}
		if !strings.Contains(lowerBrief, safeSignal) {
			t.Fatalf("reference style brief = %q, want safe style cue %q preserved", resp.ReferenceStyleBrief, safeSignal)
		}
	}
	if containsWarningFragment(resp.Warnings, "已移除品牌、Logo、原文案或过于接近原图的描述") {
		t.Fatalf("warnings = %#v, do not want unsafe-removal warning for safe broadened cues", resp.Warnings)
	}
}

func TestAnalyzeStudioReferenceStyleKeepsOrdinarySafeStylePhrases(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"minimalist coastal illustration","palette":["sunset ombre","sea glass teal"],"composition":"arched floral frame","typography":"playful rounded lettering","density":"airy balanced layout"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/reference-safe.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	lowerPrompt := strings.ToLower(resp.SanitizedPrompt)
	lowerBrief := strings.ToLower(resp.ReferenceStyleBrief)
	for _, safeSignal := range []string{
		"minimalist coastal illustration",
		"sunset ombre",
		"sea glass teal",
		"framed composition",
		"playful rounded lettering",
		"airy balanced layout",
	} {
		if !strings.Contains(lowerPrompt, safeSignal) {
			t.Fatalf("sanitized prompt = %q, want safe phrase %q preserved", resp.SanitizedPrompt, safeSignal)
		}
		if !strings.Contains(lowerBrief, safeSignal) {
			t.Fatalf("reference style brief = %q, want safe phrase %q preserved", resp.ReferenceStyleBrief, safeSignal)
		}
	}
	if containsWarningFragment(resp.Warnings, "已移除品牌、Logo、原文案或过于接近原图的描述") {
		t.Fatalf("warnings = %#v, do not want unsafe-removal warning for ordinary safe style phrases", resp.Warnings)
	}
}

func TestAnalyzeStudioReferenceStyleDerivesStructuredFallbacksForSafeOffVocabularyCues(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"souvenir keepsake vignette","palette":["sun-washed neutrals"],"typography":"ornate letterform treatment","density":"measured breathing room","product_fit":"boutique drape sensibility","mood":"quiet getaway feeling","garment_placement":"anchored print zone"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/off-vocabulary-structured.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	lowerPrompt := strings.ToLower(resp.SanitizedPrompt)
	lowerBrief := strings.ToLower(resp.ReferenceStyleBrief)
	for _, expected := range []string{
		"abstract motif direction",
		"balanced palette direction",
		"decorative typography direction",
		"balanced visual density",
		"general apparel styling",
		"balanced mood",
		"standard garment placement",
	} {
		if !strings.Contains(lowerPrompt, expected) {
			t.Fatalf("sanitized prompt = %q, want derived fallback %q", resp.SanitizedPrompt, expected)
		}
		if !strings.Contains(lowerBrief, expected) {
			t.Fatalf("reference style brief = %q, want derived fallback %q", resp.ReferenceStyleBrief, expected)
		}
	}
	if containsWarningFragment(resp.Warnings, "已移除品牌、Logo、原文案或过于接近原图的描述") {
		t.Fatalf("warnings = %#v, do not want unsafe-removal warning for safe off-vocabulary structured cues", resp.Warnings)
	}
}

func TestAnalyzeStudioReferenceStyleKeepsUnsafeWarningForExplicitProtectedStructuredField(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"souvenir keepsake vignette","typography":"Nike logo wordmark"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/unsafe-structured-field.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	if !containsWarningFragment(resp.Warnings, "已移除品牌、Logo、原文案或过于接近原图的描述") {
		t.Fatalf("warnings = %#v, want unsafe-removal warning for explicit protected structured field", resp.Warnings)
	}
	if strings.Contains(strings.ToLower(resp.SanitizedPrompt), "nike") {
		t.Fatalf("sanitized prompt contains protected token: %q", resp.SanitizedPrompt)
	}
	if !strings.Contains(strings.ToLower(resp.SanitizedPrompt), "abstract motif direction") {
		t.Fatalf("sanitized prompt = %q, want safe structured fallback to survive alongside unsafe warning", resp.SanitizedPrompt)
	}
}

func TestAnalyzeStudioReferenceStyleKeepsFieldSpecificSafeVocabularyCues(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"Koi Wave","palette":["Off White","Forest Green"],"typography":"Sans Serif","density":"Clean Layering","product_fit":"Resort Wear"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/reference-vocab.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	lowerPrompt := strings.ToLower(resp.SanitizedPrompt)
	lowerBrief := strings.ToLower(resp.ReferenceStyleBrief)
	for _, safeSignal := range []string{
		"koi wave",
		"off white",
		"forest green",
		"sans serif",
		"clean layering",
		"resort wear",
	} {
		if !strings.Contains(lowerPrompt, safeSignal) {
			t.Fatalf("sanitized prompt = %q, want safe vocabulary cue %q preserved", resp.SanitizedPrompt, safeSignal)
		}
		if !strings.Contains(lowerBrief, safeSignal) {
			t.Fatalf("reference style brief = %q, want safe vocabulary cue %q preserved", resp.ReferenceStyleBrief, safeSignal)
		}
	}
}

func TestAnalyzeStudioReferenceStyleAvoidWarningsOnlyForUnsafeSignals(t *testing.T) {
	testCases := []struct {
		name        string
		response    string
		wantWarning bool
	}{
		{
			name:        "safe avoid guidance",
			response:    `{"motif":"Retro Flowers","palette":["Cream","Red"],"composition":"Centered Badge","avoid":["avoid cluttered layouts","avoid tiny details"]}`,
			wantWarning: false,
		},
		{
			name:        "unsafe avoid guidance",
			response:    `{"motif":"Retro Flowers","palette":["Cream","Red"],"composition":"Centered Badge","avoid":["avoid Nike logo","avoid exact text lockup"]}`,
			wantWarning: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			completer := &stubReferenceAnalysisCompleter{responses: []string{tc.response}}
			svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

			resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
				ReferenceImageURLs: []string{"https://example.com/a.png"},
			})
			if err != nil {
				t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
			}

			gotWarning := containsWarningFragment(resp.Warnings, "已移除品牌、Logo、原文案或过于接近原图的描述")
			if gotWarning != tc.wantWarning {
				t.Fatalf("warnings = %#v, unsafe warning = %v, want %v", resp.Warnings, gotWarning, tc.wantWarning)
			}
		})
	}
}

func TestAnalyzeStudioReferenceStyleStripsLowercaseProtectedNamesAndKeepsSafeDescriptors(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"hello kitty bow","composition":"adidas mascot","palette":["cream"],"typography":"clean serif"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/lowercase-protected.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	lowerPrompt := strings.ToLower(resp.SanitizedPrompt)
	lowerBrief := strings.ToLower(resp.ReferenceStyleBrief)
	for _, unsafeToken := range []string{"hello kitty", "adidas"} {
		if strings.Contains(lowerPrompt, unsafeToken) {
			t.Fatalf("sanitized prompt contains lowercase protected token %q: %q", unsafeToken, resp.SanitizedPrompt)
		}
		if strings.Contains(lowerBrief, unsafeToken) {
			t.Fatalf("reference style brief contains lowercase protected token %q: %q", unsafeToken, resp.ReferenceStyleBrief)
		}
	}
	for _, unsafeResidual := range []string{"bow", "mascot"} {
		if strings.Contains(lowerPrompt, unsafeResidual) {
			t.Fatalf("sanitized prompt contains protected residual %q: %q", unsafeResidual, resp.SanitizedPrompt)
		}
		if strings.Contains(lowerBrief, unsafeResidual) {
			t.Fatalf("reference style brief contains protected residual %q: %q", unsafeResidual, resp.ReferenceStyleBrief)
		}
	}
	for _, safeSignal := range []string{"cream", "clean serif"} {
		if !strings.Contains(lowerPrompt, safeSignal) {
			t.Fatalf("sanitized prompt = %q, want safe signal %q preserved", resp.SanitizedPrompt, safeSignal)
		}
		if !strings.Contains(lowerBrief, safeSignal) {
			t.Fatalf("reference style brief = %q, want safe signal %q preserved", resp.ReferenceStyleBrief, safeSignal)
		}
	}
	if !containsWarningFragment(resp.Warnings, "已移除品牌、Logo、原文案或过于接近原图的描述") {
		t.Fatalf("warnings = %#v, want unsafe-removal warning for lowercase protected names", resp.Warnings)
	}
}

func TestAnalyzeStudioReferenceStyleDropsUnknownSuspiciousLowercaseNames(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"blorbo mascot illustration","composition":"zorplex badge layout","palette":["cream"],"typography":"clean serif","mood":"playful mood","garment_placement":"front chest placement"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/suspicious-lowercase.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	lowerPrompt := strings.ToLower(resp.SanitizedPrompt)
	lowerBrief := strings.ToLower(resp.ReferenceStyleBrief)
	for _, unsafeToken := range []string{"blorbo", "zorplex", "badge layout"} {
		if strings.Contains(lowerPrompt, unsafeToken) {
			t.Fatalf("sanitized prompt contains suspicious token %q: %q", unsafeToken, resp.SanitizedPrompt)
		}
		if strings.Contains(lowerBrief, unsafeToken) {
			t.Fatalf("reference style brief contains suspicious token %q: %q", unsafeToken, resp.ReferenceStyleBrief)
		}
	}
	for _, safeSignal := range []string{"mascot", "illustration", "cream", "clean serif", "playful mood", "front chest placement"} {
		if !strings.Contains(lowerPrompt, safeSignal) {
			t.Fatalf("sanitized prompt = %q, want safe signal %q preserved", resp.SanitizedPrompt, safeSignal)
		}
		if !strings.Contains(lowerBrief, safeSignal) {
			t.Fatalf("reference style brief = %q, want safe signal %q preserved", resp.ReferenceStyleBrief, safeSignal)
		}
	}
}

func TestAnalyzeStudioReferenceStylePreservesMoodAndGarmentPlacement(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"mood":"playful resort mood","garment_placement":"left chest placement"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/mood-placement.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	if len(completer.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(completer.calls))
	}
	prompt := strings.SplitN(completer.calls[0], "|", 2)[1]
	for _, requiredKey := range []string{"mood", "garment_placement"} {
		if !strings.Contains(prompt, requiredKey) {
			t.Fatalf("analysis prompt = %q, want json key %q included", prompt, requiredKey)
		}
	}
	for _, safeSignal := range []string{"playful mood", "left chest placement"} {
		if !strings.Contains(strings.ToLower(resp.SanitizedPrompt), safeSignal) {
			t.Fatalf("sanitized prompt = %q, want %q preserved", resp.SanitizedPrompt, safeSignal)
		}
		if !strings.Contains(strings.ToLower(resp.ReferenceStyleBrief), safeSignal) {
			t.Fatalf("reference style brief = %q, want %q preserved", resp.ReferenceStyleBrief, safeSignal)
		}
	}
}

func TestAnalyzeStudioReferenceStylePromptIncludesGlobalSafetyCategories(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"retro flowers","palette":["cream"],"composition":"centered badge"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/reference.png"},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}
	if len(completer.calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(completer.calls))
	}

	prompt := strings.SplitN(completer.calls[0], "|", 2)[1]
	for _, required := range []string{"watermark", "exact artwork"} {
		if !strings.Contains(strings.ToLower(prompt), required) {
			t.Fatalf("analysis prompt = %q, want global safety category %q", prompt, required)
		}
	}
}

func TestAnalyzeStudioReferenceStyleFiltersWatermarkAndExactArtworkSignals(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"retro flowers watermark","palette":["cream"],"composition":"centered badge","avoid":["exact artwork lockup"]}`,
		`Use a faint watermark texture around the frame and keep the exact artwork from the source poster.`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{
			"https://example.com/reference-a.png",
			"https://example.com/reference-b.png",
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	for _, field := range []string{strings.ToLower(resp.ReferenceStyleBrief), strings.ToLower(resp.SanitizedPrompt)} {
		for _, unsafeToken := range []string{"watermark", "exact artwork"} {
			if strings.Contains(field, unsafeToken) {
				t.Fatalf("response leaked unsafe token %q: brief=%q prompt=%q", unsafeToken, resp.ReferenceStyleBrief, resp.SanitizedPrompt)
			}
		}
	}
	if !containsWarningFragment(resp.Warnings, "已移除品牌、Logo、原文案或过于接近原图的描述") {
		t.Fatalf("warnings = %#v, want unsafe-removal warning for watermark/exact artwork", resp.Warnings)
	}
}

func TestAnalyzeStudioReferenceStyleErrorsWhenNoSafeSignalsSurvive(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"Hello Kitty","palette":["Nike"],"composition":"same exact layout","typography":"Taylor Swift signature quote","density":"Mickey portrait","product_fit":"Adidas logo","avoid":["Just Do It slogan"]}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/a.png"},
	})
	if err == nil {
		t.Fatal("AnalyzeStudioReferenceStyle() error = nil, want no-safe-signal failure")
	}
	if !strings.Contains(err.Error(), "reference_analysis_failed: no reusable safe style direction extracted") {
		t.Fatalf("error = %v, want no-safe-signal failure", err)
	}
}

func containsWarningFragment(warnings []string, fragment string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, fragment) {
			return true
		}
	}
	return false
}

func TestBuildResolveUploadedImagePublicURLFuncFallsBackToStorePublicURL(t *testing.T) {
	svc := &service{
		studioDeps: studioDependencies{
			uploadStore: &stubResolveUploadedImageStore{
				openResult: &StoredUploadedImage{
					Key:       "folder/reference.png",
					PublicURL: "https://cdn.example.com/folder/reference.png",
				},
			},
		},
		supportDeps: supportDependencies{
			uploadedImageRepository: &stubResolveUploadedImageRepository{
				record: &UploadedImageRecord{
					Key:       "folder/reference.png",
					PublicURL: "   ",
				},
			},
		},
	}

	resolve := buildResolveUploadedImagePublicURLFunc(svc)
	got, err := resolve(context.Background(), "folder/reference.png")
	if err != nil {
		t.Fatalf("resolveUploadedImagePublicURL() error = %v", err)
	}
	if got != "https://cdn.example.com/folder/reference.png" {
		t.Fatalf("public url = %q, want store fallback url", got)
	}
}

func TestBuildResolveUploadedImagePublicURLFuncFailsWhenRepoAndStorePublicURLsAreUnusable(t *testing.T) {
	svc := &service{
		studioDeps: studioDependencies{
			uploadStore: &stubResolveUploadedImageStore{
				openResult: &StoredUploadedImage{
					Key:       "folder/reference.png",
					PublicURL: "http://localhost:3000/folder/reference.png",
				},
			},
		},
		supportDeps: supportDependencies{
			uploadedImageRepository: &stubResolveUploadedImageRepository{
				record: &UploadedImageRecord{
					Key:       "folder/reference.png",
					PublicURL: "http://localhost:3000/folder/reference.png",
				},
			},
		},
	}

	resolve := buildResolveUploadedImagePublicURLFunc(svc)
	if _, err := resolve(context.Background(), "folder/reference.png"); err == nil {
		t.Fatal("resolveUploadedImagePublicURL() error = nil, want unusable public url failure")
	}
}

func TestUploadedListingKitImageKeyFromURL(t *testing.T) {
	testCases := []struct {
		name   string
		rawURL string
		want   string
		ok     bool
	}{
		{
			name:   "relative uploaded path",
			rawURL: "/api/v1/listing-kits/uploads/files/folder/reference.png",
			want:   "folder/reference.png",
			ok:     true,
		},
		{
			name:   "localhost uploaded url",
			rawURL: "http://localhost:3000/api/v1/listing-kits/uploads/files/folder/reference.png",
			want:   "folder/reference.png",
			ok:     true,
		},
		{
			name:   "remote host uploaded url rejected",
			rawURL: "https://assets.example.com/api/v1/listing-kits/uploads/files/folder/reference.png",
			want:   "",
			ok:     false,
		},
		{
			name:   "non upload path rejected",
			rawURL: "https://example.com/reference.png",
			want:   "",
			ok:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := uploadedListingKitImageKeyFromURL(tc.rawURL)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("uploadedListingKitImageKeyFromURL(%q) = (%q, %v), want (%q, %v)", tc.rawURL, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestStudioReferenceUploadedImageKeyFromURL(t *testing.T) {
	testCases := []struct {
		name   string
		rawURL string
		want   string
		ok     bool
	}{
		{
			name:   "relative uploaded path",
			rawURL: "/api/v1/listing-kits/uploads/files/folder/reference.png",
			want:   "folder/reference.png",
			ok:     true,
		},
		{
			name:   "localhost uploaded url",
			rawURL: "http://localhost:3000/api/v1/listing-kits/uploads/files/folder/reference.png",
			want:   "folder/reference.png",
			ok:     true,
		},
		{
			name:   "remote host uploaded url rejected",
			rawURL: "https://assets.example.com/api/v1/listing-kits/uploads/files/folder/reference.png",
			want:   "",
			ok:     false,
		},
		{
			name:   "non upload path rejected",
			rawURL: "https://example.com/reference.png",
			want:   "",
			ok:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := studioReferenceUploadedImageKeyFromURL(tc.rawURL)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("studioReferenceUploadedImageKeyFromURL(%q) = (%q, %v), want (%q, %v)", tc.rawURL, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestNormalizeGenerateRequestImageURLs(t *testing.T) {
	got := normalizeGenerateRequestImageURLs([]string{
		"",
		" /api/v1/listing-kits/uploads/files/folder/reference.png ",
		"http://localhost:3000/api/v1/listing-kits/uploads/files/folder/already-local.png",
		"https://example.com/remote.png",
	})

	want := []string{
		"http://localhost:3000/api/v1/listing-kits/uploads/files/folder/reference.png",
		"http://localhost:3000/api/v1/listing-kits/uploads/files/folder/already-local.png",
		"https://example.com/remote.png",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("normalizeGenerateRequestImageURLs() = %#v, want %#v", got, want)
	}
}

type stubResolveUploadedImageRepository struct {
	record *UploadedImageRecord
	err    error
}

func (s *stubResolveUploadedImageRepository) SaveUploadedImage(context.Context, *UploadedImageRecord) error {
	return errors.New("not implemented")
}

func (s *stubResolveUploadedImageRepository) GetUploadedImage(context.Context, string) (*UploadedImageRecord, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.record == nil {
		return nil, ErrUploadedImageNotFound
	}
	record := *s.record
	return &record, nil
}

func (s *stubResolveUploadedImageRepository) MarkUploadedImageDeleted(context.Context, string) (*UploadedImageRecord, error) {
	return nil, errors.New("not implemented")
}

type stubResolveUploadedImageStore struct {
	openResult *StoredUploadedImage
	openErr    error
}

func (s *stubResolveUploadedImageStore) Save(context.Context, *ImageUploadInput) (*StoredUploadedImage, error) {
	return nil, errors.New("not implemented")
}

func (s *stubResolveUploadedImageStore) Open(context.Context, string) (*StoredUploadedImage, error) {
	if s.openErr != nil {
		return nil, s.openErr
	}
	if s.openResult == nil {
		return nil, ErrUploadedImageNotFound
	}
	result := *s.openResult
	return &result, nil
}

func (s *stubResolveUploadedImageStore) Delete(context.Context, string) error {
	return errors.New("not implemented")
}
