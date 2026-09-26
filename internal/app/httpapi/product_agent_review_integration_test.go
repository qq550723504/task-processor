package httpapi

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"task-processor/internal/app/productsourcing"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	reviewstore "task-processor/internal/integration/persistence/product/review"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/enrichment"
	"task-processor/internal/product/review"
)

func agentReviewFixture(t *testing.T) (*titleFixture, *review.Service, context.Context, review.CandidateInput) {
	t.Helper()
	f := newTitleFixture(t)
	auth, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	store, err := reviewstore.NewRepository(f.db, func(tx *gorm.DB) (review.SourcePublicationReader, error) {
		return productsourcing.NewTransactionReader(tx)
	})
	require.NoError(t, err)
	proposer, err := enrichment.NewProposer(enrichment.Dependencies{Generator: f.g})
	require.NoError(t, err)
	reader, err := catalogstore.NewBoundedSnapshotReader(f.db, 2<<20)
	require.NoError(t, err)
	service, err := review.NewService(reader, f.sourceProducer, store, proposer, auth)
	require.NoError(t, err)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
		TenantID: "B", EffectiveOrganizationID: "B", UserID: "operator", Roles: []string{"listingkit_operator"}, TokenExpiresAt: time.Now().Add(time.Hour),
	})
	in := review.CandidateInput{
		Base: review.CreateInput{ProductKey: "product", BaseVersion: 1}, PublicationID: f.base.PublicationID, PolicyVersion: "title-review-v1",
		Candidate: enrichment.Candidate{Changes: []enrichment.FieldChange{{Field: "title", Value: "Agent suggested title", EvidenceIDs: []string{"evidence1"}}}},
	}
	return f, service, ctx, in
}

func TestProductAgentReviewIntakeReusesPendingAndIdempotencyWithoutModel(t *testing.T) {
	f, service, ctx, in := agentReviewFixture(t)
	f.g.failed.Store(true) // Any accidental regeneration fails this path.
	v, err := service.CreateFromCandidate(ctx, "agent-run:step:4", in)
	require.NoError(t, err)
	require.Equal(t, "pending", v.State)
	require.Equal(t, "Agent suggested title", v.Title)
	require.Zero(t, f.g.calls.Load())
	replayed, err := service.CreateFromCandidate(ctx, "agent-run:step:4", in)
	require.NoError(t, err)
	require.Equal(t, v, replayed)
	in.Candidate.Changes[0].Value = "Changed candidate"
	_, err = service.CreateFromCandidate(ctx, "agent-run:step:4", in)
	require.ErrorIs(t, err, review.ErrConflict)
	base, err := f.reader.GetCurrentSnapshot(ctx, catalog.SnapshotIdentity{TenantID: "B", ProductKey: "product"})
	require.NoError(t, err)
	require.Equal(t, uint64(1), base.Version, "intake must not Apply")
}

func TestProductAgentReviewIntakeRevalidatesExactBindingAndEvidence(t *testing.T) {
	for _, name := range []string{"evidence", "field", "publication", "policy", "cross organization"} {
		t.Run(name, func(t *testing.T) {
			f, service, ctx, in := agentReviewFixture(t)
			switch name {
			case "evidence":
				in.Candidate.Changes[0].EvidenceIDs = []string{"another-source"}
			case "field":
				in.Candidate.Changes[0].Field = "description"
			case "publication":
				in.PublicationID = "different-publication"
			case "policy":
				in.PolicyVersion = "other-policy"
			case "cross organization":
				in.Base.ProductKey = "product-a"
			}
			_, err := service.CreateFromCandidate(ctx, name, in)
			require.Error(t, err)
			var count int64
			require.NoError(t, f.db.Table("product_title_proposals").Count(&count).Error)
			require.Zero(t, count)
			require.Zero(t, f.g.calls.Load())
		})
	}
}
