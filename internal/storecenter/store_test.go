package storecenter_test

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/storecenter"
)

const (
	testStoreID        = "123e4567-e89b-12d3-a456-426614174000"
	testIdempotencyKey = "123e4567-e89b-12d3-a456-426614174001"
	testAllocationID   = "123e4567-e89b-12d3-a456-426614174002"
)

var (
	testCreatedAt = time.Date(2026, time.August, 31, 8, 30, 0, 0, time.UTC)
	testUpdatedAt = time.Date(2026, time.August, 31, 8, 31, 0, 0, time.UTC)
)

// Mutation caught: omitting persisted aggregate fields or initializing them
// from a wall clock would break Task 2 persistence and deterministic creation.
func TestStoreCreateInitializesPersistedAggregateState(t *testing.T) {
	store := newTestStore(t)

	assertEqual(t, "ID", store.ID(), testStoreID)
	assertEqual(t, "OrganizationID", store.OrganizationID(), "org_Exact-Value")
	assertEqual(t, "Name", store.Name(), "North Shop")
	assertEqual(t, "Platform", store.Platform(), storecenter.PlatformShein)
	assertEqual(t, "Region", store.Region(), "Singapore")
	assertEqual(t, "ExternalStoreID", store.ExternalStoreID(), "external-42")
	assertEqual(t, "ConnectionRef", store.ConnectionRef(), "")
	assertEqual(t, "QuotaAllocationID", store.QuotaAllocationID(), testAllocationID)
	assertEqual(t, "LifecycleStatus", store.LifecycleStatus(), storecenter.StoreStatusProvisioning)
	assertEqual(t, "Version", store.Version(), int64(1))
	assertEqual(t, "CreatedBy", store.CreatedBy(), "subject_Exact-Value")
	assertEqual(t, "UpdatedBy", store.UpdatedBy(), "subject_Exact-Value")
	assertEqual(t, "CreatedAt", store.CreatedAt(), testCreatedAt)
	assertEqual(t, "UpdatedAt", store.UpdatedAt(), testCreatedAt)
	if got := store.DeletedAt(); got != nil {
		t.Fatalf("DeletedAt() = %v, want nil", *got)
	}
}

// Mutation caught: removing Unicode-safe surrounding-whitespace trimming
// would store user-facing values with leading or trailing whitespace.
func TestStoreCreateNormalizesUserFacingValues(t *testing.T) {
	input := validCreateStoreInput()
	input.Name = "\u2002 North Shop \u2002"
	input.Platform = " SHEIN "
	input.Region = "\u2002 Singapore \u2002"
	input.ExternalStoreID = "\u2002 ext-42 \u2002"

	store, err := storecenter.NewStore(input)
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	assertEqual(t, "Name", store.Name(), "North Shop")
	assertEqual(t, "Region", store.Region(), "Singapore")
	assertEqual(t, "ExternalStoreID", store.ExternalStoreID(), "ext-42")
	assertEqual(t, "Platform", store.Platform(), storecenter.PlatformShein)
	assertEqual(t, "OrganizationID", store.OrganizationID(), "org_Exact-Value")
	assertEqual(t, "CreatedBy", store.CreatedBy(), "subject_Exact-Value")
}

