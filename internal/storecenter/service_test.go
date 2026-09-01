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
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, func() time.Time { return now })
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
	createdEvent := audit.eventFor(organizationID, requestKey, storecenter.AuditActionStoreCreated)
	if createdEvent.PreviousState != "" || createdEvent.NewState != storecenter.StoreStatusProvisioning {
		t.Fatalf("store_created state = %q -> %q, want empty -> provisioning", createdEvent.PreviousState, createdEvent.NewState)
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
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, func() time.Time { return time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC) })
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
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	_, err = service.Create(context.Background(), validCreateRequest())
	if !errors.Is(err, storecenter.ErrDependencyUnavailable) || err.Error() != storecenter.ErrDependencyUnavailable.Error() || strings.Contains(err.Error(), "password") {
		t.Fatalf("Create() dependency error = %q, want redacted sentinel", err)
	}
}

func TestServiceCreateRedactsWrappedSubscriptionRequired(t *testing.T) {
	ledger := &quotaLedgerFake{reserveErr: fmt.Errorf("provider token text: %w", listingsubscription.ErrSubscriptionRequired)}
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	_, err = service.Create(context.Background(), validCreateRequest())
	if !errors.Is(err, listingsubscription.ErrSubscriptionRequired) || err.Error() != listingsubscription.ErrSubscriptionRequired.Error() || strings.Contains(err.Error(), "token") {
		t.Fatalf("Create() subscription error = %q, want redacted subscription sentinel", err)
	}
}

func TestServiceCreateRejectsTypedNilAndInconsistentQuotaExceeded(t *testing.T) {
	var typedNil *listingsubscription.StoreQuotaExceededError
	for _, reserveErr := range []error{typedNil, &listingsubscription.StoreQuotaExceededError{Committed: 1, Reserved: 1, Limit: 3}} {
		service, err := storecenter.NewService(newStoreRepositoryFake(), &quotaLedgerFake{reserveErr: reserveErr}, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
		if err != nil {
			t.Fatalf("NewService(): %v", err)
		}
		_, err = service.Create(context.Background(), validCreateRequest())
		if !errors.Is(err, storecenter.ErrDependencyUnavailable) || err.Error() != storecenter.ErrDependencyUnavailable.Error() {
			t.Fatalf("Create() quota error = %v, want redacted dependency", err)
		}
	}
}

func TestServiceResumeCreateAfterCommitFailureReusesDurableCreateKey(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	ledger.commitErr = errors.New("ledger unavailable")
	repository := newStoreRepositoryFake()
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
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
	result, err := service.ResumeCreate(context.Background(), storecenter.ResumeCreateStoreRequest{
		OrganizationID:  request.OrganizationID,
		ActorSubject:    "recovery-actor",
		StoreID:         ledger.allocation.StoreID,
		ExpectedVersion: 1,
	})
	if err != nil {
		t.Fatalf("replay Create() error = %v", err)
	}
	if result.Store.LifecycleStatus() != storecenter.StoreStatusActive || repository.createCalls != 2 || ledger.releaseCalls != 0 {
		t.Fatalf("replay lifecycle/create/release = %s/%d/%d, want active/2/0", result.Store.LifecycleStatus(), repository.createCalls, ledger.releaseCalls)
	}
}

func TestServiceCreateAmbiguousCreateFoundNeverReleases(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	repository.createErr = errors.New("ambiguous persistence outcome")
	repository.createErrOnce = true
	repository.persistBeforeCreateError = true
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
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

func TestServiceResumeCreateActiveRepairsMissingTerminalAudit(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionStoreCreationCommitted: 1}
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("initial Create() error = %v, want dependency unavailable", err)
	}
	store := repository.stores[request.OrganizationID+"/"+ledger.allocation.StoreID]
	if store == nil || store.LifecycleStatus() != storecenter.StoreStatusActive {
		t.Fatalf("durable Store = %#v, want active Store", store)
	}

	result, err := service.ResumeCreate(context.Background(), storecenter.ResumeCreateStoreRequest{
		OrganizationID:  request.OrganizationID,
		ActorSubject:    "recovery-actor",
		StoreID:         store.ID(),
		ExpectedVersion: store.Version(),
	})
	if err != nil {
		t.Fatalf("ResumeCreate() error = %v", err)
	}
	if !result.Replayed || result.Store.LifecycleStatus() != storecenter.StoreStatusActive || repository.saveCalls != 1 {
		t.Fatalf("ResumeCreate() = %+v saves=%d, want active replay without reactivation", result, repository.saveCalls)
	}
	if _, err := audit.Get(context.Background(), request.OrganizationID, request.IdempotencyKey, storecenter.AuditActionStoreCreationCommitted); err != nil {
		t.Fatalf("terminal creation audit = %v, want repaired audit", err)
	}
}

func TestServiceCreateAuditFailuresPauseAtTheirDurableBoundary(t *testing.T) {
	for _, tt := range []struct {
		name                                  string
		action                                storecenter.AuditAction
		wantCreates, wantCommits, wantRelease int
		wantRetrySuccess                      bool
	}{
		{name: "before store", action: storecenter.AuditActionQuotaReserved, wantCreates: 0, wantCommits: 0, wantRelease: 1, wantRetrySuccess: false},
		{name: "after store", action: storecenter.AuditActionStoreCreated, wantCreates: 1, wantCommits: 0, wantRelease: 0, wantRetrySuccess: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			request := validCreateRequest()
			ledger := quotaForRequest(request)
			repository := newStoreRepositoryFake()
			audit := newAuditRepositoryFake()
			audit.failActions = map[storecenter.AuditAction]int{tt.action: 1}
			service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
			if err != nil {
				t.Fatalf("NewService(): %v", err)
			}
			if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
				t.Fatalf("first Create() error = %v, want dependency unavailable", err)
			}
			if repository.createCalls != tt.wantCreates || ledger.commitCalls != tt.wantCommits {
				t.Fatalf("paused calls create/commit = %d/%d, want %d/%d", repository.createCalls, ledger.commitCalls, tt.wantCreates, tt.wantCommits)
			}
			if ledger.releaseCalls != tt.wantRelease {
				t.Fatalf("paused release calls = %d, want %d", ledger.releaseCalls, tt.wantRelease)
			}
			if !tt.wantRetrySuccess {
				if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
					t.Fatalf("released retry Create(): %v, want dependency unavailable", err)
				}
				return
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

func TestServiceCreateReleasesReservationWhenInitialAuditFails(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionQuotaReserved: 1}
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("Create() error = %v, want dependency unavailable", err)
	}
	if ledger.releaseCalls != 1 || ledger.allocation.Status != listingsubscription.StoreQuotaReleased {
		t.Fatalf("released reservation = %d/%s, want one released reservation", ledger.releaseCalls, ledger.allocation.Status)
	}
	if event := audit.eventFor(request.OrganizationID, request.IdempotencyKey, storecenter.AuditActionStoreCreateFailed); event.FailureCode != storecenter.AuditFailureDependencyUnavailable {
		t.Fatalf("terminal audit after initial failure = %+v, want dependency failure", event)
	}
}

func TestServiceCreateReconcilesStaleOrphanedReservationWhenQuotaIsFull(t *testing.T) {
	request := validCreateRequest()
	allocation := listingsubscription.StoreQuotaAllocation{OrganizationID: request.OrganizationID, AllocationID: uuid.NewString(), StoreID: uuid.NewString(), RequestKey: request.IdempotencyKey, Status: listingsubscription.StoreQuotaReserved, CreatedBy: request.ActorSubject}
	stale := listingsubscription.StoreQuotaAllocation{OrganizationID: request.OrganizationID, AllocationID: uuid.NewString(), StoreID: uuid.NewString(), RequestKey: uuid.NewString(), Status: listingsubscription.StoreQuotaReserved, CreatedBy: "abandoned-actor", CreatedAt: time.Now().Add(-time.Hour)}
	ledger := &quotaLedgerFake{allocation: allocation, reserveErr: &listingsubscription.StoreQuotaExceededError{OrganizationID: request.OrganizationID, Committed: 1, Reserved: 1, Limit: 2}, reserveErrOnce: true, staleReservations: []listingsubscription.StoreQuotaAllocation{stale}, releaseOverride: func() *listingsubscription.StoreQuotaAllocation {
		released := stale
		released.Status = listingsubscription.StoreQuotaReleased
		return &released
	}()}
	repository := newStoreRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), request); err != nil {
		t.Fatalf("Create() after orphan reconciliation = %v", err)
	}
	if ledger.reserveCalls != 2 || ledger.releaseCalls != 1 {
		t.Fatalf("quota reserve/release calls = %d/%d, want 2/1", ledger.reserveCalls, ledger.releaseCalls)
	}
}

func TestServiceCreateDoesNotReleaseReservationRenewedAfterReconciliationSnapshot(t *testing.T) {
	request := validCreateRequest()
	observed := time.Now().UTC().Add(-time.Hour)
	renewed := observed.Add(time.Minute)
	stale := listingsubscription.StoreQuotaAllocation{OrganizationID: request.OrganizationID, AllocationID: uuid.NewString(), StoreID: uuid.NewString(), RequestKey: uuid.NewString(), Status: listingsubscription.StoreQuotaReserved, CreatedBy: "abandoned-actor", CreatedAt: observed, UpdatedAt: observed}
	ledger := &quotaLedgerFake{
		allocation:        currentQuotaAllocation(stale, renewed),
		reserveErr:        &listingsubscription.StoreQuotaExceededError{OrganizationID: request.OrganizationID, Committed: 1, Reserved: 1, Limit: 2},
		reserveErrOnce:    true,
		staleReservations: []listingsubscription.StoreQuotaAllocation{stale},
	}
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrLimitReached) {
		t.Fatalf("Create() error = %v, want limit reached after stale release rejection", err)
	}
	if ledger.releaseCalls != 1 || ledger.allocation.Status != listingsubscription.StoreQuotaReserved {
		t.Fatalf("stale reconciliation release = %d/%s, want one conditional attempt and reserved state", ledger.releaseCalls, ledger.allocation.Status)
	}
}

func currentQuotaAllocation(allocation listingsubscription.StoreQuotaAllocation, updatedAt time.Time) listingsubscription.StoreQuotaAllocation {
	allocation.UpdatedAt = updatedAt
	return allocation
}

