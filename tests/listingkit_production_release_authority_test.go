package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestListingKitProductionReleaseAuthorityRejectsCallerChosenProductionInputs
// catches a release workflow that can be redirected to a tag, arbitrary commit,
// or caller-provided image.  Those values are not provenance; only the exact
// main workflow commit or a previously verified release attestation is.
func TestListingKitProductionReleaseAuthorityRejectsCallerChosenProductionInputs(t *testing.T) {
	workflow := loadReleaseWorkflow(t, filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml"))
	if !strings.Contains(workflow.Jobs["deploy-api"].If, "github.ref == 'refs/heads/main'") {
		t.Fatal("tag-triggered builds must be unable to enter the production API deploy job")
	}
	prepare := workflow.Jobs["prepare"]
	if strings.Contains(joinWorkflowRuns(prepare.Steps), "inputs.source_ref") ||
		strings.Contains(joinWorkflowRuns(prepare.Steps), "inputs.api_image_digest") {
		t.Fatal("production API deploy must not accept caller-selected source refs or image digests")
	}
	workflowSource, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(workflowSource), "github.workflow_sha") {
		t.Fatal("normal production release must bind its source to the exact workflow commit")
	}

	verifier, err := os.ReadFile(filepath.Join("..", "scripts", "verify-listingkit-api-release-attestation.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"listingkit-api-release-gate/v2",
		"workflow_ref",
		"worker_wire_contract",
		"worker_replay_contract",
		"schema_contract",
		"workflow head SHA does not match attested source",
	} {
		if !strings.Contains(string(verifier), required) {
			t.Errorf("attestation verifier must bind rollback to %q", required)
		}
	}
}

// TestListingKitProductionReleaseAuthorityUsesFreshOIDCAndImmutableActions
// catches the expired-token pattern: a single OIDC credential is reused across
// multiple 900-second gates or a mutable third-party action can change code.
func TestListingKitProductionReleaseAuthorityUsesFreshOIDCAndImmutableActions(t *testing.T) {
	for _, workflowName := range []string{"listingkit-deploy.yml", "listingkit-ui-deploy.yml"} {
		workflow := loadReleaseWorkflow(t, filepath.Join("..", ".github", "workflows", workflowName))
		for _, job := range workflow.Jobs {
			for _, step := range job.Steps {
				if step.Uses == "" || strings.HasPrefix(step.Uses, "./") {
					continue
				}
				at := strings.LastIndex(step.Uses, "@")
				if at < 1 || len(step.Uses[at+1:]) != 40 {
					t.Errorf("%s step %q must pin its action to a full commit SHA, got %q", workflowName, step.Name, step.Uses)
				}
			}
		}
	}

	api := loadReleaseWorkflow(t, filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml"))
	refreshes := 0
	for _, step := range api.Jobs["deploy-api"].Steps {
		if strings.Contains(step.Run, "listingkit-refresh-github-oidc-kubeconfig.sh") {
			refreshes++
		}
	}
	if refreshes < 6 {
		t.Fatalf("API release must refresh OIDC immediately before each long gate or mutation group, got %d refreshes", refreshes)
	}
}

// TestListingKitUIReleaseHasAnExclusiveAuthorizationConfigMap catches a UI
// principal that can rewrite API/worker runtime configuration or shares a
// ConfigMap with a different release owner.
func TestListingKitUIReleaseHasAnExclusiveAuthorizationConfigMap(t *testing.T) {
	uiManifest, err := os.ReadFile(filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "base", "listingkit-ui-deployment.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(uiManifest), "listingkit-ui-auth-config") {
		t.Fatal("UI must consume its dedicated authorization ConfigMap")
	}
	sharedConfig, err := os.ReadFile(filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "base", "configmap.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	uiConfig, err := os.ReadFile(filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "base", "listingkit-ui-auth-config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(sharedConfig), "ZITADEL_SCOPES") || !strings.Contains(string(uiConfig), "ZITADEL_SCOPES") {
		t.Fatal("ZITADEL_SCOPES must exist only in the UI-owned ConfigMap")
	}
	role, err := os.ReadFile(filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "release-authority", "listingkit-ui-release-role.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(role), "listingkit-workbench-config") || !strings.Contains(string(role), "listingkit-ui-auth-config") {
		t.Fatal("UI release Role must mutate only listingkit-ui-auth-config")
	}
}

// TestListingKitUIReleaseRequestsExactProjectRoleClaim prevents the production
// release workflow from replacing provisioned project scopes with a generic
// role-only scope set that cannot identify the trusted ListingKit project.
func TestListingKitUIReleaseRequestsExactProjectRoleClaim(t *testing.T) {
	uiConfig, err := os.ReadFile(filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "base", "listingkit-ui-auth-config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(uiConfig), "ZITADEL_PROJECT_ID:") {
		t.Fatal("UI authorization ConfigMap must carry the public trusted ZITADEL project ID")
	}

	workflowSource, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "listingkit-ui-deploy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowSource)
	for _, required := range []string{
		".data.ZITADEL_PROJECT_ID",
		"urn:zitadel:iam:org:project:${project_id}:roles",
		"Invalid or missing ZITADEL_PROJECT_ID",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("production UI release must derive an exact project role scope; missing %q", required)
		}
	}
}