// Mutation caught: allowing empty, transformed opaque identity, malformed UUID,
// unsupported platform, zero creation time, or control-character data would admit invalid stores.
func TestStoreCreateRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*storecenter.CreateStoreInput)
	}{
		{"blank organization", func(in *storecenter.CreateStoreInput) { in.OrganizationID = "" }},
		{"surrounding organization whitespace", func(in *storecenter.CreateStoreInput) { in.OrganizationID = " org_Exact-Value" }},
		{"organization control character", func(in *storecenter.CreateStoreInput) { in.OrganizationID = "org\nvalue" }},
		{"organization invalid utf8", func(in *storecenter.CreateStoreInput) { in.OrganizationID = string([]byte{0xff}) }},
		{"blank actor", func(in *storecenter.CreateStoreInput) { in.ActorSubject = "" }},
		{"actor surrounding whitespace", func(in *storecenter.CreateStoreInput) { in.ActorSubject = " subject_Exact-Value" }},
		{"actor invalid utf8", func(in *storecenter.CreateStoreInput) { in.ActorSubject = string([]byte{0xff}) }},
		{"blank name", func(in *storecenter.CreateStoreInput) { in.Name = "\u2002\t" }},
		{"name control character", func(in *storecenter.CreateStoreInput) { in.Name = "North\nShop" }},
		{"name browser-trim discrepant character", func(in *storecenter.CreateStoreInput) { in.Name = "North Shop\ufeff" }},
		{"name invalid utf8", func(in *storecenter.CreateStoreInput) { in.Name = string([]byte{0xff}) }},
		{"blank platform", func(in *storecenter.CreateStoreInput) { in.Platform = " " }},
		{"unsupported platform", func(in *storecenter.CreateStoreInput) { in.Platform = "amazon" }},
		{"platform invalid utf8", func(in *storecenter.CreateStoreInput) { in.Platform = string([]byte{0xff}) }},
		{"blank region", func(in *storecenter.CreateStoreInput) { in.Region = "" }},
		{"region control character", func(in *storecenter.CreateStoreInput) { in.Region = "SG\r" }},
		{"region invalid utf8", func(in *storecenter.CreateStoreInput) { in.Region = string([]byte{0xff}) }},
		{"external store id control character", func(in *storecenter.CreateStoreInput) { in.ExternalStoreID = "ext\x00" }},
		{"external store id invalid utf8", func(in *storecenter.CreateStoreInput) { in.ExternalStoreID = string([]byte{0xff}) }},
		{"blank idempotency key", func(in *storecenter.CreateStoreInput) { in.CreateIdempotencyKey = "" }},
		{"blank quota allocation id", func(in *storecenter.CreateStoreInput) { in.QuotaAllocationID = "" }},
		{"malformed store id", func(in *storecenter.CreateStoreInput) { in.ID = "not-a-uuid" }},
		{"nil store id", func(in *storecenter.CreateStoreInput) { in.ID = "00000000-0000-0000-0000-000000000000" }},
		{"uppercase store id", func(in *storecenter.CreateStoreInput) { in.ID = strings.ToUpper(testStoreID) }},
		{"malformed idempotency key", func(in *storecenter.CreateStoreInput) { in.CreateIdempotencyKey = "not-a-uuid" }},
		{"nil idempotency key", func(in *storecenter.CreateStoreInput) {
			in.CreateIdempotencyKey = "00000000-0000-0000-0000-000000000000"
		}},
		{"uppercase idempotency key", func(in *storecenter.CreateStoreInput) { in.CreateIdempotencyKey = strings.ToUpper(testIdempotencyKey) }},
		{"malformed quota allocation id", func(in *storecenter.CreateStoreInput) { in.QuotaAllocationID = "not-a-uuid" }},
		{"nil quota allocation id", func(in *storecenter.CreateStoreInput) { in.QuotaAllocationID = "00000000-0000-0000-0000-000000000000" }},
		{"uppercase quota allocation id", func(in *storecenter.CreateStoreInput) { in.QuotaAllocationID = strings.ToUpper(testAllocationID) }},
		{"zero occurred at", func(in *storecenter.CreateStoreInput) { in.OccurredAt = time.Time{} }},
		{"name over code-point limit", func(in *storecenter.CreateStoreInput) {
			in.Name = strings.Repeat("界", storecenter.MaxStoreNameCodePoints+1)
		}},
		{"region over code-point limit", func(in *storecenter.CreateStoreInput) {
			in.Region = strings.Repeat("界", storecenter.MaxStoreRegionCodePoints+1)
		}},
		{"external store id over code-point limit", func(in *storecenter.CreateStoreInput) {
			in.ExternalStoreID = strings.Repeat("界", storecenter.MaxExternalStoreIDCodePoints+1)
		}},
		{"organization over byte limit", func(in *storecenter.CreateStoreInput) {
			in.OrganizationID = strings.Repeat("a", storecenter.MaxOrganizationIDBytes+1)
		}},
		{"actor over byte limit", func(in *storecenter.CreateStoreInput) {
			in.ActorSubject = strings.Repeat("a", storecenter.MaxSubjectBytes+1)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validCreateStoreInput()
			tt.mutate(&input)

			if _, err := storecenter.NewStore(input); err == nil {
				t.Fatal("NewStore() error = nil, want validation error")
			}
		})
	}
}

