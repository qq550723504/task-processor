package sourcehandoff

import (
	"context"
	"testing"

	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/listingkit"
	"task-processor/internal/product/sourcing"
)

func TestGenerateRequestFromEnvelopeUsesNeutralFacts(t *testing.T) {
	envelope := testAlibaba1688Envelope(t)

	request := GenerateRequestFromEnvelope(ListingKitRequestInput{
		Envelope:           envelope,
		TenantID:           " tenant-1688 ",
		UserID:             " user-1688 ",
		Platforms:          []string{" SHEIN ", "shein"},
		Country:            " US ",
		Language:           " en_US ",
		SheinStoreID:       168811,
		TargetCategoryHint: " Bags>Lunch Bags ",
	})

	if request.TenantID != "tenant-1688" || request.UserID != "user-1688" {
		t.Fatalf("request tenant/user = %q/%q, want trimmed values", request.TenantID, request.UserID)
	}
	if request.ProductURL != "https://detail.1688.com/offer/654.html" {
		t.Fatalf("ProductURL = %q, want normalized source URL", request.ProductURL)
	}
	if request.BrandHint != "Factory Lunch" {
		t.Fatalf("BrandHint = %q, want source brand", request.BrandHint)
	}
	if request.TargetCategoryHint != "Bags>Lunch Bags" {
		t.Fatalf("TargetCategoryHint = %q, want explicit category hint", request.TargetCategoryHint)
	}
	if len(request.Platforms) != 1 || request.Platforms[0] != "shein" {
		t.Fatalf("Platforms = %#v, want normalized deduped shein", request.Platforms)
	}
	if len(request.ImageURLs) != 3 {
		t.Fatalf("ImageURLs = %#v, want main/gallery/variant images", request.ImageURLs)
	}
}

func TestCreateGenerateTaskFromEnvelopeDelegatesToCreator(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	task, err := CreateGenerateTaskFromEnvelope(context.Background(), creator, ListingKitRequestInput{
		Envelope:     testAlibaba1688Envelope(t),
		TenantID:     "tenant-1688",
		UserID:       "user-1688",
		Platforms:    []string{"shein"},
		Country:      "US",
		Language:     "en_US",
		SheinStoreID: 168811,
	})
	if err != nil {
		t.Fatalf("CreateGenerateTaskFromEnvelope() error = %v", err)
	}
	if task == nil || task.ID != "task-1688" {
		t.Fatalf("task = %+v, want delegated task", task)
	}
	if creator.request == nil {
		t.Fatal("creator did not receive request")
	}
	if creator.request.ProductURL != "https://detail.1688.com/offer/654.html" {
		t.Fatalf("creator request ProductURL = %q, want source URL", creator.request.ProductURL)
	}
	if creator.request.Text == "" {
		t.Fatal("creator request Text is empty")
	}
}

func TestCreateGenerateTaskFromEnvelopeRequiresCreator(t *testing.T) {
	_, err := CreateGenerateTaskFromEnvelope(context.Background(), nil, ListingKitRequestInput{Envelope: testAlibaba1688Envelope(t)})
	if err == nil {
		t.Fatal("CreateGenerateTaskFromEnvelope(nil creator) error = nil, want error")
	}
}

func testAlibaba1688Envelope(t *testing.T) sourcing.SourceEnvelope {
	t.Helper()
	envelope := sourcing.Alibaba1688SourceEnvelope(sourcing.Alibaba1688SourceEnvelopeInput{
		Request: sourcing.Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/654.html?spm=handoff", StoreID: 11},
		Product: &alibaba1688model.Product1688{
			ID:         "654",
			Title:      "Insulated Lunch Bag",
			URL:        "https://detail.1688.com/offer/654.html?foo=bar",
			MainImage:  "https://img.example/654-main.jpg",
			Images:     []string{"https://img.example/654-main.jpg", "https://img.example/654-side.jpg"},
			MinPrice:   18.8,
			Currency:   "CNY",
			Category:   "Bags>Lunch Bags",
			Brand:      "Factory Lunch",
			Supplier:   alibaba1688model.SupplierInfo{ID: "supplier-654", Name: "Lunch Factory"},
			Variants:   []alibaba1688model.Variant{{Name: "Black", Image: "https://img.example/654-black.jpg", Price: 19.9, Attributes: map[string]any{"Color": "Black"}}},
			ProductDetails: []alibaba1688model.ProductDetail{{Content: "Thermal lunch bag with zipper."}},
		},
	})
	if len(envelope.Warnings) != 0 {
		t.Fatalf("envelope warnings = %+v, want none", envelope.Warnings)
	}
	return envelope
}

type fakeGenerateTaskCreator struct {
	request *listingkit.GenerateRequest
}

func (f *fakeGenerateTaskCreator) CreateGenerateTask(_ context.Context, request *listingkit.GenerateRequest) (*listingkit.Task, error) {
	f.request = request
	return &listingkit.Task{ID: "task-1688", Request: request}, nil
}