func TestServiceCreateAmbiguousActivationSaveIsResolvedByScopedRead(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	repository.saveErr = errors.New("ambiguous save")
	repository.persistBeforeSaveError = true
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
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

func TestServiceCreateConcurrentReplayRepairsWriteAheadAuditFailure(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionQuotaCommitStarted: 1}
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("initial Create() error = %v, want dependency unavailable", err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for range 2 {
		go func() { <-start; _, err := service.Create(context.Background(), request); errs <- err }()
	}
	close(start)
	for range 2 {
		if err := <-errs; err != nil {
			t.Fatalf("concurrent replay Create(): %v", err)
		}
	}
	if got := audit.actionsFor(request.OrganizationID, request.IdempotencyKey); !sameStrings(got, []string{"quota_reserved", "store_created", "quota_commit_started", "store_creation_committed"}) {
		t.Fatalf("recovered audit actions = %v, want one durable write-ahead phase", got)
	}
}

func TestServiceCreateReplayReturnsLaterDisabledStoreWithoutReactivation(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, audit, &serviceConnectionProvider{}, time.Now)
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

func TestServiceCreateCrossActorRecoveryKeepsReservationOwner(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	ledger.allocation.CreatedBy = request.ActorSubject
	repository := newStoreRepositoryFake()
	repository.getErr = errors.New("temporary read failure")
	repository.getErrOnce = true
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Create() error = %v, want dependency unavailable", err)
	}

	retry := request
	retry.ActorSubject = "actor-2"
	result, err := service.Create(context.Background(), retry)
	if err != nil {
		t.Fatalf("cross-actor recovery Create(): %v", err)
	}
	if result.Store.CreatedBy() != request.ActorSubject {
		t.Fatalf("recovered Store creator = %q, want reservation owner %q", result.Store.CreatedBy(), request.ActorSubject)
	}
	event, err := audit.Get(context.Background(), request.OrganizationID, request.IdempotencyKey, storecenter.AuditActionQuotaReserved)
	if err != nil || event.ActorSubject != request.ActorSubject {
		t.Fatalf("recovered quota audit actor = %v/%v, want reservation owner", event, err)
	}
}

func TestServiceCreateFoundStoreVerifiesImmutableCreateFingerprint(t *testing.T) {
	request := validCreateRequest()
	request.ExternalStoreID = "external-a"
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); err != nil {
		t.Fatalf("initial Create(): %v", err)
	}
	for _, mutate := range []func(*storecenter.CreateStoreRequest){func(r *storecenter.CreateStoreRequest) { r.Name = "changed" }, func(r *storecenter.CreateStoreRequest) { r.Region = "MY" }, func(r *storecenter.CreateStoreRequest) { r.ExternalStoreID = "external-b" }} {
		replay := request
		mutate(&replay)
		if _, err := service.Create(context.Background(), replay); !errors.Is(err, storecenter.ErrAlreadyExists) {
			t.Fatalf("changed payload replay error = %v, want ErrAlreadyExists", err)
		}
		if ledger.releaseCalls != 0 || repository.saveCalls != 1 {
			t.Fatalf("changed replay release/save = %d/%d, want 0/1", ledger.releaseCalls, repository.saveCalls)
		}
	}
}

func TestServiceCreateAmbiguousFoundStoreVerifiesFingerprint(t *testing.T) {
	original := validCreateRequest()
	original.ExternalStoreID = "external-a"
	request := original
	request.Name = "changed"
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	repository.createErr = errors.New("ambiguous create")
	repository.createErrOnce = true
	repository.storeOnCreateError = matchingStore(t, original, ledger.allocation)
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrAlreadyExists) {
		t.Fatalf("ambiguous changed payload Create() error = %v, want ErrAlreadyExists", err)
	}
	if ledger.releaseCalls != 0 {
		t.Fatalf("ambiguous fingerprint conflict release calls = %d, want 0", ledger.releaseCalls)
	}
}

func TestServiceCreateNeverReleasesAfterAmbiguousConfirmationRead(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	repository.createErr = errors.New("ambiguous create")
	repository.getErrAfterCreate = errors.New("confirmation unavailable")
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
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
			service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("Create() error = %v, want dependency unavailable", err)
	}
}

func TestServiceCreateRejectsReleaseResultWithCorrectIdentityWrongStatus(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	wrongStatus := ledger.allocation
	wrongStatus.Status = listingsubscription.StoreQuotaReserved
	ledger.releaseOverride = &wrongStatus
	repository := newStoreRepositoryFake()
	repository.createErr = storecenter.ErrAlreadyExists
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("Create() error = %v, want dependency unavailable", err)
	}
}

func TestServiceCreateDoesNotCompensateGenericCreateError(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	repository.createErr = errors.New("driver create failure")
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("Create() error = %v, want dependency unavailable", err)
	}
	if ledger.releaseCalls != 0 || len(audit.actionsFor(request.OrganizationID, request.IdempotencyKey)) != 2 {
		t.Fatalf("generic failure release/audits = %d/%v, want no release and no terminal audit", ledger.releaseCalls, audit.actionsFor(request.OrganizationID, request.IdempotencyKey))
	}
}

func TestServiceCreateTerminalFailureWithAppearingStoreNeverReleases(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	audit := newAuditRepositoryFake()
	store := matchingStore(t, request, ledger.allocation)
	audit.onRecord = func(event storecenter.AuditEvent) {
		if event.Action == storecenter.AuditActionStoreCreateFailed {
			repository.stores[request.OrganizationID+"/"+store.ID()] = cloneStore(store)
		}
	}
	repository.createErr = storecenter.ErrAlreadyExists
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("Create() error = %v, want dependency unavailable", err)
	}
	if ledger.releaseCalls != 0 {
		t.Fatalf("post-terminal Store appearance release calls = %d, want 0", ledger.releaseCalls)
	}
}

func TestServiceCreateTerminalReplayWithStorePresentNeverReleases(t *testing.T) {
	request := validCreateRequest()
	ledger := quotaForRequest(request)
	repository := newStoreRepositoryFake()
	audit := newAuditRepositoryFake()
	store := matchingStore(t, request, ledger.allocation)
	repository.stores[request.OrganizationID+"/"+store.ID()] = cloneStore(store)
	_, _, err := audit.Record(context.Background(), storecenter.AuditEvent{EventID: uuid.NewString(), OrganizationID: request.OrganizationID, StoreID: store.ID(), AllocationID: ledger.allocation.AllocationID, RequestKey: request.IdempotencyKey, Action: storecenter.AuditActionStoreCreateFailed, Outcome: storecenter.AuditOutcomeFailed, ActorSubject: request.ActorSubject, FailureCode: storecenter.AuditFailureAlreadyExists, OccurredAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("Record terminal audit: %v", err)
	}
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatalf("NewService(): %v", err)
	}
	if _, err := service.Create(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("terminal replay Create() error = %v, want dependency unavailable", err)
	}
	if ledger.releaseCalls != 0 {
		t.Fatalf("terminal replay Store-present release calls = %d, want 0", ledger.releaseCalls)
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
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, audit, &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(newStoreRepositoryFake(), ledger, audit, &serviceConnectionProvider{}, time.Now)
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

func TestServiceListGetProjectScopedStoresAndAuthoritativeQuota(t *testing.T) {
	repository := newStoreRepositoryFake()
	first := activeServiceStore(t, "org-a", "00000000-0000-4000-8000-000000000501", "00000000-0000-4000-8000-000000000601", "00000000-0000-4000-8000-000000000701", "First")
	second := activeServiceStore(t, "org-a", "00000000-0000-4000-8000-000000000502", "00000000-0000-4000-8000-000000000602", "00000000-0000-4000-8000-000000000702", "Second")
	repository.listPage = storecenter.StorePage{Stores: []storecenter.Store{*first, *second}, Total: 7}
	repository.stores["org-a/"+first.ID()] = cloneStore(first)
	limit := int64(5)
	ledger := &quotaLedgerFake{summary: listingsubscription.StoreQuotaSummary{OrganizationID: "org-a", Committed: 2, Reserved: 1, Limit: &limit, Allowed: true}}
	provider := &serviceConnectionProvider{statuses: map[string]storecenter.ConnectionStatus{first.ID(): storecenter.ConnectionStatusConnected, second.ID(): storecenter.ConnectionStatusExpired}}
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), provider, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.List(context.Background(), storecenter.ListStoresRequest{OrganizationID: "org-a", Page: 2, PageSize: 2, Platform: "shein", Status: storecenter.StoreStatusActive})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(result.Items) != 2 || result.Items[0].Store.ID() != first.ID() || result.Items[1].Store.ID() != second.ID() || result.Items[0].ConnectionStatus != storecenter.ConnectionStatusConnected || result.Items[1].ConnectionStatus != storecenter.ConnectionStatusExpired {
		t.Fatalf("List() items = %#v", result.Items)
	}
	if result.Total != 7 || result.Page != 2 || result.PageSize != 2 || result.Quota.Used != 2 || result.Quota.Reserved != 1 || result.Quota.Limit == nil || *result.Quota.Limit != 5 {
		t.Fatalf("List() metadata = %#v", result)
	}
	projection, err := service.Get(context.Background(), storecenter.GetStoreRequest{OrganizationID: "org-a", StoreID: first.ID()})
	if err != nil || projection.Store.ID() != first.ID() {
		t.Fatalf("Get() = %#v, %v", projection, err)
	}
	if _, err := service.Get(context.Background(), storecenter.GetStoreRequest{OrganizationID: "org-b", StoreID: first.ID()}); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("cross-org Get() = %v", err)
	}
}

func TestServiceReadValidationCallsNoDependencies(t *testing.T) {
	repository, ledger, provider := newStoreRepositoryFake(), &quotaLedgerFake{}, &serviceConnectionProvider{}
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), provider, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.List(context.Background(), storecenter.ListStoresRequest{OrganizationID: " org-a", Page: 0, PageSize: 101, Platform: "shopify"}); err == nil {
		t.Fatal("invalid List error = nil")
	}
	if _, err := service.Get(context.Background(), storecenter.GetStoreRequest{OrganizationID: "org-a", StoreID: "bad"}); err == nil {
		t.Fatal("invalid Get error = nil")
	}
	if repository.getCalls != 0 || repository.listCalls != 0 || ledger.summaryCalls != 0 || provider.calls != 0 {
		t.Fatalf("dependency calls = %d/%d/%d/%d", repository.getCalls, repository.listCalls, ledger.summaryCalls, provider.calls)
	}
}

func TestServiceRequiresConnectionStatusProviderIncludingTypedNil(t *testing.T) {
	var typedNil *serviceConnectionProvider
	for _, provider := range []storecenter.ConnectionStatusProvider{nil, typedNil} {
		if _, err := storecenter.NewService(newStoreRepositoryFake(), &quotaLedgerFake{}, newAuditRepositoryFake(), provider, time.Now); err == nil {
			t.Fatal("NewService() accepted nil connection provider")
		}
	}
}

