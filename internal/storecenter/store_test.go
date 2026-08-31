package storecenter

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const (
	testStoreID        = "123e4567-e89b-12d3-a456-426614174000"
	testIdempotencyKey = "123e4567-e89b-12d3-a456-426614174001"
)

// Mutation caught: removing Unicode-safe surrounding-whitespace trimming
// would store user-facing values with leading or trailing whitespace.
func TestStoreCreateNormalizesUserFacingValues(t *testing.T) {
	store, err := NewStore(CreateStoreInput{
		ID:                   testStoreID,
		OrganizationID:       "org_Exact-Value",
		CreatedBySubject:     "subject_Exact-Value",
		Name:                 "\u2002 North Shop \u2002",
		Platform:             " SHEIN ",
		Region:               "\u2002 Singapore \u2002",
		ExternalStoreID:      "\u2002 ext-42 \u2002",
		CreateIdempotencyKey: testIdempotencyKey,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}

	if got, want := store.Name, "North Shop"; got != want {
		t.Errorf("Name = %q, want %q", got, want)
	}
	if got, want := store.Region, "Singapore"; got != want {
		t.Errorf("Region = %q, want %q", got, want)
	}
	if got, want := store.ExternalStoreID, "ext-42"; got != want {
		t.Errorf("ExternalStoreID = %q, want %q", got, want)
	}
	if got, want := store.Platform, PlatformShein; got != want {
		t.Errorf("Platform = %q, want %q", got, want)
	}
	if got, want := store.OrganizationID, "org_Exact-Value"; got != want {
		t.Errorf("OrganizationID = %q, want exact %q", got, want)
	}
	if got, want := store.CreatedBySubject, "subject_Exact-Value"; got != want {
		t.Errorf("CreatedBySubject = %q, want exact %q", got, want)
	}
	if got, want := store.Status, StoreStatusProvisioning; got != want {
		t.Errorf("Status = %q, want %q", got, want)
	}
	if got, want := store.Version, int64(1); got != want {
		t.Errorf("Version = %d, want %d", got, want)
	}
}

// Mutation caught: allowing empty, transformed opaque identity, malformed UUID,
// unsupported platform, or control-character data would admit invalid stores.
func TestStoreCreateRejectsInvalidInput(t *testing.T) {
	valid := CreateStoreInput{
		ID:                   testStoreID,
		OrganizationID:       "org_Exact-Value",
		CreatedBySubject:     "subject_Exact-Value",
		Name:                 "North Shop",
		Platform:             "shein",
		Region:               "Singapore",
		ExternalStoreID:      "external-42",
		CreateIdempotencyKey: testIdempotencyKey,
	}

	tests := []struct {
		name   string
		mutate func(*CreateStoreInput)
	}{
		{"blank organization", func(in *CreateStoreInput) { in.OrganizationID = "" }},
		{"surrounding organization whitespace", func(in *CreateStoreInput) { in.OrganizationID = " org_Exact-Value" }},
		{"organization control character", func(in *CreateStoreInput) { in.OrganizationID = "org\nvalue" }},
		{"organization invalid utf8", func(in *CreateStoreInput) { in.OrganizationID = string([]byte{0xff}) }},
		{"blank subject", func(in *CreateStoreInput) { in.CreatedBySubject = "" }},
		{"subject surrounding whitespace", func(in *CreateStoreInput) { in.CreatedBySubject = " subject_Exact-Value" }},
		{"subject invalid utf8", func(in *CreateStoreInput) { in.CreatedBySubject = string([]byte{0xff}) }},
		{"blank name", func(in *CreateStoreInput) { in.Name = "\u2002\t" }},
		{"name control character", func(in *CreateStoreInput) { in.Name = "North\nShop" }},
		{"name invalid utf8", func(in *CreateStoreInput) { in.Name = string([]byte{0xff}) }},
		{"blank platform", func(in *CreateStoreInput) { in.Platform = " " }},
		{"unsupported platform", func(in *CreateStoreInput) { in.Platform = "amazon" }},
		{"platform invalid utf8", func(in *CreateStoreInput) { in.Platform = string([]byte{0xff}) }},
		{"blank region", func(in *CreateStoreInput) { in.Region = "" }},
		{"region control character", func(in *CreateStoreInput) { in.Region = "SG\r" }},
		{"region invalid utf8", func(in *CreateStoreInput) { in.Region = string([]byte{0xff}) }},
		{"external store id control character", func(in *CreateStoreInput) { in.ExternalStoreID = "ext\x00" }},
		{"external store id invalid utf8", func(in *CreateStoreInput) { in.ExternalStoreID = string([]byte{0xff}) }},
		{"blank idempotency key", func(in *CreateStoreInput) { in.CreateIdempotencyKey = "" }},
		{"malformed store id", func(in *CreateStoreInput) { in.ID = "not-a-uuid" }},
		{"nil store id", func(in *CreateStoreInput) { in.ID = "00000000-0000-0000-0000-000000000000" }},
		{"uppercase store id", func(in *CreateStoreInput) { in.ID = strings.ToUpper(testStoreID) }},
		{"malformed idempotency key", func(in *CreateStoreInput) { in.CreateIdempotencyKey = "not-a-uuid" }},
		{"nil idempotency key", func(in *CreateStoreInput) { in.CreateIdempotencyKey = "00000000-0000-0000-0000-000000000000" }},
		{"uppercase idempotency key", func(in *CreateStoreInput) { in.CreateIdempotencyKey = strings.ToUpper(testIdempotencyKey) }},
		{"name over code-point limit", func(in *CreateStoreInput) { in.Name = strings.Repeat("界", MaxStoreNameCodePoints+1) }},
		{"region over code-point limit", func(in *CreateStoreInput) { in.Region = strings.Repeat("界", MaxStoreRegionCodePoints+1) }},
		{"external store id over code-point limit", func(in *CreateStoreInput) { in.ExternalStoreID = strings.Repeat("界", MaxExternalStoreIDCodePoints+1) }},
		{"organization over byte limit", func(in *CreateStoreInput) { in.OrganizationID = strings.Repeat("a", MaxOrganizationIDBytes+1) }},
		{"subject over byte limit", func(in *CreateStoreInput) { in.CreatedBySubject = strings.Repeat("a", MaxSubjectBytes+1) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valid
			tt.mutate(&input)

			if _, err := NewStore(input); err == nil {
				t.Fatal("NewStore() error = nil, want validation error")
			}
		})
	}
}

// Mutation caught: accepting an invalid lifecycle edge, treating a no-op as a
// write, or missing a version increment would corrupt lifecycle concurrency.
func TestLifecycleTransitionsEnforceEdgesAndVersions(t *testing.T) {
	tests := []struct {
		name        string
		start       StoreStatus
		target      StoreStatus
		wantErr     error
		wantStatus  StoreStatus
		wantVersion int64
	}{
		{"provisioning activates", StoreStatusProvisioning, StoreStatusActive, nil, StoreStatusActive, 2},
		{"active disables", StoreStatusActive, StoreStatusDisabled, nil, StoreStatusDisabled, 2},
		{"disabled activates", StoreStatusDisabled, StoreStatusActive, nil, StoreStatusActive, 2},
		{"active deletes", StoreStatusActive, StoreStatusDeleting, nil, StoreStatusDeleting, 2},
		{"disabled deletes", StoreStatusDisabled, StoreStatusDeleting, nil, StoreStatusDeleting, 2},
		{"provisioning cannot disable", StoreStatusProvisioning, StoreStatusDisabled, ErrInvalidTransition, StoreStatusProvisioning, 1},
		{"provisioning cannot delete", StoreStatusProvisioning, StoreStatusDeleting, ErrInvalidTransition, StoreStatusProvisioning, 1},
		{"active cannot activate", StoreStatusActive, StoreStatusActive, ErrInvalidTransition, StoreStatusActive, 1},
		{"deleting cannot activate", StoreStatusDeleting, StoreStatusActive, ErrInvalidTransition, StoreStatusDeleting, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newTestStore(t)
			store.Status = tt.start

			err := store.TransitionTo(tt.target)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("TransitionTo(%q) error = %v, want errors.Is(_, %v)", tt.target, err, tt.wantErr)
			}
			if got := store.Status; got != tt.wantStatus {
				t.Errorf("Status = %q, want %q", got, tt.wantStatus)
			}
			if got := store.Version; got != tt.wantVersion {
				t.Errorf("Version = %d, want %d", got, tt.wantVersion)
			}
		})
	}
}

