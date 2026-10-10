package operationscockpitobservations

import (
	"context"
	"task-processor/internal/authidentity"
	o "task-processor/internal/marketplace/shein/observations"
	c "task-processor/internal/operationscockpit"
	"testing"
	"time"
)

const storeID = "123e4567-e89b-42d3-a456-426614174000"

type savedRepo struct {
	o.Repository
	head  o.Sync
	query o.Query
}

func (r *savedRepo) Heads(context.Context, string, o.Kind, []string) ([]o.Sync, []o.Sync, error) {
	return []o.Sync{r.head}, []o.Sync{r.head}, nil
}
func (r *savedRepo) List(_ context.Context, _ string, q o.Query) (o.Result, error) {
	r.query = q
	return o.Result{Items: []o.Record{}, Summary: o.Summary{Exceptional: 37}}, nil
}

type savedDirectory struct{}

func (savedDirectory) ListStores(context.Context, o.Scope) ([]string, error) {
	return []string{storeID}, nil
}

type savedMerchant struct{ o.Merchant }

func (savedMerchant) Check(context.Context) error { return nil }

type savedAccess struct {
	o.Access
	denied bool
}

func (a savedAccess) Authorize(context.Context, o.Scope, o.Kind, bool) error {
	if a.denied {
		return o.ErrForbidden
	}
	return nil
}
func (savedAccess) Open(context.Context, o.Scope, string, o.Kind, bool, *o.Binding) (o.Merchant, error) {
	return savedMerchant{}, nil
}

func TestSavedOrdersUseFullAggregateAndOriginalWindowWithoutProviderCalls(t *testing.T) {
	scope := c.Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: scope.ActorID, TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, EffectiveMemberID: "member-a"})
	at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	window := o.Window{Start: at.AddDate(0, 0, -30), End: at}
	repo := &savedRepo{head: o.Sync{ID: "123e4567-e89b-42d3-a456-426614174001", StoreID: storeID, Kind: o.Orders, Status: "completed", Binding: o.Binding{ApplicationID: "app-a"}, Range: &window, ObservedAt: &at}}
	service := &o.Service{Repository: repo, Access: savedAccess{}, Directory: savedDirectory{}}
	adapter := Orders{Service: service}
	stores := []c.StoreReference{{ID: storeID, Platform: "shein", Status: "active"}}
	view, err := adapter.Read(ctx, scope, stores)
	if err != nil || view.Exceptional != 37 || repo.query.Limit != 1 || len(view.Sources) != 1 || view.Sources[0].Start == nil || !view.Sources[0].Start.Equal(window.Start) || !view.Sources[0].ObservedAt.Equal(at) {
		t.Fatalf("saved provenance or aggregate lost: %+v %v", view, err)
	}
	// The embedded provider methods are nil: any provider call would panic.
	service.Access = savedAccess{denied: true}
	view, err = adapter.Read(ctx, scope, stores)
	if err != nil || view.State != "permission_required" {
		t.Fatalf("unavailable orders blocked manual mode: %+v %v", view, err)
	}
	view, err = (Orders{}).Read(ctx, scope, stores)
	if err != nil || view.State != "unavailable" {
		t.Fatal("unmounted optional observation capability changed manual outcome")
	}
}
