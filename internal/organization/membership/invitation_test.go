package membership

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/authz"
)

type inviteProvider struct {
	human                 *HumanIdentity
	userCalls, grantCalls int
	loseUser, loseGrant   bool
}

func (p *inviteProvider) ReadHuman(_ context.Context, organization, id string) (HumanIdentity, error) {
	if p.human == nil {
		return HumanIdentity{}, ErrNotFound
	}
	return *p.human, nil
}
func (p *inviteProvider) Write(_ context.Context, op Operation) (Acknowledgment, error) {
	if op.Step == StepUser {
		p.userCalls++
		p.human = &HumanIdentity{ID: op.TargetUserID, OrganizationID: op.Scope.OrganizationID, Username: op.Invitation.Email, Email: op.Invitation.Email, FirstName: op.Invitation.FirstName, LastName: op.Invitation.LastName}
		if p.loseUser {
			return Acknowledgment{}, errors.New("user response lost")
		}
		return Acknowledgment{ID: op.TargetUserID, At: time.Now().UTC().Format(time.RFC3339Nano)}, nil
	}
	p.grantCalls++
	if p.loseGrant {
		return Acknowledgment{}, errors.New("grant response lost")
	}
	return Acknowledgment{ID: "new-grant", At: time.Now().UTC().Format(time.RFC3339Nano)}, nil
}

func TestInviteLostUserResponseContinuesOnlyFixedVerifiedIdentity(t *testing.T) {
	a, _ := authz.NewListingKitAuthorizer(nil, nil)
	s := NewService(&directoryStub{}, a, "project")
	p := &inviteProvider{loseUser: true, loseGrant: true}
	store := &memoryReceipts{}
	c := NewCommands(s, store, p, func(ctx context.Context) (context.Context, error) { return ctx, nil })
	key := uuid.NewString()
	op, err := c.Execute(scopedContext("listingkit_admin"), key, CommandInput{Kind: CommandInvite, Role: "listingkit_viewer", Invitation: &Invitation{Email: " New@Example.com ", FirstName: "New", LastName: "Member"}})
	if err != nil || op.Phase != PhaseDispatched || p.userCalls != 1 || p.grantCalls != 0 {
		t.Fatalf("first=%+v %v", op, err)
	}
	fixed := op.TargetUserID
	// A mismatched identity is not a prerequisite for grant creation.
	p.human.OrganizationID = "foreign"
	unchanged, err := c.Verify(scopedContext("listingkit_admin"), key)
	if err != nil || unchanged.Phase != PhaseDispatched || p.grantCalls != 0 {
		t.Fatalf("mismatch=%+v %v", unchanged, err)
	}
	p.human.OrganizationID = "effective-b"
	continued, err := c.Verify(scopedContext("listingkit_admin"), key)
	if err != nil || continued.TargetUserID != fixed || continued.Step != StepGrant || continued.Phase != PhaseDispatched || continued.UserEvidence != "identity_verified" || p.userCalls != 1 || p.grantCalls != 1 {
		t.Fatalf("continued=%+v %v calls=%d/%d", continued, err, p.userCalls, p.grantCalls)
	}
	again, err := c.Verify(scopedContext("listingkit_admin"), key)
	if err != nil || again.Phase != PhaseDispatched || p.grantCalls != 1 {
		t.Fatalf("grant replay=%+v %v", again, err)
	}
}

func TestInviteRevocationPreventsPartialContinuation(t *testing.T) {
	a, _ := authz.NewListingKitAuthorizer(nil, nil)
	s := NewService(&directoryStub{}, a, "project")
	p := &inviteProvider{loseUser: true}
	store := &memoryReceipts{}
	revoked := false
	c := NewCommands(s, store, p, func(ctx context.Context) (context.Context, error) {
		if revoked {
			return nil, ErrPermission
		}
		return ctx, nil
	})
	key := uuid.NewString()
	_, err := c.Execute(scopedContext("listingkit_admin"), key, CommandInput{Kind: CommandInvite, Role: "listingkit_viewer", Invitation: &Invitation{Email: "new@example.com", FirstName: "New", LastName: "Member"}})
	if err != nil {
		t.Fatal(err)
	}
	revoked = true
	if _, err := c.Verify(scopedContext("listingkit_admin"), key); err != ErrPermission || p.grantCalls != 0 {
		t.Fatalf("revoked=%v grants=%d", err, p.grantCalls)
	}
}
