package productenrich

import (
	"context"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
)

// mockWebScraper 用于测试的 WebScraper mock
type mockWebScraper struct {
	data *ScrapedData
	err  error
}

func (m *mockWebScraper) Scrape(_ context.Context, _ string) (*ScrapedData, error) {
	return m.data, m.err
}

func newTestParser(scraper WebScraper) InputParser {
	if scraper == nil {
		scraper = &mockWebScraper{data: &ScrapedData{Title: "mock"}}
	}
	p, _ := NewInputParser(logrus.New(), &InputParserConfig{}, scraper)
	return p
}

func TestIsValidURL(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://example.com/img.jpg", true},
		{"http://example.com", true},
		{"https://a.b.c/path/to/img", true},
		{"ftp://example.com", false},
		{"example.com", false},
		{"https://", false},
		{"http://", false},
		{"", false},
		{"https:/", false},
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			got := isValidURL(tc.url)
			if got != tc.want {
				t.Errorf("isValidURL(%q) = %v, want %v", tc.url, got, tc.want)
			}
		})
	}
}

func TestInputParser_ParseInput_Validation(t *testing.T) {
	ctx := context.Background()
	p := newTestParser(nil)

	cases := []struct {
		name    string
		req     *GenerateRequest
		wantErr bool
	}{
		{"nil request", nil, true},
		{"empty request", &GenerateRequest{}, true},
		{"invalid image url", &GenerateRequest{ImageURLs: []string{"not-a-url"}}, true},
		{"empty image url", &GenerateRequest{ImageURLs: []string{""}}, true},
		{"invalid product url", &GenerateRequest{ProductURL: "not-a-url"}, true},
		{"valid text only", &GenerateRequest{Text: "some product text"}, false},
		{"valid image only", &GenerateRequest{ImageURLs: []string{"https://example.com/img.jpg"}}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p.ParseInput(ctx, tc.req)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestInputParser_ParseInput_ImageDedup(t *testing.T) {
	ctx := context.Background()
	// scraper 返回与请求重叠的图片
	scraper := &mockWebScraper{
		data: &ScrapedData{
			Title:  "Product",
			Images: []string{"https://example.com/img1.jpg", "https://example.com/img3.jpg"},
		},
	}
	p := newTestParser(scraper)

	req := &GenerateRequest{
		ImageURLs:  []string{"https://example.com/img1.jpg", "https://example.com/img2.jpg"},
		ProductURL: "https://1688.com/product/123",
	}
	result, err := p.ParseInput(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// img1 重复，最终应为 img1, img2, img3（3张，不重复）
	if len(result.Images) != 3 {
		t.Errorf("images count = %d, want 3 (deduped)", len(result.Images))
	}
}

func TestInputParser_ParseInput_TextMerge(t *testing.T) {
	ctx := context.Background()
	scraper := &mockWebScraper{
		data: &ScrapedData{
			Title:       "Product",
			Description: "scraped description",
		},
	}
	p := newTestParser(scraper)

	req := &GenerateRequest{
		Text:       "original text",
		ProductURL: "https://1688.com/product/123",
	}
	result, err := p.ParseInput(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 文本应合并
	if result.Text == "" {
		t.Error("merged text should not be empty")
	}
}

func TestInputParser_ParseInput_ScraperError(t *testing.T) {
	ctx := context.Background()
	scraper := &mockWebScraper{err: fmt.Errorf("scrape failed")}
	p := newTestParser(scraper)

	req := &GenerateRequest{ProductURL: "https://1688.com/product/123"}
	_, err := p.ParseInput(ctx, req)
	if err == nil {
		t.Error("expected error when scraper fails")
	}
}

func TestInputParser_CollectImages(t *testing.T) {
	ctx := context.Background()
	p := newTestParser(nil)
	urls := []string{"https://a.com/1.jpg", "https://b.com/2.jpg"}
	got, err := p.CollectImages(ctx, urls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(urls) {
		t.Errorf("len = %d, want %d", len(got), len(urls))
	}
}

func TestNewInputParser_Validation(t *testing.T) {
	scraper := &mockWebScraper{}
	cfg := &InputParserConfig{}
	logger := logrus.New()

	if _, err := NewInputParser(nil, cfg, scraper); err == nil {
		t.Error("expected error for nil logger")
	}
	if _, err := NewInputParser(logger, nil, scraper); err == nil {
		t.Error("expected error for nil config")
	}
	if _, err := NewInputParser(logger, cfg, nil); err == nil {
		t.Error("expected error for nil scraper")
	}
}
