package storecenter_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/listingsubscription"
	"task-processor/internal/storecenter"
)

func TestServiceCreateActivatesOneReservedStore(t *testing.T) {
	t.Parallel()
	const organizationID = "org-1"
	const actor = "subject-1"
	requestKey := uuid.NewString()
	allocationID := uuid.NewString()
	storeID := uuid.NewString()
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	repository := newStoreRepositoryFake()
	ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{
		OrganizationID: organizationID, AllocationID: allocationID, StoreID: storeID, RequestKey: requestKey, Status: listingsubscription.StoreQuotaReserved,
	}}
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, func() time.Time { return now })
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	result, err := service.Create(context.Background(), storecenter.CreateStoreRequest{
		OrganizationID: organizationID, ActorSubject: actor, IdempotencyKey: requestKey,
		Name: "  My Shop  ", Platform: " SHEIN ", Region: "  SG  ", ExternalStoreID: "  external-1  ",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Replayed {
		t.Fatal("Create() Replayed = true, want false")
	}
	if result.Store.ID() != storeID || result.Store.QuotaAllocationID() != allocationID {
		t.Fatalf("Create() IDs = store %q allocation %q, want allocated IDs", result.Store.ID(), result.Store.QuotaAllocationID())
	}
	if result.Store.LifecycleStatus() != storecenter.StoreStatusActive || result.Store.Version() != 2 {
		t.Fatalf("Create() durable lifecycle/version = %s/%d, want active/2", result.Store.LifecycleStatus(), result.Store.Version())
	}
	if result.Store.Name() != "My Shop" || result.Store.Platform() != storecenter.PlatformShein || result.Store.Region() != "SG" {
		t.Fatalf("Create() normalized Store = %+v, want normalized fields", result.Store.Snapshot())
	}
	if ledger.reserveCalls != 1 || ledger.commitCalls != 1 || ledger.releaseCalls != 0 {
		t.Fatalf("quota calls reserve/commit/release = %d/%d/%d, want 1/1/0", ledger.reserveCalls, ledger.commitCalls, ledger.releaseCalls)
	}
	if got := audit.actionsFor(organizationID, requestKey); !sameStrings(got, []string{"quota_reserved", "store_created", "quota_commit_started", "store_creation_committed"}) {
		t.Fatalf("audit actions = %v, want safe durable phases", got)
	}
}

func TestServiceCreateDefinitiveDuplicateReleasesAndReplaysStableFailure(t *testing.T) {
	const organizationID = "org-1"
	const actor = "subject-1"
	requestKey := uuid.NewString()
	allocationID := uuid.NewString()
	storeID := uuid.NewString()
	repository := newStoreRepositoryFake()
	repository.createErr = storecenter.ErrAlreadyExists
	ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{
		OrganizationID: organizationID, AllocationID: allocationID, StoreID: storeID, RequestKey: requestKey, Status: listingsubscription.StoreQuotaReserved,
	}}
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, func() time.Time { return time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	request := storecenter.CreateStoreRequest{OrganizationID: organizationID, ActorSubject: actor, IdempotencyKey: requestKey, Name: "Shop", Platform: "shein", Region: "SG"}

	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrAlreadyExists) {
		t.Fatalf("Create() error = %v, want stable ErrAlreadyExists", err)
	}
	if ledger.releaseCalls != 1 || ledger.allocation.Status != listingsubscription.StoreQuotaReleased {
		t.Fatalf("Create() release = %d/%s, want one released reservation", ledger.releaseCalls, ledger.allocation.Status)
	}
	if got := audit.actionsFor(organizationID, requestKey); !sameStrings(got, []string{"quota_reserved", "store_create_failed"}) {
		t.Fatalf("audit actions = %v, want terminal redacted failure", got)
	}

	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrAlreadyExists) {
		t.Fatalf("replay Create() error = %v, want stable ErrAlreadyExists", err)
	}
	if repository.createCalls != 1 || ledger.releaseCalls != 1 {
		t.Fatalf("replay calls create/release = %d/%d, want terminal replay without mutation", repository.createCalls, ledger.releaseCalls)
	}
}

