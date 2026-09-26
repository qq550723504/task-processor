package membership

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

type pendingStore struct {
	memoryReceipts
	list func(context.Context, OperationScope, PendingPageRequest) (PendingPage, error)
}

func (s *pendingStore) ListPending(ctx context.Context, scope OperationScope, page PendingPageRequest) (PendingPage, error) {
	return s.list(ctx, scope, page)
}

func TestPendingListAuthorityAndUntrustedResults(t *testing.T) {
	for _, scenario := range []string{"valid", "viewer", "revoked during read", "expired during read", "actor changed during read", "cancel during read", "foreign actor", "foreign org", "foreign project", "foreign envelope", "terminal", "oversize", "duplicate", "cursor", "invalid limit"} {
		t.Run(scenario, func(t *testing.T) {
			a, _ := authz.NewListingKitAuthorizer(nil, nil)
			directory := &directoryStub{}
			writer := &writerStub{}
			reads := 0
			revoked := false
			ctx, cancel := context.WithCancel(scopedContext("listingkit_admin"))
			defer cancel()
			store := &pendingStore{}
			store.list = func(ctx context.Context, scope OperationScope, page PendingPageRequest) (PendingPage, error) {
				reads++
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("missing read deadline")
				}
				if scope != (OperationScope{ProjectID: "project", OrganizationID: "effective-b", ActorID: "actor"}) {
					t.Fatal("wrong derived scope")
				}
				op := Operation{Scope: scope, Key: uuid.NewString(), Phase: PhaseDispatched}
				result := PendingPage{Scope: scope, Items: []Operation{op}}
				switch scenario {
				case "revoked during read":
					revoked = true
				case "cancel during read":
					cancel()
				case "foreign actor":
					result.Items[0].Scope.ActorID = "foreign"
				case "foreign org":
					result.Items[0].Scope.OrganizationID = "foreign"
				case "foreign project":
					result.Items[0].Scope.ProjectID = "foreign"
				case "foreign envelope":
					result.Scope.ActorID = "foreign"
				case "terminal":
					result.Items[0].Phase = PhaseCompleted
				case "oversize", "duplicate":
					result.Items = append(result.Items, op)
				case "cursor":
					result.Next = uuid.NewString()
				}
				return result, nil
			}
			commands := NewCommands(NewService(directory, a, "project"), store, writer, func(ctx context.Context) (context.Context, error) {
				if revoked {
					return nil, ErrPermission
				}
				if reads > 0 && (scenario == "expired during read" || scenario == "actor changed during read") {
					identity, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
					if scenario == "expired during read" {
						identity.TokenExpiresAt = time.Now().Add(-time.Second)
					} else {
						identity.UserID = "other-admin"
					}
					ctx = authidentity.WithAuthenticatedIdentity(ctx, identity)
				}
				return ctx, nil
			})
			page := PendingPageRequest{Limit: 1}
			if scenario == "duplicate" {
				page.Limit = 2
			}
			if scenario == "invalid limit" {
				page.Limit = 101
			}
			if scenario == "viewer" {
				ctx = scopedContext("listingkit_viewer")
			}
			result, err := commands.ListPending(ctx, page)
			if scenario == "valid" {
				if err != nil || len(result.Items) != 1 {
					t.Fatalf("valid read failed: %v", err)
				}
			} else if err == nil || len(result.Items) != 0 {
				t.Fatal("unsafe page returned")
			}
			if (scenario == "viewer" || scenario == "invalid limit") && reads != 0 {
				t.Fatal("invalid request reached store")
			}
			if writer.calls != 0 || directory.calls != 0 {
				t.Fatal("listing receipts accessed member provider")
			}
		})
	}
}
