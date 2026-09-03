package a1688

import (
	"context"
	"errors"
	"testing"

	sourcea1688 "task-processor/internal/integration/crawler/a1688"
	"task-processor/internal/listingkit"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
)

func TestPrepareListingKitTaskHandoffBuildsRequest(t *testing.T) {
	handoff, err := PrepareListingKitTaskHandoff(ListingKitTaskInput{
		Source:             testAlibaba1688SourceEnvelopeInput(),
		TenantID:           " tenant-1688 ",
		UserID:             " user-1688 ",
		Platforms:          []string{" SHEIN ", "shein"},
		Country:            " US ",
		Language:           " en_US ",
		SheinStoreID:       168811,
		TargetCategoryHint: " Bags>Lunch Bags ",
	})
	if err != nil {
		t.Fatalf("PrepareListingKitTaskHandoff() error = %v", err)
	}
	if handoff == nil {
		t.Fatal("handoff is nil")
	}
	if got := handoff.Envelope.Identity.SourceKey(); got != "crawler:1688:777" {
		t.Fatalf("SourceKey() = %q, want crawler:1688:777", got)
	}
	if handoff.Request.ProductKey != "crawler:1688:777" {
		t.Fatalf("ProductKey = %q, want normalized source identity", handoff.Request.ProductKey)
	}
	if handoff.Request.BrandHint != "Factory Lunch" {
		t.Fatalf("BrandHint = %q, want source brand", handoff.Request.BrandHint)
	}
	if handoff.Request.TargetCategoryHint != "Bags>Lunch Bags" {
		t.Fatalf("TargetCategoryHint = %q, want explicit category hint", handoff.Request.TargetCategoryHint)
	}
	if handoff.Request.Source == nil || handoff.Request.Source.URL != "https://detail.1688.com/offer/777.html" {
		t.Fatalf("Source = %#v, want normalized source reference", handoff.Request.Source)
	}
	if len(handoff.Request.Platforms) != 1 || handoff.Request.Platforms[0] != "shein" {
		t.Fatalf("Platforms = %#v, want normalized deduped shein", handoff.Request.Platforms)
	}
}

func TestCreateListingKitTaskDelegatesToCreator(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	task, handoff, err := CreateListingKitTask(context.Background(), creator, ListingKitTaskInput{
		Source:       testAlibaba1688SourceEnvelopeInput(),
		TenantID:     "tenant-1688",
		UserID:       "user-1688",
		Platforms:    []string{"shein"},
		Country:      "US",
		Language:     "en_US",
		SheinStoreID: 168811,
	})
	if err != nil {
		t.Fatalf("CreateListingKitTask() error = %v", err)
	}
	if task == nil || task.ID != "task-1688" {
		t.Fatalf("task = %+v, want delegated task", task)
	}
	if handoff == nil || handoff.Request.ProductKey == "" {
		t.Fatalf("handoff = %+v, want prepared request", handoff)
	}
	if creator.request == nil || creator.request.ProductKey != "crawler:1688:777" {
		t.Fatalf("creator request = %+v, want normalized source request", creator.request)
	}
}

func TestCreateListingKitTaskRequiresCreator(t *testing.T) {
	task, handoff, err := CreateListingKitTask(context.Background(), nil, ListingKitTaskInput{Source: testAlibaba1688SourceEnvelopeInput()})
	if err == nil {
		t.Fatal("CreateListingKitTask(nil creator) error = nil, want error")
	}
	if task != nil || handoff != nil {
		t.Fatalf("task/handoff = (%+v, %+v), want nils when creator is missing", task, handoff)
	}
}

func TestCreateListingKitTaskBlocksMissingProduct(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	task, handoff, err := CreateListingKitTask(context.Background(), creator, ListingKitTaskInput{
		Source: sourcea1688.Alibaba1688SourceEnvelopeInput{
			Request: sourcea1688.Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/778.html"},
			Error:   errors.New("crawler failed"),
		},
		Platforms: []string{"shein"},
	})
	if err == nil {
		t.Fatal("CreateListingKitTask(missing product) error = nil, want error")
	}
	if task != nil {
		t.Fatalf("task = %+v, want nil", task)
	}
	if handoff == nil || handoff.Envelope.Identity.SourceID != "778" {
		t.Fatalf("handoff = %+v, want envelope for explainability", handoff)
	}
	if creator.request != nil {
		t.Fatalf("creator request = %+v, want no task creation on blocking warning", creator.request)
	}
}

func testAlibaba1688SourceEnvelopeInput() sourcea1688.Alibaba1688SourceEnvelopeInput {
	return sourcea1688.Alibaba1688SourceEnvelopeInput{
		Request: sourcea1688.Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/777.html?spm=adapter", StoreID: 11},
		Product: &sourcea1688.Alibaba1688ProductSnapshot{
			ID:       "777",
			Title:    "Insulated Lunch Bag",
			URL:      "https://detail.1688.com/offer/777.html?foo=bar",
			Images:   []string{"https://img.example/777-main.jpg", "https://img.example/777-side.jpg"},
			MinPrice: 18.8,
			Currency: "CNY",
			Category: "Bags>Lunch Bags",
			Brand:    "Factory Lunch",
			Supplier: sourcea1688.Alibaba1688SupplierSnapshot{ID: "supplier-777", Name: "Lunch Factory"},
			Variants: []sourcea1688.Alibaba1688VariantSnapshot{{
				Name:       "Black",
				Image:      "https://img.example/777-black.jpg",
				Price:      19.9,
				Attributes: map[string]any{"Color": "Black"},
			}},
			ProductDetails: []sourcea1688.Alibaba1688ProductDetailSnapshot{{Content: "Thermal lunch bag with zipper."}},
		},
		SourceRunID: "run-1688",
		RequestID:   "request-1688",
	}
}

type fakeGenerateTaskCreator struct {
	request *listingkit.GenerateRequest
	events  *[]string
}

func (f *fakeGenerateTaskCreator) CreateGenerateTask(_ context.Context, request *listingkit.GenerateRequest) (*listingkit.Task, error) {
	if f.events != nil {
		*f.events = append(*f.events, "create")
	}
	f.request = request
	return &listingkit.Task{ID: "task-1688", Request: request}, nil
}

type recordingSourcePublisher struct {
	request *sourcing.PublishRequest
	events  *[]string
}

func (p *recordingSourcePublisher) Publish(_ context.Context, request sourcing.PublishRequest) (catalog.PublishedSnapshot, error) {
	if p.events != nil {
		*p.events = append(*p.events, "publish")
	}
	p.request = &request
	return catalog.PublishedSnapshot{
		Identity:      catalog.SnapshotIdentity{TenantID: request.TenantID, ProductKey: request.ProductKey},
		Version:       1,
		PublicationID: request.PublicationID,
	}, nil
}
