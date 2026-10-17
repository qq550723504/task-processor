package imageagent_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"task-processor/internal/imageagent"
)

func imageMethodRef() agent.SkillVersionRef {
	return agent.SkillVersionRef{Kind: "skill-version-v1", ID: "2bb819bd-af13-4e68-95e0-1cc86130fb1f", OwnerKind: "PLATFORM", OwnerID: "official", Version: 1, Digest: strings.Repeat("f", 64), PolicyVersion: "skill-method-v1"}
}

type methodFixture struct {
	text              string
	unavailable, held bool
	calls             int
}

func (m *methodFixture) AcquireImageMethod(context.Context, imageagent.ExecutionIdentity, agent.SkillVersionRef) (string, func(), error) {
	m.calls++
	if m.unavailable {
		return "", nil, imageagent.ErrCommandBlocked
	}
	m.held = true
	return m.text, func() { m.held = false }, nil
}

func TestImageMethodBindsPlanQuoteAndInputWithoutPersistingMaterial(t *testing.T) {
	plain, _, _, _, _, _, ctx, input := imageSetServiceFixture(t)
	original, err := plain.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	m := &methodFixture{text: "private-method-material-only-provider"}
	s, _, _, _, _, _, ctx, input := imageSetServiceFixture(t, imageagent.WithImageMethodContext(m))
	ref := imageMethodRef()
	input.Method = &ref
	selected, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	require.False(t, m.held)
	require.Equal(t, ref, selected.Projection.Plan.Set.Method.Skill)
	require.NotEqual(t, original.QuoteDigest, selected.QuoteDigest)
	require.NotEqual(t, original.PlanDigest, selected.PlanDigest)
	require.NotEqual(t, original.Projection.Plan.Set.InputDigest, selected.Projection.Plan.Set.InputDigest)
	require.Equal(t, original.Points, selected.Points)
	require.Equal(t, original.Images, selected.Images)
	raw, err := json.Marshal(selected.Projection)
	require.NoError(t, err)
	require.NotContains(t, string(raw), m.text)
	input.Method = nil
	_, err = s.PrepareImageSet(ctx, input)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
}

func TestImageMethodRejectsOversizeBeforeCreatingRun(t *testing.T) {
	m := &methodFixture{text: strings.Repeat("x", 16<<10)}
	s, repo, _, _, _, _, ctx, input := imageSetServiceFixture(t, imageagent.WithImageMethodContext(m))
	ref := imageMethodRef()
	input.Method = &ref
	_, err := s.PrepareImageSet(ctx, input)
	require.Error(t, err)
	require.False(t, m.held)
	input.Method = nil
	_, err = s.PrepareImageSet(ctx, input)
	require.NoError(t, err, "oversize material must not leave a frozen run bound to the request key")
	require.NotNil(t, repo)
}

type methodFenceCommit struct {
	imageagent.Repository
	m *methodFixture
	t *testing.T
}

func (r *methodFenceCommit) CommitProjection(ctx context.Context, c imageagent.ProjectionCommit) (imageagent.RunProjection, error) {
	if c.EventType == "run.confirmed" {
		require.True(r.t, r.m.held, "method fence must span the confirmation CAS")
	}
	return r.Repository.CommitProjection(ctx, c)
}

func TestNewImageConfirmationHoldsMethodFenceAndOriginalReceiptRecoveryDoesNotReacquire(t *testing.T) {
	for _, lostCAS := range []bool{false, true} {
		t.Run(map[bool]string{false: "new", true: "original_receipt"}[lostCAS], func(t *testing.T) {
			m := &methodFixture{text: "bounded method"}
			wrap := func(r imageagent.Repository) imageagent.Repository {
				if lostCAS {
					return &confirmCommitFailure{Repository: r, fail: true}
				}
				return &methodFenceCommit{Repository: r, m: m, t: t}
			}
			s, _, _, cfg, _, _, ctx, input := imageSetServiceFixtureWithRepository(t, wrap, imageagent.WithImageMethodContext(m))
			ref := imageMethodRef()
			input.Method = &ref
			prepared, err := s.PrepareImageSet(ctx, input)
			require.NoError(t, err)
			_, err = s.ConfirmImagePlan(ctx, confirmSetInput(prepared))
			if lostCAS {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.False(t, m.held)
			require.Equal(t, 1, cfg.admissions)
			calls := m.calls
			m.unavailable = true
			_, err = s.ConfirmImagePlan(ctx, confirmSetInput(prepared))
			require.NoError(t, err)
			require.Equal(t, calls, m.calls)
			require.Equal(t, 1, cfg.admissions)
		})
	}
}

func TestImageMethodBindingRequiresWiredPort(t *testing.T) {
	s, _, _, _, _, _, ctx, input := imageSetServiceFixture(t)
	raw, err := json.Marshal(map[string]any{"Method": imageMethodRef()})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &input))
	_, err = s.PrepareImageSet(ctx, input)
	require.Error(t, err, "a selected method must not silently execute without its use/material port")
}
