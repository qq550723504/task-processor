package storecenter_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/storecenter"
)

func TestServiceReadValidationCallsNoDependencies(t *testing.T) {
	repository, provider := newStoreRepositoryFake(), &serviceConnectionProvider{}
	service, err := storecenter.NewService(repository, newAuditRepositoryFake(), provider, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.List(context.Background(), storecenter.ListStoresRequest{OrganizationID: " org-a", Page: 0, PageSize: 101, Platform: "shopify"}); err == nil {
		t.Fatal("invalid List error = nil")
	}
	if _, err := service.Get(context.Background(), storecenter.GetStoreRequest{OrganizationID: "org-a", StoreID: "bad"}); err == nil {
		t.Fatal("invalid Get error = nil")
	}
	if repository.getCalls != 0 || repository.listCalls != 0 || provider.calls != 0 {
		t.Fatalf("dependency calls = %d/%d/%d", repository.getCalls, repository.listCalls, provider.calls)
	}
}

func TestServiceRequiresConnectionStatusProviderIncludingTypedNil(t *testing.T) {
	var typedNil *serviceConnectionProvider
	for _, provider := range []storecenter.ConnectionStatusProvider{nil, typedNil} {
		if _, err := storecenter.NewService(newStoreRepositoryFake(), newAuditRepositoryFake(), provider, time.Now); err == nil {
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
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
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
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, time.Now)
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
					OrganizationID: "org-a", StoreID: original.ID(), RequestKey: operationKey,
					Action: storecenter.AuditActionStoreUpdateStarted, Outcome: storecenter.AuditOutcomeUnknown, ActorSubject: "editor",
					SafeFieldNames: []string{"quota_allocation_id"}, PreviousState: storecenter.RecordStatusActive, NewState: storecenter.RecordStatusActive,
					FailureCode: storecenter.AuditFailureNone, StoreVersion: original.Version(),
				}
			}
			service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, time.Now)
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

func TestServiceLifecycleRejectsIllegalStatesAndStaleVersions(t *testing.T) {
	tests := []struct {
		name            string
		status          storecenter.RecordStatus
		action          string
		expectedVersion func(*storecenter.Store) int64
		want            error
	}{
		{"disable disabled", storecenter.RecordStatusDisabled, "disable", func(store *storecenter.Store) int64 { return store.Version() }, storecenter.ErrInvalidTransition},
		{"enable active", storecenter.RecordStatusActive, "enable", func(store *storecenter.Store) int64 { return store.Version() }, storecenter.ErrInvalidTransition},
		{"enable deleting", storecenter.RecordStatusDeleting, "enable", func(store *storecenter.Store) int64 { return store.Version() }, storecenter.ErrInvalidTransition},
		{"stale disable", storecenter.RecordStatusActive, "disable", func(store *storecenter.Store) int64 { return store.Version() + 1 }, storecenter.ErrVersionConflict},
		{"stale enable", storecenter.RecordStatusDisabled, "enable", func(store *storecenter.Store) int64 { return store.Version() + 1 }, storecenter.ErrVersionConflict},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: uuid.NewString(), OrganizationID: "org-a", ActorSubject: "creator", Name: "Store", Platform: "shein", Region: "SG", CreateIdempotencyKey: uuid.NewString(), OccurredAt: time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)})
			if err != nil {
				t.Fatal(err)
			}
			if test.status == storecenter.RecordStatusDisabled {
				if err := store.TransitionTo(storecenter.RecordStatusDisabled, "operator", store.UpdatedAt().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
			}
			if test.status == storecenter.RecordStatusDeleting {
				if err := store.BeginDelete(uuid.NewString(), "operator", store.UpdatedAt().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
			}
			repository := newStoreRepositoryFake()
			repository.stores["org-a/"+store.ID()] = cloneStore(store)
			service, err := storecenter.NewService(repository, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
			if err != nil {
				t.Fatal(err)
			}
			request := storecenter.StoreLifecycleRequest{OrganizationID: "org-a", ActorSubject: "operator", StoreID: store.ID(), ExpectedVersion: test.expectedVersion(store)}
			if test.action == "disable" {
				_, err = service.Disable(context.Background(), request)
			} else {
				_, err = service.Enable(context.Background(), request)
			}
			if !errors.Is(err, test.want) || repository.saveCalls != 0 {
				t.Fatalf("%s() = %v saves=%d", test.action, err, repository.saveCalls)
			}
		})
	}
}