func TestServiceCreateRejectsInvalidInputBeforeDependencies(t *testing.T) {
	repository := newStoreRepositoryFake()
	ledger := &quotaLedgerFake{}
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, time.Now)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	_, err = service.Create(context.Background(), storecenter.CreateStoreRequest{OrganizationID: " org-1", ActorSubject: "actor", IdempotencyKey: uuid.NewString(), Name: "Shop", Platform: "shein", Region: "SG"})
	if err == nil {
		t.Fatal("Create() invalid input error = nil")
	}
	if ledger.reserveCalls != 0 || repository.createCalls != 0 || audit.recordCalls != 0 {
		t.Fatalf("invalid Create() calls reserve/create/audit = %d/%d/%d, want 0/0/0", ledger.reserveCalls, repository.createCalls, audit.recordCalls)
	}
}

func TestServiceCreateMapsQuotaExceededWithoutRepositoryMutation(t *testing.T) {
	repository := newStoreRepositoryFake()
	ledger := &quotaLedgerFake{reserveErr: &listingsubscription.StoreQuotaExceededError{Committed: 2, Reserved: 1, Limit: 3}}
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, time.Now)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	_, err = service.Create(context.Background(), validCreateRequest())
	var limit *storecenter.StoreLimitReachedError
	if !errors.Is(err, storecenter.ErrLimitReached) || !errors.As(err, &limit) || limit.Committed != 2 || limit.Reserved != 1 || limit.Limit != 3 {
		t.Fatalf("Create() error = %#v, want safe limit error", err)
	}
	if repository.createCalls != 0 || audit.recordCalls != 0 {
		t.Fatalf("quota rejection calls create/audit = %d/%d, want 0/0", repository.createCalls, audit.recordCalls)
	}
}

func TestServiceCreateRejectsMalformedQuotaExceededDetails(t *testing.T) {
	ledger := &quotaLedgerFake{reserveErr: &listingsubscription.StoreQuotaExceededError{Committed: -1, Reserved: 0, Limit: 0}}
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, newAuditRepositoryFake(), time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	_, err = service.Create(context.Background(), validCreateRequest())
	if !errors.Is(err, storecenter.ErrDependencyUnavailable) || err.Error() != storecenter.ErrDependencyUnavailable.Error() {
		t.Fatalf("Create() malformed limit error = %v, want safe dependency sentinel", err)
	}
}

func TestServiceCreateRedactsDependencyTextEvenWhenSentinelIsWrapped(t *testing.T) {
	ledger := &quotaLedgerFake{reserveErr: fmt.Errorf("driver password text: %w", storecenter.ErrDependencyUnavailable)}
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, newAuditRepositoryFake(), time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	_, err = service.Create(context.Background(), validCreateRequest())
	if !errors.Is(err, storecenter.ErrDependencyUnavailable) || err.Error() != storecenter.ErrDependencyUnavailable.Error() || strings.Contains(err.Error(), "password") {
		t.Fatalf("Create() dependency error = %q, want redacted sentinel", err)
	}
}

func TestServiceCreateCommitFailureResumesWithoutRecreate(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	ledger.commitErr = errors.New("ledger unavailable")
	repository := newStoreRepositoryFake()
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, time.Now)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Create() error = %v, want dependency unavailable", err)
	}
	created, err := repository.Get(context.Background(), request.OrganizationID, ledger.allocation.StoreID)
	if err != nil || created.LifecycleStatus() != storecenter.StoreStatusProvisioning {
		t.Fatalf("commit failure Store = %v/%v, want provisioning durable Store", created, err)
	}
	ledger.commitErr = nil
	result, err := service.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("replay Create() error = %v", err)
	}
	if result.Store.LifecycleStatus() != storecenter.StoreStatusActive || repository.createCalls != 1 || ledger.releaseCalls != 0 {
		t.Fatalf("replay lifecycle/create/release = %s/%d/%d, want active/1/0", result.Store.LifecycleStatus(), repository.createCalls, ledger.releaseCalls)
	}
}

