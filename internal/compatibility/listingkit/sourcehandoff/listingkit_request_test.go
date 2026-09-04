package sourcehandoff

import (
	"context"
	"testing"

	sourcea1688 "task-processor/internal/integration/crawler/a1688"
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
	if request.ProductKey != "crawler:1688:654" {
		t.Fatalf("ProductKey = %q, want normalized source identity", request.ProductKey)
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
	if request.Source == nil || request.Source.URL != "https://detail.1688.com/offer/654.html" {
		t.Fatalf("Source = %#v, want normalized source reference", request.Source)
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
	if creator.request.ProductKey != "crawler:1688:654" {
		t.Fatalf("creator request ProductKey = %q, want source identity", creator.request.ProductKey)
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
	envelope := sourcea1688.Alibaba1688SourceEnvelope(sourcea1688.Alibaba1688SourceEnvelopeInput{
		Request: sourcea1688.Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/654.html?spm=handoff", StoreID: 11},
		Product: &sourcea1688.Alibaba1688ProductSnapshot{
			ID:             "654",
			Title:          "Insulated Lunch Bag",
			URL:            "https://detail.1688.com/offer/654.html?foo=bar",
			MainImage:      "https://img.example/654-main.jpg",
			Images:         []string{"https://img.example/654-main.jpg", "https://img.example/654-side.jpg"},
			MinPrice:       18.8,
			Currency:       "CNY",
			Category:       "Bags>Lunch Bags",
			Brand:          "Factory Lunch",
			Supplier:       sourcea1688.Alibaba1688SupplierSnapshot{ID: "supplier-654", Name: "Lunch Factory"},
			Variants:       []sourcea1688.Alibaba1688VariantSnapshot{{Name: "Black", Image: "https://img.example/654-black.jpg", Price: 19.9, Attributes: map[string]any{"Color": "Black"}}},
			ProductDetails: []sourcea1688.Alibaba1688ProductDetailSnapshot{{Content: "Thermal lunch bag with zipper."}},
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
