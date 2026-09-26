package imageagentworker

import (
	"context"
	"testing"

	"task-processor/internal/imageagent"

	"github.com/stretchr/testify/require"
)

type generationProjectionReader struct {
	imageagent.Repository
	run imageagent.Run
}

func (r generationProjectionReader) GetProjection(context.Context, imageagent.RunScope) (imageagent.RunProjection, error) {
	return imageagent.RunProjection{Run: r.run}, nil
}

type generationLiveAuthorizer struct {
	calls    int
	identity imageagent.ExecutionIdentity
}

func (a *generationLiveAuthorizer) AuthorizeExecution(_ context.Context, identity imageagent.ExecutionIdentity) error {
	a.calls++
	a.identity = identity
	return nil
}

func TestGenerationResourceAdmissionUsesDurableCanonicalMemberAndLiveGrant(t *testing.T) {
	run := imageagent.Run{ID: "run-1", ScopeProtocol: imageagent.OrganizationScopeProtocol, TenantID: "org-1", UserID: "actor-1", MemberID: "grant-1", BusinessTaskID: "acquisition-1"}
	intent := imageagent.GenerationIntent{Identity: imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: run.TenantID, OwnerUserID: run.UserID, RunID: run.ID}}, MemberID: run.MemberID}
	for _, mode := range []string{"exact", "new_member_same_actor", "other_org", "other_actor", "legacy_scope"} {
		t.Run(mode, func(t *testing.T) {
			altered := run
			switch mode {
			case "new_member_same_actor":
				altered.MemberID = "grant-2"
			case "other_org":
				altered.TenantID = "org-2"
			case "other_actor":
				altered.UserID = "actor-2"
			case "legacy_scope":
				altered.ScopeProtocol = ""
			}
			live := &generationLiveAuthorizer{}
			gate := generationResourceAuthorizer{repository: generationProjectionReader{run: altered}, authorizer: live}
			err := gate.AuthorizeImageGeneration(context.Background(), intent)
			if mode == "exact" {
				require.NoError(t, err)
				require.Equal(t, 1, live.calls)
				require.Equal(t, "grant-1", live.identity.MemberID)
				require.Equal(t, "acquisition-1", live.identity.BusinessTaskID)
			} else {
				require.ErrorIs(t, err, imageagent.ErrIdentityRequired)
				require.Zero(t, live.calls)
			}
		})
	}
}

func TestGenerationSourceCannotFetchOutsideExactCatalog(t *testing.T) {
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/image.png"}}})
	require.NoError(t, err)
	for _, mode := range []string{"wrong_id", "changed_url", "private_url", "missing_manifest"} {
		t.Run(mode, func(t *testing.T) {
			copy := catalog
			copy.Assets = append([]imageagent.AuthorizedAsset(nil), catalog.Assets...)
			input := imageagent.SlotExecutionInput{Slot: imageagent.Slot{SourceAssetIDs: []string{"source-1"}}, AssetCatalog: copy}
			switch mode {
			case "wrong_id":
				input.Slot.SourceAssetIDs = []string{"not-in-catalog"}
			case "changed_url":
				input.AssetCatalog.Assets[0].URL = "https://other.example/image.png"
			case "private_url":
				input.AssetCatalog.Assets[0].URL = "http://127.0.0.1/private"
			case "missing_manifest":
				input.AssetCatalog.Manifest.Hash = ""
			}
			_, err := readGenerationSource(context.Background(), input)
			require.ErrorIs(t, err, imageagent.ErrValidation)
		})
	}
}