func TestServiceUpdateAuditFailureRepairsWithoutSecondSave(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", "00000000-0000-4000-8000-000000000504", "00000000-0000-4000-8000-000000000604", "00000000-0000-4000-8000-000000000704", "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionStoreUpdated: 1}
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	request := storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: " After ", Region: store.Region()}
	if _, err := service.Update(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Update() = %v", err)
	}
	result, err := service.Update(context.Background(), request)
	if err != nil || !result.Replayed || result.Store.Store.Name() != "After" || result.Store.Store.Version() != request.ExpectedVersion+1 || repository.saveCalls != 1 {
		t.Fatalf("replay Update() = %#v, %v saves=%d", result, err, repository.saveCalls)
	}
	operationKey := uuid.NewSHA1(uuid.NameSpaceOID, []byte("org-a\n"+store.ID()+"\nupdate\n"+fmt.Sprint(store.Version()))).String()
	completed := audit.eventFor("org-a", operationKey, storecenter.AuditActionStoreUpdated)
	if !sameStrings(completed.SafeFieldNames, []string{"name"}) {
		t.Fatalf("repaired update fields = %v, want authoritative name-only intent", completed.SafeFieldNames)
	}
}

func TestServiceMutationReplayPreservesOriginalActor(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionStoreUpdated: 1}
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	request := storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "actor-a", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: "After", Region: store.Region()}
	if _, err := service.Update(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Update() = %v", err)
	}
	retry := request
	retry.ActorSubject = "actor-b"
	if _, err := service.Update(context.Background(), retry); err != nil {
		t.Fatalf("cross-actor replay Update() = %v", err)
	}
	operationKey := uuid.NewSHA1(uuid.NameSpaceOID, []byte("org-a\n"+store.ID()+"\nupdate\n"+fmt.Sprint(store.Version()))).String()
	completed := audit.eventFor("org-a", operationKey, storecenter.AuditActionStoreUpdated)
	if completed.ActorSubject != "actor-a" {
		t.Fatalf("repaired audit actor = %q, want original actor-a", completed.ActorSubject)
	}
}

func TestServiceUpdateRepairRequiresValidAuthoritativeIntent(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		name := "missing"
		if corrupt {
			name = "corrupt"
		}
		t.Run(name, func(t *testing.T) {
			original := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
			persisted := cloneStore(original)
			if _, err := persisted.EditBasic("After", persisted.Region(), "editor", persisted.UpdatedAt().Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			repository := newStoreRepositoryFake()
			repository.stores["org-a/"+persisted.ID()] = persisted
			audit := newAuditRepositoryFake()
			operationKey := uuid.NewSHA1(uuid.NameSpaceOID, []byte("org-a\n"+original.ID()+"\nupdate\n"+fmt.Sprint(original.Version()))).String()
			if corrupt {
				audit.events["org-a/"+operationKey+"/"+string(storecenter.AuditActionStoreUpdateStarted)] = storecenter.AuditEvent{
					OrganizationID: "org-a", StoreID: original.ID(), AllocationID: original.QuotaAllocationID(), RequestKey: operationKey,
					Action: storecenter.AuditActionStoreUpdateStarted, Outcome: storecenter.AuditOutcomeUnknown, ActorSubject: "editor",
					SafeFieldNames: []string{"quota_allocation_id"}, PreviousState: storecenter.StoreStatusActive, NewState: storecenter.StoreStatusActive,
					FailureCode: storecenter.AuditFailureNone, StoreVersion: original.Version(),
				}
			}
			service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.Update(context.Background(), storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: original.ID(), ExpectedVersion: original.Version(), Name: "After", Region: original.Region()})
			if !errors.Is(err, storecenter.ErrDependencyUnavailable) || repository.saveCalls != 0 {
				t.Fatalf("repair with %s intent = %v saves=%d", name, err, repository.saveCalls)
			}
		})
	}
}

func TestServiceDisableEnableDoNotTouchQuota(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", "00000000-0000-4000-8000-000000000505", "00000000-0000-4000-8000-000000000605", "00000000-0000-4000-8000-000000000705", "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	ledger := &quotaLedgerFake{}
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return time.Now().UTC().Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	disabled, err := service.Disable(context.Background(), storecenter.StoreLifecycleRequest{OrganizationID: "org-a", ActorSubject: "operator", StoreID: store.ID(), ExpectedVersion: store.Version()})
	if err != nil || disabled.Store.Store.LifecycleStatus() != storecenter.StoreStatusDisabled {
		t.Fatalf("Disable() = %#v, %v", disabled, err)
	}
	enabled, err := service.Enable(context.Background(), storecenter.StoreLifecycleRequest{OrganizationID: "org-a", ActorSubject: "operator", StoreID: store.ID(), ExpectedVersion: disabled.Store.Store.Version()})
	if err != nil || enabled.Store.Store.LifecycleStatus() != storecenter.StoreStatusActive {
		t.Fatalf("Enable() = %#v, %v", enabled, err)
	}
	if ledger.reserveCalls+ledger.commitCalls+ledger.releaseCalls+ledger.deallocateCalls != 0 {
		t.Fatal("lifecycle touched quota")
	}
}

func TestServiceLifecycleRejectsIllegalStatesAndStaleVersions(t *testing.T) {
	tests := []struct {
		name            string
		status          storecenter.LifecycleStatus
		action          string
		expectedVersion func(*storecenter.Store) int64
		want            error
	}{
		{"disable provisioning", storecenter.StoreStatusProvisioning, "disable", func(store *storecenter.Store) int64 { return store.Version() }, storecenter.ErrInvalidTransition},
		{"disable disabled", storecenter.StoreStatusDisabled, "disable", func(store *storecenter.Store) int64 { return store.Version() }, storecenter.ErrInvalidTransition},
		{"enable active", storecenter.StoreStatusActive, "enable", func(store *storecenter.Store) int64 { return store.Version() }, storecenter.ErrInvalidTransition},
		{"enable deleting", storecenter.StoreStatusDeleting, "enable", func(store *storecenter.Store) int64 { return store.Version() }, storecenter.ErrInvalidTransition},
		{"stale disable", storecenter.StoreStatusActive, "disable", func(store *storecenter.Store) int64 { return store.Version() - 1 }, storecenter.ErrVersionConflict},
		{"stale enable", storecenter.StoreStatusDisabled, "enable", func(store *storecenter.Store) int64 { return store.Version() - 1 }, storecenter.ErrVersionConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: uuid.NewString(), OrganizationID: "org-a", ActorSubject: "creator", Name: "Store", Platform: "shein", Region: "SG", CreateIdempotencyKey: uuid.NewString(), QuotaAllocationID: uuid.NewString(), OccurredAt: time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)})
			if err != nil {
				t.Fatal(err)
			}
			if test.status != storecenter.StoreStatusProvisioning {
				if err := store.TransitionTo(storecenter.StoreStatusActive, "creator", store.UpdatedAt().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
			}
			if test.status == storecenter.StoreStatusDisabled {
				if err := store.TransitionTo(storecenter.StoreStatusDisabled, "operator", store.UpdatedAt().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
			}
			if test.status == storecenter.StoreStatusDeleting {
				if err := store.BeginDelete(uuid.NewString(), "operator", store.UpdatedAt().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
			}
			repository := newStoreRepositoryFake()
			repository.stores["org-a/"+store.ID()] = cloneStore(store)
			ledger := &quotaLedgerFake{}
			service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
			if err != nil {
				t.Fatal(err)
			}
			request := storecenter.StoreLifecycleRequest{OrganizationID: "org-a", ActorSubject: "operator", StoreID: store.ID(), ExpectedVersion: test.expectedVersion(store)}
			if test.action == "disable" {
				_, err = service.Disable(context.Background(), request)
			} else {
				_, err = service.Enable(context.Background(), request)
			}
			if !errors.Is(err, test.want) || repository.saveCalls != 0 || ledger.reserveCalls+ledger.commitCalls+ledger.releaseCalls+ledger.deallocateCalls != 0 {
				t.Fatalf("%s() = %v saves=%d quota calls=%d/%d/%d/%d", test.action, err, repository.saveCalls, ledger.reserveCalls, ledger.commitCalls, ledger.releaseCalls, ledger.deallocateCalls)
			}
		})
	}
}

func TestServiceDeleteDeallocatesThenSoftDeletesAndReplays(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", "00000000-0000-4000-8000-000000000506", "00000000-0000-4000-8000-000000000606", "00000000-0000-4000-8000-000000000706", "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}}
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return time.Now().UTC().Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	request := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()}
	result, err := service.Delete(context.Background(), request)
	if err != nil || result.Replayed || result.Version != store.Version()+2 || ledger.deallocateCalls != 1 || repository.softDeleteCalls != 1 {
		t.Fatalf("Delete() = %#v, %v calls=%d/%d", result, err, ledger.deallocateCalls, repository.softDeleteCalls)
	}
	replay, err := service.Delete(context.Background(), request)
	if err != nil || !replay.Replayed || ledger.deallocateCalls != 1 || repository.softDeleteCalls != 1 {
		t.Fatalf("Delete() replay = %#v, %v", replay, err)
	}
}

func TestServiceDeleteAuditsWriteAheadIntentAndTruthfulDurablePhases(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}}
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	request := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()}
	if _, err := service.Delete(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	started := audit.eventFor("org-a", request.OperationKey, storecenter.AuditActionDeleteStarted)
	if started.Outcome != storecenter.AuditOutcomeUnknown || started.FailureCode != storecenter.AuditFailureNone || started.PreviousState != storecenter.StoreStatusActive || started.NewState != storecenter.StoreStatusDeleting || started.StoreVersion != store.Version() || !sameStrings(started.SafeFieldNames, []string{"lifecycle_status"}) {
		t.Fatalf("delete_started = %+v, want truthful write-ahead intent", started)
	}
	marked := audit.eventFor("org-a", request.OperationKey, storecenter.AuditActionStoreMarkedDeleting)
	if marked.Outcome != storecenter.AuditOutcomeSucceeded || marked.PreviousState != storecenter.StoreStatusActive || marked.NewState != storecenter.StoreStatusDeleting || marked.StoreVersion != store.Version()+1 || !sameStrings(marked.SafeFieldNames, []string{"lifecycle_status"}) {
		t.Fatalf("store_marked_deleting = %+v, want durable lifecycle success", marked)
	}
	deallocated := audit.eventFor("org-a", request.OperationKey, storecenter.AuditActionQuotaDeallocated)
	if deallocated.Outcome != storecenter.AuditOutcomeSucceeded || deallocated.PreviousState != storecenter.StoreStatusDeleting || deallocated.NewState != storecenter.StoreStatusDeleting || deallocated.StoreVersion != store.Version()+1 || !sameStrings(deallocated.SafeFieldNames, []string{"quota_allocation_id"}) {
		t.Fatalf("quota_deallocated = %+v, want durable quota success", deallocated)
	}
	completed := audit.eventFor("org-a", request.OperationKey, storecenter.AuditActionDeleteComplete)
	if completed.Outcome != storecenter.AuditOutcomeSucceeded || completed.PreviousState != storecenter.StoreStatusDeleting || completed.NewState != "" || completed.StoreVersion != store.Version()+2 || !sameStrings(completed.SafeFieldNames, []string{"lifecycle_status"}) {
		t.Fatalf("delete_complete = %+v, want durable deletion success", completed)
	}
}