// Mutation caught: accepting an invalid lifecycle edge, treating a no-op as a
// write, or missing an audit/version update would corrupt lifecycle concurrency.
func TestLifecycleTransitionsEnforceEdgesAndVersions(t *testing.T) {
	tests := []struct {
		name        string
		start       storecenter.StoreStatus
		target      storecenter.StoreStatus
		wantErr     error
		wantStatus  storecenter.StoreStatus
		wantVersion int64
	}{
		{"provisioning activates", storecenter.StoreStatusProvisioning, storecenter.StoreStatusActive, nil, storecenter.StoreStatusActive, 2},
		{"active disables", storecenter.StoreStatusActive, storecenter.StoreStatusDisabled, nil, storecenter.StoreStatusDisabled, 3},
		{"disabled activates", storecenter.StoreStatusDisabled, storecenter.StoreStatusActive, nil, storecenter.StoreStatusActive, 4},
		{"active cannot bypass begin delete", storecenter.StoreStatusActive, storecenter.StoreStatusDeleting, storecenter.ErrInvalidTransition, storecenter.StoreStatusActive, 2},
		{"disabled cannot bypass begin delete", storecenter.StoreStatusDisabled, storecenter.StoreStatusDeleting, storecenter.ErrInvalidTransition, storecenter.StoreStatusDisabled, 3},
		{"provisioning cannot disable", storecenter.StoreStatusProvisioning, storecenter.StoreStatusDisabled, storecenter.ErrInvalidTransition, storecenter.StoreStatusProvisioning, 1},
		{"provisioning cannot delete", storecenter.StoreStatusProvisioning, storecenter.StoreStatusDeleting, storecenter.ErrInvalidTransition, storecenter.StoreStatusProvisioning, 1},
		{"active cannot activate", storecenter.StoreStatusActive, storecenter.StoreStatusActive, storecenter.ErrInvalidTransition, storecenter.StoreStatusActive, 2},
		{"deleting cannot activate", storecenter.StoreStatusDeleting, storecenter.StoreStatusActive, storecenter.ErrInvalidTransition, storecenter.StoreStatusDeleting, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newStoreAtStatus(t, tt.start)
			before := store.Snapshot()

			err := store.TransitionTo(tt.target, "subject_Update", testUpdatedAt)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("TransitionTo(%q) error = %v, want errors.Is(_, %v)", tt.target, err, tt.wantErr)
			}
			assertEqual(t, "LifecycleStatus", store.LifecycleStatus(), tt.wantStatus)
			assertEqual(t, "Version", store.Version(), tt.wantVersion)
			if tt.wantErr == nil {
				assertEqual(t, "UpdatedBy", store.UpdatedBy(), "subject_Update")
				assertEqual(t, "UpdatedAt", store.UpdatedAt(), testUpdatedAt)
			} else if got := store.Snapshot(); !reflect.DeepEqual(got, before) {
				t.Fatalf("invalid transition mutated Store: got %#v, want %#v", got, before)
			}
		})
	}
}

// Mutation caught: exposing mutable aggregate fields or leaking a mutable
// persistence snapshot into Store would permit identity changes outside its rules.
func TestStoreContractAggregateStateIsPrivateAndSnapshotIsDetached(t *testing.T) {
	aggregateType := reflect.TypeOf(storecenter.Store{})
	for i := 0; i < aggregateType.NumField(); i++ {
		field := aggregateType.Field(i)
		if field.IsExported() {
			t.Errorf("Store field %q is exported; aggregate state must be private", field.Name)
		}
	}

	store := newTestStore(t)
	snapshot := store.Snapshot()
	snapshot.ID = "123e4567-e89b-12d3-a456-426614174099"
	snapshot.OrganizationID = "other-org"
	snapshot.Platform = storecenter.Platform("other")

	assertEqual(t, "Store ID after snapshot mutation", store.ID(), testStoreID)
	assertEqual(t, "Store OrganizationID after snapshot mutation", store.OrganizationID(), "org_Exact-Value")
	assertEqual(t, "Store Platform after snapshot mutation", store.Platform(), storecenter.PlatformShein)
}

