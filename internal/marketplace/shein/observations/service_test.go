package observations

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type serviceRepo struct {
	Repository
	sync          Sync
	record        Record
	commits       int
	stops         int
	heads, latest []Sync
	rows          []Record
}

func (r *serviceRepo) Heads(context.Context, string, Kind, []string) ([]Sync, []Sync, error) {
	return r.heads, r.latest, nil
}
func (r *serviceRepo) List(context.Context, string, Query) (Result, error) {
	return Result{Items: []Record{}}, nil
}

func (r *serviceRepo) ReadSync(context.Context, string, string) (Sync, error) { return r.sync, nil }
func (r *serviceRepo) CommitPage(_ context.Context, s Sync, c Checkpoint, rows []Record, status string, at time.Time) (Sync, error) {
	r.commits++
	r.rows = rows
	s.Progress = c
	s.Status = status
	return s, nil
}
func (r *serviceRepo) Stop(_ context.Context, s Sync, status, code string) (Sync, error) {
	r.stops++
	s.Status = status
	s.ErrorCode = code
	return s, nil
}
func (r *serviceRepo) Record(context.Context, string, string, Kind, string, string) (Record, error) {
	return r.record, nil
}

type serviceAccess struct {
	Access
	merchant *serviceMerchant
	err      error
	subject  Scope
}

func (a *serviceAccess) Authorize(context.Context, Scope, Kind, bool) error { return a.err }
func (a *serviceAccess) Open(_ context.Context, s Scope, _ string, _ Kind, _ bool, expected *Binding) (Merchant, error) {
	a.subject = s
	if a.err != nil {
		return nil, a.err
	}
	if expected != nil && *expected != a.merchant.binding {
		return nil, ErrNotFound
	}
	return a.merchant, nil
}

type serviceMerchant struct {
	Merchant
	binding     Binding
	products    ProductPage
	orders      OrderPage
	details     []Order
	detailErr   error
	afterDetail func()
	err         error
	after       func()
	trackCalls  int
}

func (m *serviceMerchant) Binding() Binding            { return m.binding }
func (m *serviceMerchant) Check(context.Context) error { return m.err }
func (m *serviceMerchant) Products(context.Context, int) (ProductPage, error) {
	if m.after != nil {
		m.after()
	}
	return m.products, nil
}
func (m *serviceMerchant) Orders(context.Context, Window, int) (OrderPage, error) {
	return m.orders, nil
}
func (m *serviceMerchant) OrderDetails(context.Context, []string) ([]Order, error) {
	if m.afterDetail != nil {
		m.afterDetail()
	}
	return m.details, m.detailErr
}
func (m *serviceMerchant) Track(context.Context, string, string) ([]Track, error) {
	m.trackCalls++
	return []Track{}, nil
}
func serviceFixture() (Service, *serviceRepo, *serviceAccess, *serviceMerchant) {
	id := uuid.NewString()
	store := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Second)
	binding := Binding{OrganizationID: "org-a", StoreID: store}
	r := &serviceRepo{sync: Sync{ID: id, StoreID: store, Owner: Scope{"org-a", "actor-a", "original-member"}, Kind: Products, Binding: binding, Status: "pending", Revision: 1, CreatedAt: now, Progress: Checkpoint{Page: 1}}}
	m := &serviceMerchant{binding: binding, products: ProductPage{Total: 1, Items: []Product{{ID: "product-a", SKCs: []SKC{}}}}}
	a := &serviceAccess{merchant: m}
	s := Service{Repository: r, Access: a, Now: func() time.Time { return now }}
	return s, r, a, m
}
func TestSyncDropsLateRevokedPageAndUsesPersistedOriginalMember(t *testing.T) {
	s, r, a, m := serviceFixture()
	m.after = func() { m.err = ErrForbidden }
	_, e := s.Step(context.Background(), "org-a", r.sync.ID)
	require.NoError(t, e)
	require.Zero(t, r.commits)
	require.Equal(t, 1, r.stops)
	require.Equal(t, "original-member", a.subject.MemberID)
}
func TestOrderDetailsWrongMembershipNeverAdvanceAndIAMOutageNeverBecomesRevocation(t *testing.T) {
	s, r, a, m := serviceFixture()
	r.sync.Kind = Orders
	r.sync.Progress.Windows = []Window{{r.sync.CreatedAt.Add(-time.Hour), r.sync.CreatedAt}}
	m.orders = OrderPage{Items: []OrderRef{{ID: "order-a"}}}
	m.details = []Order{{ID: "another-order", Site: "shein-us"}}
	_, e := s.Step(context.Background(), "org-a", r.sync.ID)
	require.ErrorIs(t, e, ErrUnavailable)
	require.Zero(t, r.commits)
	a.err = ErrUnavailable
	_, e = s.Step(context.Background(), "org-a", r.sync.ID)
	require.ErrorIs(t, e, ErrUnavailable)
	require.Zero(t, r.stops)
}
func TestLogisticsCannotLookupForeignPackageOrOrder(t *testing.T) {
	s, r, _, m := serviceFixture()
	r.sync.Kind = Orders
	r.record = Record{ID: "order-a", StoreID: r.sync.StoreID, SyncID: r.sync.ID, Order: &Order{ID: "order-a", Site: "shein-us"}}
	m.details = []Order{{ID: "order-a", Site: "shein-us", Packages: []Package{{ID: "package-a"}}}}
	_, e := s.Logistics(context.Background(), r.sync.Owner, r.sync.StoreID, r.sync.ID, "order-a", "foreign-package")
	require.ErrorIs(t, e, ErrNotFound)
	require.Zero(t, m.trackCalls)
	_, e = s.Logistics(context.Background(), r.sync.Owner, r.sync.StoreID, r.sync.ID, "order-a", "package-a")
	require.NoError(t, e)
	require.Equal(t, 1, m.trackCalls)
}

