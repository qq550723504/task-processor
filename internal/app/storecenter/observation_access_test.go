package storecenterapp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	o "task-processor/internal/marketplace/shein/observations"
	"task-processor/internal/storecenter"
	"testing"
	"time"
)

type observationMaterialReader struct {
	material storecenter.ProductExecutionMaterial
	err      error
	subject  storecenter.ObservationSubject
}

func (r *observationMaterialReader) ReadObservation(_ context.Context, s storecenter.ObservationSubject, _ string, _ storecenter.ObservationAuthorizer, _ time.Time) (storecenter.ProductExecutionMaterial, error) {
	r.subject = s
	return r.material, r.err
}

type observationLiveAccess struct{}

func (observationLiveAccess) AuthorizeObservation(context.Context, storecenter.ObservationSubject) (storecenter.ObservationAuthorization, error) {
	return storecenter.ObservationAuthorization{}, nil
}

type observationProvider struct {
	storecenter.OfficialConnectionProvider
	OfficialObservationProvider
	calls int
	after func()
}

func (p *observationProvider) Application() storecenter.OfficialApplication {
	return storecenter.OfficialApplication{AppID: "app-a", Version: BoundOfficialRevision("v1", storecenter.ApplicationSelfOperated)}
}
func (p *observationProvider) QueryObservedProducts(context.Context, storecenter.OfficialMerchantCredential, int) (o.ProductPage, error) {
	p.calls++
	if p.after != nil {
		p.after()
	}
	return o.ProductPage{Items: []o.Product{{ID: "spu-a"}}}, nil
}
func observationHandleFixture(t *testing.T) (*OfficialObservationAccess, *observationMaterialReader, *observationProvider, storecenter.ObservationSubject) {
	t.Helper()
	p := &observationProvider{}
	apps, e := NewOfficialApplicationRegistry([]OfficialApplicationRegistration{{Provider: p, Protection: productSecretProtection{}, Type: storecenter.ApplicationSelfOperated}})
	require.NoError(t, e)
	r := &observationMaterialReader{material: storecenter.ProductExecutionMaterial{StoreVersion: 1, Platform: "shein", ServiceExpiresAt: time.Now().Add(time.Hour), Connection: storecenter.OfficialConnectionView{AttemptID: "connection-a", Version: 1, Status: "connected"}, Attempt: storecenter.OfficialConnectionAttempt{OrganizationID: "org-a", StoreID: "store-a", AttemptID: "connection-a", AppID: "app-a", AppVersion: p.Application().Version, State: "verified", KeyID: "key-a", Ciphertext: "encrypted-a"}}}
	a, e := NewOfficialObservationAccess(r, observationLiveAccess{}, apps)
	require.NoError(t, e)
	s := storecenter.ObservationSubject{OrganizationID: "org-a", ActorID: "actor-a", MemberID: "member-a", Purpose: storecenter.ObservationPurposeProducts, Sync: true}
	return a, r, p, s
}
func TestObservationAdmissionCannotBorrowPublishPurpose(t *testing.T) {
	a, _, p, s := observationHandleFixture(t)
	s.Purpose = storecenter.ProductPurposePublish
	_, e := a.Authorize(context.Background(), s, "store-a", nil)
	require.ErrorIs(t, e, o.ErrForbidden)
	require.Zero(t, p.calls)
}
func TestObservationHandleDropsResultsAfterRevocationAndPinsOriginalMember(t *testing.T) {
	a, r, p, s := observationHandleFixture(t)
	h, e := a.Authorize(context.Background(), s, "store-a", nil)
	require.NoError(t, e)
	p.after = func() { r.err = storecenter.ErrNotFound }
	// Interface assertion lets this test fail meaningfully before the handle is implemented.
	merchant, ok := any(h).(o.Merchant)
	require.True(t, ok)
	result, e := merchant.Products(context.Background(), 1)
	require.ErrorIs(t, e, o.ErrForbidden)
	require.Empty(t, result.Items)
	require.Equal(t, "member-a", r.subject.MemberID)
	b, e := json.Marshal(h)
	require.NoError(t, e)
	require.JSONEq(t, "{}", string(b))
	require.NotContains(t, string(b), "private")
	r.err = errors.New("IAM temporarily unavailable")
	p.after = nil
	_, e = a.Authorize(context.Background(), s, "store-a", nil)
	require.ErrorIs(t, e, o.ErrUnavailable)
}
func TestObservationHandleRejectsWrongKindAndChangedConnectionBeforeProvider(t *testing.T) {
	a, r, p, s := observationHandleFixture(t)
	h, e := a.Authorize(context.Background(), s, "store-a", nil)
	require.NoError(t, e)
	merchant, ok := any(h).(o.Merchant)
	require.True(t, ok)
	_, e = merchant.OrderDetails(context.Background(), []string{"order-a"})
	require.ErrorIs(t, e, o.ErrForbidden)
	require.Zero(t, p.calls)
	r.material.Connection.Version++
	_, e = merchant.Products(context.Background(), 1)
	require.ErrorIs(t, e, o.ErrConflict)
	require.Zero(t, p.calls)
}
func TestObservationLeaseExpiryIsTemporaryNotMembershipRevocation(t *testing.T) {
	a, _, p, s := observationHandleFixture(t)
	h, e := a.Authorize(context.Background(), s, "store-a", nil)
	require.NoError(t, e)
	a.now = func() time.Time { return h.expiresAt.Add(time.Second) }
	_, e = h.Products(context.Background(), 1)
	require.ErrorIs(t, e, o.ErrUnavailable)
	require.Zero(t, p.calls)
}
