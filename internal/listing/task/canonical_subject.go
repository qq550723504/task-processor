package task

import (
	"context"
	"errors"
	"strings"
)

const (
	MaxTenantIDBytes = 64
	MaxUserIDBytes   = 128
	MaxRoleBytes     = 128
	MaxRoles         = 32
	MaxTaskIDBytes   = 128
)

var (
	ErrInvalidActor                = errors.New("invalid listing task actor")
	ErrInvalidTaskID               = errors.New("invalid listing task id")
	ErrCanonicalSubjectNotFound    = errors.New("canonical subject not found")
	ErrCanonicalSubjectNotReady    = errors.New("canonical subject not ready")
	ErrCanonicalSubjectUnavailable = errors.New("canonical subject repository unavailable")
)

// Actor is the trusted identity used to scope a task-owned resource lookup.
// Callers must populate it from an authenticated server-side identity.
type Actor struct {
	TenantID string
	UserID   string
	Roles    []string
}

// SourceLineage identifies the source from which a task's canonical product
// was derived. It deliberately excludes provider-specific payloads.
type SourceLineage struct {
	Key      string `json:"key,omitempty"`
	Type     string `json:"type,omitempty"`
	Platform string `json:"platform,omitempty"`
	ID       string `json:"id,omitempty"`
	URL      string `json:"url,omitempty"`
}

// CanonicalSubject is the narrow task-to-product binding required by
// canonical product readers.
type CanonicalSubject struct {
	TaskID          string
	TenantID        string
	OwnerUserID     string
	ProductKey      string
	SnapshotVersion uint64
	Source          *SourceLineage
}

func (s CanonicalSubject) Clone() CanonicalSubject {
	clone := s
	if s.Source != nil {
		source := *s.Source
		clone.Source = &source
	}
	return clone
}

type CanonicalSubjectReader interface {
	ReadCanonicalSubject(ctx context.Context, actor Actor, taskID string) (CanonicalSubject, error)
}

// TenantAdminChecker is the narrow authorization seam needed to decide
// whether an identity may bypass owner scope inside its authenticated tenant.
type TenantAdminChecker interface {
	IsTenantAdmin(userID string, roles []string) bool
}

func ValidateActor(actor Actor) error {
	if !canonicalString(actor.TenantID, MaxTenantIDBytes) ||
		!canonicalString(actor.UserID, MaxUserIDBytes) ||
		len(actor.Roles) == 0 || len(actor.Roles) > MaxRoles {
		return ErrInvalidActor
	}
	for _, role := range actor.Roles {
		if !canonicalString(role, MaxRoleBytes) {
			return ErrInvalidActor
		}
	}
	return nil
}

func ValidateTaskID(taskID string) error {
	if !canonicalString(taskID, MaxTaskIDBytes) {
		return ErrInvalidTaskID
	}
	return nil
}

// CanReadCanonicalSubject applies the task ownership invariant after storage
// filtering. Administrative roles may bypass owner scope only within their
// already authenticated tenant.
func CanReadCanonicalSubject(actor Actor, subject CanonicalSubject, tenantAdmins TenantAdminChecker) bool {
	if ValidateActor(actor) != nil ||
		actor.TenantID != subject.TenantID ||
		!canonicalString(subject.OwnerUserID, MaxUserIDBytes) {
		return false
	}
	if actor.UserID == subject.OwnerUserID {
		return true
	}
	return tenantAdmins != nil && tenantAdmins.IsTenantAdmin(actor.UserID, actor.Roles)
}

func canonicalString(value string, maxBytes int) bool {
	return value != "" && value == strings.TrimSpace(value) && len(value) <= maxBytes
}
