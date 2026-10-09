package assetpublication

import (
	"context"
	"github.com/stretchr/testify/require"
	"strings"
	"task-processor/internal/imageagent"
	productasset "task-processor/internal/product/asset"
	"testing"
)

type selectedSetFixture struct {
	commands []productasset.ImageSetCommand
}

func (f *selectedSetFixture) Select(_ context.Context, command productasset.ImageSetCommand) (productasset.ApprovalReceipt, error) {
	f.commands = append(f.commands, *imageagent.CloneImageSetCommand(&command))
	return productasset.ApprovalReceipt{ActionID: command.ActionID, AssetIDs: []string{"selected-image"}}, nil
}

func TestImageSetPublisherConsumesExactSelectionAfterOriginalPublicationBoundary(t *testing.T) {
	projection, _, source, choice := imageSetCandidateFixture(t)
	action := "1b912e40-d50a-48f5-a12b-62c73a42e4b8"
	selection := productasset.ImageSetCommand{ActionID: action, SelectionDigest: strings.Repeat("b", 64), Source: productasset.SourceSelectionRequest{ItemID: source.ItemID, OriginalPublicationID: source.OriginalPublicationID, OriginalSnapshotVersion: source.OriginalSnapshotVersion, EffectiveCatalogVersion: source.EffectiveCatalogVersion, TargetPlatform: source.TargetPlatform}, Choices: []productasset.ImageSetChoice{choice}}
	projection.PendingCommand = &imageagent.PendingCommandReceipt{ActionID: action, Kind: "approve_results", Phase: imageagent.ImageSetApprovalPublicationStarted, PlanRevision: 1, SelectionDigest: selection.SelectionDigest, ResultDigest: projection.ResultDigest}
	selector := &selectedSetFixture{}
	publisher, err := NewImageSetPublisher(staticProjectionSource{projection: projection}, selector)
	require.NoError(t, err)
	input := imageagent.PublishImageSetInput{RunID: projection.Run.ID, TenantID: projection.Run.TenantID, UserID: projection.Run.UserID, PlanRevision: 1, ResultDigest: projection.ResultDigest, Selection: selection}
	ack, err := publisher.PublishApprovedImageSet(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, action, ack.ActionID)
	require.Equal(t, []string{"selected-image"}, ack.AssetIDs)
	require.Equal(t, selection, selector.commands[0])
	require.Equal(t, projection.ResultDigest, ack.ResultDigest)
	for _, kind := range []string{"no_boundary", "changed_selection", "changed_result"} {
		t.Run(kind, func(t *testing.T) {
			changed := projection
			receipt := *projection.PendingCommand
			changed.PendingCommand = &receipt
			switch kind {
			case "no_boundary":
				changed.PendingCommand = nil
			case "changed_selection":
				receipt.SelectionDigest = strings.Repeat("c", 64)
			case "changed_result":
				receipt.ResultDigest = strings.Repeat("c", 64)
			}
			rejected := &selectedSetFixture{}
			p, err := NewImageSetPublisher(staticProjectionSource{projection: changed}, rejected)
			require.NoError(t, err)
			_, err = p.PublishApprovedImageSet(context.Background(), input)
			require.Error(t, err)
			require.Empty(t, rejected.commands)
		})
	}
}
