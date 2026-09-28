package inviteflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/google/uuid"
	"net/mail"
	"strings"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"time"
)

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func (s *Service) principal(ctx context.Context) (authidentity.AuthenticatedIdentity, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !authidentity.IsBoundedIdentifier(identity.UserID) || !s.now().Before(identity.TokenExpiresAt) {
		return identity, ErrPermission
	}
	if ctx.Err() != nil {
		return identity, ErrUnavailable
	}
	return identity, nil
}
func (s *Service) visible(inv Invitation) Invitation {
	if inv.State == Pending && !s.now().Before(inv.ExpiresAt) {
		inv.State = Expired
	}
	return inv
}
func (s *Service) Create(ctx context.Context, key, contact, role string) (Invitation, error) {
	if s == nil || s.Store == nil || s.Manager == nil || s.RoleAllowed == nil {
		return Invitation{}, ErrUnavailable
	}
	id, err := uuid.Parse(key)
	if err != nil || id == uuid.Nil || id.String() != key {
		return Invitation{}, ErrInvalid
	}
	contact = strings.ToLower(strings.TrimSpace(contact))
	address, err := mail.ParseAddress(contact)
	if err != nil || address.Address != contact || len(contact) > 200 || strings.HasSuffix(contact, "@phone.invalid") || !s.RoleAllowed(role) {
		return Invitation{}, ErrInvalid
	}
	identity, err := s.Manager(ctx, role)
	if err != nil {
		return Invitation{}, ErrPermission
	}
	raw, _ := json.Marshal([]string{contact, role})
	digest := sha256.Sum256(raw)
	fingerprint := hex.EncodeToString(digest[:])
	existing, readErr := s.Store.Read(ctx, key)
	if readErr == nil {
		if existing.ProjectID != s.ProjectID || existing.OrganizationID != identity.EffectiveOrganizationID || existing.CreatorID != identity.UserID || existing.Fingerprint != fingerprint {
			return Invitation{}, ErrConflict
		}
		return s.visible(existing), nil
	}
	if readErr != ErrNotFound {
		return Invitation{}, ErrUnavailable
	}
	if s.Notify == nil {
		return Invitation{}, ErrUnavailable
	}
	now := s.now()
	name := ""
	for _, grant := range identity.OrganizationGrants {
		if grant.ProjectID == s.ProjectID && grant.OrganizationID == identity.EffectiveOrganizationID {
			name = grant.OrganizationName
		}
	}
	inv := Invitation{ID: key, ProjectID: s.ProjectID, OrganizationID: identity.EffectiveOrganizationID, OrganizationName: name, CreatorID: identity.UserID, Contact: contact, Role: role, Fingerprint: fingerprint, State: Pending, Revision: 1, CreatedAt: now, ExpiresAt: now.Add(7 * 24 * time.Hour), DeliveryState: "not_attempted"}
	saved, replay, err := s.Store.Create(ctx, inv)
	if err != nil {
		return Invitation{}, err
	}
	if replay {
		return s.visible(saved), nil
	}
	return s.deliver(ctx, saved)
}
func (s *Service) List(ctx context.Context, limit, offset int) (Page, error) {
	if s == nil || s.Store == nil || s.Manager == nil || limit < 1 || limit > 100 || offset < 0 || offset > 10000 {
		return Page{}, ErrInvalid
	}
	identity, err := s.Manager(ctx, "")
	if err != nil {
		return Page{}, ErrPermission
	}
	page, err := s.Store.List(ctx, identity.EffectiveOrganizationID, limit, offset)
	if err != nil {
		return Page{}, err
	}
	for i, inv := range page.Items {
		if inv.ProjectID != s.ProjectID || inv.OrganizationID != identity.EffectiveOrganizationID {
			return Page{}, ErrUnavailable
		}
		page.Items[i] = s.visible(inv)
	}
	return page, nil
}
func (s *Service) PendingCount(ctx context.Context) (int, error) {
	if s == nil || s.Store == nil || s.Reader == nil {
		return 0, ErrUnavailable
	}
	identity, err := s.Reader(ctx)
	if err != nil {
		return 0, ErrPermission
	}
	page, err := s.Store.List(ctx, identity.EffectiveOrganizationID, 1, 0)
	return page.Pending, err
}
func (s *Service) ReadAdmin(ctx context.Context, id string) (Invitation, error) {
	identity, err := s.Manager(ctx, "")
	if err != nil {
		return Invitation{}, ErrPermission
	}
	inv, err := s.Store.Read(ctx, id)
	if err != nil {
		return Invitation{}, err
	}
	if inv.ProjectID != s.ProjectID || inv.OrganizationID != identity.EffectiveOrganizationID {
		return Invitation{}, ErrNotFound
	}
	return s.reconcile(ctx, s.visible(inv))
}
func (s *Service) Cancel(ctx context.Context, id string) (Invitation, error) {
	inv, err := s.ReadAdmin(ctx, id)
	if err != nil {
		return Invitation{}, err
	}
	if inv.State == Cancelled {
		return inv, nil
	}
	return s.Store.Change(ctx, id, inv.Revision, Change{State: Cancelled, Now: s.now()})
}
func (s *Service) recipient(ctx context.Context, inv Invitation) (authidentity.AuthenticatedIdentity, error) {
	identity, err := s.principal(ctx)
	if err != nil {
		return identity, err
	}
	if inv.ProjectID != s.ProjectID || s.ReadSelf == nil {
		return identity, ErrPermission
	}
	self, err := s.ReadSelf(ctx)
	if err != nil {
		return identity, ErrUnavailable
	}
	if self.UserID != identity.UserID || self.Email == nil || self.EmailVerified == nil || !*self.EmailVerified || strings.ToLower(strings.TrimSpace(*self.Email)) != inv.Contact || inv.RecipientID != "" && inv.RecipientID != identity.UserID {
		return identity, ErrPermission
	}
	return identity, nil
}
func (s *Service) ReadRecipient(ctx context.Context, id string) (Invitation, error) {
	inv, err := s.Store.Read(ctx, id)
	if err != nil {
		return Invitation{}, err
	}
	if _, err = s.recipient(ctx, inv); err != nil {
		return Invitation{}, err
	}
	return s.reconcile(ctx, s.visible(inv))
}
func (s *Service) Decline(ctx context.Context, id string) (Invitation, error) {
	inv, err := s.Store.Read(ctx, id)
	if err != nil {
		return Invitation{}, err
	}
	identity, err := s.recipient(ctx, inv)
	if err != nil {
		return Invitation{}, err
	}
	if inv.State == Declined && inv.RecipientID == identity.UserID {
		return inv, nil
	}
	return s.Store.Change(ctx, id, inv.Revision, Change{State: Declined, RecipientID: identity.UserID, Now: s.now()})
}
func (s *Service) creatorAllowed(ctx context.Context, inv Invitation) bool {
	if s.ReadGrant == nil || s.Authorize == nil || s.RoleAllowed == nil || !s.RoleAllowed(inv.Role) {
		return false
	}
	grant, err := s.ReadGrant(ctx, inv.OrganizationID, inv.CreatorID)
	return err == nil && grant.Found && grant.State == "active" && s.Authorize(inv.CreatorID, grant.Roles, authz.PermissionWorkbenchOrganizationMemberManage)
}
func (s *Service) Accept(ctx context.Context, id string) (Invitation, error) {
	inv, err := s.Store.Read(ctx, id)
	if err != nil {
		return Invitation{}, err
	}
	identity, err := s.recipient(ctx, inv)
	if err != nil {
		return Invitation{}, err
	}
	if inv.State == Accepting || inv.State == Accepted {
		return s.reconcile(ctx, inv)
	}
	if inv.State != Pending || !s.now().Before(inv.ExpiresAt) {
		return s.visible(inv), ErrConflict
	}
	if s.ReadGrant == nil || s.WriteGrant == nil || !s.creatorAllowed(ctx, inv) {
		return Invitation{}, ErrPermission
	}
	existing, err := s.ReadGrant(ctx, inv.OrganizationID, identity.UserID)
	if err != nil {
		return Invitation{}, ErrUnavailable
	}
	if existing.Found {
		return inv, ErrConflict
	}
	claimed, err := s.Store.Change(ctx, id, inv.Revision, Change{State: Accepting, RecipientID: identity.UserID, DispatchID: uuid.NewString(), Now: s.now()})
	if err != nil {
		current, readErr := s.Store.Read(ctx, id)
		if readErr == nil && current.RecipientID == identity.UserID && (current.State == Accepting || current.State == Accepted) {
			return s.reconcile(ctx, current)
		}
		return Invitation{}, err
	}
	// Only the committed claim winner may write. UNKNOWN never regains dispatch.
	if _, err = s.recipient(ctx, claimed); err != nil || !s.creatorAllowed(ctx, claimed) || ctx.Err() != nil {
		return claimed, nil
	}
	if deadline, ok := ctx.Deadline(); ok && !s.now().Before(deadline) {
		return claimed, nil
	}
	_ = s.WriteGrant(ctx, claimed)
	return s.reconcile(ctx, claimed)
}
func (s *Service) reconcile(ctx context.Context, inv Invitation) (Invitation, error) {
	if inv.State != Accepting || s.ReadGrant == nil {
		return inv, nil
	}
	grant, err := s.ReadGrant(ctx, inv.OrganizationID, inv.RecipientID)
	if err != nil || !grant.Found || grant.State != "active" || len(grant.Roles) != 1 || grant.Roles[0] != inv.Role || !authidentity.IsBoundedIdentifier(grant.ID) {
		return inv, nil
	}
	saved, err := s.Store.Change(ctx, inv.ID, inv.Revision, Change{State: Accepted, RecipientID: inv.RecipientID, AuthorizationID: grant.ID, Now: s.now()})
	if err != nil {
		return inv, nil
	}
	return saved, nil
}
func (s *Service) Resend(ctx context.Context, id string) (Invitation, error) {
	inv, err := s.ReadAdmin(ctx, id)
	if err != nil {
		return Invitation{}, err
	}
	return s.deliver(ctx, inv)
}
func (s *Service) deliver(ctx context.Context, inv Invitation) (Invitation, error) {
	if s.Notify == nil {
		return inv, ErrUnavailable
	}
	identity, err := s.Manager(ctx, "")
	if err != nil || identity.EffectiveOrganizationID != inv.OrganizationID {
		return inv, ErrPermission
	}
	attempt := uuid.NewString()
	claimed, err := s.Store.ClaimDelivery(ctx, inv.ID, attempt, s.now())
	if err != nil {
		return s.visible(inv), err
	}
	status := "mail_server_accepted"
	if err = s.Notify(ctx, claimed); err != nil {
		status = "delivery_unknown"
	}
	saved, err := s.Store.FinishDelivery(ctx, inv.ID, attempt, status, s.now())
	if err != nil {
		return claimed, nil
	}
	return s.visible(saved), nil
}
