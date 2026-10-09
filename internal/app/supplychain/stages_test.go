package supplychainapp

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/review"
	"testing"
)

type stageFactsFixture struct {
	preparation.StageFactsReader
	item preparation.OperationItem
}

func (f stageFactsFixture) LatestOptimization(context.Context, collection.Scope, string, string) (preparation.OperationItem, error) {
	return f.item, nil
}

type stageReviewFixture struct {
	value review.Record
	err   error
}

func (f stageReviewFixture) Read(context.Context, review.Scope, string) (review.Record, error) {
	return f.value, f.err
}
func TestSelectedOptimizationMustBeReviewedAndReboundBeforeUpload(t *testing.T) {
	for _, state := range []string{"pending", "accepted", "applied", "rejected"} {
		t.Run(state, func(t *testing.T) {
			s, scope, records, merchant, _, _, _ := uploadFixture(t)
			saved := records.saved
			id := uuid.NewString()
			projection := ReviewProjection{Facts: stageFactsFixture{item: preparation.OperationItem{SourceID: saved.Source.ID, RecordID: saved.ID, RecordRevision: saved.Revision, Status: preparation.ItemReview, ResultReference: id}}, Reviews: stageReviewFixture{value: review.Record{ID: id, Org: scope.OrganizationID, Owner: scope.ActorID, State: state, Input: review.CreateInput{ProductKey: saved.Source.Source.ProductKey, BaseVersion: saved.EffectiveVersion}}}}
			s.dependencies.ReviewGate = projection.RequireUploadReady
			_, err := s.Upload(context.Background(), scope, uuid.NewString(), saved.ID)
			if state == "rejected" {
				require.NoError(t, err)
				require.Equal(t, 1, merchant.publishes)
			} else {
				require.ErrorIs(t, err, record.ErrNotReady)
				require.Zero(t, merchant.transforms)
				require.Zero(t, merchant.publishes)
			}
		})
	}
}

func TestStageFilterUsesOriginalSourceKindSKUAndBatchAcrossPages(t *testing.T) {
	f := preparation.SourceStageFacts{Title: "T-shirt", ProductKey: "product-a", SourceKind: "own", SKUs: "SKU-SECOND-PAGE"}
	require.True(t, matchesStageFilter(f, "review", "review", collection.Query{Keyword: "sku-second"}, "own", "batch"))
	require.False(t, matchesStageFilter(f, "review", "review", collection.Query{Keyword: "sku-second"}, "acquisition", "batch"))
	require.True(t, matchesStageFilter(f, "review", "review", collection.Query{Keyword: "summer batch"}, "", "Summer Batch"))
	require.False(t, matchesStageFilter(f, "review", "uploaded", collection.Query{}, "", "batch"))
}
