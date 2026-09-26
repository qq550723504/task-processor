package productsourcing

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authz"
	"task-processor/internal/product/sourcing"
)

// browserProviderStub is a deterministic PublicAcquirer: it never touches the
// network and counts invocations so replay/coordination can be asserted.
type browserProviderStub struct {
	calls    atomic.Int32
	evidence sourcing.AcquisitionEvidence
	delay    time.Duration
}

func (p *browserProviderStub) Acquire(ctx context.Context, _ sourcing.AcquisitionSource) (sourcing.AcquisitionEvidence, error) {
	p.calls.Add(1)
	if p.delay > 0 {
		select {
		case <-time.After(p.delay):
		case <-ctx.Done():
			return sourcing.AcquisitionEvidence{}, ctx.Err()
		}
	}
	return p.evidence, nil
}

func browserPostgresEvidence() sourcing.AcquisitionEvidence {
	title, amount, currency, sku := "Browser PG bottle", "12.50", "CNY", "sku-pg"
	return sourcing.AcquisitionEvidence{
		SchemaVersion: 1, OfferID: "981645030344", SourceURL: "https://detail.1688.com/offer/981645030344.html",
		Title: &title, ContentSHA256: repeated('e', 64), ParserVersion: "1688-browser-dom/v1",
		CapturedAt: time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC),
		Attributes: []sourcing.AcquisitionAttribute{{Name: "material", Value: "steel"}},
		Variants:   []sourcing.AcquisitionVariant{{SourceID: &sku, Price: &sourcing.AcquisitionPrice{Amount: amount, Currency: &currency}}},
		Images:     []sourcing.AcquisitionImage{{URL: "https://cbu01.alicdn.com/pg.jpg", Role: "primary"}},
	}
}

func newBrowserPostgresService(t *testing.T, provider sourcing.PublicAcquirer) *BrowserAcquisitionService {
	t.Helper()
	db := acquisitionDatabase(t)
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	live := liveRolesFunc(func(context.Context, string, string) ([]string, error) {
		return []string{"listingkit_operator"}, nil
	})
	require.NoError(t, InstallAcquisitionSchema(db))
	service, err := NewBrowserPublicAcquisition(context.Background(), db, live, permissions, provider, 90*time.Second)
	require.NoError(t, err)
	return service
}

// The browser path must publish through the real repository (StartPrepared +
// validCommand(public_browser)), be exactly readable afterwards, replay from
// the durable row without re-invoking the provider, and conflict on a
// same-key/different-offer request.
func TestBrowserPublicAcquisitionPostgresChain(t *testing.T) {
	provider := &browserProviderStub{evidence: browserPostgresEvidence()}
	service := newBrowserPostgresService(t, provider)

	ctx := acquisitionIdentity("org-browser", "actor-browser")
	key := uuid.NewString()
	first, err := service.Acquire(ctx, key, "981645030344")
	require.NoError(t, err)
	require.NotNil(t, first.Publication)
	require.Equal(t, sourcing.AcquisitionPublished, first.Operation.State)
	require.Equal(t, "Browser PG bottle", first.Publication.Snapshot.Title)
	require.Equal(t, "crawler:1688:981645030344", first.Publication.Receipt.ProductKey)
	require.EqualValues(t, 1, first.Publication.Receipt.CatalogVersion)
	require.Equal(t, sourcing.AcquisitionProducerKind, first.Publication.Receipt.Producer.Kind, "no new producer kind")
	require.EqualValues(t, 1, provider.calls.Load())

	// Exact read-back through the current snapshot reader.
	product, err := service.ReadPublished(ctx, first.Operation.ID)
	require.NoError(t, err)
	require.Equal(t, "Browser PG bottle", product.Snapshot.Snapshot.Title)

	// Same-key replay resolves from the durable row without re-acquiring.
	replay, err := service.Acquire(ctx, key, "http://DETAIL.1688.COM/offer/981645030344.html?tracking=x#y")
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, first.Publication, replay.Publication)
	require.EqualValues(t, 1, provider.calls.Load(), "replay must not re-invoke the provider")

	// Same key, different offer: conflict, no new fetch.
	_, err = service.Acquire(ctx, key, "111111111111")
	require.ErrorIs(t, err, sourcing.ErrAcquisitionConflict)
	require.EqualValues(t, 1, provider.calls.Load())
}

// A provider-scoped timeout must not admit an operation or leave a row.
func TestBrowserPublicAcquisitionPostgresProviderTimeoutAdmitsNothing(t *testing.T) {
	db := acquisitionDatabase(t)
	provider := &browserProviderStub{delay: 5 * time.Second}
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	live := liveRolesFunc(func(context.Context, string, string) ([]string, error) {
		return []string{"listingkit_operator"}, nil
	})
	require.NoError(t, InstallAcquisitionSchema(db))
	tiny, err := NewBrowserPublicAcquisition(context.Background(), db, live, permissions, provider, 30*time.Millisecond)
	require.NoError(t, err)
	_, err = tiny.Acquire(acquisitionIdentity("org-timeout", "actor-timeout"), uuid.NewString(), "981645030344")
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// No operation row was admitted.
	var count int64
	require.NoError(t, db.Raw("SELECT count(*) FROM public.product_acquisition_operations WHERE organization_id=?", "org-timeout").Scan(&count).Error)
	require.EqualValues(t, 0, count)
}
