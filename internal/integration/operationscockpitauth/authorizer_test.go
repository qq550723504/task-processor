package operationscockpitauth

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/httproute"
	c "task-processor/internal/operationscockpit"
	"task-processor/internal/workbenchcontext"
	"testing"
	"time"
)

type resolverFixture struct {
	id    authidentity.AuthenticatedIdentity
	calls int
}

func (r *resolverFixture) Resolve(_ context.Context, policy httproute.OrganizationAccessPolicy, input workbenchcontext.ResolveInput) (authidentity.AuthenticatedIdentity, error) {
	if policy != httproute.OrganizationAccessPolicyLiveWrite || input.BearerToken != "fixture-token" {
		return r.id, errors.New("fresh boundary absent")
	}
	r.calls++
	return r.id, nil
}
func TestCockpitEveryReadUsesFreshOriginalMembership(t *testing.T) {
	original := authidentity.AuthenticatedIdentity{UserID: "actor-a", TenantID: "org-a", EffectiveOrganizationID: "org-a", EffectiveMemberID: "member-a", TokenExpiresAt: time.Now().Add(time.Hour), Roles: []string{"listingkit_admin"}}
	resolver := &resolverFixture{id: original}
	a, err := New(resolver, authz.DefaultListingKitAuthorizer(), &gorm.DB{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(authidentity.WithAuthenticatedIdentity(context.Background(), original), time.Minute)
	defer cancel()
	ctx, err = a.Bind(ctx, "Bearer fixture-token")
	if err != nil {
		t.Fatal(err)
	}
	scope := c.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	access, err := a.Current(ctx, scope)
	if err != nil || !access.GoalsManage {
		t.Fatalf("current manager denied: %+v %v", access, err)
	}
	resolver.id.Roles = []string{"listingkit_viewer"}
	access, err = a.Current(ctx, scope)
	if err != nil || access.GoalsManage || access.GoalsRead {
		t.Fatalf("stale JWT role retained: %+v %v", access, err)
	}
	resolver.id.EffectiveMemberID = "rejoined-member"
	if _, err = a.Current(ctx, scope); !errors.Is(err, c.ErrForbidden) {
		t.Fatalf("rejoined member inherited session: %v", err)
	}
	resolver.id = original
	resolver.id.EffectiveOrganizationID = "org-b"
	if _, err = a.Current(ctx, scope); !errors.Is(err, c.ErrForbidden) {
		t.Fatalf("cross-org response accepted: %v", err)
	}
	if resolver.calls != 4 {
		t.Fatalf("fresh calls %d", resolver.calls)
	}
	if _, err = a.Current(context.Background(), scope); !errors.Is(err, c.ErrForbidden) {
		t.Fatalf("unbound identity accepted: %v", err)
	}
}
