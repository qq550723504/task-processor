package membership

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

type dispatchGateReceipts struct {
	memoryReceipts
	step        OperationStep
	afterCommit func()
}

func (s *dispatchGateReceipts) Apply(ctx context.Context, scope OperationScope, key string, revision int64, change OperationChange) (Operation, error) {
	op, err := s.memoryReceipts.Apply(ctx, scope, key, revision, change)
	if err == nil && change.Event == EventDispatch && op.Step == s.step {
		s.afterCommit()
	}
	return op, err
}

func TestDispatchCommitRechecksExpiryAndCancellationBeforeEveryProviderStep(t *testing.T) {
	for _, step := range []OperationStep{StepRole, StepRemove, StepUser, StepGrant} {
		for _, mode := range []string{"expiry", "cancel", "deadline"} {
			t.Run(string(step)+"/"+mode, func(t *testing.T) {
				a, _ := authz.NewListingKitAuthorizer(nil, nil)
				member := Member{ID: "grant", UserID: "member", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_viewer"}}
				s := NewService(&directoryStub{page: Page{Items: []Member{member}, Total: 1}}, a, "project")
				identity, _ := authidentity.AuthenticatedIdentityFromContext(scopedContext("listingkit_admin"))
				deadline := time.Now().Add(100 * time.Millisecond)
				if mode == "expiry" {
					identity.TokenExpiresAt = deadline
				}
				base := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
				ctx, cancel := context.WithCancel(base)
				if mode == "deadline" {
					cancel()
					ctx, cancel = context.WithDeadline(base, deadline)
				}
				defer cancel()
				store := &dispatchGateReceipts{step: step, afterCommit: func() {
					if mode != "cancel" {
						time.Sleep(time.Until(deadline) + time.Millisecond)
					} else {
						cancel()
					}
				}}
				writer := &inviteProvider{}
				refresh := func(ctx context.Context) (context.Context, error) { return ctx, nil }
				commands := NewCommands(s, store, writer, refresh)
				input := CommandInput{Kind: CommandRole, AuthorizationID: "grant", ExpectedVersion: observedVersion(member), Role: "listingkit_operator"}
				if step == StepRemove {
					input.Kind = CommandRemove
					input.Role = ""
				}
				if step == StepUser || step == StepGrant {
					input = CommandInput{Kind: CommandInvite, Role: "listingkit_viewer", Invitation: &Invitation{Email: "new@example.com", FirstName: "New", LastName: "Member"}}
				}
				key := uuid.NewString()
				_, _ = commands.Execute(ctx, key, input)
				wantUser := 0
				if step == StepGrant {
					wantUser = 1
				}
				if writer.userCalls != wantUser || writer.grantCalls != 0 {
					t.Fatalf("provider sends after dispatch gate: user=%d grant=%d", writer.userCalls, writer.grantCalls)
				}
				if store.op == nil || store.op.Phase != PhaseDispatched || store.op.Step != step {
					t.Fatalf("reservation not retained: %+v", store.op)
				}
				_, _ = NewCommands(s, store, writer, refresh).Execute(scopedContext("listingkit_admin"), key, input)
				if writer.userCalls != wantUser || writer.grantCalls != 0 {
					t.Fatal("rebuild resent dispatched operation")
				}
			})
		}
	}
}
