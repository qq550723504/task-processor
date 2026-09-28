package membership

import (
	"context"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"time"
)

type Counts struct{ Total, Active, Administrators, Inactive int }
type SummaryReader interface {
	Counts(context.Context, string) (Counts, error)
}

type Summary struct {
	SchemaVersion  string    `json:"schemaVersion"`
	UserID         string    `json:"userId"`
	OrganizationID string    `json:"organizationId"`
	Total          int       `json:"total"`
	Active         int       `json:"active"`
	Administrators int       `json:"administrators"`
	Inactive       int       `json:"inactive"`
	Source         string    `json:"source"`
	ReadAt         time.Time `json:"readAt"`
}

func (s *Service) Summary(ctx context.Context) (Summary, error) {
	identity, err := s.authorize(ctx, authz.PermissionWorkbenchOrganizationMemberRead)
	if err != nil {
		return Summary{}, err
	}
	reader, ok := s.directory.(SummaryReader)
	if !ok {
		return Summary{}, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	counts, err := reader.Counts(ctx, identity.EffectiveOrganizationID)
	if err != nil || ctx.Err() != nil {
		return Summary{}, ErrUnavailable
	}
	if !time.Now().Before(identity.TokenExpiresAt) {
		return Summary{}, ErrAuthentication
	}
	for _, count := range []int{counts.Total, counts.Active, counts.Administrators, counts.Inactive} {
		if count < 0 || count > 10000 {
			return Summary{}, ErrInvalidResponse
		}
	}
	return Summary{SchemaVersion: "membership-summary-v1", UserID: identity.UserID, OrganizationID: identity.EffectiveOrganizationID, Total: counts.Total, Active: counts.Active, Administrators: counts.Administrators, Inactive: counts.Inactive, Source: "zitadel_authorization_v2", ReadAt: time.Now().UTC()}, nil
}

// InvitationManager consumes the same live identity and protected-role rules as
// existing membership commands; recipient acceptance has separate admission.
func (s *Service) InvitationManager(ctx context.Context, role string) (authidentity.AuthenticatedIdentity, error) {
	identity, err := s.authorize(ctx, authz.PermissionWorkbenchOrganizationMemberManage)
	if err != nil {
		return identity, err
	}
	if role != "" && !s.assignable(role) {
		return identity, ErrPermission
	}
	return identity, nil
}
func (s *Service) AssignableInvitationRole(role string) bool { return s != nil && s.assignable(role) }
func (s *Service) InvitationReader(ctx context.Context) (authidentity.AuthenticatedIdentity, error) {
	return s.authorize(ctx, authz.PermissionWorkbenchOrganizationMemberRead)
}