// Mutation caught: changing Store identity fields during a lifecycle write
// would let a state transition re-scope or re-platform a durable store.
func TestLifecycleTransitionPreservesImmutableIdentity(t *testing.T) {
	store := newTestStore(t)
	wantID, wantOrganizationID, wantPlatform := store.ID, store.OrganizationID, store.Platform

	if err := store.TransitionTo(StoreStatusActive); err != nil {
		t.Fatalf("TransitionTo(active) error = %v", err)
	}

	if store.ID != wantID || store.OrganizationID != wantOrganizationID || store.Platform != wantPlatform {
		t.Fatalf("identity changed after transition: got (%q, %q, %q), want (%q, %q, %q)", store.ID, store.OrganizationID, store.Platform, wantID, wantOrganizationID, wantPlatform)
	}
}

// Mutation caught: exposing a credential-shaped JSON field on the domain type
// would make secret disclosure possible once Store is serialized by a caller.
func TestStoreContractHasNoJSONVisibleCredentialFields(t *testing.T) {
	forbidden := []string{"password", "token", "cookie", "secret", "credential", "username"}
	typeOfStore := reflect.TypeOf(Store{})
	for i := 0; i < typeOfStore.NumField(); i++ {
		field := typeOfStore.Field(i)
		if !field.IsExported() || field.Tag.Get("json") == "-" {
			continue
		}
		name := strings.ToLower(field.Name)
		for _, word := range forbidden {
			if strings.Contains(name, word) {
				t.Errorf("Store field %q is JSON-visible and credential-shaped", field.Name)
			}
		}
	}
}