func TestServiceCreateAmbiguousCreateFoundNeverReleases(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	repository.createErr = errors.New("ambiguous persistence outcome")
	repository.persistBeforeCreateError = true
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, time.Now)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	result, err := service.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Store.LifecycleStatus() != storecenter.StoreStatusActive || ledger.releaseCalls != 0 {
		t.Fatalf("ambiguous Create() lifecycle/release = %s/%d, want active/0", result.Store.LifecycleStatus(), ledger.releaseCalls)
	}
}

func TestServiceCreateFinalAuditFailureResumesWithoutReactivation(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionStoreCreationCommitted: 1}
	service, err := storecenter.NewService(repository, ledger, audit, time.Now)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Create() error = %v, want dependency unavailable", err)
	}
	if repository.saveCalls != 1 {
		t.Fatalf("first Create() saves = %d, want one activation", repository.saveCalls)
	}
	result, err := service.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("replay Create() error = %v", err)
	}
	if !result.Replayed || result.Store.LifecycleStatus() != storecenter.StoreStatusActive || repository.saveCalls != 1 {
		t.Fatalf("replay = %+v saves=%d, want active replay without reactivation", result, repository.saveCalls)
	}
}

func TestServiceCreateAuditFailuresPauseAtTheirDurableBoundary(t *testing.T) {
	for _, tt := range []struct {
		name                     string
		action                   storecenter.AuditAction
		wantCreates, wantCommits int
	}{
		{name: "before store", action: storecenter.AuditActionQuotaReserved, wantCreates: 0, wantCommits: 0},
		{name: "after store", action: storecenter.AuditActionStoreCreated, wantCreates: 1, wantCommits: 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := validCreateRequest()
			ledger := quotaForRequest(request)
			repository := newStoreRepositoryFake()
			audit := newAuditRepositoryFake()
			audit.failActions = map[storecenter.AuditAction]int{tt.action: 1}
			service, err := storecenter.NewService(repository, ledger, audit, time.Now)
			if err != nil {
				t.Fatalf("NewService(): %v", err)
			}
			if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
				t.Fatalf("first Create() error = %v, want dependency unavailable", err)
			}
			if repository.createCalls != tt.wantCreates || ledger.commitCalls != tt.wantCommits {
				t.Fatalf("paused calls create/commit = %d/%d, want %d/%d", repository.createCalls, ledger.commitCalls, tt.wantCreates, tt.wantCommits)
			}
			result, err := service.Create(context.Background(), request)
			if err != nil {
				t.Fatalf("replay Create(): %v", err)
			}
			if result.Store.LifecycleStatus() != storecenter.StoreStatusActive || ledger.releaseCalls != 0 {
				t.Fatalf("replay = %+v release=%d, want active/no compensation", result, ledger.releaseCalls)
			}
		})
	}
}

func TestServiceCreateAmbiguousActivationSaveIsResolvedByScopedRead(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	repository.saveErr = errors.New("ambiguous save")
	repository.persistBeforeSaveError = true
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	result, err := service.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if result.Store.LifecycleStatus() != storecenter.StoreStatusActive || ledger.releaseCalls != 0 {
		t.Fatalf("Create() lifecycle/release = %s/%d, want active/0", result.Store.LifecycleStatus(), ledger.releaseCalls)
	}
}

