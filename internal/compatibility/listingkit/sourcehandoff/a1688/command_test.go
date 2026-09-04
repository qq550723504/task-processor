package a1688

import (
	"context"
	"errors"
	"strings"
	"testing"

	"task-processor/internal/authidentity"
	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/listingkit"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/sourceaccount"
)

func TestTaskCommandServiceCreateTaskDelegatesToListingKitCreator(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	service := NewTaskCommandService(creator, validStoreAccessValidator())

	result, err := service.CreateTask(authenticatedCommandContext("101", "user-1688"), CreateTaskCommand{
		URL:             " https://detail.1688.com/offer/888.html?spm=command ",
		Product:         commandProduct1688("888"),
		RawSnapshot:     "raw-888",
		SourceRunID:     "run-888",
		RequestID:       "request-888",
		SourceAccountID: 3001,
		TenantID:        " 101 ",
		UserID:          " user-1688 ",
		Platforms:       []string{" SHEIN ", "shein"},
		Country:         " US ",
		Language:        " en_US ",
		SheinStoreID:    168811,
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
	if got := result.Handoff.Envelope.Identity.Key(); got != "1688:cn:888" {
		t.Fatalf("Key() = %q, want neutral source identity", got)
	}
	if result.Handoff.Request.ProductKey != "crawler:1688:888" {
		t.Fatalf("ProductKey = %q, want normalized source identity", result.Handoff.Request.ProductKey)
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
	if creator.request == nil || creator.request.ProductKey != "crawler:1688:888" {
		t.Fatalf("creator request = %+v, want normalized request", creator.request)
	}
	if creator.request.Source == nil {
		t.Fatal("creator request Source = nil, want 1688 source reference")
	}
	if creator.request.Source.Key != "crawler:1688:888" ||
		creator.request.Source.Platform != "1688" ||
		creator.request.Source.ID != "888" ||
		creator.request.Source.URL != "https://detail.1688.com/offer/888.html" {
		t.Fatalf("creator request Source = %+v, want normalized 1688 identity", creator.request.Source)
	}
}

func TestTaskCommandServicePublishesSourceSnapshotBeforeTaskCreation(t *testing.T) {
	events := []string{}
	creator := &fakeGenerateTaskCreator{events: &events}
	publisher := &recordingSourcePublisher{events: &events}
	service := NewTaskCommandService(creator, validStoreAccessValidator(), publisher)

	result, err := service.CreateTask(authenticatedCommandContext("101", "user-1688"), CreateTaskCommand{
		URL:          "https://detail.1688.com/offer/888.html",
		Product:      commandProduct1688("888"),
		SourceRunID:  "run-888",
		RequestID:    "request-888",
		TenantID:     "101",
		UserID:       "user-1688",
		SheinStoreID: 168811,
		Platforms:    []string{"shein"},
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if result == nil || result.Task == nil {
		t.Fatalf("result = %+v, want created task", result)
	}
	if publisher.request == nil {
		t.Fatal("source snapshot was not published")
	}
	if publisher.request.TenantID != "101" || publisher.request.ProductKey != "crawler:1688:888" {
		t.Fatalf("publication identity = %+v, want tenant 101 and crawler:1688:888", publisher.request)
	}
	if len(publisher.request.Envelope.AssetCandidates) == 0 {
		t.Fatal("published envelope has no source assets")
	}
	if got := strings.Join(events, ","); got != "publish,create" {
		t.Fatalf("event order = %q, want publish,create", got)
	}
	if result.Task.SourceSnapshotVersion != 1 {
		t.Fatalf("task source snapshot version = %d, want published version 1", result.Task.SourceSnapshotVersion)
	}
}

func TestTaskCommandServicePinsChangedImportsToDistinctPublicationIdentitiesWithoutSourceRunID(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	publisher := &recordingSourcePublisher{}
	service := NewTaskCommandService(creator, validStoreAccessValidator(), publisher)
	command := CreateTaskCommand{
		URL: "https://detail.1688.com/offer/888.html", TenantID: "101", UserID: "user-1688",
		SheinStoreID: 168811, Platforms: []string{"shein"}, Product: commandProduct1688("888"),
	}
	if _, err := service.CreateTask(authenticatedCommandContext("101", "user-1688"), command); err != nil {
		t.Fatalf("first CreateTask() error = %v", err)
	}
	firstPublicationID := publisher.request.PublicationID
	command.Product.Title = "updated title"
	if _, err := service.CreateTask(authenticatedCommandContext("101", "user-1688"), command); err != nil {
		t.Fatalf("second CreateTask() error = %v", err)
	}
	if publisher.request.PublicationID == firstPublicationID {
		t.Fatalf("publication ID = %q, want changed identity for changed raw snapshot", publisher.request.PublicationID)
	}
}

func TestTaskCommandServiceBindsTaskCreationToIdempotentSourceIdentity(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	publisher := &recordingSourcePublisher{}
	service := NewTaskCommandService(creator, validStoreAccessValidator(), publisher)
	command := CreateTaskCommand{
		URL: "https://detail.1688.com/offer/888.html", TenantID: "101", UserID: "user-1688",
		SheinStoreID: 168811, Platforms: []string{"shein"}, Product: commandProduct1688("888"),
		SourceRunID: "run-888", RequestID: "request-888",
	}
	if _, err := service.CreateTask(authenticatedCommandContext("101", "user-1688"), command); err != nil {
		t.Fatalf("first CreateTask() error = %v", err)
	}
	if creator.request.IdempotencyKey != "source-run:run-888" {
		t.Fatalf("IdempotencyKey = %q, want task creation bound to the source run identity", creator.request.IdempotencyKey)
	}
	if publisher.request == nil || creator.request.IdempotencyKey != publisher.request.PublicationID {
		t.Fatalf("idempotency key %q does not match publication ID %v, want a single durable source identity", creator.request.IdempotencyKey, publisher.request)
	}
}

func TestSourcePublicationIDBoundsLongSourceRunIDs(t *testing.T) {
	publicationID := sourcePublicationID(sourcing.SourceEnvelope{Trace: sourcing.SourceTrace{SourceRunID: strings.Repeat("r", 118)}})
	if len(publicationID) > 128 {
		t.Fatalf("publication ID length = %d, want at most 128", len(publicationID))
	}
	if !strings.HasPrefix(publicationID, "source-run-hash:") {
		t.Fatalf("publication ID = %q, want hashed long source run ID", publicationID)
	}
}

func TestTaskCommandServiceNormalizesSourceAccountAndValidatesSheinTargetStore(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	validator := validStoreAccessValidator()

	result, err := NewTaskCommandService(creator, validator).CreateTask(authenticatedCommandContext("101", "user-1688"), CreateTaskCommand{
		URL:             "https://detail.1688.com/offer/3001.html",
		Product:         commandProduct1688("3001"),
		SourceAccountID: 3001,
		TenantID:        "101",
		UserID:          "user-1688",
		SheinStoreID:    168811,
		Platforms:       []string{"shein"},
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if result.Handoff.Envelope.Identity.StoreID != 0 {
		t.Fatalf("source identity store id = %d, want neutral store id omitted", result.Handoff.Envelope.Identity.StoreID)
	}
	if len(validator.calls) != 1 || validator.calls[0].storeID != 168811 || validator.calls[0].platform != "SHEIN" {
		t.Fatalf("validator calls = %+v, want only SHEIN target validation", validator.calls)
	}
}

func TestTaskCommandServiceRejectsWrongStorePlatform(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	validator := &storeAccessValidatorFake{errs: map[int64]error{
		3001: listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable"),
	}}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "101", UserID: "user-1"})
	_, err := NewTaskCommandService(creator, validator).CreateTask(ctx, CreateTaskCommand{URL: "https://detail.1688.com/offer/893.html", Product: commandProduct1688("893"), TenantID: "101", UserID: "user-1", SourceAccountID: 3001, SheinStoreID: 168811, Platforms: []string{"shein"}})
	if listingkit.StoreAccessErrorCode(err) != listingkit.StoreAccessUnavailable {
		t.Fatalf("StoreAccessErrorCode() = %q, want unavailable (err=%v)", listingkit.StoreAccessErrorCode(err), err)
	}
	if creator.request != nil {
		t.Fatalf("creator request = %+v, want no task creation", creator.request)
	}
}

func TestTaskCommandServiceRejectsDisabledSourceAccount(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	validator := validStoreAccessValidator()
	validator.errs[3001] = listingkit.NewStoreAccessError(listingkit.StoreAccessDisabled, "store is disabled")
	_, err := NewTaskCommandService(creator, validator).CreateTask(authenticatedCommandContext("101", "user-1"), CreateTaskCommand{URL: "https://detail.1688.com/offer/894.html", Product: commandProduct1688("894"), TenantID: "101", UserID: "user-1", SourceAccountID: 3001, SheinStoreID: 168811, Platforms: []string{"shein"}})
	if listingkit.StoreAccessErrorCode(err) != listingkit.StoreAccessDisabled || creator.request != nil {
		t.Fatalf("err=%v request=%+v, want disabled store rejection", err, creator.request)
	}
}

func TestTaskCommandServiceRejectsUnavailableSourceAccountBeforeTaskCreation(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	validator := &storeAccessValidatorFake{
		errs: map[int64]error{3001: listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable")},
	}

	_, err := NewTaskCommandService(creator, validator).CreateTask(authenticatedCommandContext("101", "user-1"), CreateTaskCommand{
		URL:             "https://detail.1688.com/offer/895.html",
		Product:         commandProduct1688("895"),
		TenantID:        "101",
		UserID:          "user-1",
		SourceAccountID: 3001,
		SheinStoreID:    168811,
		Platforms:       []string{"shein"},
	})

	if listingkit.StoreAccessErrorCode(err) != listingkit.StoreAccessUnavailable {
		t.Fatalf("StoreAccessErrorCode() = %q, want %q (err=%v)", listingkit.StoreAccessErrorCode(err), listingkit.StoreAccessUnavailable, err)
	}
	if creator.request != nil {
		t.Fatalf("creator request = %+v, want nil", creator.request)
	}
	if len(validator.calls) != 1 || validator.calls[0].platform != "SHEIN" {
		t.Fatalf("validator calls = %+v, want SHEIN validation before source account rejection", validator.calls)
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

func (v *storeAccessValidatorFake) ValidateSourceAccountAccess(_ context.Context, tenantID, accountID int64) (sourceaccount.Access, error) {
	if err := v.errs[accountID]; err != nil {
		return sourceaccount.Access{}, err
	}
	return sourceaccount.Access{ID: accountID, TenantID: tenantID, Platform: sourceaccount.PlatformAlibaba1688, Enabled: true}, nil
}

func TestTaskCommandServiceRejectsMismatchedContextTenant(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "tenant-verified", UserID: "user-verified"})

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

func TestTaskCommandServiceRejectsLegacyListingKitIdentityWithoutAuthenticatedIdentity(t *testing.T) {
	creator := &fakeGenerateTaskCreator{}
	ctx := listingkit.WithTenantID(context.Background(), "101")
	ctx = listingkit.WithRequestIdentity(ctx, listingkit.RequestIdentity{TenantID: "101", UserID: "user-legacy"})

	_, err := NewTaskCommandService(creator).CreateTask(ctx, CreateTaskCommand{
		URL:       "https://detail.1688.com/offer/893.html",
		Product:   commandProduct1688("893"),
		TenantID:  "101",
		UserID:    "user-legacy",
		Platforms: []string{"shein"},
	})
	if err == nil {
		t.Fatal("CreateTask() error = nil, want verified authidentity requirement")
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
		TenantID: "101", UserID: "user-1", SourceAccountID: 3001, SheinStoreID: 168811,
		Platforms: []string{"shein"},
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if result.Handoff.Request.ProductKey != "crawler:1688:889" {
		t.Fatalf("ProductKey = %q, want source identity from product URL fallback", result.Handoff.Request.ProductKey)
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
		TenantID: "101", UserID: "user-1", SourceAccountID: 3001, SheinStoreID: 168811,
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
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: tenantID, UserID: userID})
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
