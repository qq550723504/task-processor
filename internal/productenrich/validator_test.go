package productenrich

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestInputValidator_ValidateText(t *testing.T) {
	ctx := context.Background()
	v := NewInputValidator(nil)

	cases := []struct {
		name        string
		text        string
		wantLen     int
		wantKeyword bool
	}{
		{"empty text", "", 0, false},
		{"short text", "hello world", 11, true},
		{"long text", "这是一段很长的产品描述，包含了很多关键词和产品特性信息", 0, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := v.ValidateText(ctx, tc.text)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantLen > 0 && result.Length != tc.wantLen {
				t.Errorf("length = %d, want %d", result.Length, tc.wantLen)
			}
			if result.HasKeywords != tc.wantKeyword {
				t.Errorf("HasKeywords = %v, want %v", result.HasKeywords, tc.wantKeyword)
			}
			if result.RawText != tc.text {
				t.Errorf("RawText = %q, want %q", result.RawText, tc.text)
			}
		})
	}
}

func TestInputValidator_ValidateScrapedData(t *testing.T) {
	ctx := context.Background()
	v := NewInputValidator(nil)

	cases := []struct {
		name           string
		data           *ScrapedData
		wantErr        bool
		wantHasTitle   bool
		wantHasDesc    bool
		wantHasImages  bool
		wantImageCount int
	}{
		{"nil data returns error", nil, true, false, false, false, 0},
		{
			name: "full data",
			data: &ScrapedData{
				Title:       "Test Product",
				Description: "A great product",
				Images:      []string{"https://example.com/img1.jpg", "https://example.com/img2.jpg"},
			},
			wantHasTitle: true, wantHasDesc: true, wantHasImages: true, wantImageCount: 2,
		},
		{
			name:         "empty data",
			data:         &ScrapedData{},
			wantHasTitle: false, wantHasDesc: false, wantHasImages: false, wantImageCount: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := v.ValidateScrapedData(ctx, tc.data)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.HasTitle != tc.wantHasTitle {
				t.Errorf("HasTitle = %v, want %v", result.HasTitle, tc.wantHasTitle)
			}
			if result.HasDescription != tc.wantHasDesc {
				t.Errorf("HasDescription = %v, want %v", result.HasDescription, tc.wantHasDesc)
			}
			if result.HasImages != tc.wantHasImages {
				t.Errorf("HasImages = %v, want %v", result.HasImages, tc.wantHasImages)
			}
			if result.ImageCount != tc.wantImageCount {
				t.Errorf("ImageCount = %d, want %d", result.ImageCount, tc.wantImageCount)
			}
		})
	}
}

func TestInputValidator_ImageScoreTable(t *testing.T) {
	// 验证图片评分递减收益模型
	ctx := context.Background()
	v := NewInputValidator(nil)

	// 使用可信 CDN 域名，跳过 HTTP 验证
	trustedBase := "https://cbu01.alicdn.com/img/"
	makeURLs := func(n int) []string {
		urls := make([]string, n)
		for i := range urls {
			urls[i] = trustedBase + string(rune('a'+i)) + ".jpg"
		}
		return urls
	}

	scoreTable := []float64{0, 40, 60, 75, 85, 100}

	for count := 0; count <= 5; count++ {
		t.Run("image_count_"+string(rune('0'+count)), func(t *testing.T) {
			result, err := v.Validate(ctx, &ParsedInput{Images: makeURLs(count)})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := scoreTable[count]
			if result.ImageScore != want {
				t.Errorf("ImageScore = %.0f, want %.0f (count=%d)", result.ImageScore, want, count)
			}
		})
	}
}

func TestInputValidator_ValidateImages_InfersFormatFromContentType(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodHead {
			t.Fatalf("unexpected method %s", r.Method)
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	v := NewInputValidator(&InputValidatorConfig{HTTPTimeout: 2 * time.Second})
	result, err := v.Validate(ctx, &ParsedInput{
		Images: []string{server.URL + "/image-without-extension"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ImageScore != 40 {
		t.Fatalf("ImageScore = %.0f, want 40", result.ImageScore)
	}
	if result.ImageValidation == nil || result.ImageValidation.ValidCount != 1 {
		t.Fatalf("ValidCount = %d, want 1", result.ImageValidation.ValidCount)
	}
	if got := result.ImageValidation.ValidImages[0].Format; got != "jpg" {
		t.Fatalf("image format = %q, want jpg", got)
	}
}

func TestInputValidator_ValidateImages_FallsBackToGetWhenHeadRejected(t *testing.T) {
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(http.StatusForbidden)
		case http.MethodGet:
			if got := r.Header.Get("Range"); got != "bytes=0-0" {
				t.Fatalf("Range header = %q, want bytes=0-0", got)
			}
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	v := NewInputValidator(&InputValidatorConfig{HTTPTimeout: 2 * time.Second})
	result, err := v.Validate(ctx, &ParsedInput{
		Images: []string{server.URL + "/blocked-head.png"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ImageScore != 40 {
		t.Fatalf("ImageScore = %.0f, want 40", result.ImageScore)
	}
	if result.ImageValidation == nil || result.ImageValidation.ValidCount != 1 {
		t.Fatalf("ValidCount = %d, want 1", result.ImageValidation.ValidCount)
	}
	if got := result.ImageValidation.ValidImages[0].Format; got != "png" {
		t.Fatalf("image format = %q, want png", got)
	}
}