// Mutation caught: dropping Organization ID from a repository operation would
// make an accidental UUID-only cross-Organization read or write possible.
func TestStoreContractRepositoryIsOrganizationScoped(t *testing.T) {
	repositoryType := reflect.TypeOf((*Repository)(nil)).Elem()
	contextType := reflect.TypeOf((*context.Context)(nil)).Elem()
	stringType := reflect.TypeOf("")

	methods := []struct {
		name      string
		minInputs int
	}{
		{"CreateOrReplay", 3},
		{"List", 3},
		{"Get", 3},
		{"Save", 4},
		{"SoftDelete", 4},
	}
	for _, tt := range methods {
		method, ok := repositoryType.MethodByName(tt.name)
		if !ok {
			t.Errorf("Repository is missing %s", tt.name)
			continue
		}
		if method.Type.NumIn() < tt.minInputs {
			t.Errorf("Repository.%s has %d inputs, want at least %d", tt.name, method.Type.NumIn(), tt.minInputs)
			continue
		}
		if got := method.Type.In(0); got != contextType {
			t.Errorf("Repository.%s first input = %v, want context.Context", tt.name, got)
		}
		if got := method.Type.In(1); got != stringType {
			t.Errorf("Repository.%s second input = %v, want Organization ID string", tt.name, got)
		}
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(CreateStoreInput{
		ID:                   testStoreID,
		OrganizationID:       "org_Exact-Value",
		CreatedBySubject:     "subject_Exact-Value",
		Name:                 "North Shop",
		Platform:             "shein",
		Region:               "Singapore",
		ExternalStoreID:      "external-42",
		CreateIdempotencyKey: testIdempotencyKey,
	})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	return store
}