func TestServiceCreateConcurrentExactRequestsConverge(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	results := make(chan storecenter.CreateStoreResult, 2)
	errorsOut := make(chan error, 2)
	var start sync.WaitGroup
	start.Add(1)
	for range 2 {
		go func() {
			start.Wait()
			result, err := service.Create(context.Background(), request)
			results <- result
			errorsOut <- err
		}()
	}
	start.Done()
	var ids []string
	for range 2 {
		result, err := <-results, <-errorsOut
		if err != nil {
			t.Fatalf("concurrent Create() error = %v", err)
		}
		ids = append(ids, result.Store.ID())
	}
	if ids[0] != ledger.allocation.StoreID || ids[1] != ledger.allocation.StoreID || len(repository.stores) != 1 || ledger.releaseCalls != 0 {
		t.Fatalf("concurrent IDs/stores/releases = %v/%d/%d, want one allocated durable Store and no release", ids, len(repository.stores), ledger.releaseCalls)
	}
}

func TestServiceCreateReplayReturnsLaterDisabledStoreWithoutReactivation(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	created, err := service.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("initial Create(): %v", err)
	}
	disabled := cloneStore(created.Store)
	if err := disabled.TransitionTo(storecenter.StoreStatusDisabled, request.ActorSubject, disabled.UpdatedAt().Add(time.Second)); err != nil {
		t.Fatalf("TransitionTo(disabled): %v", err)
	}
	if err := repository.Save(context.Background(), request.OrganizationID, disabled, created.Store.Version()); err != nil {
		t.Fatalf("Save(disabled): %v", err)
	}
	savesBeforeReplay := repository.saveCalls
	replayed, err := service.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("replay Create(): %v", err)
	}
	if !replayed.Replayed || replayed.Store.LifecycleStatus() != storecenter.StoreStatusDisabled || repository.saveCalls != savesBeforeReplay {
		t.Fatalf("replay = %+v saves=%d, want disabled durable replay without activation", replayed, repository.saveCalls)
	}
}

func TestServiceCreateCrossActorReplayKeepsFirstDurableAuditActor(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	db, err := gorm.Open(sqlite.Open("file:"+t.TempDir()+"/service-audit.db?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open SQLite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("audit DB handle: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := storecenter.AutoMigrateAuditRepository(db); err != nil {
		t.Fatalf("migrate audit: %v", err)
	}
	audit, err := storecenter.NewGormAuditRepository(db)
	if err != nil {
		t.Fatalf("NewGormAuditRepository: %v", err)
	}
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, audit, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); err != nil {
		t.Fatalf("first Create(): %v", err)
	}
	retry := request
	retry.ActorSubject = "actor-2"
	result, err := service.Create(context.Background(), retry)
	if err != nil {
		t.Fatalf("cross-actor replay Create(): %v", err)
	}
	if !result.Replayed {
		t.Fatal("cross-actor replay Replayed = false")
	}
	event, err := audit.Get(context.Background(), request.OrganizationID, request.IdempotencyKey, storecenter.AuditActionQuotaReserved)
	if err != nil || event.ActorSubject != request.ActorSubject {
		t.Fatalf("durable audit actor = %v/%v, want first actor", event, err)
	}
}

func TestServiceCreateNeverReleasesAfterAmbiguousConfirmationRead(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	repository.createErr = errors.New("ambiguous create")
	repository.getErrAfterCreate = errors.New("confirmation unavailable")
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("Create() error = %v, want dependency unavailable", err)
	}
	if ledger.releaseCalls != 0 {
		t.Fatalf("ambiguous confirmation release calls = %d, want 0", ledger.releaseCalls)
	}
}

func TestServiceCreateNeverResurrectsReleasedOrAllocatedAllocationWithoutStore(t *testing.T) {
	for _, status := range []listingsubscription.StoreQuotaAllocationStatus{listingsubscription.StoreQuotaReleased, listingsubscription.StoreQuotaAllocated} {
		t.Run(string(status), func(t *testing.T) {
			request := validCreateRequest()
			ledger := quotaForRequest(request)
			ledger.allocation.Status = status
			repository := newStoreRepositoryFake()
			service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), time.Now)
			if err != nil {
				t.Fatalf("NewService(): %v", err)
			}
			if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
				t.Fatalf("Create() error = %v, want dependency unavailable", err)
			}
			if repository.createCalls != 0 || ledger.releaseCalls != 0 {
				t.Fatalf("inconsistent allocation create/release = %d/%d, want 0/0", repository.createCalls, ledger.releaseCalls)
			}
		})
	}
}

