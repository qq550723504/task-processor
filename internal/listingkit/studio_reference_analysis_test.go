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

func TestAnalyzeStudioReferenceStyleRejectsExternalReferenceImages(t *testing.T) {
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: &stubReferenceAnalysisCompleter{}})

	_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"https://example.com/reference.png"},
	})
	if err == nil {
		t.Fatal("AnalyzeStudioReferenceStyle() error = nil, want uploaded-image validation failure")
	}
	if !strings.Contains(err.Error(), "uploaded listingkit image") {
		t.Fatalf("error = %v, want uploaded-image validation failure", err)
	}
}

func TestAnalyzeStudioReferenceStyleNormalizesUploadedReferenceURLs(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"retro flowers","palette":["cream"],"composition":"centered badge"}`,
		`{"motif":"watercolor texture","palette":["coral"],"composition":"arched floral frame"}`,
		`{"motif":"botanical pattern","palette":["teal"],"composition":"airy balanced layout"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{
			"/api/v1/listing-kits/uploads/files/folder/ref-a.png",
			"http://localhost:3000/api/v1/listing-kits/uploads/files/folder/ref-b.png",
			"https://assets.example.com/api/v1/listing-kits/uploads/files/folder/ref-c.png",
		},
	})
	if err != nil {
		t.Fatalf("AnalyzeStudioReferenceStyle() error = %v", err)
	}

	wantURLs := []string{
		"http://localhost:3000/api/v1/listing-kits/uploads/files/folder/ref-a.png",
		"http://localhost:3000/api/v1/listing-kits/uploads/files/folder/ref-b.png",
		"http://localhost:3000/api/v1/listing-kits/uploads/files/folder/ref-c.png",
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
			"/api/v1/listing-kits/uploads/files/a.png",
			"/api/v1/listing-kits/uploads/files/b.png",
			"/api/v1/listing-kits/uploads/files/c.png",
			"/api/v1/listing-kits/uploads/files/d.png",
			"/api/v1/listing-kits/uploads/files/e.png",
			"/api/v1/listing-kits/uploads/files/f.png",
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
		"hello kitty", "bow", "diagonal split badge", "adidas", "trefoil", "just do it", "mickey", "taylor swift", "face portrait", "same diagonal split layout",
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
		ReferenceImageURLs: []string{"/api/v1/listing-kits/uploads/files/a.png"},
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

func TestAnalyzeStudioReferenceStyleUsesPartialSuccess(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{
		responses: []string{`{"motif":"western floral","palette":["tan","red"],"composition":"center badge"}`},
		errAt:     2,
	}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{
			"/api/v1/listing-kits/uploads/files/a.png",
			"/api/v1/listing-kits/uploads/files/b.png",
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
		ReferenceImageURLs: []string{"/api/v1/listing-kits/uploads/files/a.png"},
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

func TestAnalyzeStudioReferenceStyleDoesNotWarnForSafeStructuredJSONQuotes(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"Retro Flowers","palette":["Cream","Red"],"composition":"Centered Badge","typography":"Old English"}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	resp, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"/api/v1/listing-kits/uploads/files/a.png"},
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
		ReferenceImageURLs: []string{"/api/v1/listing-kits/uploads/files/a.png"},
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
		ReferenceImageURLs: []string{"/api/v1/listing-kits/uploads/files/reference-safe.png"},
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
		"arched floral frame",
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
				ReferenceImageURLs: []string{"/api/v1/listing-kits/uploads/files/a.png"},
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

func TestAnalyzeStudioReferenceStyleErrorsWhenNoSafeSignalsSurvive(t *testing.T) {
	completer := &stubReferenceAnalysisCompleter{responses: []string{
		`{"motif":"Hello Kitty","palette":["Nike"],"composition":"same exact layout","typography":"Taylor Swift signature quote","density":"Mickey portrait","product_fit":"Adidas logo","avoid":["Just Do It slogan"]}`,
	}}
	svc := newTaskStudioMediaService(taskStudioMediaServiceConfig{promptDiversifier: completer})

	_, err := svc.AnalyzeStudioReferenceStyle(context.Background(), &StudioReferenceAnalysisRequest{
		ReferenceImageURLs: []string{"/api/v1/listing-kits/uploads/files/a.png"},
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
			name:   "remote host uploaded url uses path only",
			rawURL: "https://assets.example.com/api/v1/listing-kits/uploads/files/folder/reference.png",
			want:   "folder/reference.png",
			ok:     true,
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