func TestServiceUpdateAmbiguousSaveWithUnchangedDurableStateIsDependencyUnavailable(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", "00000000-0000-4000-8000-000000000507", "00000000-0000-4000-8000-000000000607", "00000000-0000-4000-8000-000000000707", "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	repository.saveErr = errors.New("database timeout")
	service, err := storecenter.NewService(repository, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
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
				service, err := storecenter.NewService(repository, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
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

func TestServiceListBoundsConnectionConcurrencyAndPreservesOrder(t *testing.T) {
	repository := newStoreRepositoryFake()
	stores := make([]storecenter.Store, 20)
	for i := range stores {
		store := activeServiceStore(t, "org-a", fmt.Sprintf("00000000-0000-4000-8000-%012d", 800+i), fmt.Sprintf("00000000-0000-4000-8000-%012d", 900+i), fmt.Sprintf("00000000-0000-4000-8000-%012d", 1000+i), fmt.Sprintf("Store %02d", i))
		stores[i] = *store
	}
	repository.listPage = storecenter.StorePage{Stores: stores, Total: int64(len(stores))}
	provider := &serviceConnectionProvider{delay: 20 * time.Millisecond}
	service, err := storecenter.NewService(repository, newAuditRepositoryFake(), provider, time.Now)
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

func TestServiceUpdateNoOpAuditsWithoutSavingOrBumpingVersion(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Store")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	service, err := storecenter.NewService(repository, newAuditRepositoryFake(), &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
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
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
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
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
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

func TestServiceUpdateRacedIntentUsesRecordedActorForStoreWrite(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	audit := newAuditRepositoryFake()
	audit.beforeRecord = func(event storecenter.AuditEvent) {
		if event.Action != storecenter.AuditActionStoreUpdateStarted {
			return
		}
		existing := event
		existing.ActorSubject = "actor-a"
		audit.events[event.OrganizationID+"/"+event.RequestKey+"/"+string(event.Action)] = existing
	}
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.Update(context.Background(), storecenter.UpdateStoreRequest{
		OrganizationID: store.OrganizationID(), ActorSubject: "actor-b", StoreID: store.ID(), ExpectedVersion: store.Version(), Name: "After", Region: store.Region(),
	})
	if err != nil {
		t.Fatalf("raced Update() = %v", err)
	}
	if result.Store.Store.UpdatedBy() != "actor-a" {
		t.Fatalf("raced Store UpdatedBy = %q, want recorded actor-a", result.Store.Store.UpdatedBy())
	}
	operationKey := uuid.NewSHA1(uuid.NameSpaceOID, []byte(store.OrganizationID()+"\n"+store.ID()+"\nupdate\n"+fmt.Sprint(store.Version()))).String()
	completed := audit.eventFor(store.OrganizationID(), operationKey, storecenter.AuditActionStoreUpdated)
	if completed.ActorSubject != "actor-a" {
		t.Fatalf("raced completed audit actor = %q, want actor-a", completed.ActorSubject)
	}
}

func TestServiceUpdateRejectsChangedPayloadForExistingIntent(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	repository.saveErr = errors.New("save unavailable")
	audit := newAuditRepositoryFake()
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, time.Now)
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
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, time.Now)
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
	if err != nil || current.RecordStatus() != storecenter.RecordStatusActive || current.Version() != store.Version()+2 {
		t.Fatalf("store after rejected lifecycle repair = %#v, %v", current, err)
	}
}

func TestServiceDoesNotRepairLifecycleAuditAfterLaterMutationWithoutDurableResult(t *testing.T) {
	repository := newStoreRepositoryFake()
	store := activeServiceStore(t, "org-a", uuid.NewString(), uuid.NewString(), uuid.NewString(), "Before")
	repository.stores["org-a/"+store.ID()] = cloneStore(store)
	audit := newAuditRepositoryFake()
	audit.failActions = map[storecenter.AuditAction]int{storecenter.AuditActionStoreDisabled: 1}
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, time.Now)
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
	if err != nil || current.RecordStatus() != storecenter.RecordStatusActive || current.Version() != store.Version()+2 {
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
			OccurredAt: time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC),
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, _, createErr = repository.CreateOrReplay(context.Background(), "org-a", store); createErr != nil {
			t.Fatal(createErr)
		}
		return store
	}
	_ = persistActive("SG")
	candidate := persistActive("MY")
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, func() time.Time {
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
	service, err := storecenter.NewService(repository, audit, &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
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
	service, err := storecenter.NewService(repository, newAuditRepositoryFake(), &serviceConnectionProvider{}, func() time.Time { return store.UpdatedAt().Add(time.Minute) })
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

func activeServiceStore(t *testing.T, organizationID, id, key, allocationID, name string) *storecenter.Store {
	t.Helper()
	store, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: id, OrganizationID: organizationID, ActorSubject: "creator", Name: name, Platform: "shein", Region: "SG", CreateIdempotencyKey: key, OccurredAt: time.Date(2026, 8, 30, 9, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	snapshot := store.Snapshot()
	snapshot.Version = 2
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
	if store.RecordStatus() != storecenter.RecordStatusDeleting {
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

type storeCreateFingerprint struct{ id, key, name, platform, region, external string }

func fingerprintFor(store *storecenter.Store) storeCreateFingerprint {
	return storeCreateFingerprint{id: store.ID(), key: store.CreateIdempotencyKey(), name: store.Name(), platform: string(store.Platform()), region: store.Region(), external: store.ExternalStoreID()}
}

type auditRepositoryFake struct {
	mu           sync.Mutex
	events       map[string]storecenter.AuditEvent
	recordErr    error
	failActions  map[storecenter.AuditAction]int
	recordCalls  int
	onRecord     func(storecenter.AuditEvent)
	beforeRecord func(storecenter.AuditEvent)
}

func newAuditRepositoryFake() *auditRepositoryFake {
	return &auditRepositoryFake{events: map[string]storecenter.AuditEvent{}}
}
func (f *auditRepositoryFake) Record(_ context.Context, event storecenter.AuditEvent) (storecenter.AuditEvent, bool, error) {
	if f.beforeRecord != nil {
		beforeRecord := f.beforeRecord
		f.beforeRecord = nil
		beforeRecord(event)
	}
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
		if existing.StoreID != event.StoreID || existing.Outcome != event.Outcome || existing.PayloadFingerprint != event.PayloadFingerprint || existing.PreviousState != event.PreviousState || existing.NewState != event.NewState || existing.FailureCode != event.FailureCode || existing.StoreVersion != event.StoreVersion || !sameStrings(existing.SafeFieldNames, event.SafeFieldNames) {
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