func TestServiceCreateRejectsAllocationStoreRequestMismatchWithoutCompensation(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	mismatched, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: ledger.allocation.StoreID, OrganizationID: request.OrganizationID, ActorSubject: request.ActorSubject, Name: request.Name, Platform: request.Platform, Region: request.Region, CreateIdempotencyKey: uuid.NewString(), QuotaAllocationID: uuid.NewString(), OccurredAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("NewStore(mismatch): %v", err)
	}
	repository.stores[request.OrganizationID+"/"+ledger.allocation.StoreID] = mismatched
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("Create() error = %v, want dependency unavailable", err)
	}
	if repository.createCalls != 0 || ledger.releaseCalls != 0 {
		t.Fatalf("mismatch create/release = %d/%d, want 0/0", repository.createCalls, ledger.releaseCalls)
	}
}

func TestServiceCreateRejectsReleaseResultIdentityMismatch(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	ledger.releaseOverride = &listingsubscription.StoreQuotaAllocation{OrganizationID: request.OrganizationID, AllocationID: uuid.NewString(), StoreID: ledger.allocation.StoreID, RequestKey: request.IdempotencyKey, Status: listingsubscription.StoreQuotaReleased}
	repository := newStoreRepositoryFake()
	repository.createErr = storecenter.ErrAlreadyExists
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("Create() error = %v, want dependency unavailable", err)
	}
}

func TestServiceCreateTerminalFailureReplayRejectsReleaseResultMismatch(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	ledger.releaseOverride = &listingsubscription.StoreQuotaAllocation{OrganizationID: request.OrganizationID, AllocationID: uuid.NewString(), StoreID: ledger.allocation.StoreID, RequestKey: request.IdempotencyKey, Status: listingsubscription.StoreQuotaReleased}
	audit := newAuditRepositoryFake()
	_, _, err := audit.Record(context.Background(), storecenter.AuditEvent{EventID: uuid.NewString(), OrganizationID: request.OrganizationID, StoreID: ledger.allocation.StoreID, AllocationID: ledger.allocation.AllocationID, RequestKey: request.IdempotencyKey, Action: storecenter.AuditActionStoreCreateFailed, Outcome: storecenter.AuditOutcomeFailed, ActorSubject: request.ActorSubject, FailureCode: storecenter.AuditFailureAlreadyExists, OccurredAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("Record terminal audit: %v", err)
	}
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, audit, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("terminal replay Create() error = %v, want dependency unavailable", err)
	}
}

func TestServiceCreateCommitFailureAuditWriteAheadRecoversOnReplay(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	ledger.commitErr = errors.New("commit unavailable")
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionQuotaCommitFailed: 1}
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, audit, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Create() error = %v, want dependency unavailable", err)
	}
	if got := audit.actionsFor(request.OrganizationID, request.IdempotencyKey); !sameStrings(got, []string{"quota_reserved", "store_created", "quota_commit_started"}) {
		t.Fatalf("first audit actions = %v, want durable write-ahead only", got)
	}
	ledger.commitErr = nil
	result, err := service.Create(context.Background(), request)
	if err != nil {
		t.Fatalf("replay Create(): %v", err)
	}
	if result.Store.LifecycleStatus() != storecenter.StoreStatusActive || ledger.commitCalls != 2 {
		t.Fatalf("replay Store/commits = %s/%d, want active/2", result.Store.LifecycleStatus(), ledger.commitCalls)
	}
}

func validCreateRequest() storecenter.CreateStoreRequest {
	return storecenter.CreateStoreRequest{OrganizationID: "org-1", ActorSubject: "actor-1", IdempotencyKey: uuid.NewString(), Name: "Shop", Platform: "shein", Region: "SG"}
}
func quotaForRequest(request storecenter.CreateStoreRequest) *quotaLedgerFake {
	return &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: request.OrganizationID, AllocationID: uuid.NewString(), StoreID: uuid.NewString(), RequestKey: request.IdempotencyKey, Status: listingsubscription.StoreQuotaReserved}}
}

