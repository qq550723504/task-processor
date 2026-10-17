package assetpublication

import (
	"context"
	"github.com/stretchr/testify/require"
	"strings"
	"task-processor/internal/agent"
	"task-processor/internal/imageagent"
	d "task-processor/internal/product/asset"
	"testing"
)

type selectionMethodFixture struct {
	held bool
	ref  agent.SkillVersionRef
}

func (m *selectionMethodFixture) AcquireImageMethod(_ context.Context, _ imageagent.ExecutionIdentity, ref agent.SkillVersionRef) (string, func(), error) {
	m.held = true
	m.ref = ref
	return "bounded method", func() { m.held = false }, nil
}

func TestCandidateMethodFenceUsesExactLineageAndLeavesReleaseToAssetOwner(t *testing.T) {
	p, facts, source, choice := imageSetCandidateFixture(t)
	ref := agent.SkillVersionRef{Kind: "skill-version-v1", ID: "2bb819bd-af13-4e68-95e0-1cc86130fb1f", OwnerKind: "PLATFORM", OwnerID: "official", Version: 1, Digest: strings.Repeat("f", 64), PolicyVersion: "skill-method-v1"}
	p.Plan.Set.Method = &imageagent.ImageMethodBinding{Skill: ref, PolicyVersion: imageagent.ImageMethodPolicyVersion}
	var err error
	p.Plan.Set.QuoteDigest, err = imageagent.ImageSetQuoteDigest(p.Plan)
	require.NoError(t, err)
	p.Run.ImageAdmission.Command.QuoteDigest = p.Plan.Set.QuoteDigest
	rebindLineageAdmission(t, &p)
	p.ResultDigest, err = imageagent.ImageSetResultDigest(p.Plan, p.Slots, p.RecoverableEffects)
	require.NoError(t, err)
	choice.ResultDigest = p.ResultDigest
	reader, err := NewImageSetCandidateReader(staticProjectionSource{projection: p}, facts, facts, staticPublicURLResolver{})
	require.NoError(t, err)
	input := d.ImageSetCommand{ApprovingResult: candidateApprovalBinding(p), Choices: []d.ImageSetChoice{choice}}
	_, err = reader.AcquireImageSetSelection(context.Background(), source, input)
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	methods := &selectionMethodFixture{}
	reader.WithImageMethods(methods)
	release, err := reader.AcquireImageSetSelection(context.Background(), source, input)
	require.NoError(t, err)
	require.True(t, methods.held)
	require.Equal(t, ref, methods.ref)
	release()
	require.False(t, methods.held)
}
