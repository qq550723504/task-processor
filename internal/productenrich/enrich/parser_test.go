package enrich_test

import (
	"context"
	"fmt"
	"testing"

	productenrich "task-processor/internal/productenrich"
	productenrichenrich "task-processor/internal/productenrich/enrich"

	"github.com/sirupsen/logrus"
)

type mockWebScraper struct {
	data *productenrich.ScrapedData
	err  error
}

func (m *mockWebScraper) Scrape(_ context.Context, _ string) (*productenrich.ScrapedData, error) {
	return m.data, m.err
}

func newTestParser(t *testing.T, scraper productenrich.WebScraper) productenrich.InputParser {
	t.Helper()

	if scraper == nil {
		scraper = &mockWebScraper{data: &productenrich.ScrapedData{Title: "mock"}}
	}

	parser, err := productenrichenrich.NewInputParser(logrus.New(), &productenrich.InputParserConfig{}, scraper)
	if err != nil {
		t.Fatalf("NewInputParser() error = %v", err)
	}

	return parser
}

func TestInputParser_ParseInput_Validation(t *testing.T) {
	ctx := context.Background()
	p := newTestParser(t, nil)

	cases := []struct {
		name    string
		req     *productenrich.GenerateRequest
		wantErr bool
	}{
		{"nil request", nil, true},
		{"empty request", &productenrich.GenerateRequest{}, true},
		{"invalid image url", &productenrich.GenerateRequest{ImageURLs: []string{"not-a-url"}}, true},
		{"empty image url", &productenrich.GenerateRequest{ImageURLs: []string{""}}, true},
		{"invalid product url", &productenrich.GenerateRequest{ProductURL: "not-a-url"}, true},
		{"non-1688 product url", &productenrich.GenerateRequest{ProductURL: "https://example.com/offer/123.html"}, true},
		{"valid text only", &productenrich.GenerateRequest{Text: "some product text"}, false},
		{"valid image only", &productenrich.GenerateRequest{ImageURLs: []string{"https://example.com/img.jpg"}}, false},
		{"valid 1688 product url", &productenrich.GenerateRequest{ProductURL: "https://detail.1688.com/offer/123.html"}, false},
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
	scraper := &mockWebScraper{
		data: &productenrich.ScrapedData{
			Title:  "Product",
			Images: []string{"https://example.com/img1.jpg", "https://example.com/img3.jpg", "https://example.com/img3.jpg"},
		},
	}
	p := newTestParser(t, scraper)

	req := &productenrich.GenerateRequest{
		ImageURLs:  []string{"https://example.com/img1.jpg", "https://example.com/img2.jpg"},
		ProductURL: "https://detail.1688.com/offer/123.html",
	}
	result, err := p.ParseInput(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Images) != 3 {
		t.Errorf("images count = %d, want 3 (deduped)", len(result.Images))
	}
}

func TestInputParser_ParseInput_TextMerge(t *testing.T) {
	ctx := context.Background()
	scraper := &mockWebScraper{
		data: &productenrich.ScrapedData{
			Title:       "Product",
			Description: "scraped description",
		},
	}
	p := newTestParser(t, scraper)

	req := &productenrich.GenerateRequest{
		Text:       "original text",
		ProductURL: "https://detail.1688.com/offer/123.html",
	}
	result, err := p.ParseInput(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Text == "" {
		t.Error("merged text should not be empty")
	}
}

func TestInputParser_ParseInput_ScraperError(t *testing.T) {
	ctx := context.Background()
	scraper := &mockWebScraper{err: fmt.Errorf("scrape failed")}
	p := newTestParser(t, scraper)

	req := &productenrich.GenerateRequest{ProductURL: "https://detail.1688.com/offer/123.html"}
	_, err := p.ParseInput(ctx, req)
	if err == nil {
		t.Error("expected error when scraper fails")
	}
}

func TestInputParser_ParseInput_NilScraperResult(t *testing.T) {
	ctx := context.Background()
	scraper := &mockWebScraper{data: nil}
	p := newTestParser(t, scraper)

	req := &productenrich.GenerateRequest{ProductURL: "https://detail.1688.com/offer/123.html"}
	_, err := p.ParseInput(ctx, req)
	if err == nil {
		t.Fatal("expected error when scraper returns nil data")
	}
}

func TestInputParser_CollectImages(t *testing.T) {
	ctx := context.Background()
	p := newTestParser(t, nil)
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
	cfg := &productenrich.InputParserConfig{}
	logger := logrus.New()

	if _, err := productenrichenrich.NewInputParser(nil, cfg, scraper); err == nil {
		t.Error("expected error for nil logger")
	}
	if _, err := productenrichenrich.NewInputParser(logger, nil, scraper); err == nil {
		t.Error("expected error for nil config")
	}
	if _, err := productenrichenrich.NewInputParser(logger, cfg, nil); err == nil {
		t.Error("expected error for nil scraper")
	}
}