type storeRepositoryFake struct {
	mu                       sync.Mutex
	stores                   map[string]*storecenter.Store
	createErr                error
	getErr                   error
	getErrAfterCreate        error
	saveErr                  error
	persistBeforeCreateError bool
	persistBeforeSaveError   bool
	createCalls              int
	getCalls                 int
	saveCalls                int
}

func newStoreRepositoryFake() *storeRepositoryFake {
	return &storeRepositoryFake{stores: map[string]*storecenter.Store{}}
}

func (f *storeRepositoryFake) CreateOrReplay(_ context.Context, organizationID string, store *storecenter.Store) (*storecenter.Store, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createCalls++
	if f.createErr != nil {
		if f.getErrAfterCreate != nil {
			f.getErr = f.getErrAfterCreate
		}
		if f.persistBeforeCreateError {
			f.stores[organizationID+"/"+store.ID()] = cloneStore(store)
		}
		return nil, false, f.createErr
	}
	if existing := f.stores[organizationID+"/"+store.ID()]; existing != nil {
		return cloneStore(existing), true, nil
	}
	f.stores[organizationID+"/"+store.ID()] = cloneStore(store)
	return cloneStore(store), false, nil
}
func (f *storeRepositoryFake) List(context.Context, string, storecenter.StoreListQuery) (storecenter.StorePage, error) {
	return storecenter.StorePage{}, errors.New("not used")
}
func (f *storeRepositoryFake) Get(_ context.Context, organizationID, storeID string) (*storecenter.Store, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCalls++
	if f.getErr != nil {
		return nil, f.getErr
	}
	store := f.stores[organizationID+"/"+storeID]
	if store == nil {
		return nil, storecenter.ErrNotFound
	}
	return cloneStore(store), nil
}
func (f *storeRepositoryFake) Save(_ context.Context, organizationID string, store *storecenter.Store, expectedVersion int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.saveCalls++
	if f.saveErr != nil {
		if f.persistBeforeSaveError {
			f.stores[organizationID+"/"+store.ID()] = cloneStore(store)
		}
		return f.saveErr
	}
	key := organizationID + "/" + store.ID()
	if durable := f.stores[key]; durable == nil {
		return storecenter.ErrNotFound
	} else if durable.Version() != expectedVersion {
		return storecenter.ErrVersionConflict
	}
	f.stores[key] = cloneStore(store)
	return nil
}
func (f *storeRepositoryFake) SoftDelete(context.Context, string, string, int64) error {
	return errors.New("not used")
}

func cloneStore(store *storecenter.Store) *storecenter.Store {
	cloned, err := storecenter.RehydrateStore(store.Snapshot())
	if err != nil {
		panic(err)
	}
	return cloned
}

type quotaLedgerFake struct {
	mu                                      sync.Mutex
	allocation                              listingsubscription.StoreQuotaAllocation
	releaseOverride                         *listingsubscription.StoreQuotaAllocation
	reserveErr, commitErr, releaseErr       error
	reserveCalls, commitCalls, releaseCalls int
}

