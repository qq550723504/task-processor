package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	imageapp "task-processor/internal/app/imageagent"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	"task-processor/internal/imageagent"
	imagetemporal "task-processor/internal/imageagent/temporal"
)

var errWorkerSourceGrantReached = errors.New("controlled activity stops after live source authorization, before reservation")

type fullImageWorkerRepository struct {
	imageSetHTTPRepository
	imageagent.SlotExternalEffectRepository
	imageagent.SlotExternalEffectV3Repository
	catalog imageagent.AssetCatalog
}

func (r fullImageWorkerRepository) GetAssetCatalog(context.Context, imageagent.RunScope) (imageagent.AssetCatalog, error) {
	return r.catalog, nil
}
func (r fullImageWorkerRepository) GetSlotExternalEffectV3(context.Context, imageagent.SlotExternalEffectIdentity) (imageagent.SlotEffectV3Attempt, error) {
	return imageagent.SlotEffectV3Attempt{}, imageagent.ErrRunNotFound
}

type fullImageWorkerArtifacts struct {
	imagetemporal.DurableArtifactStore
}
type fullImageWorkerPublisher struct {
	imageagent.ApprovedImageSetPublisher
}
type fullImageWorkerSourceProbe struct {
	imageagent.BudgetedStagedSlotExecutor
	live  supplyapp.OrganizationExecutionAuthorizer
	calls int
}

func (e *fullImageWorkerSourceProbe) ExecuteSlot(context.Context, imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	return imageagent.SlotExecutionResult{}, imageagent.ErrCommandBlocked
}
func (e *fullImageWorkerSourceProbe) QuoteSlot(ctx context.Context, input imageagent.SlotExecutionInput, _ imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || !id.TokenExpiresAt.IsZero() || id.TenantID != input.TenantID || id.UserID != input.UserID || id.EffectiveMemberID != input.OrganizationIdentity.MemberID {
		return imageagent.SlotUsageQuote{}, imageagent.ErrIdentityRequired
	}
	source := imageapp.ImagePublicationScopeAuthorizer{Live: imageSourceScopeAuthority{current: e.live}}
	if _, err := source.Authorize(ctx); err != nil {
		return imageagent.SlotUsageQuote{}, err
	}
	e.calls++
	return imageagent.SlotUsageQuote{}, errWorkerSourceGrantReached
}

func TestFullImageTemporalV3AuthorizesDurableSubjectAndOriginalSourceWithoutBrowserContext(t *testing.T) {
	var replaced atomic.Bool
	var role atomic.Value
	role.Store("listingkit_admin")
	var iamCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		iamCalls.Add(1)
		member := "member"
		if replaced.Load() {
			member = "replacement"
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"pagination":{"totalResult":"1"},"authorizations":[{"id":%q,"project":{"id":"project"},"organization":{"id":"org"},"user":{"id":"actor"},"state":"STATE_ACTIVE","roles":[{"key":%q}]}]}`, member, role.Load().(string))
	}))
	defer server.Close()
	permissions, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	live := supplyapp.OrganizationExecutionAuthorizer{Client: zitadel.NewAuthorizationClient(server.URL, server.Client()), ServiceToken: func(context.Context) (string, error) { return "controlled-service-token", nil }, ProjectID: "project", Permissions: permissions}
	id := imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, RunID: "run", TenantID: "org", UserID: "actor", MemberID: "member", BusinessTaskID: "source"}
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product"}, Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://images.example.org/source.png"}}})
	require.NoError(t, err)
	repo := fullImageWorkerRepository{imageSetHTTPRepository: imageSetHTTPRepository{projection: imageagent.RunProjection{Run: imageagent.Run{ID: id.RunID, ScopeProtocol: id.ScopeProtocol, TenantID: id.TenantID, UserID: id.UserID, MemberID: id.MemberID, BusinessTaskID: id.BusinessTaskID}}}, catalog: catalog}
	probe := &fullImageWorkerSourceProbe{live: live}
	activities, err := imagetemporal.NewActivities(imagetemporal.ActivityDependencies{Repository: repo, SlotExecutor: probe, ExecutionAuthorizer: newFullImageWorkerAuthorizer(live), ImageSetPublisher: &fullImageWorkerPublisher{}, StagedSlotExecutor: probe, ArtifactStore: &fullImageWorkerArtifacts{}})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, browserIdentity := authidentity.AuthenticatedIdentityFromContext(ctx)
	require.False(t, browserIdentity)
	input := imagetemporal.ExecuteSlotV3ActivityInput{RunID: id.RunID, Identity: id, PlanRevision: 1, Slot: imageagent.Slot{ID: "overview"}, Attempt: 1, AssetCatalog: catalog, BudgetAuthorization: true}
	_, err = activities.ExecuteSlotV3(ctx, input)
	require.ErrorIs(t, err, errWorkerSourceGrantReached, "the actual v3 authorization seam must reach the source grant before any provider reservation")
	require.Equal(t, 1, probe.calls)
	require.EqualValues(t, 2, iamCalls.Load(), "image and original source must each read current IAM")
	for _, defect := range []string{"activity member mismatch", "current member replaced", "image permission revoked"} {
		t.Run(defect, func(t *testing.T) {
			bad := input
			switch defect {
			case "activity member mismatch":
				bad.Identity.MemberID = "forged"
			case "current member replaced":
				replaced.Store(true)
			case "image permission revoked":
				role.Store("listingkit_viewer")
			}
			before := probe.calls
			_, err := activities.ExecuteSlotV3(ctx, bad)
			require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
			require.Equal(t, before, probe.calls, "denial must precede source I/O and reservation")
			replaced.Store(false)
			role.Store("listingkit_admin")
		})
	}
}
