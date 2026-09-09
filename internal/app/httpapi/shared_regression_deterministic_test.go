package httpapi

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"task-processor/internal/listing/record"
	"task-processor/internal/marketplace/shein/draft"
	sheinvalidator "task-processor/internal/marketplace/shein/validator"
	contract "task-processor/internal/marketplace/validator"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/sourcing"
	sheinpub "task-processor/internal/publishing/shein"

	"github.com/stretchr/testify/require"
)

func TestSharedRegressionDeterministic(t *testing.T) {
	m, cases := loadSharedRegression(t, "deterministic")
	require.Equal(t, sheinvalidator.DiagnosticRuleVersion, m.RuleVersion)
	require.Equal(t, sheinvalidator.BindingVersion, m.BindingVersion)
	for _, c := range cases {
		t.Run(c.CaseID, func(t *testing.T) {
			require.Contains(t, c.Layers, "deterministic")
			raw := []byte(c.Package)
			if c.Source != nil {
				before, err := json.Marshal(c.Source)
				require.NoError(t, err)
				snapshot, err := sourcing.ToSnapshot(*c.Source)
				if c.SourceError != "" {
					require.Equal(t, "source_identity_required", c.SourceError)
					require.ErrorIs(t, err, sourcing.ErrSourceIdentityRequired)
					require.Equal(t, catalog.ProductSnapshot{}, snapshot)
					t.Log("layer=deterministic result=PASS source rejected before publication")
					return
				}
				require.NoError(t, err)
				require.NotNil(t, c.Snapshot)
				require.Equal(t, *c.Snapshot, snapshot, "manually reviewed canonical facts")
				cloned, err := catalog.CloneProductSnapshot(snapshot)
				require.NoError(t, err)
				require.Equal(t, *c.Snapshot, cloned)
				require.NoError(t, catalog.ValidatePublishRequest(catalog.PublishRequest{Identity: catalog.SnapshotIdentity{TenantID: "200", ProductKey: c.CaseID}, PublicationID: c.CaseID, Snapshot: cloned}))
				raw, err = (draft.Builder{}).Build(context.Background(), cloned, sharedApprovedInventory(c, 1), sharedRecordInput(c, 1))
				require.NoError(t, err)
				assertSharedIncompletePackage(t, raw, c)
				after, err := json.Marshal(c.Source)
				require.NoError(t, err)
				require.Equal(t, before, after, "source evidence mutated")
			}
			if c.ErrorCode != "" {
				req := sharedBoundRequest(m, raw, contract.Publish)
				require.NotNil(t, c.Freshness)
				req.Freshness = *c.Freshness
				got, err := (sheinvalidator.DiagnosticValidator{}).Validate(req)
				var typed *contract.Error
				require.ErrorAs(t, err, &typed)
				require.Equal(t, c.ErrorCode, typed.Code)
				require.Equal(t, contract.DiagnosticResult{}, got)
				require.NotNil(t, typed.Freshness)
				require.Equal(t, c.Freshness.Status, typed.Freshness.Status)
				require.Equal(t, c.Freshness.Coverage, typed.Freshness.Coverage)
				require.Equal(t, c.FreshnessCauses, typed.Freshness.Causes)
				t.Log("layer=deterministic result=PASS adverse evidence retained")
				return
			}
			digests := map[string]bool{}
			for _, want := range c.Reports {
				t.Run(string(want.Action), func(t *testing.T) {
					req := sharedBoundRequest(m, raw, want.Action)
					got, err := (sheinvalidator.DiagnosticValidator{}).Validate(req)
					require.NoError(t, err)
					assertSharedReport(t, m, want, got, c.Source != nil)
					require.Equal(t, m.SemanticTime, got.Input.ReadAt)
					require.Equal(t, m.SemanticTime, got.Input.EvaluatedAt)
					require.False(t, digests[got.Input.Digest], "different actions must have different bindings")
					digests[got.Input.Digest] = true
					for range 3 {
						again, err := (sheinvalidator.DiagnosticValidator{}).Validate(req)
						require.NoError(t, err)
						require.Equal(t, got, again, "repeatability supplements the independent semantic oracle")
					}
					if c.Binding != nil {
						require.Equal(t, c.Binding.Digest, got.Input.Digest, "existing independently reviewed fixed vector")
						req.ExpectedDigest = c.Binding.Digest
						req.Input = c.Binding.Equivalent
						equivalent, err := (sheinvalidator.DiagnosticValidator{}).Validate(req)
						require.NoError(t, err)
						require.Equal(t, got, equivalent)
						req.Input = c.Binding.Changed
						failed, err := (sheinvalidator.DiagnosticValidator{}).Validate(req)
						var typed *contract.Error
						require.ErrorAs(t, err, &typed)
						require.Equal(t, contract.StaleInput, typed.Code)
						require.Equal(t, "expected_digest", typed.Field)
						require.Equal(t, contract.DiagnosticResult{}, failed)
						req.ExpectedDigest = ""
						changed, err := (sheinvalidator.DiagnosticValidator{}).Validate(req)
						require.NoError(t, err)
						require.NotEqual(t, c.Binding.Digest, changed.Input.Digest)
					}
					t.Logf("layer=deterministic result=PASS action=%s digest=%s", want.Action, got.Input.Digest)
				})
			}
		})
	}
}

