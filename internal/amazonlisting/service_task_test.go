package amazonlisting

import (
	"context"
	"strings"
	"testing"
)

func TestCreateGenerateTask_RejectsTextOnlyRequest(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Marketplace: "amazon",
		Text:        "only text is not enough",
	})
	if err == nil {
		t.Fatal("expected text-only request to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid request: provide product_url, or provide both image_urls and text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateGenerateTask_RejectsImageOnlyRequest(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Marketplace: "amazon",
		ImageURLs:   []string{"https://example.com/a.jpg"},
	})
	if err == nil {
		t.Fatal("expected image-only request to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid request: provide product_url, or provide both image_urls and text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateGenerateTask_AllowsProductURLOnlyRequest(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Marketplace: "amazon",
		ProductURL:  "https://example.com/product",
	})
	if err != nil {
		t.Fatalf("expected product-url-only request to pass, got %v", err)
	}
	if task == nil || task.ID == "" {
		t.Fatal("expected task to be created")
	}
}

func TestCreateGenerateTask_AllowsImageAndTextRequest(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Marketplace: "amazon",
		ImageURLs:   []string{"https://example.com/a.jpg"},
		Text:        "supplemental product description",
	})
	if err != nil {
		t.Fatalf("expected image+text request to pass, got %v", err)
	}
	if task == nil || task.ID == "" {
		t.Fatal("expected task to be created")
	}
}
