// Package memberinvite creates least-privilege ListingKit member invitations.
package memberinvite

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
)

var (
	ErrInvalidRequest = errors.New("invalid ListingKit member invitation request")
	ErrConflict       = errors.New("ListingKit member invitation conflict")
)

type InviteRequest struct {
	TenantID   string
	GivenName  string
	FamilyName string
	Email      string
	Role       string
}

type Invitation struct {
	TenantID        string
	UserID          string
	Email           string
	Role            string
	AuthorizationID string
}

// Provider is authoritative for creating the ZITADEL identity and role assignment.
type Provider interface {
	Invite(context.Context, InviteRequest) (Invitation, error)
}

// IncompleteError reports that the identity exists but its project role was not assigned.
// Callers can use UserID to repair the role assignment without recreating the identity.
type IncompleteError struct {
	UserID string
	Err    error
}

func (e *IncompleteError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.UserID) == "" {
		return "ListingKit member invitation incomplete"
	}
	return fmt.Sprintf("ListingKit member invitation incomplete for user %s", e.UserID)
}

func (e *IncompleteError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type Service struct {
	provider Provider
}

func NewService(provider Provider) *Service {
	return &Service{provider: provider}
}

func (s *Service) Invite(ctx context.Context, request InviteRequest) (Invitation, error) {
	request, err := normalizeRequest(request)
	if err != nil {
		return Invitation{}, err
	}
	if s == nil || s.provider == nil {
		return Invitation{}, ErrInvalidRequest
	}

	invitation, err := s.provider.Invite(ctx, request)
	if err != nil {
		return Invitation{}, err
	}
	invitation.TenantID = request.TenantID
	invitation.Email = request.Email
	invitation.Role = request.Role
	return invitation, nil
}

func AllowedRole(role string) bool {
	switch strings.TrimSpace(role) {
	case "listingkit_viewer", "listingkit_operator", "listingkit_admin":
		return true
	default:
		return false
	}
}

func normalizeRequest(request InviteRequest) (InviteRequest, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.GivenName = strings.TrimSpace(request.GivenName)
	request.FamilyName = strings.TrimSpace(request.FamilyName)
	request.Email = strings.TrimSpace(request.Email)
	request.Role = strings.TrimSpace(request.Role)
	if request.TenantID == "" || request.GivenName == "" || request.FamilyName == "" || !AllowedRole(request.Role) {
		return InviteRequest{}, ErrInvalidRequest
	}
	address, err := mail.ParseAddress(request.Email)
	if err != nil || address.Address != request.Email {
		return InviteRequest{}, ErrInvalidRequest
	}
	return request, nil
}
