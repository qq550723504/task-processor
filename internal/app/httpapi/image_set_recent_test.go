package httpapi

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
)

type recentImageSetRepository struct {
	imageagent.Repository
	projections map[string]imageagent.RunProjection
}

func (r recentImageSetRepository) GetProjection(ctx context.Context, scope imageagent.RunScope) (imageagent.RunProjection, error) {
	if err := ctx.Err(); err != nil {
		return imageagent.RunProjection{}, err
	}
	p, ok := r.projections[scope.RunID]
	if !ok || scope != imageagent.ScopeForRun(p.Run) {
		return imageagent.RunProjection{}, imageagent.ErrRunNotFound
	}
	return p, nil
}

type recentImageSetReader func(context.Context, imageagent.ExecutionIdentity, string, string, int) ([]imageagent.ImageSetRunSummary, string, error)

func (f recentImageSetReader) ListImageSets(ctx context.Context, id imageagent.ExecutionIdentity, contextID, cursor string, size int) ([]imageagent.ImageSetRunSummary, string, error) {
	return f(ctx, id, contextID, cursor, size)
}

type recentImageSetSources func(context.Context, imageagent.ExecutionIdentity, imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error)

func (f recentImageSetSources) ReadImageSetSource(ctx context.Context, id imageagent.ExecutionIdentity, input imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	return f(ctx, id, input)
}

func recentImageSetFixture(t *testing.T, count int) (*fullImageApplication, context.Context, []imageagent.ImageSetRunSummary, map[string]imageagent.RunProjection) {
	t.Helper()
	items := make([]imageagent.ImageSetRunSummary, count)
	projections := make(map[string]imageagent.RunProjection, count)
	for i := range items {
		runID := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
		contextID := fmt.Sprintf("source-%d", i)
		items[i] = imageagent.ImageSetRunSummary{RunID: runID, ContextKind: imageagent.ImageSourceAcquisition, ContextID: contextID, Status: imageagent.RunStatusExecuting, TargetPlatform: "product"}
		projections[runID] = imageagent.RunProjection{
			Run:  imageagent.Run{ID: runID, TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: contextID, ScopeProtocol: imageagent.OrganizationScopeProtocol, Status: imageagent.RunStatusCompleted},
			Plan: imageagent.Plan{Set: &imageagent.ImageSetPlan{Source: imageagent.ImageSourceBinding{ContextKind: items[i].ContextKind, OperationID: contextID, EffectiveVersion: uint64(i + 1), ApplyReceiptID: "receipt-" + contextID}, Target: imageagent.ImageTarget{Platform: items[i].TargetPlatform}}},
		}
	}
	service, err := imageagent.NewService(recentImageSetRepository{projections: projections}, imageSetHTTPWorkflow{}, closedImageSetCatalog{}, imageagent.WithOrganizationScope())
	require.NoError(t, err)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member"})
	a := &fullImageApplication{service: service, recent: recentImageSetReader(func(_ context.Context, id imageagent.ExecutionIdentity, contextID, cursor string, size int) ([]imageagent.ImageSetRunSummary, string, error) {
		require.Equal(t, imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: "org", UserID: "actor", MemberID: "member"}, id)
		require.Empty(t, contextID)
		require.Equal(t, "previous", cursor)
		require.Equal(t, count, size)
		return append([]imageagent.ImageSetRunSummary(nil), items...), "next", nil
	})}
	return a, ctx, items, projections
}

func TestFullImageRecentPageChecksEveryLiveSourceWithinOneDeadline(t *testing.T) {
	const count = 20
	a, ctx, items, projections := recentImageSetFixture(t, count)
	var started atomic.Int32
	allStarted := make(chan struct{})
	badBinding := make(chan string, count)
	a.readSources = recentImageSetSources(func(ctx context.Context, id imageagent.ExecutionIdentity, input imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
		var expected *imageagent.RunProjection
		for _, p := range projections {
			if p.Run.BusinessTaskID == input.ContextID {
				copy := p
				expected = &copy
				break
			}
		}
		if expected == nil || id != (imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: input.ContextID}) || input.ContextKind != expected.Plan.Set.Source.ContextKind || input.EffectiveCatalogVersion != expected.Plan.Set.Source.EffectiveVersion || input.ApplyReceiptID != expected.Plan.Set.Source.ApplyReceiptID || input.Target.Platform != expected.Plan.Set.Target.Platform {
			badBinding <- input.ContextID
		}
		if started.Add(1) == count {
			close(allStarted)
		}
		select {
		case <-allStarted:
			if input.ContextID == items[7].ContextID {
				return imageagent.ImageSetPreparation{}, imageagent.ErrIdentityRequired
			}
			return imageagent.ImageSetPreparation{}, nil
		case <-ctx.Done():
			return imageagent.ImageSetPreparation{}, ctx.Err()
		}
	})
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	got, next, err := a.readRecent(ctx, "", "previous", count)
	require.NoError(t, err)
	require.EqualValues(t, count, started.Load(), "each item requires its own live authorization/source check")
	require.Empty(t, badBinding)
	require.Equal(t, "next", next)
	expected := append(append([]imageagent.ImageSetRunSummary(nil), items[:7]...), items[8:]...)
	for i := range expected {
		expected[i].Status = imageagent.RunStatusCompleted
	}
	require.Equal(t, expected, got, "omit inaccessible items while preserving the original pagination order")
}

func TestFullImageRecentPageCancellationNeverReturnsPartialSuccess(t *testing.T) {
	a, ctx, _, _ := recentImageSetFixture(t, 1)
	a.readSources = recentImageSetSources(func(ctx context.Context, _ imageagent.ExecutionIdentity, _ imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
		<-ctx.Done()
		return imageagent.ImageSetPreparation{}, ctx.Err()
	})
	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	got, next, err := a.readRecent(ctx, "", "previous", 1)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Nil(t, got)
	require.Empty(t, next)
}

func TestFullImageRecentPageRejectsInvalidScopeBeforeLiveSourceReads(t *testing.T) {
	for _, kind := range []string{"member", "source", "oversized_page"} {
		t.Run(kind, func(t *testing.T) {
			count := 3
			if kind == "oversized_page" {
				count = 41
			}
			a, ctx, items, projections := recentImageSetFixture(t, count)
			last := projections[items[count-1].RunID]
			switch kind {
			case "member":
				last.Run.MemberID = "replacement"
			case "source":
				last.Plan.Set.Source.OperationID = "other-source"
			}
			projections[last.Run.ID] = last
			var calls atomic.Int32
			a.readSources = recentImageSetSources(func(context.Context, imageagent.ExecutionIdentity, imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
				calls.Add(1)
				return imageagent.ImageSetPreparation{}, nil
			})
			got, next, err := a.readRecent(ctx, "", "previous", count)
			require.Error(t, err)
			require.Nil(t, got)
			require.Empty(t, next)
			require.Zero(t, calls.Load())
		})
	}
}