func TestServiceDeleteStartedAuditFailureCannotMoveStoreAndRetryConverges(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}}
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionDeleteStarted: 1}
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	request := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()}
	if _, err := service.Delete(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Delete() = %v, want dependency error", err)
	}
	current, err := repository.Get(context.Background(), "org-a", store.ID())
	if err != nil || current.LifecycleStatus() != storecenter.StoreStatusActive || current.Version() != store.Version() || repository.saveCalls != 0 || ledger.deallocateCalls != 0 {
		t.Fatalf("failed intent mutated state = %+v/%v saves=%d dealloc=%d", current, err, repository.saveCalls, ledger.deallocateCalls)
	}
	if _, err := service.Delete(context.Background(), request); err != nil {
		t.Fatalf("retry Delete() = %v", err)
	}
}

func TestServiceUpdateAmbiguousSaveWithUnchangedDurableStateIsDependencyUnavailable(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", "00000000-0000-4000-8000-000000000507", "00000000-0000-4000-8000-000000000607", "00000000-0000-4000-8000-000000000707", "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	repository.saveErr = errors.New("database timeout")
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Update(context.Background(), storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: "After", Region: "MY"})
	if !errors.Is(err, storecenter.ErrDependencyUnavailable) || errors.Is(err, storecenter.ErrVersionConflict) {
		t.Fatalf("Update() unchanged readback error = %v, want dependency unavailable", err)
	}
}

func TestServiceUpdateRejectsWrongIdentityAcrossPostSaveReadbackClassifications(t *testing.T) {
	for _, branch := range []struct {
		name    string
		saveErr error
		build   func(*storecenter.Store) *storecenter.Store
	}{
		{"would-be replay", errors.New("ambiguous save"), func(store *storecenter.Store) *storecenter.Store {
			readback := cloneStore(store)
			if _, err := readback.EditBasic("After", "MY", "editor", readback.UpdatedAt().Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			return readback
		}},
		{"would-be already exists", storecenter.ErrAlreadyExists, func(store *storecenter.Store) *storecenter.Store { return cloneStore(store) }},
		{"would-be version conflict", errors.New("ambiguous save"), func(store *storecenter.Store) *storecenter.Store {
			snapshot := store.Snapshot()
			snapshot.Name, snapshot.Version, snapshot.UpdatedBy, snapshot.UpdatedAt = "Diverged", snapshot.Version+2, "other-editor", snapshot.UpdatedAt.Add(2*time.Minute)
			readback, err := storecenter.RehydrateStore(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			return readback
		}},
	} {
		for _, identity := range []string{"organization", "store ID"} {
			t.Run(branch.name+"/wrong "+identity, func(t *testing.T) {
				store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
				readback := branch.build(store)
				snapshot := readback.Snapshot()
				if identity == "organization" {
					snapshot.OrganizationID = "org-b"
				} else {
					snapshot.ID = uuid.NewString()
				}
				readback, err := storecenter.RehydrateStore(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				repository := newStoreRepositoryFake()
				repository.stores["org-a/"+store.ID()] = cloneStore(store)
				repository.saveErr = branch.saveErr
				repository.storeOnSaveError = readback
				service, err := storecenter.NewService(repository, &quotaLedgerFake{}, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
				if err != nil {
					t.Fatal(err)
				}
				_, err = service.Update(context.Background(), storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: "After", Region: "MY"})
				if !errors.Is(err, storecenter.ErrDependencyUnavailable) || errors.Is(err, storecenter.ErrAlreadyExists) || errors.Is(err, storecenter.ErrVersionConflict) {
					t.Fatalf("Update() with wrong %s %s readback = %v, want redacted dependency error", identity, branch.name, err)
				}
			})
		}
	}
}

func TestServiceDeleteRejectsCorruptDurableDeallocationAudit(t *testing.T) {
	storeID := "00000000-0000-4000-8000-000000000508"
	operationKey := uuid.NewString()
	audit := newAuditRepositoryFake()
	audit.events["org-a/"+operationKey+"/"+string(storecenter.AuditActionQuotaDeallocated)] = storecenter.AuditEvent{
		OrganizationID: "org-a", StoreID: storeID, AllocationID: "not-a-uuid", RequestKey: operationKey,
		Action: storecenter.AuditActionQuotaDeallocated, Outcome: storecenter.AuditOutcomeSucceeded, StoreVersion: 3,
	}
	service, err := storecenter.NewService(newStoreRepositoryFake(), &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Delete(context.Background(), storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: storeID, ExpectedVersion: 3, OperationKey: operationKey})
	if !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("Delete() corrupt deallocation audit = %v, want dependency unavailable", err)
	}
}

func TestServiceDeleteRejectsActionSpecificCorruptRecoveryAudits(t *testing.T) {
	for _, test := range []struct {
		name   string
		action storecenter.AuditAction
		mutate func(*storecenter.AuditEvent)
	}{
		{"wrong outcome", storecenter.AuditActionQuotaDeallocated, func(event *storecenter.AuditEvent) { event.Outcome = storecenter.AuditOutcomeUnknown }},
		{"wrong failure", storecenter.AuditActionQuotaDeallocated, func(event *storecenter.AuditEvent) { event.FailureCode = storecenter.AuditFailureDependencyUnavailable }},
		{"wrong quota state", storecenter.AuditActionQuotaDeallocated, func(event *storecenter.AuditEvent) { event.PreviousState = storecenter.StoreStatusActive }},
		{"wrong quota fields", storecenter.AuditActionQuotaDeallocated, func(event *storecenter.AuditEvent) { event.SafeFieldNames = []string{"lifecycle_status"} }},
		{"implausible quota version", storecenter.AuditActionQuotaDeallocated, func(event *storecenter.AuditEvent) { event.StoreVersion = 9 }},
		{"wrong complete outcome", storecenter.AuditActionDeleteComplete, func(event *storecenter.AuditEvent) { event.Outcome = storecenter.AuditOutcomeUnknown }},
		{"wrong complete state", storecenter.AuditActionDeleteComplete, func(event *storecenter.AuditEvent) { event.NewState = storecenter.StoreStatusDeleting }},
		{"wrong complete fields", storecenter.AuditActionDeleteComplete, func(event *storecenter.AuditEvent) { event.SafeFieldNames = []string{"quota_allocation_id"} }},
		{"implausible complete version", storecenter.AuditActionDeleteComplete, func(event *storecenter.AuditEvent) { event.StoreVersion = 9 }},
		{"wrong action identity", storecenter.AuditActionDeleteComplete, func(event *storecenter.AuditEvent) { event.Action = storecenter.AuditActionQuotaDeallocated }},
	} {
		t.Run(test.name, func(t *testing.T) {
			storeID, operationKey, allocationID := uuid.NewString(), uuid.NewString(), uuid.NewString()
			event := taskFiveAuditEvent(test.action)
			event.OrganizationID, event.StoreID, event.RequestKey, event.AllocationID = "org-a", storeID, operationKey, allocationID
			if test.action == storecenter.AuditActionQuotaDeallocated {
				event.StoreVersion = 3
			} else {
				event.StoreVersion = 4
			}
			test.mutate(&event)
			audit := newAuditRepositoryFake()
			audit.events["org-a/"+operationKey+"/"+string(test.action)] = event
			service, err := storecenter.NewService(newStoreRepositoryFake(), &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, time.Now)
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.Delete(context.Background(), storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: storeID, ExpectedVersion: 2, OperationKey: operationKey})
			if !errors.Is(err, storecenter.ErrDependencyUnavailable) {
				t.Fatalf("Delete() corrupt %s audit = %v, want dependency unavailable", test.action, err)
			}
		})
	}
}

func TestServiceListBoundsConnectionConcurrencyAndPreservesOrder(t *testing.T) {
	repository := newStoreRepositoryFake()
	stores := make([]storecenter.Store, 20)
	for i := range stores {
		store := activeServiceStore(t, "org-a", fmt.Sprintf("00000000-0000-4000-8000-%012d", 800+i), fmt.Sprintf("00000000-0000-4000-8000-%012d", 900+i), fmt.Sprintf("00000000-0000-4000-8000-%012d", 1000+i), fmt.Sprintf("Store %02d", i))
		stores[i] = *store
	}
	repository.listPage = storecenter.StorePage{Stores: stores, Total: int64(len(stores))}
	limit := int64(30)
	ledger := &quotaLedgerFake{summary: listingsubscription.StoreQuotaSummary{OrganizationID: "org-a", Limit: &limit, Allowed: true}}
	provider := &serviceConnectionProvider{delay: 20 * time.Millisecond}
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), provider, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.List(context.Background(), storecenter.ListStoresRequest{OrganizationID: "org-a", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatal(err)
	}
	if provider.maxActive > 8 || provider.maxActive < 2 {
		t.Fatalf("provider max concurrency = %d, want 2..8", provider.maxActive)
	}
	for i := range result.Items {
		if result.Items[i].Store.ID() != stores[i].ID() {
			t.Fatalf("item %d ID = %q, want %q", i, result.Items[i].Store.ID(), stores[i].ID())
		}
	}
}

func TestServiceListRejectsMalformedQuotaProducerOutput(t *testing.T) {
	for _, summary := range []listingsubscription.StoreQuotaSummary{
		{OrganizationID: "org-b", Committed: 0, Reserved: 0, Allowed: false, Reason: "subscription_required"},
		{OrganizationID: "org-a", Committed: -1, Reserved: 0, Allowed: false, Reason: "subscription_required"},
		{OrganizationID: "org-a", Committed: 0, Reserved: 0, Allowed: true},
		{OrganizationID: "org-a", Committed: 0, Reserved: 0, Limit: pointerInt64(1), Allowed: false, Reason: "store_limit_reached"},
	} {
		repository := newStoreRepositoryFake()
		ledger := &quotaLedgerFake{summary: summary}
		service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := service.List(context.Background(), storecenter.ListStoresRequest{OrganizationID: "org-a", Page: 1, PageSize: 20}); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
			t.Fatalf("List() malformed summary %#v error = %v", summary, err)
		}
	}
}