func sharedRecordInput(c sharedCase, version uint64) record.Input {
	return record.Input{ProductKey: c.CaseID, SnapshotVersion: version, StoreID: "11111111-1111-4111-8111-111111111111", Country: "US", Language: "en", Action: contract.SaveDraft}
}

func sharedApprovedInventory(c sharedCase, version uint64) productasset.ApprovedAssetInventory {
	return productasset.ApprovedAssetInventory{
		Scope:  productasset.InventoryScope{TenantID: "200", ProductKey: c.CaseID, TargetPlatform: "shein", SourceSnapshotVersion: version},
		Assets: []productasset.ApprovedAsset{{ID: "controlled-approved-main", RunID: "controlled-run", PlanRevision: 1, SlotID: "main", Attempt: 1, Role: productasset.RoleMain, URL: "https://fixtures.invalid/approved/main.jpg"}},
	}
}

func sharedBoundRequest(m sharedManifest, raw []byte, action contract.Action) contract.BoundRequest[[]byte] {
	return contract.BoundRequest[[]byte]{Input: raw, Target: contract.Target{Marketplace: "shein"}, Action: action, RuleVersion: m.RuleVersion, BindingVersion: m.BindingVersion, ReadAt: m.SemanticTime, EvaluatedAt: m.SemanticTime, Freshness: contract.ExternalFreshness{Status: contract.NotEvaluated}}
}

func assertSharedIncompletePackage(t *testing.T, raw []byte, c sharedCase) {
	t.Helper()
	pkg, err := sheinpub.DecodePersistedPackageStrict(raw)
	require.NoError(t, err)
	require.NotNil(t, pkg.Images)
	require.Contains(t, pkg.Images.MainImage, "https://fixtures.invalid/")
	require.True(t, sheinpub.HasSubmitImage(pkg), "controlled approved image must become draft imagery")
	for _, image := range c.Source.AssetCandidates {
		require.NotContains(t, string(raw), image.URL)
	}
}

func assertSharedReport(t *testing.T, m sharedManifest, want sharedReport, got contract.DiagnosticResult, hasExactApprovedImage bool) {
	t.Helper()
	require.True(t, got.DiagnosticOnly)
	require.Equal(t, "shein.offline_package", got.Scope)
	require.Equal(t, contract.Target{Marketplace: "shein"}, got.Target)
	require.Equal(t, want.Action, got.Action)
	require.Equal(t, m.RuleVersion, got.RuleVersion)
	require.Equal(t, m.BindingVersion, got.Input.BindingVersion)
	require.True(t, contract.ValidContentDigest(got.Input.Digest))
	require.Equal(t, want.Status, got.OfflineChecks.Status)
	require.Equal(t, want.DraftAllowsBlockers, got.ActionPolicy.ReadinessBlockersAllowed)
	require.Equal(t, contract.NotEvaluated, got.Freshness.Status)
	require.Empty(t, got.Freshness.Coverage)
	require.Nil(t, got.Freshness.Evidence)
	require.Equal(t, []string{"external_package_freshness", "online_template_freshness", "store_authorization", "cookie", "pod", "human_review", "approved_asset_provenance_and_consent", "submission_gate"}, got.NotEvaluated)
	require.Equal(t, map[string]string{"external_package_freshness": "no_authoritative_package_freshness"}, got.NotEvaluatedReasons)
	keys := func(checks []contract.Check) []string {
		out := []string{}
		for _, check := range checks {
			require.Contains(t, got.OfflineChecks.Checks, check)
			require.NotEmpty(t, check.Category)
			require.NotEmpty(t, check.Paths)
			require.NotEmpty(t, check.Guidance)
			out = append(out, check.Rule+"/"+check.Code)
		}
		slices.Sort(out)
		return out
	}
	blockers, warnings := keys(got.OfflineChecks.Blockers), keys(got.OfflineChecks.Warnings)
	expectedBlockers := append([]string(nil), want.Blockers...)
	if hasExactApprovedImage {
		expectedBlockers = slices.DeleteFunc(expectedBlockers, func(value string) bool { return value == "images/image_upload_failed" })
		require.NotContains(t, blockers, "images/image_upload_failed", "the exact approved main image must discharge the image blocker")
	}
	if want.Exact {
		rules := []string{}
		for _, check := range got.OfflineChecks.Checks {
			rules = append(rules, check.Rule)
		}
		require.ElementsMatch(t, []string{"category", "category_review", "attributes", "attribute_review", "sale_attributes", "request_draft", "preview_product", "images", "final_images", "variant_image_coverage", "variants", "pricing", "final_review", "manual_notes", "source_facts"}, rules, "complete synthetic rule inventory")
		require.ElementsMatch(t, expectedBlockers, blockers)
		require.ElementsMatch(t, want.Warnings, warnings)
	} else {
		for _, key := range expectedBlockers {
			require.Contains(t, blockers, key)
		}
		for _, key := range want.Warnings {
			require.Contains(t, warnings, key)
		}
	}
}