type serviceDirectory struct{ store string }

func (d serviceDirectory) ListStores(context.Context, Scope) ([]string, error) {
	return []string{d.store}, nil
}
func TestEnsureExpiredExecutionEndsAsPartialWithoutRestart(t *testing.T) {
	s, r, _, _ := serviceFixture()
	s.Directory = serviceDirectory{r.sync.StoreID}
	r.sync.CreatedAt = r.sync.CreatedAt.Add(-25 * time.Hour)
	result, e := s.Ensure(context.Background(), r.sync.Owner, r.sync.ID)
	require.NoError(t, e)
	require.Equal(t, "partial", result.Status)
	require.Contains(t, result.Progress.Notes, "duration_limit")
	require.Equal(t, 1, r.commits)
}
func TestListExcludesLatestMetadataFromReplacedConnection(t *testing.T) {
	s, r, _, m := serviceFixture()
	s.Directory = serviceDirectory{r.sync.StoreID}
	m.binding.ApplicationID = "current-application"
	current := r.sync
	current.Binding = m.binding
	current.Status = "completed"
	r.heads = []Sync{current}
	stale := r.sync
	stale.Binding.ApplicationID = "old-application"
	stale.Progress.Seen = 123
	r.latest = []Sync{stale}
	result, e := s.List(context.Background(), r.sync.Owner, Query{Kind: Products, Limit: 20})
	require.NoError(t, e)
	require.Len(t, result.Syncs, 1)
	require.Empty(t, result.Latest)
	r.latest = []Sync{current}
	result, e = s.List(context.Background(), r.sync.Owner, Query{Kind: Products, Limit: 20})
	require.NoError(t, e)
	require.Equal(t, []Sync{current}, result.Latest)
}

func TestOrderSyncKeepsListStatusOnlyWhenDetailOmitsIt(t *testing.T) {
	for _, present := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing-detail-status", true: "current-detail-status"}[present], func(t *testing.T) {
			s, r, _, m := serviceFixture()
			r.sync.Kind = Orders
			r.sync.Progress.Windows = []Window{{r.sync.CreatedAt.Add(-time.Hour), r.sync.CreatedAt}}
			count := 1
			m.orders = OrderPage{ReportedCount: &count, Items: []OrderRef{{ID: "order-a", Status: 4, CreatedAt: r.sync.CreatedAt, UpdatedAt: r.sync.CreatedAt}}}
			m.details = []Order{{ID: "order-a", Site: "shein-us"}}
			want := 4
			if present {
				want = 7
				m.details[0].Status = &want
			}
			_, err := s.Step(context.Background(), "org-a", r.sync.ID)
			require.NoError(t, err)
			require.Len(t, r.rows, 1)
			require.NotNil(t, r.rows[0].Order.Status)
			require.Equal(t, want, *r.rows[0].Order.Status)
		})
	}
}

func TestOrderDetailProviderOutageReturnsSavedStaleOnlyAfterLiveAccessRecheck(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		providerErr, checkErr, openErr error
		want                           error
	}{
		{"provider-outage", ErrUnavailable, nil, nil, nil},
		{"late-revocation", ErrUnavailable, ErrForbidden, nil, ErrForbidden},
		{"late-IAM-outage", ErrUnavailable, ErrUnavailable, nil, ErrUnavailable},
		{"initial-IAM-outage", ErrUnavailable, nil, ErrUnavailable, ErrUnavailable},
		{"provider-forbidden", ErrForbidden, nil, nil, ErrForbidden},
		{"provider-not-found", ErrNotFound, nil, nil, ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, a, m := serviceFixture()
			r.sync.Kind = Orders
			observed := r.sync.CreatedAt.Add(-time.Hour)
			r.record = Record{StoreID: r.sync.StoreID, SyncID: r.sync.ID, ID: "order-a", ObservedAt: observed, Order: &Order{ID: "order-a", Site: "shein-us", Items: []OrderItem{{ID: "saved-item"}}, Packages: []Package{{ID: "saved-package"}}}}
			m.detailErr = tc.providerErr
			m.afterDetail = func() { m.err = tc.checkErr }
			a.err = tc.openErr
			result, err := s.Detail(context.Background(), r.sync.Owner, r.sync.StoreID, r.sync.ID, "order-a", Orders)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
				require.Empty(t, result.ID)
				return
			}
			require.NoError(t, err)
			require.Equal(t, observed, result.ObservedAt)
			require.Equal(t, r.record.Order, result.Order)
			var wire map[string]any
			raw, err := json.Marshal(result)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, &wire))
			require.Equal(t, true, wire["stale"])
			_, err = s.Logistics(context.Background(), r.sync.Owner, r.sync.StoreID, r.sync.ID, "order-a", "saved-package")
			require.ErrorIs(t, err, ErrUnavailable, "saved packages cannot authorize a current Track request")
			require.Zero(t, m.trackCalls)
		})
	}
}
