package memberinvite

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestServiceInviteRejectsPlatformAdminRole(t *testing.T) {
	service := NewService(providerStub{})
	_, err := service.Invite(context.Background(), InviteRequest{
		TenantID: "org-1", GivenName: "Jane", FamilyName: "Doe",
		Email: "jane@example.com", Role: "platform_admin",
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("err = %v", err)
	}
}

func TestServiceInviteRejectsMalformedEmailWithoutCallingProvider(t *testing.T) {
	provider := providerStub{onInvite: func(InviteRequest) { t.Fatal("provider was called for an invalid request") }}
	service := NewService(provider)
	_, err := service.Invite(context.Background(), InviteRequest{
		TenantID: "org-1", GivenName: "Jane", FamilyName: "Doe",
		Email: "not-an-email", Role: "listingkit_viewer",
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("err = %v", err)
	}
}

func TestServiceInvitePreservesCreatedUserOnRoleFailure(t *testing.T) {
	service := NewService(providerStub{err: &IncompleteError{UserID: "user-1", Err: errors.New("grant denied")}})
	_, err := service.Invite(context.Background(), validInviteRequest())
	var incomplete *IncompleteError
	if !errors.As(err, &incomplete) || incomplete.UserID != "user-1" {
		t.Fatalf("err = %#v", err)
	}
}

func TestServiceInviteRedactsIncompleteProviderCauseFromErrorChain(t *testing.T) {
	service := NewService(providerStub{err: &IncompleteError{UserID: "user-1", Err: errors.New("provider-secret")}})
	_, err := service.Invite(context.Background(), validInviteRequest())
	if err == nil {
		t.Fatal("Invite returned nil error")
	}
	for current := err; current != nil; current = errors.Unwrap(current) {
		if strings.Contains(current.Error(), "provider-secret") {
			t.Fatalf("error leaked provider text: %v", current)
		}
	}
}

func TestServiceInviteNormalizesRequestBeforePassingItToProvider(t *testing.T) {
	var received InviteRequest
	provider := providerStub{
		invitation: Invitation{UserID: "user-1", AuthorizationID: "authorization-1"},
		onInvite:   func(request InviteRequest) { received = request },
	}
	service := NewService(provider)
	got, err := service.Invite(context.Background(), InviteRequest{
		TenantID: " org-1 ", GivenName: " Jane ", FamilyName: " Doe ",
		Email: " Jane@Example.com ", Role: " listingkit_viewer ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if received != (InviteRequest{TenantID: "org-1", GivenName: "Jane", FamilyName: "Doe", Email: "Jane@Example.com", Role: "listingkit_viewer"}) {
		t.Fatalf("provider request = %#v", received)
	}
	if got != (Invitation{TenantID: "org-1", UserID: "user-1", Email: "Jane@Example.com", Role: "listingkit_viewer", AuthorizationID: "authorization-1"}) {
		t.Fatalf("invitation = %#v", got)
	}
}

func TestInviteAcceptsEmailWithVerifiedPhoneDeliveryRequest(t *testing.T) {
	var received InviteRequest
	provider := providerStub{
		invitation: Invitation{UserID: "user-1", AuthorizationID: "authorization-1"},
		onInvite:   func(request InviteRequest) { received = request },
	}
	request := validInviteRequest()
	request.Phone = "+8613712345678"
	request.Username = "jane-phone"

	if _, err := NewService(provider).Invite(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if got := received.Phone; got != "+8613712345678" {
		t.Fatalf("provider phone = %q", got)
	}
	if got := received.Username; got != "jane-phone" {
		t.Fatalf("provider username = %q", got)
	}
}

func TestInviteRejectsPhoneWithoutRequiredEmail(t *testing.T) {
	request := validInviteRequest()
	request.Email = ""
	request.Phone = "+8613712345678"
	request.Username = "jane-phone"

	_, err := NewService(providerStub{}).Invite(context.Background(), request)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("err = %v", err)
	}
}

func TestInviteRejectsMalformedPhone(t *testing.T) {
	request := validInviteRequest()
	request.Phone = "13712345678"
	request.Username = "jane-phone"

	_, err := NewService(providerStub{}).Invite(context.Background(), request)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("err = %v", err)
	}
}

func TestInviteRejectsUnpairedPhoneFields(t *testing.T) {
	for name, request := range map[string]InviteRequest{
		"phone without username": {TenantID: "org-1", GivenName: "Jane", FamilyName: "Doe", Email: "jane@example.com", Phone: "+8613712345678", Role: "listingkit_viewer"},
		"username without phone": {TenantID: "org-1", GivenName: "Jane", FamilyName: "Doe", Email: "jane@example.com", Username: "jane-phone", Role: "listingkit_viewer"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := NewService(providerStub{}).Invite(context.Background(), request)
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

type providerStub struct {
	invitation Invitation
	err        error
	onInvite   func(InviteRequest)
}

func (p providerStub) Invite(_ context.Context, request InviteRequest) (Invitation, error) {
	if p.onInvite != nil {
		p.onInvite(request)
	}
	return p.invitation, p.err
}

func validInviteRequest() InviteRequest {
	return InviteRequest{TenantID: "org-1", GivenName: "Jane", FamilyName: "Doe", Email: "jane@example.com", Role: "listingkit_viewer"}
}
