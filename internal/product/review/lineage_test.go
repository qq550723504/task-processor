package review

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/sourcing"
	"testing"
	"time"
)

type lineageCatalog map[uint64]catalog.PublishedSnapshot

func (r lineageCatalog) GetSnapshot(_ context.Context, id catalog.SnapshotIdentity, v uint64) (catalog.PublishedSnapshot, error) {
	p, ok := r[v]
	if !ok || p.Identity != id {
		return catalog.PublishedSnapshot{}, catalog.ErrSnapshotNotReady
	}
	return p, nil
}

type lineageStore struct {
	Store
	records map[string]Record
}

func (r lineageStore) ReadAppliedPublication(_ context.Context, scope Scope, product string, version uint64, publication string) (Record, error) {
	for _, p := range r.records {
		if p.Org == scope.Org && p.Owner == scope.Actor && p.Input.ProductKey == product && p.Receipt != nil && p.Receipt.ProductVersion == version && p.Receipt.PublicationID == publication {
			return p, nil
		}
	}
	return Record{}, ErrNotFound
}

type lineageSources struct{ original sourcing.PersistedPublication }

func (s lineageSources) Read(_ context.Context, id string) (sourcing.PersistedPublication, error) {
	if id != s.original.Receipt.PublicationID {
		return sourcing.PersistedPublication{}, sourcing.ErrSourcePublicationNotFound
	}
	return s.original, nil
}
func (s lineageSources) AuthorizeRead(ctx context.Context) (context.Context, error) { return ctx, nil }

func TestReviewSourceReadsTwoAppliedTitleVersionsWithoutFabricatingSource(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	ctx = authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: "org", UserID: "owner"})
	identity := catalog.SnapshotIdentity{TenantID: "org", ProductKey: "product"}
	one := catalog.PublishedSnapshot{Identity: identity, Version: 1, PublicationID: "original", Snapshot: catalog.ProductSnapshot{Title: "Source title", Description: "Immutable description"}}
	two := one
	two.Version = 2
	two.PublicationID = "review:first"
	two.Snapshot.Title = "First reviewed title"
	three := two
	three.Version = 3
	three.PublicationID = "review:second"
	three.Snapshot.Title = "Second reviewed title"
	envelope := sourcing.SourceEnvelope{Identity: sourcing.SourceIdentity{SourceType: sourcing.SourceTypeManualImport, SourcePlatform: "fixture", SourceID: "source", SourceVersion: "v1"}, RawReference: sourcing.RawSourceReference{ReferenceID: "source"}}
	makeRecord := func(base, out catalog.PublishedSnapshot) Record {
		id := uuid.NewString()
		return Record{ID: id, Org: "org", Owner: "owner", Input: CreateInput{ProductKey: "product", BaseVersion: base.Version}, BasePublicationID: base.PublicationID, Policy: "title-review-v1", Before: base.Snapshot.Title, Title: out.Snapshot.Title, State: "applied", Revision: 2, Original: enrichment.Proposal{Changes: []enrichment.FieldChange{{Field: "title", Value: out.Snapshot.Title, EvidenceIDs: []string{"source"}}}}, Receipt: &Receipt{ProposalID: id, Revision: 2, ProductVersion: out.Version, PublicationID: out.PublicationID, Actor: "owner", At: time.Now().UTC()}}
	}
	first, second := makeRecord(one, two), makeRecord(two, three)
	snapshots := lineageCatalog{1: one, 2: two, 3: three}
	sources := lineageSources{sourcing.PersistedPublication{Receipt: sourcing.PublicationReceipt{OrganizationID: "org", ActorID: "owner", ProductKey: "product", PublicationID: "original", CatalogPublicationID: "original", CatalogVersion: 1}, Envelope: envelope, Snapshot: one.Snapshot}}
	store := lineageStore{records: map[string]Record{first.ID: first, second.ID: second}}
	service := &Service{store: store}
	base, source, err := service.source(ctx, snapshots, sources, Scope{Org: "org", Actor: "owner"}, CreateInput{ProductKey: "product", BaseVersion: 3})
	require.NoError(t, err)
	require.Equal(t, three, base)
	require.Equal(t, envelope, source, "effective Catalog changes must preserve the real original source provenance")
	for _, mode := range []string{"wrong owner", "missing receipt", "description changed", "base publication drift"} {
		t.Run(mode, func(t *testing.T) {
			bad := second
			changed := three
			switch mode {
			case "wrong owner":
				bad.Owner = "other"
			case "missing receipt":
				bad.Receipt = nil
			case "description changed":
				changed.Snapshot.Description = "Unreviewed"
			case "base publication drift":
				bad.BasePublicationID = "other"
			}
			snapshots[3] = changed
			store.records[second.ID] = bad
			_, _, err := service.source(ctx, snapshots, sources, Scope{Org: "org", Actor: "owner"}, CreateInput{ProductKey: "product", BaseVersion: 3})
			require.Error(t, err)
			snapshots[3] = three
			store.records[second.ID] = second
		})
	}
}