func TestServiceMutationAndDeleteValidationCallNoDependencies(t *testing.T) {
	repository, ledger, audit, provider := newStoreRepositoryFake(), &quotaLedgerFake{}, newAuditRepositoryFake(), &serviceConnectionProvider{}
	service, err := storecenter.NewService(repository, ledger, audit, provider, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = service.Update(context.Background(), storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: " actor", StoreID: uuid.NewString(), ExpectedVersion: 1, Name: "Name", Region: "SG"})
	_, _ = service.Disable(context.Background(), storecenter.StoreLifecycleRequest{OrganizationID: "org-a", ActorSubject: "actor", StoreID: uuid.NewString(), ExpectedVersion: 0})
	_, _ = service.Delete(context.Background(), storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "actor", StoreID: uuid.NewString(), ExpectedVersion: 1, OperationKey: "bad"})
	if repository.getCalls+repository.saveCalls+repository.softDeleteCalls != 0 || ledger.deallocateCalls != 0 || audit.recordCalls != 0 || provider.calls != 0 {
		t.Fatalf("invalid mutation dependency calls repo=%d/%d/%d quota=%d audit=%d provider=%d", repository.getCalls, repository.saveCalls, repository.softDeleteCalls, ledger.deallocateCalls, audit.recordCalls, provider.calls)
	}
}

func TestServiceDeleteDeallocationFailureLeavesOwnedDeletingStoreAndSameKeyResumes(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", "00000000-0000-4000-8000-000000000509", "00000000-0000-4000-8000-000000000609", "00000000-0000-4000-8000-000000000709", "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}, deallocateErr: errors.New("quota timeout")}
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, func() time.Time { return time.Now().UTC().Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	first := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: key}
	if _, err := service.Delete(context.Background(), first); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Delete() = %v", err)
	}
	deleting, err := repository.Get(context.Background(), "org-a", store.ID())
	if err != nil || deleting.LifecycleStatus() != storecenter.StoreStatusDeleting || deleting.DeleteOperationKey() != key {
		t.Fatalf("durable deleting = %#v, %v", deleting, err)
	}
	wrong := first
	wrong.ExpectedVersion = deleting.Version()
	wrong.OperationKey = uuid.NewString()
	if _, err := service.Delete(context.Background(), wrong); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("recovery before quota is available = %v", err)
	}
	ledger.deallocateErr = nil
	result, err := service.Delete(context.Background(), wrong)
	if err != nil || result.Version != deleting.Version()+1 || repository.saveCalls != 1 || ledger.deallocateCalls != 3 {
		t.Fatalf("resumed Delete() = %#v, %v saves=%d dealloc=%d", result, err, repository.saveCalls, ledger.deallocateCalls)
	}
}

func TestServiceDeleteRecoversMarkedDeletingAuditWithNewKey(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}}
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionStoreMarkedDeleting: 1}
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, func() time.Time { return time.Now().UTC().Add(time.Hour) })
	if err != nil {
		t.Fatal(err)
	}
	first := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()}
	if _, err := service.Delete(context.Background(), first); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Delete() = %v", err)
	}
	deleting, err := repository.Get(context.Background(), "org-a", store.ID())
	if err != nil {
		t.Fatal(err)
	}
	resumed := first
	resumed.ExpectedVersion = deleting.Version()
	resumed.OperationKey = uuid.NewString()
	result, err := service.Delete(context.Background(), resumed)
	if err != nil || result.Version != deleting.Version()+1 || !result.Replayed {
		t.Fatalf("new-key recovery Delete() = %#v, %v", result, err)
	}
}

func TestServiceDeleteRecoversReleasedQuotaAfterDeallocationAuditFailureWithNewKey(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}}
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionQuotaDeallocated: 1}
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	first := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()}
	if _, err := service.Delete(context.Background(), first); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Delete() = %v", err)
	}
	deleting, err := repository.Get(context.Background(), "org-a", store.ID())
	if err != nil || deleting.LifecycleStatus() != storecenter.StoreStatusDeleting || ledger.allocation.Status != listingsubscription.StoreQuotaReleased {
		t.Fatalf("durable state after audit failure = %#v/%v quota=%s", deleting, err, ledger.allocation.Status)
	}
	resumed := first
	resumed.ExpectedVersion = deleting.Version()
	resumed.OperationKey = uuid.NewString()
	result, err := service.Delete(context.Background(), resumed)
	if err != nil || result.Version != deleting.Version()+1 || !result.Replayed || repository.softDeleteCalls != 1 {
		t.Fatalf("new-key recovery after released quota = %#v, %v soft-deletes=%d", result, err, repository.softDeleteCalls)
	}
}

func TestServiceDeleteReplaysMissingCompletionAfterSoftDeleteWithReplacementKey(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}}
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionDeleteComplete: 1}
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	first := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "actor-a", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()}
	if _, err := service.Delete(context.Background(), first); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Delete() = %v", err)
	}
	retry := first
	retry.ActorSubject = "actor-b"
	retry.OperationKey = uuid.NewString()
	result, err := service.Delete(context.Background(), retry)
	if err != nil || !result.Replayed || result.Version != first.ExpectedVersion+2 {
		t.Fatalf("replacement-key Delete() = %#v, %v", result, err)
	}
	completed := audit.eventFor("org-a", first.OperationKey, storecenter.AuditActionDeleteComplete)
	if completed.ActorSubject != "actor-a" {
		t.Fatalf("repaired delete actor = %q, want original actor-a", completed.ActorSubject)
	}
}

// Controller contract: only the exact same operation key may retry with the
// original pre-delete version after the durable begin-delete Save advanced it
// once. The companion test above covers retrying with the current version.
func TestServiceDeleteSameKeyOriginalVersionReplayAfterDurableBeginDelete(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}, deallocateErr: errors.New("quota timeout")}
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	request := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()}
	if _, err := service.Delete(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Delete() = %v", err)
	}
	deleting, err := repository.Get(context.Background(), "org-a", store.ID())
	if err != nil || deleting.Version() != request.ExpectedVersion+1 || deleting.DeleteOperationKey() != request.OperationKey {
		t.Fatalf("durable deleting state = %+v/%v", deleting, err)
	}
	ledger.deallocateErr = nil
	result, err := service.Delete(context.Background(), request)
	if err != nil || result.Version != request.ExpectedVersion+2 || !result.Replayed {
		t.Fatalf("original-version replay Delete() = %#v, %v", result, err)
	}
}

func TestServiceDeleteRepairsAuditFailuresAtDurableBoundaries(t *testing.T) {
	for _, action := range []storecenter.AuditAction{storecenter.AuditActionStoreMarkedDeleting, storecenter.AuditActionQuotaDeallocated, storecenter.AuditActionDeleteComplete} {
		t.Run(string(action), func(t *testing.T) {
			repository := newStoreRepositoryFake()
			store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
			repository.stores["org-a/"+store.ID()] = cloneStore(store)
			ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}}
			audit := newAuditRepositoryFake()
			audit.failActions = map[storecenter.AuditAction]int{action: 1}
			service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, func() time.Time { return time.Now().UTC().Add(time.Hour) })
			if err != nil {
				t.Fatal(err)
			}
			request := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()}
			if _, err := service.Delete(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
				t.Fatalf("first Delete() = %v", err)
			}
			if current, getErr := repository.Get(context.Background(), "org-a", store.ID()); getErr == nil {
				request.ExpectedVersion = current.Version()
			}
			result, err := service.Delete(context.Background(), request)
			if err != nil || !result.Replayed && action == storecenter.AuditActionDeleteComplete {
				t.Fatalf("repair Delete() = %#v, %v", result, err)
			}
		})
	}
}

func TestServiceDeleteCrossOrganizationProbeNeverDeallocates(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	ledger := &quotaLedgerFake{}
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Delete(context.Background(), storecenter.DeleteStoreRequest{OrganizationID: "org-b", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()})
	if !errors.Is(err, storecenter.ErrNotFound) || ledger.deallocateCalls != 0 || repository.saveCalls != 0 || repository.softDeleteCalls != 0 {
		t.Fatalf("cross-org Delete() = %v calls=%d/%d/%d", err, ledger.deallocateCalls, repository.saveCalls, repository.softDeleteCalls)
	}
}

func TestServiceDeleteRejectsMismatchedDeallocationResult(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	wrong := listingsubscription.StoreQuotaAllocation{OrganizationID: "org-b", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaReleased}
	ledger := &quotaLedgerFake{allocation: wrong, deallocateOverride: &wrong}
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Delete(context.Background(), storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()})
	if !errors.Is(err, storecenter.ErrDependencyUnavailable) || repository.softDeleteCalls != 0 {
		t.Fatalf("mismatched deallocation Delete() = %v soft-deletes=%d", err, repository.softDeleteCalls)
	}
	current, getErr := repository.Get(context.Background(), "org-a", store.ID())
	if getErr != nil || current.LifecycleStatus() != storecenter.StoreStatusDeleting {
		t.Fatalf("mismatched deallocation durable store = %#v, %v", current, getErr)
	}
}

func TestServiceDeleteRejectsWrongIdentityAfterAmbiguousBeginDeleteSave(t *testing.T) {
	for _, identity := range []string{"organization", "store ID"} {
		t.Run("wrong "+identity, func(t *testing.T) {
			repository := newStoreRepositoryFake()
			store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
			repository.stores["org-a/"+store.ID()] = cloneStore(store)
			ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}}
			audit := newAuditRepositoryFake()
			operationKey := uuid.NewString()
			readback := cloneStore(store)
			if err := readback.BeginDelete(operationKey, "admin", readback.UpdatedAt().Add(time.Minute)); err != nil {
				t.Fatal(err)
			}
			snapshot := readback.Snapshot()
			if identity == "organization" {
				snapshot.OrganizationID = "org-b"
			} else {
				snapshot.ID = uuid.NewString()
			}
			readback, err := storecenter.RehydrateStore(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			repository.saveErr = errors.New("ambiguous save")
			repository.storeOnSaveError = readback
			service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.Delete(context.Background(), storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: operationKey})
			if !errors.Is(err, storecenter.ErrDependencyUnavailable) || ledger.deallocateCalls != 0 || repository.softDeleteCalls != 0 {
				t.Fatalf("Delete() wrong %s readback = %v dealloc=%d soft-delete=%d", identity, err, ledger.deallocateCalls, repository.softDeleteCalls)
			}
			if marked := audit.eventFor("org-a", operationKey, storecenter.AuditActionStoreMarkedDeleting); marked.Action != "" {
				t.Fatalf("wrong %s readback recorded false durable success: %+v", identity, marked)
			}
		})
	}
}

func TestServiceDeleteResolvesAmbiguousSoftDeleteOnlyWhenScopedReadIsNotFound(t *testing.T) {
	for _, persisted := range []bool{false, true} {
		t.Run(fmt.Sprintf("persisted_%t", persisted), func(t *testing.T) {
			repository := newStoreRepositoryFake()
			store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
			repository.stores["org-a/"+store.ID()] = cloneStore(store)
			repository.softDeleteErr = errors.New("soft delete timeout")
			repository.persistBeforeSoftDeleteError = persisted
			ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}}
			service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
			if err != nil {
				t.Fatal(err)
			}
			_, err = service.Delete(context.Background(), storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()})
			if persisted && err != nil {
				t.Fatalf("persisted ambiguous Delete() = %v", err)
			}
			if !persisted && !errors.Is(err, storecenter.ErrDependencyUnavailable) {
				t.Fatalf("unpersisted ambiguous Delete() = %v", err)
			}
		})
	}
}

