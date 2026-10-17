package assetpersistence

import (
	"context"
	"github.com/stretchr/testify/require"
	d "task-processor/internal/product/asset"
	"testing"
)

type methodSelectionReader struct {
	setCandidateReader
	fenceContext context.Context
	held, denied bool
	gates        int
}

func (r *methodSelectionReader) AcquireImageSetSelection(ctx context.Context, _ d.SourceSelection, _ d.ImageSetCommand) (func(), error) {
	r.fenceContext = ctx
	r.gates++
	if r.denied {
		return nil, d.ErrSourceApprovalForbidden
	}
	r.held = true
	return func() { r.held = false }, nil
}

type methodCommitRepository struct {
	d.Repository
	reader  *methodSelectionReader
	t       *testing.T
	commits int
}

func (r *methodCommitRepository) CommitApproval(ctx context.Context, c d.ApprovalCommit) (d.ApprovalReceipt, error) {
	if r.commits == 0 {
		require.True(r.t, r.reader.held, "the method fence must span the original Asset commit")
		require.NoError(r.t, r.reader.fenceContext.Err(), "prepare must not cancel the PG fence before Asset commit")
	}
	r.commits++
	return r.Repository.CommitApproval(ctx, c)
}

func TestSelectionHoldsMethodFenceUntilCommitAndReplaysOriginalReceiptFirst(t *testing.T) {
	sources, input := setSelectionFixture()
	reader := &methodSelectionReader{}
	_, base := setSelectionService(t, sources, &reader.setCandidateReader)
	repo := &methodCommitRepository{Repository: base, reader: reader, t: t}
	service, err := d.NewImageSetService(sources, repo, base.(d.ImageSetInventoryReader), base.(d.ApprovalCommitReader), reader, genericMaterialIdentity{})
	require.NoError(t, err)
	preview, err := service.Preview(context.Background(), input)
	require.NoError(t, err)
	require.False(t, reader.held)
	input.SelectionDigest = preview.Digest
	first, err := service.Select(context.Background(), input)
	require.NoError(t, err)
	require.False(t, reader.held)
	gates := reader.gates
	reader.denied = true
	replay, err := service.Select(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, first, replay)
	require.Equal(t, gates, reader.gates)
	input.ActionID = "another-action"
	_, err = service.Select(context.Background(), input)
	require.ErrorIs(t, err, d.ErrSourceApprovalForbidden)
}
