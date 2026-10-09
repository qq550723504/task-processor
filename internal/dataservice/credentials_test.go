package dataservice

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/product/collection"
)

type credentialMemory struct {
	key     Credential
	hash    string
	command string
}

func (m *credentialMemory) Creation(_ context.Context, scope collection.Scope, command string) (Credential, error) {
	if command != m.command || scope.OrganizationID != m.key.Scope.OrganizationID || scope.ActorID != m.key.Scope.ActorID {
		return Credential{}, ErrNotFound
	}
	return m.key, nil
}

func (m *credentialMemory) Create(_ context.Context, k Credential, command, hash string) (Credential, bool, error) {
	if m.command == command {
		if m.hash != hash {
			return Credential{}, false, ErrConflict
		}
		return m.key, true, nil
	}
	m.key = k
	m.command = command
	m.hash = hash
	return k, false, nil
}
func (m *credentialMemory) Read(_ context.Context, id string) (Credential, error) {
	if m.key.ID != id {
		return Credential{}, ErrNotFound
	}
	return m.key, nil
}
func (m *credentialMemory) List(context.Context, collection.Scope) ([]Credential, error) {
	return []Credential{m.key}, nil
}
func (m *credentialMemory) History(context.Context, collection.Scope, string, int) (CredentialHistoryPage, error) {
	return CredentialHistoryPage{Items: []Credential{m.key}}, nil
}
func (m *credentialMemory) Change(_ context.Context, s collection.Scope, id, command, hash string, revision int64, patch KeyPatch) (Credential, error) {
	if s != m.key.Scope || revision != m.key.Revision {
		return Credential{}, ErrConflict
	}
	m.key.State = patch.State
	m.key.Revision++
	return m.key, nil
}

type credentialAccess struct {
	allowed bool
	scope   collection.Scope
}

func (a *credentialAccess) Resolve(context.Context, string) (collection.Scope, error) {
	if !a.allowed {
		return collection.Scope{}, ErrForbidden
	}
	return a.scope, nil
}
func (a *credentialAccess) Check(_ context.Context, s collection.Scope, _ string) error {
	if !a.allowed || s != a.scope {
		return ErrForbidden
	}
	return nil
}

func TestSecretIsOnlyReturnedOnceAndRevocationIsLive(t *testing.T) {
	ctx := context.Background()
	scope := collection.Scope{OrganizationID: "org", ActorID: "actor", MemberID: "grant-original"}
	store := &credentialMemory{}
	access := &credentialAccess{true, scope}
	service, err := NewCredentialService(store, access)
	if err != nil {
		t.Fatal(err)
	}
	input := KeyInput{Name: "client", ExpiresAt: time.Now().Add(time.Hour).UTC(), DailyRows: 20, MonthlyCostFen: 100, Permissions: []string{PermissionAcquire, PermissionResult}}
	command := uuid.NewString()
	created, err := service.Create(ctx, command, input)
	if err != nil || created.Secret == "" {
		t.Fatalf("%#v %v", created, err)
	}
	replayed, err := service.Create(ctx, command, input)
	if err != nil || replayed.Secret != "" || !replayed.Replayed {
		t.Fatalf("secret replayed: %#v %v", replayed, err)
	}
	metadata, err := service.Creation(ctx, command)
	if err != nil || metadata.ID != created.Key.ID {
		t.Fatalf("creation lookup: %#v %v", metadata, err)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil || strings.Contains(string(encoded), created.Secret) || strings.Contains(string(encoded), metadata.Digest) {
		t.Fatal("creation metadata exposed a credential")
	}
	principal, err := service.Authenticate(ctx, "DataKey "+created.Key.ID+"."+created.Secret, "127.0.0.1", PermissionAcquire)
	if err != nil || principal.Scope != scope {
		t.Fatalf("%#v %v", principal, err)
	}
	access.scope.MemberID = "grant-recreated"
	if _, err = service.Authenticate(ctx, "DataKey "+created.Key.ID+"."+created.Secret, "127.0.0.1", PermissionAcquire); !errors.Is(err, ErrForbidden) {
		t.Fatalf("recreated grant authorized: %v", err)
	}
	access.scope = scope
	access.allowed = false
	if _, err = service.Authenticate(ctx, "DataKey "+created.Key.ID+"."+created.Secret, "127.0.0.1", PermissionAcquire); !errors.Is(err, ErrForbidden) {
		t.Fatalf("revoked access authorized: %v", err)
	}
}

func TestCredentialRejectsPrivilegeAndCIDRExpansion(t *testing.T) {
	now := time.Now()
	valid := KeyInput{Name: "key", ExpiresAt: now.Add(time.Hour), DailyRows: 1, MonthlyCostFen: 5, Permissions: []string{PermissionResult}}
	for _, mutate := range []func(*KeyInput){func(v *KeyInput) { v.Permissions = []string{"admin"} }, func(v *KeyInput) { v.CIDRs = []string{"*"} }, func(v *KeyInput) { v.MonthlyCostFen = 0 }, func(v *KeyInput) { v.ExpiresAt = now.Add(366 * 24 * time.Hour) }} {
		input := valid
		mutate(&input)
		if _, err := NormalizeKeyInput(input, now); err == nil {
			t.Fatalf("unsafe input accepted: %#v", input)
		}
	}
}