func (f *quotaLedgerFake) Reserve(_ context.Context, _ listingsubscription.StoreQuotaReserveInput) (listingsubscription.StoreQuotaReserveResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reserveCalls++
	if f.reserveErr != nil {
		return listingsubscription.StoreQuotaReserveResult{}, f.reserveErr
	}
	return listingsubscription.StoreQuotaReserveResult{Allocation: f.allocation, AllocationID: f.allocation.AllocationID, StoreID: f.allocation.StoreID, Existing: f.reserveCalls > 1}, nil
}
func (f *quotaLedgerFake) Commit(_ context.Context, _ listingsubscription.StoreQuotaTransitionInput) (listingsubscription.StoreQuotaTransitionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commitCalls++
	if f.commitErr != nil {
		return listingsubscription.StoreQuotaTransitionResult{}, f.commitErr
	}
	f.allocation.Status = listingsubscription.StoreQuotaAllocated
	return listingsubscription.StoreQuotaTransitionResult{Allocation: f.allocation}, nil
}
func (f *quotaLedgerFake) ReleaseReservation(_ context.Context, _ listingsubscription.StoreQuotaTransitionInput) (listingsubscription.StoreQuotaTransitionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseCalls++
	if f.releaseErr != nil {
		return listingsubscription.StoreQuotaTransitionResult{}, f.releaseErr
	}
	if f.releaseOverride != nil {
		return listingsubscription.StoreQuotaTransitionResult{Allocation: *f.releaseOverride}, nil
	}
	f.allocation.Status = listingsubscription.StoreQuotaReleased
	return listingsubscription.StoreQuotaTransitionResult{Allocation: f.allocation}, nil
}
func (f *quotaLedgerFake) Deallocate(context.Context, listingsubscription.StoreQuotaTransitionInput) (listingsubscription.StoreQuotaTransitionResult, error) {
	return listingsubscription.StoreQuotaTransitionResult{}, errors.New("not used")
}
func (f *quotaLedgerFake) GetByRequestKey(context.Context, string, string) (*listingsubscription.StoreQuotaAllocation, error) {
	return nil, errors.New("not used")
}
func (f *quotaLedgerFake) Summary(context.Context, string) (listingsubscription.StoreQuotaSummary, error) {
	return listingsubscription.StoreQuotaSummary{}, errors.New("not used")
}

type auditRepositoryFake struct {
	mu          sync.Mutex
	events      map[string]storecenter.AuditEvent
	recordErr   error
	failActions map[storecenter.AuditAction]int
	recordCalls int
}

func newAuditRepositoryFake() *auditRepositoryFake {
	return &auditRepositoryFake{events: map[string]storecenter.AuditEvent{}}
}
func (f *auditRepositoryFake) Record(_ context.Context, event storecenter.AuditEvent) (storecenter.AuditEvent, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordCalls++
	if f.failActions[event.Action] > 0 {
		f.failActions[event.Action]--
		return storecenter.AuditEvent{}, false, errors.New("audit unavailable")
	}
	if f.recordErr != nil {
		return storecenter.AuditEvent{}, false, f.recordErr
	}
	key := event.OrganizationID + "/" + event.RequestKey + "/" + string(event.Action)
	if existing, ok := f.events[key]; ok {
		if existing.StoreID != event.StoreID || existing.AllocationID != event.AllocationID || existing.Outcome != event.Outcome || existing.ActorSubject != event.ActorSubject || existing.PreviousState != event.PreviousState || existing.NewState != event.NewState || existing.FailureCode != event.FailureCode || !sameStrings(existing.SafeFieldNames, event.SafeFieldNames) {
			return storecenter.AuditEvent{}, false, storecenter.ErrAuditIdentityMismatch
		}
		return existing, true, nil
	}
	f.events[key] = event
	return event, false, nil
}
func (f *auditRepositoryFake) Get(_ context.Context, organizationID, requestKey string, action storecenter.AuditAction) (*storecenter.AuditEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	event, ok := f.events[organizationID+"/"+requestKey+"/"+string(action)]
	if !ok {
		return nil, storecenter.ErrNotFound
	}
	return &event, nil
}
func (f *auditRepositoryFake) actionsFor(organizationID, requestKey string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0)
	for _, event := range f.events {
		if event.OrganizationID == organizationID && event.RequestKey == requestKey {
			out = append(out, string(event.Action))
		}
	}
	return out
}
func sameStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]bool{}
	for _, value := range got {
		seen[value] = true
	}
	for _, value := range want {
		if !seen[value] {
			return false
		}
	}
	return true
}
