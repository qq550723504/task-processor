package imageagent

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsolatedTrialGeneratedURLPolicyAcceptsOnlyExactVerifiedArtifact(t *testing.T) {
	base := "https://localhost:19444/image-agent-assets/issue487-images"
	policy, err := NewIsolatedTrialGeneratedURLPolicy(base, "issue487-images")
	require.NoError(t, err)
	input := SlotExecutionInput{TenantID: "org-a", UserID: "user-a", RunID: "run-a", PlanRevision: 2, Slot: Slot{ID: "main"}, Attempt: 1}
	ownerKey, err := ArtifactOwnerKey("user-a")
	require.NoError(t, err)
	hash := strings.Repeat("a", 64)
	staged := StagedAssetRef{ObjectKey: "image-agent/staging/org-a/" + ownerKey + "/run-a/2/main/1/0-" + hash + ".png", SHA256: hash, SizeBytes: 8, ContentType: "image/png", Width: 1, Height: 1, SourceAssetID: "catalog-image-1", Operations: []string{"render_white_background"}}
	stagedURL := base + "/" + staged.ObjectKey
	require.NoError(t, policy.ValidateStaged(input, staged, 0, stagedURL))
	require.Error(t, policy.ValidateStaged(input, staged, 0, "https://localhost:19444/other/"+staged.ObjectKey))
	require.Error(t, policy.ValidateStaged(input, staged, 0, stagedURL+"?redirect=https://evil.example"))
	wrong := staged
	wrong.ObjectKey = strings.Replace(wrong.ObjectKey, "/org-a/", "/org-b/", 1)
	require.Error(t, policy.ValidateStaged(input, wrong, 0, base+"/"+wrong.ObjectKey))
	require.Error(t, policy.ValidateStaged(input, staged, 1, stagedURL))
	public := DurableAssetIdentity{ObjectKey: strings.Replace(staged.ObjectKey, "/staging/", "/public/", 1), SHA256: hash}
	require.NoError(t, policy.ValidatePublished(input, public, 0, base+"/"+public.ObjectKey))
	require.Error(t, policy.ValidatePublished(input, public, 0, "https://evil.example/"+public.ObjectKey))
}

func TestIsolatedTrialGeneratedURLPolicyRejectsNonFixedOrigin(t *testing.T) {
	for _, base := range []string{
		"http://localhost:19444/image-agent-assets/issue487-images",
		"https://127.0.0.1:19444/image-agent-assets/issue487-images",
		"https://localhost/image-agent-assets/issue487-images",
		"https://localhost:19444/image-agent-assets/other",
		"https://localhost:19444/image-agent-assets/issue487-images?x=1",
		"https://localhost:19444/image-agent-assets/issue487-images/../other",
	} {
		_, err := NewIsolatedTrialGeneratedURLPolicy(base, "issue487-images")
		require.Error(t, err, base)
	}
}

func TestRecoveredStagedRefNeverEntersTemporalJSON(t *testing.T) {
	value := GeneratedAsset{URL: "https://example.test/image", StagedRef: &StagedAssetRef{ObjectKey: "private-key"}}
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-key")
}