func TestServiceDeleteWithDurableLedgersRemovesStoreAndLowersCommittedQuotaOnce(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.TempDir()+"/delete-integration.db?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		t.Fatal(err)
	}
	if err := storecenter.AutoMigrateAuditRepository(db); err != nil {
		t.Fatal(err)
	}
	if err := listingsubscription.AutoMigrateRepository(db); err != nil {
		t.Fatal(err)
	}
	repository, err := storecenter.NewGormStoreRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	audit, err := storecenter.NewGormAuditRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	activeSnapshot := store.Snapshot()
	activeSnapshot.ConnectionRef = ""
	store, err = storecenter.RehydrateStore(activeSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	pristine := store.Snapshot()
	pristine.LifecycleStatus, pristine.Version, pristine.UpdatedAt = storecenter.StoreStatusProvisioning, 1, pristine.CreatedAt
	pristine.UpdatedBy, pristine.ConnectionRef = pristine.CreatedBy, ""
	candidate, err := storecenter.RehydrateStore(pristine)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.CreateOrReplay(context.Background(), "org-a", candidate); err != nil {
		t.Fatal(err)
	}
	if err := repository.Save(context.Background(), "org-a", store, 1); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := db.Exec("INSERT INTO saas_store_quota_buckets (organization_id, committed, reserved, version, updated_at) VALUES (?, ?, ?, ?, ?)", "org-a", 1, 0, 1, now).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO saas_store_quota_allocations (allocation_id, organization_id, store_id, request_key, status, created_by, updated_by, allocated_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", store.QuotaAllocationID(), "org-a", store.ID(), store.CreateIdempotencyKey(), "allocated", "creator", "creator", now, now, now).Error; err != nil {
		t.Fatal(err)
	}
	service, err := storecenter.NewService(repository, listingsubscription.NewGormStoreQuotaLedger(listingsubscription.NewGormRepository(db)), audit, &serviceConnectionProvider{}, func() time.Time { return now.Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	request := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()}
	if _, err := service.Delete(context.Background(), request); err != nil {
		t.Fatalf("Delete() = %v", err)
	}
	if _, err := repository.Get(context.Background(), "org-a", store.ID()); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("deleted Get() = %v", err)
	}
	var committed int64
	if err := db.Table("saas_store_quota_buckets").Select("committed").Where("organization_id = ?", "org-a").Scan(&committed).Error; err != nil || committed != 0 {
		t.Fatalf("committed after Delete = %d, %v", committed, err)
	}
	if _, err := service.Delete(context.Background(), request); err != nil {
		t.Fatalf("Delete replay = %v", err)
	}
	if err := db.Table("saas_store_quota_buckets").Select("committed").Where("organization_id = ?", "org-a").Scan(&committed).Error; err != nil || committed != 0 {
		t.Fatalf("committed after replay = %d, %v", committed, err)
	}
}

func pointerInt64(value int64) *int64 { return &value }

func TestServiceUpdateNoOpAuditsWithoutSavingOrBumpingVersion(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	request := storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: store.Name(), Region: store.Region()}
	first, err := service.Update(context.Background(), request)
	if err != nil || first.Replayed || first.Store.Store.Version() != store.Version() || repository.saveCalls != 0 {
		t.Fatalf("first no-op Update() = %#v, %v saves=%d", first, err, repository.saveCalls)
	}
	second, err := service.Update(context.Background(), request)
	if err != nil || !second.Replayed || repository.saveCalls != 0 {
		t.Fatalf("replay no-op Update() = %#v, %v saves=%d", second, err, repository.saveCalls)
	}
}

func TestServiceUpdateAuditsOnlyActualFieldsThroughDurableIntent(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	request := storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: "After", Region: store.Region()}
	if _, err := service.Update(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	operationKey := uuid.NewSHA1(uuid.NameSpaceOID, []byte("org-a\n"+store.ID()+"\nupdate\n"+fmt.Sprint(store.Version()))).String()
	started := audit.eventFor("org-a", operationKey, storecenter.AuditActionStoreUpdateStarted)
	if started.Action != storecenter.AuditActionStoreUpdateStarted || started.Outcome != storecenter.AuditOutcomeUnknown || started.FailureCode != storecenter.AuditFailureNone || started.StoreVersion != store.Version() || !sameStrings(started.SafeFieldNames, []string{"name"}) {
		t.Fatalf("update intent = %+v, want unknown version-%d name-only checkpoint", started, store.Version())
	}
	completed := audit.eventFor("org-a", operationKey, storecenter.AuditActionStoreUpdated)
	if completed.Outcome != storecenter.AuditOutcomeSucceeded || completed.StoreVersion != store.Version()+1 || !sameStrings(completed.SafeFieldNames, []string{"name"}) {
		t.Fatalf("completed update = %+v, want succeeded name-only fields", completed)
	}
}

func TestServiceUpdateIntentFailureDoesNotSaveAndRetryConverges(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionStoreUpdateStarted: 1}
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	request := storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: "After", Region: "MY"}
	if _, err := service.Update(context.Background(), request); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Update() = %v, want dependency error", err)
	}
	if repository.saveCalls != 0 {
		t.Fatalf("Save calls after failed intent = %d, want 0", repository.saveCalls)
	}
	result, err := service.Update(context.Background(), request)
	if err != nil || result.Store.Store.Name() != "After" || repository.saveCalls != 1 {
		t.Fatalf("retry Update() = %#v, %v saves=%d", result, err, repository.saveCalls)
	}
}

func TestServiceUpdateReplayUsesDurableIntentActorForStoreWrite(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	repository.saveErr = errors.New("save unavailable")
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	first := storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "actor-a", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: "After", Region: store.Region()}
	if _, err := service.Update(context.Background(), first); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Update() = %v, want dependency error", err)
	}
	repository.saveErr = nil
	retry := first
	retry.ActorSubject = "actor-b"
	result, err := service.Update(context.Background(), retry)
	if err != nil {
		t.Fatalf("cross-actor retry Update() = %v", err)
	}
	if result.Store.Store.UpdatedBy() != "actor-a" {
		t.Fatalf("replayed Store UpdatedBy = %q, want durable intent actor-a", result.Store.Store.UpdatedBy())
	}
	operationKey := uuid.NewSHA1(uuid.NameSpaceOID, []byte("org-a\n"+store.ID()+"\nupdate\n"+fmt.Sprint(store.Version()))).String()
	completed := audit.eventFor("org-a", operationKey, storecenter.AuditActionStoreUpdated)
	if completed.ActorSubject != "actor-a" {
		t.Fatalf("completed audit actor = %q, want actor-a", completed.ActorSubject)
	}
}

func TestServiceUpdateRejectsChangedPayloadForExistingIntent(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	repository.saveErr = errors.New("save unavailable")
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	first := storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: "First", Region: store.Region()}
	if _, err := service.Update(context.Background(), first); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Update() = %v, want dependency error", err)
	}
	repository.saveErr = nil
	second := first
	second.Name = "Second"
	if _, err := service.Update(context.Background(), second); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("changed retry Update() = %v, want dependency error", err)
	}
	if repository.saveCalls != 1 {
		t.Fatalf("changed retry Save calls = %d, want 1", repository.saveCalls)
	}
	current, err := repository.Get(context.Background(), "org-a", store.ID())
	if err != nil || current.Name() != "Before" || current.Version() != store.Version() {
		t.Fatalf("store after changed retry = %#v, %v", current, err)
	}
}

func TestServiceDoesNotRepairUpdateAuditAfterLaterMutationWithoutDurableResult(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionStoreUpdated: 1}
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	update := storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: "After", Region: store.Region()}
	if _, err := service.Update(context.Background(), update); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Update() = %v, want dependency error", err)
	}
	disable, err := service.Disable(context.Background(), storecenter.StoreLifecycleRequest{OrganizationID: "org-a", ActorSubject: "operator", StoreID: store.ID(), ExpectedVersion: store.Version() + 1})
	if err != nil || disable.Store.Store.Version() != store.Version()+2 {
		t.Fatalf("later Disable() = %#v, %v", disable, err)
	}
	if _, err := service.Update(context.Background(), update); !errors.Is(err, storecenter.ErrVersionConflict) {
		t.Fatalf("replayed Update() = %v, want version conflict", err)
	}
	current, err := repository.Get(context.Background(), "org-a", store.ID())
	if err != nil || current.Name() != "After" || current.Version() != store.Version()+2 {
		t.Fatalf("store after rejected repair = %#v, %v", current, err)
	}
}

func TestServiceDoesNotRepairUnpersistedUpdateAfterLaterLifecycleMutations(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	repository.saveErr = errors.New("save unavailable")
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	update := storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: "After", Region: store.Region()}
	if _, err := service.Update(context.Background(), update); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("failed Update() = %v, want dependency error", err)
	}
	repository.saveErr = nil
	disabled, err := service.Disable(context.Background(), storecenter.StoreLifecycleRequest{OrganizationID: "org-a", ActorSubject: "operator", StoreID: store.ID(), ExpectedVersion: store.Version()})
	if err != nil || disabled.Store.Store.Version() != store.Version()+1 {
		t.Fatalf("Disable() = %#v, %v", disabled, err)
	}
	enabled, err := service.Enable(context.Background(), storecenter.StoreLifecycleRequest{OrganizationID: "org-a", ActorSubject: "operator", StoreID: store.ID(), ExpectedVersion: store.Version() + 1})
	if err != nil || enabled.Store.Store.Version() != store.Version()+2 {
		t.Fatalf("Enable() = %#v, %v", enabled, err)
	}
	if _, err := service.Update(context.Background(), update); !errors.Is(err, storecenter.ErrVersionConflict) {
		t.Fatalf("replay Update() = %v, want version conflict", err)
	}
	current, err := repository.Get(context.Background(), "org-a", store.ID())
	if err != nil || current.Name() != "Before" || current.Version() != store.Version()+2 {
		t.Fatalf("store after rejected repair = %#v, %v", current, err)
	}
}

func TestServiceDoesNotRepairUnpersistedLifecycleAfterLaterProfileMutations(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	repository.saveErr = errors.New("save unavailable")
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	disable := storecenter.StoreLifecycleRequest{OrganizationID: "org-a", ActorSubject: "operator", StoreID: store.ID(), ExpectedVersion: store.Version()}
	if _, err := service.Disable(context.Background(), disable); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("failed Disable() = %v, want dependency error", err)
	}
	repository.saveErr = nil
	for version, name := range []string{"First", "Second"} {
		request := storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: store.ID(), ExpectedVersion: store.Version() + int64(version), Name: name, Region: store.Region()}
		if _, err := service.Update(context.Background(), request); err != nil {
			t.Fatalf("later Update(%q) = %v", name, err)
		}
	}
	if _, err := service.Disable(context.Background(), disable); !errors.Is(err, storecenter.ErrVersionConflict) {
		t.Fatalf("replayed Disable() = %v, want version conflict", err)
	}
	current, err := repository.Get(context.Background(), "org-a", store.ID())
	if err != nil || current.LifecycleStatus() != storecenter.StoreStatusActive || current.Version() != store.Version()+2 {
		t.Fatalf("store after rejected lifecycle repair = %#v, %v", current, err)
	}
}