// Mutation caught: making rehydration accept an invalid persisted record would
// let corrupt rows bypass the same aggregate invariants as NewStore.
func TestStoreRehydrateRejectsInvalidPersistedState(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*storecenter.StoreSnapshot)
	}{
		{"nil quota allocation id", func(s *storecenter.StoreSnapshot) { s.QuotaAllocationID = "00000000-0000-0000-0000-000000000000" }},
		{"zero created at", func(s *storecenter.StoreSnapshot) { s.CreatedAt = time.Time{} }},
		{"updated before created", func(s *storecenter.StoreSnapshot) { s.UpdatedAt = testCreatedAt.Add(-time.Second) }},
		{"deleted active store", func(s *storecenter.StoreSnapshot) {
			now := testUpdatedAt
			s.LifecycleStatus = storecenter.StoreStatusActive
			s.DeletedAt = &now
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := newTestStore(t).Snapshot()
			tt.mutate(&snapshot)
			if _, err := storecenter.RehydrateStore(snapshot); err == nil {
				t.Fatal("RehydrateStore() error = nil, want invariant error")
			}
		})
	}
}

// Mutation caught: accepting lifecycle states at versions that cannot be
// reached from provisioning version 1 would admit impossible persisted rows.
func TestStoreRehydrateEnforcesLifecycleMinimumVersions(t *testing.T) {
	tests := []struct {
		name    string
		status  storecenter.LifecycleStatus
		version int64
		wantErr bool
	}{
		{"provisioning starts at one", storecenter.StoreStatusProvisioning, 1, false},
		{"provisioning permits later edits", storecenter.StoreStatusProvisioning, 8, false},
		{"active rejects creation version", storecenter.StoreStatusActive, 1, true},
		{"active permits first transition", storecenter.StoreStatusActive, 2, false},
		{"active permits later edits", storecenter.StoreStatusActive, 8, false},
		{"disabled rejects version two", storecenter.StoreStatusDisabled, 2, true},
		{"disabled permits first disable", storecenter.StoreStatusDisabled, 3, false},
		{"disabled permits later edits", storecenter.StoreStatusDisabled, 9, false},
		{"deleting rejects version two", storecenter.StoreStatusDeleting, 2, true},
		{"deleting permits direct active delete", storecenter.StoreStatusDeleting, 3, false},
		{"deleting permits later edits", storecenter.StoreStatusDeleting, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := newTestStore(t).Snapshot()
			snapshot.LifecycleStatus = tt.status
			snapshot.Version = tt.version
			if tt.status == storecenter.StoreStatusDeleting {
				snapshot.DeleteOperationKey = uuid.NewString()
			}

			_, err := storecenter.RehydrateStore(snapshot)
			if (err != nil) != tt.wantErr {
				t.Fatalf("RehydrateStore() error = %v, want error = %t", err, tt.wantErr)
			}
		})
	}
}

// Mutation caught: changing identity fields during a lifecycle write would let
// a state transition re-scope or re-platform a durable store.
func TestLifecycleTransitionPreservesImmutableIdentity(t *testing.T) {
	store := newTestStore(t)
	wantID, wantOrganizationID, wantPlatform := store.ID(), store.OrganizationID(), store.Platform()

	if err := store.TransitionTo(storecenter.StoreStatusActive, "subject_Update", testUpdatedAt); err != nil {
		t.Fatalf("TransitionTo(active) error = %v", err)
	}

	if store.ID() != wantID || store.OrganizationID() != wantOrganizationID || store.Platform() != wantPlatform {
		t.Fatalf("identity changed after transition: got (%q, %q, %q), want (%q, %q, %q)", store.ID(), store.OrganizationID(), store.Platform(), wantID, wantOrganizationID, wantPlatform)
	}
}

