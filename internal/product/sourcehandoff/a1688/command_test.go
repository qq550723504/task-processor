package a1688

import (
	"context"
	"errors"
	"testing"

	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/listingkit"
)

func TestTaskCommandServiceCreateTaskDelegatesToListingKitCreator(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	service := NewTaskCommandService(creator)

	result, err := service.CreateTask(context.Background(), CreateTaskCommand{
		URL:         " https://detail.1688.com/offer/888.html?spm=command ",
		Product:     commandProduct1688("888"),
		RawSnapshot: "raw-888",
		SourceRunID: "run-888",
		RequestID:   "request-888",
		TenantID:    " tenant-1688 ",
		UserID:      " user-1688 ",
		Platforms:   []string{" SHEIN ", "shein"},
		Country:     " US ",
		Language:    " en_US ",
		SheinStoreID: 168811,
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if result == nil || result.Task == nil || result.Task.ID != "task-1688" {
		t.Fatalf("result = %+v, want delegated task", result)
	}
	if result.Handoff == nil {
		t.Fatal("handoff is nil")
	}
	if got := result.Handoff.Envelope.Identity.SourceKey(); got != "crawler:1688:888" {
		t.Fatalf("SourceKey() = %q, want crawler:1688:888", got)
	}
	if result.Handoff.Request.ProductURL != "https://detail.1688.com/offer/888.html" {
		t.Fatalf("ProductURL = %q, want normalized command URL", result.Handoff.Request.ProductURL)
	}
	if result.Handoff.Request.TenantID != "tenant-1688" || result.Handoff.Request.UserID != "user-1688" {
		t.Fatalf("request tenant/user = %q/%q, want trimmed values", result.Handoff.Request.TenantID, result.Handoff.Request.UserID)
	}
	if len(result.Handoff.Request.Platforms) != 1 || result.Handoff.Request.Platforms[0] != "shein" {
		t.Fatalf("Platforms = %#v, want normalized deduped shein", result.Handoff.Request.Platforms)
	}
	if creator.request == nil || creator.request.ProductURL != "https://detail.1688.com/offer/888.html" {
		t.Fatalf("creator request = %+v, want normalized request", creator.request)
	}
}

func TestTaskCommandServiceCreateTaskFallsBackToProductURL(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	product := commandProduct1688("889")
	product.URL = "https://detail.1688.com/offer/889.html?from=product"

	result, err := NewTaskCommandService(creator).CreateTask(context.Background(), CreateTaskCommand{
		Product:   product,
		Platforms: []string{"shein"},
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if result.Handoff.Request.ProductURL != "https://detail.1688.com/offer/889.html" {
		t.Fatalf("ProductURL = %q, want product URL fallback", result.Handoff.Request.ProductURL)
	}
}

func TestTaskCommandServiceCreateTaskRequiresCreator(t *testing.T) {
	result, err := NewTaskCommandService(nil).CreateTask(context.Background(), CreateTaskCommand{URL: "https://detail.1688.com/offer/890.html"})
	if err == nil {
		t.Fatal("CreateTask(nil creator) error = nil, want error")
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil when creator is missing", result)
	}
}

func TestTaskCommandServiceCreateTaskRequiresURL(t *testing.T) {
	result, err := NewTaskCommandService(&fakeGenerateTaskCreator{}).CreateTask(context.Background(), CreateTaskCommand{})
	if err == nil {
		t.Fatal("CreateTask(missing URL) error = nil, want error")
	}
	if result != nil {
		t.Fatalf("result = %+v, want nil when URL is missing", result)
	}
}

func TestTaskCommandServiceCreateTaskReturnsHandoffOnSourceError(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	result, err := NewTaskCommandService(creator).CreateTask(context.Background(), CreateTaskCommand{
		URL:       "https://detail.1688.com/offer/891.html",
		Error:     errors.New("crawler failed"),
		Platforms: []string{"shein"},
	})
	if err == nil {
		t.Fatal("CreateTask(source error) error = nil, want error")
	}
	if result == nil || result.Handoff == nil || result.Handoff.Envelope.Identity.SourceID != "891" {
		t.Fatalf("result = %+v, want handoff with source identity", result)
	}
	if creator.request != nil {
		t.Fatalf("creator request = %+v, want no task creation", creator.request)
	}
}

func commandProduct1688(id string) *alibaba1688model.Product1688 {
	return &alibaba1688model.Product1688{
		ID:       id,
		Title:    "Insulated Lunch Bag",
		URL:      "https://detail.1688.com/offer/" + id + ".html?foo=bar",
		Images:   []string{"https://img.example/" + id + "-main.jpg", "https://img.example/" + id + "-side.jpg"},
		MinPrice: 18.8,
		Currency: "CNY",
		Category: "Bags>Lunch Bags",
		Brand:    "Factory Lunch",
		Supplier: alibaba1688model.SupplierInfo{ID: "supplier-" + id, Name: "Lunch Factory"},
		Variants: []alibaba1688model.Variant{{
			Name:       "Black",
			Image:      "https://img.example/" + id + "-black.jpg",
			Price:      19.9,
			Attributes: map[string]any{"Color": "Black"},
		}},
		ProductDetails: []alibaba1688model.ProductDetail{{Content: "Thermal lunch bag with zipper."}},
	}
}

var _ = listingkit.GenerateRequest{}