func TestServiceDoesNotRepairLifecycleAuditAfterLaterMutationWithoutDurableResult(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionStoreDisabled: 1}
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	disableRequest := storecenter.StoreLifecycleRequest{OrganizationID: "org-a", ActorSubject: "operator", StoreID: store.ID(), ExpectedVersion: store.Version()}
	if _, err := service.Disable(context.Background(), disableRequest); !errors.Is(err, storecenter.ErrDependencyUnavailable) {
		t.Fatalf("first Disable() = %v, want dependency error", err)
	}
	enable, err := service.Enable(context.Background(), storecenter.StoreLifecycleRequest{OrganizationID: "org-a", ActorSubject: "operator", StoreID: store.ID(), ExpectedVersion: store.Version() + 1})
	if err != nil || enable.Store.Store.Version() != store.Version()+2 {
		t.Fatalf("later Enable() = %#v, %v", enable, err)
	}
	if _, err := service.Disable(context.Background(), disableRequest); !errors.Is(err, storecenter.ErrVersionConflict) {
		t.Fatalf("replayed Disable() = %v, want version conflict", err)
	}
	current, err := repository.Get(context.Background(), "org-a", store.ID())
	if err != nil || current.LifecycleStatus() != storecenter.StoreStatusActive || current.Version() != store.Version()+2 {
		t.Fatalf("store after rejected lifecycle repair = %#v, %v", current, err)
	}
}

// A deterministic identity collision is a safe business conflict, not an
// infrastructure outage. The service must preserve the stable sentinel and
// never expose SQLite/GORM details.
func TestServiceUpdatePreservesRedactedAlreadyExistsFromGormSave(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.TempDir()+"/update-collision.db?_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		t.Fatal(err)
	}
	if err := storecenter.AutoMigrateAuditRepository(db); err != nil {
		t.Fatal(err)
	}
	repository, err := storecenter.NewGormStoreRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	audit, err := storecenter.NewGormAuditRepository(db)
	if err != nil {
		t.Fatal(err)
	}
	persistActive := func(region string) *storecenter.Store {
		store, createErr := storecenter.NewStore(storecenter.CreateStoreInput{
			ID: uuid.NewString(), OrganizationID: "org-a", ActorSubject: "creator", Name: "Store " + region,
			Platform: "shein", Region: region, ExternalStoreID: "shared-external", CreateIdempotencyKey: uuid.NewString(),
			QuotaAllocationID: uuid.NewString(), OccurredAt: time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC),
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, _, createErr = repository.CreateOrReplay(context.Background(), "org-a", store); createErr != nil {
			t.Fatal(createErr)
		}
		if createErr = store.TransitionTo(storecenter.StoreStatusActive, "creator", store.UpdatedAt().Add(time.Second)); createErr != nil {
			t.Fatal(createErr)
		}
		if createErr = repository.Save(context.Background(), "org-a", store, 1); createErr != nil {
			t.Fatal(createErr)
		}
		return store
	}
	_ = persistActive("SG")
	candidate := persistActive("MY")
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, func() time.Time {
		return candidate.UpdatedAt().Add(time.Minute)
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Update(context.Background(), storecenter.UpdateStoreRequest{
		OrganizationID: "org-a", ActorSubject: "editor", StoreID: candidate.ID(), ExpectedVersion: candidate.Version(), Name: candidate.Name(), Region: "SG",
	})
	if !errors.Is(err, storecenter.ErrAlreadyExists) {
		t.Fatalf("Update() error = %v, want ErrAlreadyExists", err)
	}
	if err.Error() != storecenter.ErrAlreadyExists.Error() {
		t.Fatalf("Update() error = %q, want redacted %q", err, storecenter.ErrAlreadyExists)
	}
}

func TestServiceUpdateAfterNoOpAtSameVersionDoesNotPoisonRealMutation(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	base := storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: store.Name(), Region: store.Region()}
	if _, err := service.Update(context.Background(), base); err != nil {
		t.Fatal(err)
	}
	base.Name = "Changed"
	result, err := service.Update(context.Background(), base)
	if err != nil || result.Store.Store.Name() != "Changed" || result.Store.Store.Version() != store.Version()+1 {
		t.Fatalf("real Update after no-op = %#v, %v", result, err)
	}
}

func TestServiceConcurrentDifferentSameVersionEditsHaveOneWinner(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	service, err := storecenter.NewService(repository, &quotaLedgerFake{}, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var group sync.WaitGroup
	for _, name := range []string{"Left", "Right"} {
		name := name
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := service.Update(context.Background(), storecenter.UpdateStoreRequest{OrganizationID: "org-a", ActorSubject: "editor", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: name, Region: "SG"})
			results <- err
		}()
	}
	close(start)
	group.Wait()
	close(results)
	var successes, conflicts int
	for err := range results {
		if err == nil {
			successes++
		} else if errors.Is(err, storecenter.ErrVersionConflict) {
			conflicts++
		} else {
			t.Fatalf("concurrent Update() = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 || repository.saveCalls < 1 || repository.saveCalls > 2 {
		t.Fatalf("concurrent edits success/conflict/saves = %d/%d/%d", successes, conflicts, repository.saveCalls)
	}
}

func TestServiceConcurrentDifferentDeleteKeysHaveOneOwner(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}}
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, ledger, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	type output struct {
		key string
		err error
	}
	results := make(chan output, 2)
	var group sync.WaitGroup
	for range 2 {
		key := uuid.NewString()
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := service.Delete(context.Background(), storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: key})
			results <- output{key: key, err: err}
		}()
	}
	close(start)
	group.Wait()
	close(results)
	var successes, failures int
	for result := range results {
		if result.err == nil {
			successes++
			continue
		} else if errors.Is(result.err, storecenter.ErrNotFound) || errors.Is(result.err, storecenter.ErrDependencyUnavailable) || errors.Is(result.err, storecenter.ErrVersionConflict) || errors.Is(result.err, storecenter.ErrInvalidTransition) {
			failures++
		} else {
			t.Fatalf("concurrent Delete() = %v", result.err)
		}
		for _, action := range []storecenter.AuditAction{storecenter.AuditActionStoreMarkedDeleting, storecenter.AuditActionQuotaDeallocated, storecenter.AuditActionDeleteComplete} {
			if event := audit.eventFor("org-a", result.key, action); event.Action != "" {
				t.Fatalf("losing operation %s recorded false success %s: %+v", result.key, action, event)
			}
		}
	}
	if successes != 1 || failures != 1 || ledger.deallocateCalls != 1 {
		t.Fatalf("different-key deletes success/failure/deallocate = %d/%d/%d", successes, failures, ledger.deallocateCalls)
	}
}

func TestServiceDeletePreservesLostDeleteOwnershipAsInvalidTransition(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)

	otherKey := uuid.NewString()
	other := cloneStore(store)
	if err := other.BeginDelete(otherKey, "other-admin", store.UpdatedAt().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	repository.saveErr = storecenter.ErrVersionConflict
	repository.storeOnSaveError = other
	service, err := storecenter.NewService(repository, &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}}, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Delete(context.Background(), storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: uuid.NewString()})
	if !errors.Is(err, storecenter.ErrInvalidTransition) {
		t.Fatalf("lost delete ownership error = %v, want ErrInvalidTransition", err)
	}
}

func TestServiceConcurrentSameDeleteKeyConverges(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	ledger := &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: "org-a", AllocationID: store.QuotaAllocationID(), StoreID: store.ID(), RequestKey: store.CreateIdempotencyKey(), Status: listingsubscription.StoreQuotaAllocated}}
	service, err := storecenter.NewService(repository, ledger, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	request := storecenter.DeleteStoreRequest{OrganizationID: "org-a", ActorSubject: "admin", StoreID: store.ID(), ExpectedVersion: store.Version(), OperationKey: key}
	start := make(chan struct{})
	results := make(chan struct {
		result storecenter.DeleteStoreResult
		err    error
	}, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			result, err := service.Delete(context.Background(), request)
			results <- struct {
				result storecenter.DeleteStoreResult
				err    error
			}{result, err}
		}()
	}
	close(start)
	group.Wait()
	close(results)
	var replayed int
	for output := range results {
		if output.err != nil {
			output.result, output.err = service.Delete(context.Background(), request)
		}
		if output.err != nil || output.result.StoreID != store.ID() {
			t.Fatalf("same-key Delete() = %#v, %v", output.result, output.err)
		}
		if output.result.Replayed {
			replayed++
		}
	}
	if ledger.allocation.Status != listingsubscription.StoreQuotaReleased || ledger.deallocateCalls < 1 || ledger.deallocateCalls > 2 || replayed < 1 {
		t.Fatalf("same-key convergence status/calls/replays = %s/%d/%d", ledger.allocation.Status, ledger.deallocateCalls, replayed)
	}
}

func validCreateRequest() storecenter.CreateStoreRequest {
	return storecenter.CreateStoreRequest{OrganizationID: "org-1", ActorSubject: "actor-1", IdempotencyKey: uuid.NewString(), Name: "Shop", Platform: "shein", Region: "SG"}
}
func quotaForRequest(request storecenter.CreateStoreRequest) *quotaLedgerFake {
	return &quotaLedgerFake{allocation: listingsubscription.StoreQuotaAllocation{OrganizationID: request.OrganizationID, AllocationID: uuid.NewString(), StoreID: uuid.NewString(), RequestKey: request.IdempotencyKey, Status: listingsubscription.StoreQuotaReserved}}
}

