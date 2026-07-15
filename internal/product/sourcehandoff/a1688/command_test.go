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
	service := NewTaskCommandService(creator, validStoreAccessValidator())

	result, err := service.CreateTask(authenticatedCommandContext("101", "user-1688"), CreateTaskCommand{
		URL:           " https://detail.1688.com/offer/888.html?spm=command ",
		Product:       commandProduct1688("888"),
		RawSnapshot:   "raw-888",
		SourceRunID:   "run-888",
		RequestID:     "request-888",
		SourceStoreID: 3001,
		TenantID:      " 101 ",
		UserID:        " user-1688 ",
		Platforms:     []string{" SHEIN ", "shein"},
		Country:       " US ",
		Language:      " en_US ",
		SheinStoreID:  168811,
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
	if got := result.Handoff.Envelope.Identity.Key(); got != "1688:cn:888:3001" {
		t.Fatalf("Key() = %q, want source store identity", got)
	}
	if result.Handoff.Request.ProductURL != "https://detail.1688.com/offer/888.html" {
		t.Fatalf("ProductURL = %q, want normalized command URL", result.Handoff.Request.ProductURL)
	}
	if result.Handoff.Request.TenantID != "101" || result.Handoff.Request.UserID != "user-1688" {
		t.Fatalf("request tenant/user = %q/%q, want trimmed values", result.Handoff.Request.TenantID, result.Handoff.Request.UserID)
	}
	if result.Handoff.Request.SheinStoreID != 168811 {
		t.Fatalf("SheinStoreID = %d, want target store id", result.Handoff.Request.SheinStoreID)
	}
	if len(result.Handoff.Request.Platforms) != 1 || result.Handoff.Request.Platforms[0] != "shein" {
		t.Fatalf("Platforms = %#v, want normalized deduped shein", result.Handoff.Request.Platforms)
	}
	if creator.request == nil || creator.request.ProductURL != "https://detail.1688.com/offer/888.html" {
		t.Fatalf("creator request = %+v, want normalized request", creator.request)
	}
}

func TestTaskCommandServiceRejectsWrongStorePlatform(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	validator := &storeAccessValidatorFake{errs: map[int64]error{
		3001: listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable"),
	}}
	ctx := listingkit.WithTenantID(context.Background(), "101")
	ctx = listingkit.WithRequestIdentity(ctx, listingkit.RequestIdentity{TenantID: "101", UserID: "user-1"})
	_, err := NewTaskCommandService(creator, validator).CreateTask(ctx, CreateTaskCommand{URL: "https://detail.1688.com/offer/893.html", Product: commandProduct1688("893"), TenantID: "101", UserID: "user-1", SourceStoreID: 3001, SheinStoreID: 168811, Platforms: []string{"shein"}})
	if listingkit.StoreAccessErrorCode(err) != listingkit.StoreAccessUnavailable {
		t.Fatalf("StoreAccessErrorCode() = %q, want unavailable (err=%v)", listingkit.StoreAccessErrorCode(err), err)
	}
	if creator.request != nil {
		t.Fatalf("creator request = %+v, want no task creation", creator.request)
	}
}

func TestTaskCommandServiceRejectsDisabledSourceStore(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	validator := validStoreAccessValidator()
	validator.errs[3001] = listingkit.NewStoreAccessError(listingkit.StoreAccessDisabled, "store is disabled")
	_, err := NewTaskCommandService(creator, validator).CreateTask(authenticatedCommandContext("101", "user-1"), CreateTaskCommand{URL: "https://detail.1688.com/offer/894.html", Product: commandProduct1688("894"), TenantID: "101", UserID: "user-1", SourceStoreID: 3001, SheinStoreID: 168811, Platforms: []string{"shein"}})
	if listingkit.StoreAccessErrorCode(err) != listingkit.StoreAccessDisabled || creator.request != nil {
		t.Fatalf("err=%v request=%+v, want disabled store rejection", err, creator.request)
	}
}

func TestTaskCommandServiceRejectsUnavailableSourceStoreBeforeTaskCreation(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	validator := &storeAccessValidatorFake{
		errs: map[int64]error{3001: listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable")},
	}

	_, err := NewTaskCommandService(creator, validator).CreateTask(authenticatedCommandContext("101", "user-1"), CreateTaskCommand{
		URL:           "https://detail.1688.com/offer/895.html",
		Product:       commandProduct1688("895"),
		TenantID:      "101",
		UserID:        "user-1",
		SourceStoreID: 3001,
		SheinStoreID:  168811,
		Platforms:     []string{"shein"},
	})

	if listingkit.StoreAccessErrorCode(err) != listingkit.StoreAccessUnavailable {
		t.Fatalf("StoreAccessErrorCode() = %q, want %q (err=%v)", listingkit.StoreAccessErrorCode(err), listingkit.StoreAccessUnavailable, err)
	}
	if creator.request != nil {
		t.Fatalf("creator request = %+v, want nil", creator.request)
	}
	if len(validator.calls) != 1 || validator.calls[0].platform != "1688" {
		t.Fatalf("validator calls = %+v, want source store only", validator.calls)
	}
}

type storeAccessValidatorCall struct {
	tenantID int64
	storeID  int64
	platform string
}

type storeAccessValidatorFake struct {
	errs  map[int64]error
	calls []storeAccessValidatorCall
}

func (v *storeAccessValidatorFake) ValidateStoreAccess(_ context.Context, tenantID, storeID int64, platform string) (listingkit.StoreAccess, error) {
	v.calls = append(v.calls, storeAccessValidatorCall{tenantID: tenantID, storeID: storeID, platform: platform})
	if err := v.errs[storeID]; err != nil {
		return listingkit.StoreAccess{}, err
	}
	return listingkit.StoreAccess{ID: storeID, TenantID: tenantID, Platform: platform, Enabled: true}, nil
}

func TestTaskCommandServiceRejectsMismatchedContextTenant(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	ctx := listingkit.WithTenantID(context.Background(), "tenant-verified")
	ctx = listingkit.WithRequestIdentity(ctx, listingkit.RequestIdentity{TenantID: "tenant-verified", UserID: "user-verified"})

	_, err := NewTaskCommandService(creator).CreateTask(ctx, CreateTaskCommand{
		URL:       "https://detail.1688.com/offer/892.html",
		Product:   commandProduct1688("892"),
		TenantID:  "tenant-attacker",
		UserID:    "user-verified",
		Platforms: []string{"shein"},
	})
	if err == nil {
		t.Fatal("CreateTask() error = nil, want tenant mismatch rejection")
	}
	if creator.request != nil {
		t.Fatalf("creator request = %+v, want no task creation", creator.request)
	}
}

func TestTaskCommandServiceCreateTaskFallsBackToProductURL(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	product := commandProduct1688("889")
	product.URL = "https://detail.1688.com/offer/889.html?from=product"

	result, err := NewTaskCommandService(creator, validStoreAccessValidator()).CreateTask(authenticatedCommandContext("101", "user-1"), CreateTaskCommand{
		Product:  product,
		TenantID: "101", UserID: "user-1", SourceStoreID: 3001, SheinStoreID: 168811,
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
	result, err := NewTaskCommandService(creator, validStoreAccessValidator()).CreateTask(authenticatedCommandContext("101", "user-1"), CreateTaskCommand{
		URL:      "https://detail.1688.com/offer/891.html",
		TenantID: "101", UserID: "user-1", SourceStoreID: 3001, SheinStoreID: 168811,
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

func authenticatedCommandContext(tenantID, userID string) context.Context {
	ctx := listingkit.WithTenantID(context.Background(), tenantID)
	return listingkit.WithRequestIdentity(ctx, listingkit.RequestIdentity{TenantID: tenantID, UserID: userID})
}

func validStoreAccessValidator() *storeAccessValidatorFake {
	return &storeAccessValidatorFake{errs: make(map[int64]error)}
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