// Mutation caught: adding a credential-shaped Go field or JSON tag alias to a
// domain-visible Store type would make secret disclosure possible on serialization.
func TestStoreContractHasNoJSONVisibleCredentialFields(t *testing.T) {
	forbidden := []string{"password", "token", "cookie", "secret", "credential", "username"}
	for _, typeOfValue := range []reflect.Type{
		reflect.TypeOf(storecenter.Store{}),
		reflect.TypeOf(storecenter.StoreSnapshot{}),
	} {
		for i := 0; i < typeOfValue.NumField(); i++ {
			field := typeOfValue.Field(i)
			jsonName, visible := jsonFieldName(field)
			if !visible {
				continue
			}
			for _, candidate := range []string{field.Name, jsonName} {
				for _, word := range forbidden {
					if strings.Contains(strings.ToLower(candidate), word) {
						t.Errorf("%s field %q has credential-shaped JSON-visible name %q", typeOfValue.Name(), field.Name, jsonName)
					}
				}
			}
		}
	}
}

// Mutation caught: adding an unreviewed Repository method could reintroduce a
// UUID-only data boundary even if the current five methods remain scoped.
func TestStoreContractRepositoryHasOnlyOrganizationScopedMethods(t *testing.T) {
	repositoryType := reflect.TypeOf((*storecenter.Repository)(nil)).Elem()
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	stringType := reflect.TypeOf("")
	wantMethods := []string{"CreateOrReplay", "Get", "List", "Save", "SoftDelete"}

	gotMethods := make([]string, 0, repositoryType.NumMethod())
	for i := 0; i < repositoryType.NumMethod(); i++ {
		method := repositoryType.Method(i)
		gotMethods = append(gotMethods, method.Name)
		if method.Type.NumIn() < 3 {
			t.Errorf("Repository.%s has %d inputs, want context plus Organization ID", method.Name, method.Type.NumIn())
			continue
		}
		if got := method.Type.In(0); got != contextType {
			t.Errorf("Repository.%s first input = %v, want context.Context", method.Name, got)
		}
		if got := method.Type.In(1); got != stringType {
			t.Errorf("Repository.%s second input = %v, want Organization ID string", method.Name, got)
		}
	}
	sort.Strings(gotMethods)
	if !reflect.DeepEqual(gotMethods, wantMethods) {
		t.Errorf("Repository methods = %v, want exactly %v", gotMethods, wantMethods)
	}
}

func newTestStore(t *testing.T) *storecenter.Store {
	t.Helper()
	store, err := storecenter.NewStore(validCreateStoreInput())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return store
}

