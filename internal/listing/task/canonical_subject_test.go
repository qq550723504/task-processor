package task

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestActorValidation(t *testing.T) {
	valid := Actor{TenantID: "tenant-1", UserID: "user-1", Roles: []string{"listingkit_operator"}}
	if err := ValidateActor(valid); err != nil {
		t.Fatalf("ValidateActor(valid) error = %v", err)
	}

	tests := []struct {
		name  string
		actor Actor
	}{
		{name: "empty tenant", actor: Actor{UserID: valid.UserID, Roles: valid.Roles}},
		{name: "blank tenant", actor: Actor{TenantID: " ", UserID: valid.UserID, Roles: valid.Roles}},
		{name: "trimmed tenant", actor: Actor{TenantID: " tenant-1", UserID: valid.UserID, Roles: valid.Roles}},
		{name: "long tenant", actor: Actor{TenantID: strings.Repeat("t", MaxTenantIDBytes+1), UserID: valid.UserID, Roles: valid.Roles}},
		{name: "empty user", actor: Actor{TenantID: valid.TenantID, Roles: valid.Roles}},
		{name: "blank user", actor: Actor{TenantID: valid.TenantID, UserID: " ", Roles: valid.Roles}},
		{name: "trimmed user", actor: Actor{TenantID: valid.TenantID, UserID: "user-1 ", Roles: valid.Roles}},
		{name: "long user", actor: Actor{TenantID: valid.TenantID, UserID: strings.Repeat("u", MaxUserIDBytes+1), Roles: valid.Roles}},
		{name: "empty roles", actor: Actor{TenantID: valid.TenantID, UserID: valid.UserID}},
		{name: "blank role", actor: Actor{TenantID: valid.TenantID, UserID: valid.UserID, Roles: []string{" "}}},
		{name: "trimmed role", actor: Actor{TenantID: valid.TenantID, UserID: valid.UserID, Roles: []string{" role"}}},
		{name: "long role", actor: Actor{TenantID: valid.TenantID, UserID: valid.UserID, Roles: []string{strings.Repeat("r", MaxRoleBytes+1)}}},
		{name: "too many roles", actor: Actor{TenantID: valid.TenantID, UserID: valid.UserID, Roles: make([]string, MaxRoles+1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateActor(tt.actor); !errors.Is(err, ErrInvalidActor) {
				t.Fatalf("ValidateActor() error = %v, want %v", err, ErrInvalidActor)
			}
		})
	}
}

func TestCanonicalSubjectTaskIDValidation(t *testing.T) {
	for _, taskID := range []string{"", " ", " task-1", "task-1 ", strings.Repeat("t", MaxTaskIDBytes+1)} {
		if err := ValidateTaskID(taskID); !errors.Is(err, ErrInvalidTaskID) {
			t.Fatalf("ValidateTaskID(%q) error = %v, want %v", taskID, err, ErrInvalidTaskID)
		}
	}
	if err := ValidateTaskID(strings.Repeat("t", MaxTaskIDBytes)); err != nil {
		t.Fatalf("ValidateTaskID(max) error = %v", err)
	}
}

func TestCanReadCanonicalSubject(t *testing.T) {
	subject := CanonicalSubject{TaskID: "task-1", TenantID: "tenant-1", OwnerUserID: "owner-1", ProductKey: "product-1"}
	tests := []struct {
		name  string
		actor Actor
		want  bool
	}{
		{name: "owner", actor: Actor{TenantID: "tenant-1", UserID: "owner-1", Roles: []string{"listingkit_operator"}}, want: true},
		{name: "cross owner", actor: Actor{TenantID: "tenant-1", UserID: "owner-2", Roles: []string{"listingkit_operator"}}},
		{name: "configured admin same tenant", actor: Actor{TenantID: "tenant-1", UserID: "admin-1", Roles: []string{"configured-admin"}}, want: true},
		{name: "platform admin cross tenant", actor: Actor{TenantID: "tenant-2", UserID: "platform-1", Roles: []string{"platform_admin"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := tenantAdminCheckerStub{allowedRole: "configured-admin"}
			if got := CanReadCanonicalSubject(tt.actor, subject, checker); got != tt.want {
				t.Fatalf("CanReadCanonicalSubject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanonicalSubjectCloneDefensivelyCopiesSourceLineage(t *testing.T) {
	subject := CanonicalSubject{Source: &SourceLineage{Key: "source-key"}}
	clone := subject.Clone()
	clone.Source.Key = "changed"
	if subject.Source.Key != "source-key" {
		t.Fatalf("Clone() shared SourceLineage pointer")
	}
}

func TestCanonicalSubjectReaderContract(t *testing.T) {
	var _ CanonicalSubjectReader = canonicalSubjectReaderStub{}
	var _ TenantAdminChecker = tenantAdminCheckerStub{}
	for _, err := range []error{ErrInvalidActor, ErrInvalidTaskID, ErrCanonicalSubjectNotFound, ErrCanonicalSubjectNotReady, ErrCanonicalSubjectUnavailable} {
		if err == nil {
			t.Fatal("contract error must be non-nil")
		}
	}
}

type tenantAdminCheckerStub struct{ allowedRole string }

func (s tenantAdminCheckerStub) IsTenantAdmin(_ string, roles []string) bool {
	for _, role := range roles {
		if role == s.allowedRole {
			return true
		}
	}
	return false
}

type canonicalSubjectReaderStub struct{}

func (canonicalSubjectReaderStub) ReadCanonicalSubject(context.Context, Actor, string) (CanonicalSubject, error) {
	return CanonicalSubject{}, nil
}
