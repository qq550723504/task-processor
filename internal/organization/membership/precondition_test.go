package membership

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"task-processor/internal/authz"
)

func TestVerifiedTargetConflictHasDurableTerminalReceipt(t *testing.T) {
	for _, kind := range []CommandKind{CommandRole, CommandRemove} {
		t.Run(string(kind), func(t *testing.T) {
			a, _ := authz.NewListingKitAuthorizer(nil, nil)
			member := Member{ID: "grant", UserID: "member", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_viewer"}}
			s := NewService(&directoryStub{page: Page{Items: []Member{member}, Total: 1}}, a, "project")
			store := &memoryReceipts{}
			writer := &writerStub{}
			c := NewCommands(s, store, writer, func(ctx context.Context) (context.Context, error) { return ctx, nil })
			input := CommandInput{Kind: kind, AuthorizationID: "grant", ExpectedVersion: strings.Repeat("0", 64)}
			if kind == CommandRole {
				input.Role = "listingkit_operator"
			}
			key := uuid.NewString()
			op, err := c.Execute(scopedContext("listingkit_admin"), key, input)
			if !errors.Is(err, ErrConflict) || op.Phase != PhaseRejected || writer.calls != 0 {
				t.Fatalf("conflict=%+v %v sends=%d", op, err, writer.calls)
			}
			op, err = c.ReadOperation(scopedContext("listingkit_admin"), key)
			if err != nil || op.Phase != PhaseRejected {
				t.Fatalf("durable rejection=%+v %v", op, err)
			}
			op, err = c.Execute(scopedContext("listingkit_admin"), key, input)
			if err != nil || op.Phase != PhaseRejected || writer.calls != 0 {
				t.Fatalf("replay=%+v %v sends=%d", op, err, writer.calls)
			}
		})
	}
}

func TestVerifiedTargetDisappearingBeforeDispatchIsDurablyRejected(t *testing.T) {
	a, _ := authz.NewListingKitAuthorizer(nil, nil)
	member := Member{ID: "grant", UserID: "member", OrganizationID: "effective-b", ProjectID: "project", Roles: []string{"listingkit_viewer"}}
	d := &directoryStub{page: Page{Items: []Member{member}, Total: 1}}
	d.onRead = func() {
		if d.calls == 1 {
			d.page = Page{}
		}
	}
	store := &memoryReceipts{}
	writer := &writerStub{}
	c := NewCommands(NewService(d, a, "project"), store, writer, func(ctx context.Context) (context.Context, error) { return ctx, nil })
	op, err := c.Execute(scopedContext("listingkit_admin"), uuid.NewString(), CommandInput{Kind: CommandRemove, AuthorizationID: "grant", ExpectedVersion: observedVersion(member)})
	if !errors.Is(err, ErrConflict) || op.Phase != PhaseRejected || writer.calls != 0 {
		t.Fatalf("disappeared target=%+v %v sends=%d", op, err, writer.calls)
	}
}