func TestStoreEditBasicOwnsNormalizedMutableFields(t *testing.T) {
	store := newTestStore(t)
	if err := store.TransitionTo(storecenter.StoreStatusActive, "creator", store.UpdatedAt().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	before := store.Snapshot()
	changed, err := store.EditBasic("  Updated Shop  ", "  MY  ", "editor", store.UpdatedAt().Add(time.Second))
	if err != nil {
		t.Fatalf("EditBasic() error = %v", err)
	}
	if !changed || store.Name() != "Updated Shop" || store.Region() != "MY" || store.Version() != before.Version+1 {
		t.Fatalf("EditBasic() = changed %v snapshot %+v", changed, store.Snapshot())
	}
	after := store.Snapshot()
	if after.OrganizationID != before.OrganizationID || after.Platform != before.Platform || after.ExternalStoreID != before.ExternalStoreID || after.QuotaAllocationID != before.QuotaAllocationID || after.CreateIdempotencyKey != before.CreateIdempotencyKey || after.ConnectionRef != before.ConnectionRef || after.CreatedBy != before.CreatedBy || !after.CreatedAt.Equal(before.CreatedAt) {
		t.Fatalf("EditBasic() changed immutable identity: before=%+v after=%+v", before, after)
	}
	changed, err = store.EditBasic("Updated Shop", "MY", "editor", store.UpdatedAt())
	if err != nil || changed || store.Version() != after.Version {
		t.Fatalf("no-op EditBasic() = %v, %v version %d", changed, err, store.Version())
	}
}

func TestStoreEditBasicRejectsTransitionalStates(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.EditBasic("Name", "SG", "editor", store.UpdatedAt()); !errors.Is(err, storecenter.ErrInvalidTransition) {
		t.Fatalf("provisioning EditBasic() error = %v", err)
	}
	if err := store.TransitionTo(storecenter.StoreStatusActive, "creator", store.UpdatedAt().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := store.BeginDelete(uuid.NewString(), "deleter", store.UpdatedAt().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.EditBasic("Name", "SG", "editor", store.UpdatedAt()); !errors.Is(err, storecenter.ErrInvalidTransition) {
		t.Fatalf("deleting EditBasic() error = %v", err)
	}
}

func TestStoreBeginDeleteBindsCanonicalKeyOnce(t *testing.T) {
	store := newTestStore(t)
	if err := store.TransitionTo(storecenter.StoreStatusActive, "creator", store.UpdatedAt().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	beforeVersion := store.Version()
	if err := store.BeginDelete(key, "deleter", store.UpdatedAt().Add(time.Second)); err != nil {
		t.Fatalf("BeginDelete() error = %v", err)
	}
	if store.LifecycleStatus() != storecenter.StoreStatusDeleting || store.DeleteOperationKey() != key || store.Version() != beforeVersion+1 {
		t.Fatalf("BeginDelete() snapshot = %+v", store.Snapshot())
	}
	if err := store.BeginDelete(key, "other-actor", store.UpdatedAt()); err != nil || store.Version() != beforeVersion+1 {
		t.Fatalf("same-key replay BeginDelete() = %v version %d", err, store.Version())
	}
	if err := store.BeginDelete(uuid.NewString(), "deleter", store.UpdatedAt()); !errors.Is(err, storecenter.ErrInvalidTransition) {
		t.Fatalf("different-key BeginDelete() error = %v", err)
	}
}

func TestStoreDeleteKeyRehydrationInvariant(t *testing.T) {
	active := newTestStore(t).Snapshot()
	active.DeleteOperationKey = uuid.NewString()
	if _, err := storecenter.RehydrateStore(active); err == nil {
		t.Fatal("active Store with delete key rehydrated")
	}
	deleting := newTestStore(t)
	if err := deleting.TransitionTo(storecenter.StoreStatusActive, "creator", deleting.UpdatedAt().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	deletingSnapshot := deleting.Snapshot()
	deletingSnapshot.LifecycleStatus = storecenter.StoreStatusDeleting
	deletingSnapshot.Version++
	deletingSnapshot.UpdatedAt = deletingSnapshot.UpdatedAt.Add(time.Second)
	if _, err := storecenter.RehydrateStore(deletingSnapshot); err == nil {
		t.Fatal("deleting Store without delete key rehydrated")
	}
}

func newStoreAtStatus(t *testing.T, status storecenter.StoreStatus) *storecenter.Store {
	t.Helper()
	snapshot := newTestStore(t).Snapshot()
	snapshot.LifecycleStatus = status
	snapshot.Version = minimumVersionForStatus(status)
	if status == storecenter.StoreStatusDeleting {
		snapshot.DeleteOperationKey = uuid.NewString()
	}
	store, err := storecenter.RehydrateStore(snapshot)
	if err != nil {
		t.Fatalf("RehydrateStore() error = %v", err)
	}
	return store
}

func minimumVersionForStatus(status storecenter.LifecycleStatus) int64 {
	switch status {
	case storecenter.StoreStatusProvisioning:
		return 1
	case storecenter.StoreStatusActive:
		return 2
	case storecenter.StoreStatusDisabled, storecenter.StoreStatusDeleting:
		return 3
	default:
		return 1
	}
}

func validCreateStoreInput() storecenter.CreateStoreInput {
	return storecenter.CreateStoreInput{
		ID:                   testStoreID,
		OrganizationID:       "org_Exact-Value",
		ActorSubject:         "subject_Exact-Value",
		Name:                 "North Shop",
		Platform:             "shein",
		Region:               "Singapore",
		ExternalStoreID:      "external-42",
		CreateIdempotencyKey: testIdempotencyKey,
		QuotaAllocationID:    testAllocationID,
		OccurredAt:           testCreatedAt,
	}
}

func jsonFieldName(field reflect.StructField) (string, bool) {
	if !field.IsExported() {
		return "", false
	}
	tag := field.Tag.Get("json")
	name, _, _ := strings.Cut(tag, ",")
	if name == "-" {
		return "", false
	}
	if name == "" {
		return field.Name, true
	}
	return name, true
}

func assertEqual[T comparable](t *testing.T, label string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}