func activeServiceStore(t *testing.T, organizationID, id, key, allocationID, name string) *storecenter.Store {
	t.Helper()
	store, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: id, OrganizationID: organizationID, ActorSubject: "creator", Name: name, Platform: "shein", Region: "SG", CreateIdempotencyKey: key, QuotaAllocationID: allocationID, OccurredAt: time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.TransitionTo(storecenter.StoreStatusActive, "creator", store.UpdatedAt().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	snapshot := store.Snapshot()
	snapshot.ConnectionRef = "opaque-" + id
	store, err = storecenter.RehydrateStore(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

type serviceConnectionProvider struct {
	mu                       sync.Mutex
	statuses                 map[string]storecenter.ConnectionStatus
	err                      error
	calls, active, maxActive int
	delay                    time.Duration
}

func (p *serviceConnectionProvider) Status(ctx context.Context, input storecenter.ConnectionStatusInput) (storecenter.ConnectionStatus, error) {
	p.mu.Lock()
	p.calls++
	p.active++
	if p.active > p.maxActive {
		p.maxActive = p.active
	}
	p.mu.Unlock()
	defer func() { p.mu.Lock(); p.active--; p.mu.Unlock() }()
	if p.delay > 0 {
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if p.err != nil {
		return "", p.err
	}
	if status := p.statuses[input.StoreID]; status != "" {
		return status, nil
	}
	return storecenter.ConnectionStatusUnavailable, nil
}

type storeRepositoryFake struct {
	mu                           sync.Mutex
	stores                       map[string]*storecenter.Store
	fingerprints                 map[string]storeCreateFingerprint
	createErr                    error
	createErrOnce                bool
	storeOnCreateError           *storecenter.Store
	getErr                       error
	getErrOnce                   bool
	getErrAfterCreate            error
	saveErr                      error
	storeOnSaveError             *storecenter.Store
	persistBeforeCreateError     bool
	persistBeforeSaveError       bool
	createCalls                  int
	getCalls                     int
	saveCalls                    int
	listPage                     storecenter.StorePage
	listErr                      error
	listCalls                    int
	lastListOrganization         string
	lastListQuery                storecenter.StoreListQuery
	softDeleteErr                error
	persistBeforeSoftDeleteError bool
	softDeleteCalls              int
}

func newStoreRepositoryFake() *storeRepositoryFake {
	return &storeRepositoryFake{stores: map[string]*storecenter.Store{}, fingerprints: map[string]storeCreateFingerprint{}}
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
			f.fingerprints[organizationID+"/"+store.ID()] = fingerprintFor(store)
		}
		if f.storeOnCreateError != nil {
			f.stores[organizationID+"/"+store.ID()] = cloneStore(f.storeOnCreateError)
			f.fingerprints[organizationID+"/"+store.ID()] = fingerprintFor(f.storeOnCreateError)
		}
		err := f.createErr
		if f.createErrOnce {
			f.createErr = nil
		}
		return nil, false, err
	}
	key := organizationID + "/" + store.ID()
	if existing := f.stores[key]; existing != nil {
		if f.fingerprints[key] != fingerprintFor(store) {
			return nil, false, storecenter.ErrAlreadyExists
		}
		return cloneStore(existing), true, nil
	}
	f.stores[key] = cloneStore(store)
	f.fingerprints[key] = fingerprintFor(store)
	return cloneStore(store), false, nil
}
func (f *storeRepositoryFake) List(_ context.Context, organizationID string, query storecenter.StoreListQuery) (storecenter.StorePage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listCalls++
	f.lastListOrganization, f.lastListQuery = organizationID, query
	if f.listErr != nil {
		return storecenter.StorePage{}, f.listErr
	}
	return f.listPage, nil
}
func (f *storeRepositoryFake) Get(_ context.Context, organizationID, storeID string) (*storecenter.Store, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCalls++
	if f.getErr != nil {
		err := f.getErr
		if f.getErrOnce {
			f.getErr = nil
		}
		return nil, err
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
		if f.storeOnSaveError != nil {
			f.stores[organizationID+"/"+store.ID()] = cloneStore(f.storeOnSaveError)
		} else if f.persistBeforeSaveError {
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
func (f *storeRepositoryFake) SoftDelete(_ context.Context, organizationID, storeID string, expectedVersion int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.softDeleteCalls++
	key := organizationID + "/" + storeID
	store := f.stores[key]
	if store == nil {
		return storecenter.ErrNotFound
	}
	if store.Version() != expectedVersion {
		return storecenter.ErrVersionConflict
	}
	if store.LifecycleStatus() != storecenter.StoreStatusDeleting {
		return storecenter.ErrInvalidTransition
	}
	if f.softDeleteErr != nil {
		if f.persistBeforeSoftDeleteError {
			delete(f.stores, key)
		}
		return f.softDeleteErr
	}
	delete(f.stores, key)
	return nil
}

func cloneStore(store *storecenter.Store) *storecenter.Store {
	cloned, err := storecenter.RehydrateStore(store.Snapshot())
	if err != nil {
		panic(err)
	}
	return cloned
}

func matchingStore(t *testing.T, request storecenter.CreateStoreRequest, allocation listingsubscription.StoreQuotaAllocation) *storecenter.Store {
	t.Helper()
	store, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: allocation.StoreID, OrganizationID: request.OrganizationID, ActorSubject: request.ActorSubject, Name: request.Name, Platform: request.Platform, Region: request.Region, ExternalStoreID: request.ExternalStoreID, CreateIdempotencyKey: request.IdempotencyKey, QuotaAllocationID: allocation.AllocationID, OccurredAt: time.Now().UTC()})
	if err != nil {
		t.Fatalf("NewStore(matching): %v", err)
	}
	return store
}

type storeCreateFingerprint struct{ id, allocationID, key, name, platform, region, external string }

func fingerprintFor(store *storecenter.Store) storeCreateFingerprint {
	return storeCreateFingerprint{id: store.ID(), allocationID: store.QuotaAllocationID(), key: store.CreateIdempotencyKey(), name: store.Name(), platform: string(store.Platform()), region: store.Region(), external: store.ExternalStoreID()}
}

type quotaLedgerFake struct {
	mu                                      sync.Mutex
	allocation                              listingsubscription.StoreQuotaAllocation
	releaseOverride                         *listingsubscription.StoreQuotaAllocation
	reserveErr, commitErr, releaseErr       error
	reserveErrOnce                          bool
	reserveCalls, commitCalls, releaseCalls int
	staleReservations                       []listingsubscription.StoreQuotaAllocation
	summary                                 listingsubscription.StoreQuotaSummary
	summaryErr                              error
	summaryCalls                            int
	deallocateErr                           error
	deallocateOverride                      *listingsubscription.StoreQuotaAllocation
	deallocateCalls                         int
}

func (f *quotaLedgerFake) Reserve(_ context.Context, input listingsubscription.StoreQuotaReserveInput) (listingsubscription.StoreQuotaReserveResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reserveCalls++
	if f.reserveErr != nil {
		err := f.reserveErr
		if f.reserveErrOnce {
			f.reserveErr = nil
		}
		return listingsubscription.StoreQuotaReserveResult{}, err
	}
	if f.allocation.RequestFingerprint == "" {
		f.allocation.RequestFingerprint = input.RequestFingerprint
	} else if f.allocation.RequestFingerprint != input.RequestFingerprint {
		return listingsubscription.StoreQuotaReserveResult{}, listingsubscription.ErrStoreQuotaIdentityMismatch
	}
	return listingsubscription.StoreQuotaReserveResult{Allocation: f.allocation, AllocationID: f.allocation.AllocationID, StoreID: f.allocation.StoreID, Existing: f.reserveCalls > 1}, nil
}
func (f *quotaLedgerFake) RenewReservation(_ context.Context, _ listingsubscription.StoreQuotaTransitionInput) (listingsubscription.StoreQuotaTransitionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return listingsubscription.StoreQuotaTransitionResult{Allocation: f.allocation}, nil
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
func (f *quotaLedgerFake) ReleaseReservation(_ context.Context, input listingsubscription.StoreQuotaTransitionInput) (listingsubscription.StoreQuotaTransitionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseCalls++
	if input.ExpectedUpdatedAt != nil && !f.allocation.UpdatedAt.Equal(input.ExpectedUpdatedAt.UTC()) {
		return listingsubscription.StoreQuotaTransitionResult{}, listingsubscription.ErrStoreQuotaStale
	}
	if f.releaseErr != nil {
		return listingsubscription.StoreQuotaTransitionResult{}, f.releaseErr
	}
	if f.releaseOverride != nil {
		return listingsubscription.StoreQuotaTransitionResult{Allocation: *f.releaseOverride}, nil
	}
	f.allocation.Status = listingsubscription.StoreQuotaReleased
	return listingsubscription.StoreQuotaTransitionResult{Allocation: f.allocation}, nil
}
func (f *quotaLedgerFake) Deallocate(_ context.Context, _ listingsubscription.StoreQuotaTransitionInput) (listingsubscription.StoreQuotaTransitionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deallocateCalls++
	if f.deallocateErr != nil {
		return listingsubscription.StoreQuotaTransitionResult{}, f.deallocateErr
	}
	if f.deallocateOverride != nil {
		return listingsubscription.StoreQuotaTransitionResult{Allocation: *f.deallocateOverride}, nil
	}
	f.allocation.Status = listingsubscription.StoreQuotaReleased
	return listingsubscription.StoreQuotaTransitionResult{Allocation: f.allocation, Existing: f.deallocateCalls > 1}, nil
}
func (f *quotaLedgerFake) GetByRequestKey(context.Context, string, string) (*listingsubscription.StoreQuotaAllocation, error) {
	return nil, errors.New("not used")
}
func (f *quotaLedgerFake) Summary(context.Context, string) (listingsubscription.StoreQuotaSummary, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.summaryCalls++
	return f.summary, f.summaryErr
}

func (f *quotaLedgerFake) ListReservedBefore(context.Context, string, time.Time) ([]listingsubscription.StoreQuotaAllocation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]listingsubscription.StoreQuotaAllocation(nil), f.staleReservations...), nil
}

type auditRepositoryFake struct {
	mu          sync.Mutex
	events      map[string]storecenter.AuditEvent
	recordErr   error
	failActions map[storecenter.AuditAction]int
	recordCalls int
	onRecord    func(storecenter.AuditEvent)
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
		if existing.StoreID != event.StoreID || existing.AllocationID != event.AllocationID || existing.Outcome != event.Outcome || existing.PayloadFingerprint != event.PayloadFingerprint || existing.PreviousState != event.PreviousState || existing.NewState != event.NewState || existing.FailureCode != event.FailureCode || existing.StoreVersion != event.StoreVersion || !sameStrings(existing.SafeFieldNames, event.SafeFieldNames) {
			return storecenter.AuditEvent{}, false, storecenter.ErrAuditIdentityMismatch
		}
		return existing, true, nil
	}
	f.events[key] = event
	if f.onRecord != nil {
		f.onRecord(event)
	}
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
func (f *auditRepositoryFake) GetByStoreID(_ context.Context, organizationID, storeID string, action storecenter.AuditAction) (*storecenter.AuditEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var found *storecenter.AuditEvent
	for _, event := range f.events {
		if event.OrganizationID == organizationID && event.StoreID == storeID && event.Action == action {
			copy := event
			if found == nil || copy.OccurredAt.After(found.OccurredAt) {
				found = &copy
			}
		}
	}
	if found == nil {
		return nil, storecenter.ErrNotFound
	}
	return found, nil
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

func (f *auditRepositoryFake) eventFor(organizationID, requestKey string, action storecenter.AuditAction) storecenter.AuditEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.events[organizationID+"/"+requestKey+"/"+string(action)]
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
