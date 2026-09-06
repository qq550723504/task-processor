package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	contract "task-processor/internal/marketplace/validator"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"

	"github.com/stretchr/testify/require"
)

const sharedRegressionRoot = "testdata/product_shein_v1/"

type sharedManifest struct {
	DatasetID      string    `json:"dataset_id"`
	Version        string    `json:"version"`
	ContentDigest  string    `json:"content_digest"`
	RuleVersion    string    `json:"rule_version"`
	BindingVersion string    `json:"binding_version"`
	SemanticTime   time.Time `json:"semantic_time"`
}

type sharedCase struct {
	CaseID          string                      `json:"case_id"`
	Layers          []string                    `json:"layers"`
	Basis           string                      `json:"basis"`
	Source          *sourcing.SourceEnvelope    `json:"source,omitempty"`
	Snapshot        *catalog.ProductSnapshot    `json:"expected_snapshot,omitempty"`
	SourceError     string                      `json:"source_error,omitempty"`
	Package         json.RawMessage             `json:"package,omitempty"`
	Reports         []sharedReport              `json:"reports,omitempty"`
	Binding         *sharedBinding              `json:"binding,omitempty"`
	Freshness       *contract.ExternalFreshness `json:"freshness,omitempty"`
	ErrorCode       contract.ErrorCode          `json:"error_code,omitempty"`
	FreshnessCauses []string                    `json:"freshness_causes,omitempty"`
}

type sharedReport struct {
	Action              contract.Action `json:"action"`
	Status              contract.Status `json:"status"`
	Blockers            []string        `json:"blockers"`
	Warnings            []string        `json:"warnings"`
	Exact               bool            `json:"exact"`
	DraftAllowsBlockers bool            `json:"draft_allows_blockers"`
}

type sharedBinding struct {
	Digest     string          `json:"digest"`
	Equivalent json.RawMessage `json:"equivalent"`
	Changed    json.RawMessage `json:"changed"`
}

func sharedDecode(t *testing.T, raw []byte, value any) {
	t.Helper()
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	require.NoError(t, d.Decode(value))
	require.ErrorIs(t, d.Decode(new(any)), io.EOF)
}

func loadSharedRegression(t *testing.T, layer string) (sharedManifest, []sharedCase) {
	t.Helper()
	raw, err := os.ReadFile(sharedRegressionRoot + "manifest.json")
	require.NoError(t, err)
	var manifest sharedManifest
	sharedDecode(t, raw, &manifest)
	require.Equal(t, "product-shein-shared", manifest.DatasetID)
	require.Equal(t, "1.0.0", manifest.Version)
	require.False(t, manifest.SemanticTime.IsZero())
	raw, err = os.ReadFile(sharedRegressionRoot + "cases.json")
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	require.Equal(t, manifest.ContentDigest, "sha256:"+hex.EncodeToString(sum[:]), "dataset bytes changed: explicit version/hash and independent expected review required")
	var cases []sharedCase
	sharedDecode(t, raw, &cases)
	require.Len(t, cases, 9)
	seen := map[string]bool{}
	for _, c := range cases {
		require.Regexp(t, `^PS-[0-9]{3}$`, c.CaseID)
		require.False(t, seen[c.CaseID], "duplicate case_id %s", c.CaseID)
		seen[c.CaseID] = true
		require.NotEmpty(t, c.Basis)
		require.NotEmpty(t, c.Layers)
		uniqueLayers := slices.Compact(slices.Sorted(slices.Values(c.Layers)))
		require.Len(t, c.Layers, len(uniqueLayers))
		for _, layer := range c.Layers {
			require.Contains(t, []string{"deterministic", "postgres"}, layer)
		}
		branches := 0
		if c.SourceError != "" {
			branches++
			require.NotNil(t, c.Source)
		}
		if c.ErrorCode != "" {
			branches++
			require.NotNil(t, c.Freshness)
			require.NotEmpty(t, c.Package)
		}
		if len(c.Reports) > 0 {
			branches++
		}
		require.Equal(t, 1, branches, "case %s must have exactly one executable outcome", c.CaseID)
		require.NotEqual(t, c.Source != nil, len(c.Package) > 0, "case %s needs exactly one input source", c.CaseID)
		if c.Source != nil && c.SourceError == "" {
			require.NotNil(t, c.Snapshot)
		}
		actions := map[contract.Action]bool{}
		for _, report := range c.Reports {
			require.Contains(t, []contract.Action{contract.Publish, contract.SaveDraft}, report.Action)
			require.False(t, actions[report.Action], "duplicate action")
			actions[report.Action] = true
			require.Contains(t, []contract.Status{contract.Ready, contract.ReadyWithWarnings, contract.Blocked}, report.Status)
		}
		if slices.Contains(c.Layers, "postgres") {
			require.NotNil(t, c.Source)
			require.NotNil(t, c.Snapshot)
			require.True(t, actions[contract.Publish] && actions[contract.SaveDraft])
		}
	}
	sha, err := exec.Command("git", "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	t.Logf("dataset=%s version=%s hash=%s code_sha=%s rule=%s binding=%s layer=%s", manifest.DatasetID, manifest.Version, manifest.ContentDigest, bytes.TrimSpace(sha), manifest.RuleVersion, manifest.BindingVersion, layer)
	return manifest, cases
}
