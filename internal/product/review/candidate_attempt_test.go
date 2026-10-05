package review

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/sourcing"
)

type candidateAttemptStore struct {
	Store
	runs int
}

func (*candidateAttemptStore) Preflight(context.Context, Operation) (View, bool, error) {
	return View{}, false, nil
}

func (s *candidateAttemptStore) Run(context.Context, Operation, func(Tx) (View, error)) (View, error) {
	s.runs++
	return View{}, ErrUnavailable
}

func TestCandidateIntakeReportsTransactionAttempt(t *testing.T) {
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	input := CandidateInput{Base: CreateInput{ProductKey: "product", BaseVersion: 7},
		PublicationID: "source-publication", PolicyVersion: "title-review-v1",
		Candidate: enrichment.Candidate{Changes: []enrichment.FieldChange{{Field: "title", Value: "Reviewed title", EvidenceIDs: []string{"evidence"}}}}}
	published := catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: "org", ProductKey: "product"},
		Version: 7, PublicationID: "source-publication", Snapshot: catalog.ProductSnapshot{Title: "source title"}}
	for _, test := range []struct {
		name      string
		roles     []string
		mutate    func(*catalog.PublishedSnapshot)
		attempted bool
		want      error
	}{
		{"authorization rejected", []string{"listingkit_viewer"}, nil, false, ErrForbidden},
		{"source changed", []string{"listingkit_admin"}, func(p *catalog.PublishedSnapshot) { p.PublicationID = "changed" }, false, ErrConflict},
		{"transaction failed", []string{"listingkit_admin"}, nil, true, ErrUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := published
			if test.mutate != nil {
				test.mutate(&base)
			}
			store := &candidateAttemptStore{}
			source := &sourcePublicationReaderStub{value: sourcing.PersistedPublication{
				Receipt: sourcing.PublicationReceipt{OrganizationID: "org", ProductKey: "product", CatalogVersion: 7,
					PublicationID: "source-publication", CatalogPublicationID: "source-publication"}, Snapshot: published.Snapshot,
				Envelope: sourcing.SourceEnvelope{Identity: sourcing.SourceIdentity{SourceType: sourcing.SourceTypeManualImport,
					SourcePlatform: "fixture", SourceID: "source", SourceVersion: "v1"}, RawReference: sourcing.RawSourceReference{ReferenceID: "evidence"}},
			}}
			service, err := NewCandidateService(exactCatalogReaderStub{published: base}, source, store, auth)
			require.NoError(t, err)
			ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
				TenantID: "org", EffectiveOrganizationID: "org", UserID: "owner", Roles: test.roles, TokenExpiresAt: time.Now().Add(time.Hour)})
			_, attempted, err := service.CreateFromCandidateWithTransactionAttempt(ctx, "agent:run", input)
			require.ErrorIs(t, err, test.want)
			require.Equal(t, test.attempted, attempted)
			require.Equal(t, test.attempted, store.runs == 1)
		})
	}
}
