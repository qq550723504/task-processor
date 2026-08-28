package tests

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	listingKitRepository         = "qq550723504/task-processor"
	listingKitAPIOIDCEnvironment = "listingkit-api-production"
	listingKitUIOIDCEnvironment  = "listingkit-ui-production"
	listingKitAPIOIDCAudience    = "api://listingkit-api-production"
	listingKitUIOIDCAudience     = "api://listingkit-ui-production"
	listingKitAPIWorkflowRef     = listingKitRepository + "/.github/workflows/listingkit-deploy.yml@refs/heads/main"
	listingKitUIWorkflowRef      = listingKitRepository + "/.github/workflows/listingkit-ui-deploy.yml@refs/heads/main"
)

func TestListingKitReleaseAuthorityUsesDistinctOIDCIdentities(t *testing.T) {
	t.Parallel()

	api := loadReleaseAuthorityWorkflow(t, "listingkit-deploy.yml")
	ui := loadReleaseAuthorityWorkflow(t, "listingkit-ui-deploy.yml")

	assertOIDCDeployJob(t, api, "deploy-api", listingKitAPIOIDCEnvironment, listingKitAPIOIDCAudience)
	assertOIDCDeployJob(t, ui, "deploy-ui", listingKitUIOIDCEnvironment, listingKitUIOIDCAudience)

	if listingKitAPIOIDCEnvironment == listingKitUIOIDCEnvironment || listingKitAPIOIDCAudience == listingKitUIOIDCAudience {
		t.Fatal("API and UI release identities must not share an environment or audience")
	}
}

func TestListingKitReleaseAuthorityMachinePolicyOwnsAllSecurityInputs(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "release-authority", "release-policy.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read machine release policy: %v", err)
	}

	var policy struct {
		ReleaseAuthority struct {
			Repository string `yaml:"repository"`
			Namespace  string `yaml:"namespace"`
			OIDC       struct {
				Issuer         string `yaml:"issuer"`
				UsernamePrefix string `yaml:"usernamePrefix"`
				Identities     map[string]struct {
					Environment string `yaml:"environment"`
					Audience    string `yaml:"audience"`
					Subject     string `yaml:"subject"`
					WorkflowRef string `yaml:"workflowRef"`
					Job         string `yaml:"job"`
				} `yaml:"identities"`
			} `yaml:"oidc"`
			Tools struct {
				ConftestVersion string `yaml:"conftestVersion"`
				OPAVersion      string `yaml:"opaVersion"`
			} `yaml:"tools"`
			ProducerRouting struct {
				ImageLabel string `yaml:"imageLabel"`
				Annotation string `yaml:"annotation"`
				Contract   string `yaml:"contract"`
			} `yaml:"producerRouting"`
			ReleaseIdentity struct {
				Owner       string `yaml:"owner"`
				Deployment  string `yaml:"deployment"`
				Step        string `yaml:"step"`
				Annotations struct {
					RunID      string `yaml:"runId"`
					RunAttempt string `yaml:"runAttempt"`
					Image      string `yaml:"image"`
					Routing    string `yaml:"routingContract"`
				} `yaml:"annotations"`
			} `yaml:"releaseIdentity"`
			RBAC struct {
				Kustomization string   `yaml:"kustomization"`
				Paths         []string `yaml:"paths"`
			} `yaml:"rbac"`
			ReleaseGateRunners struct {
				Manifest  string   `yaml:"manifest"`
				HoldImage string   `yaml:"holdImage"`
				Names     []string `yaml:"names"`
			} `yaml:"releaseGateRunners"`
			ProtectedResources []struct {
				Owner         string   `yaml:"owner"`
				Resource      string   `yaml:"resource"`
				ResourceNames []string `yaml:"resourceNames"`
				Verbs         []string `yaml:"verbs"`
			} `yaml:"protectedResources"`
			Documentation struct {
				Paths              []string `yaml:"paths"`
				CanonicalWorkflows []string `yaml:"canonicalWorkflows"`
			} `yaml:"documentation"`
		} `yaml:"releaseAuthority"`
	}
	if err := yaml.Unmarshal(content, &policy); err != nil {
		t.Fatalf("decode machine release policy: %v", err)
	}

	authority := policy.ReleaseAuthority
	if authority.Repository != "qq550723504/task-processor" || authority.Namespace != "task-processor" {
		t.Fatalf("release policy repository/namespace drifted: %#v", authority)
	}
	if authority.OIDC.Issuer != "https://token.actions.githubusercontent.com" || authority.OIDC.UsernamePrefix != "github:" {
		t.Fatalf("release policy OIDC trust drifted: %#v", authority.OIDC)
	}
	api := authority.OIDC.Identities["api"]
	ui := authority.OIDC.Identities["ui"]
	if api.Environment != listingKitAPIOIDCEnvironment || ui.Environment != listingKitUIOIDCEnvironment ||
		api.Audience != listingKitAPIOIDCAudience || ui.Audience != listingKitUIOIDCAudience ||
		api.Subject == "" || ui.Subject == "" || api.Subject == ui.Subject ||
		api.WorkflowRef != listingKitAPIWorkflowRef || ui.WorkflowRef != listingKitUIWorkflowRef || api.Job == ui.Job {
		t.Fatalf("release policy must own distinct API/UI identities: api=%#v ui=%#v", api, ui)
	}
	if authority.Tools.ConftestVersion == "" || authority.Tools.OPAVersion == "" || strings.Contains(authority.Tools.ConftestVersion, "latest") || strings.Contains(authority.Tools.OPAVersion, "latest") {
		t.Fatalf("release policy must pin Conftest and embedded OPA versions: %#v", authority.Tools)
	}
	if authority.ProducerRouting.ImageLabel != "org.opencontainers.image.listingkit.image-agent-routing" ||
		authority.ProducerRouting.Annotation != "listingkit.sh/image-agent-routing-contract" ||
		authority.ProducerRouting.Contract != "image-agent-v3-new-starts-v1" {
		t.Fatalf("release policy must own the immutable v3 producer-routing contract: %#v", authority.ProducerRouting)
	}
	if authority.ReleaseIdentity.Owner != "api" || authority.ReleaseIdentity.Deployment != "product-listing-api" ||
		authority.ReleaseIdentity.Step != "Stamp API release identity and restart Pods" ||
		authority.ReleaseIdentity.Annotations.RunID != "listingkit.sh/api-release-run-id" ||
		authority.ReleaseIdentity.Annotations.RunAttempt != "listingkit.sh/api-release-run-attempt" ||
		authority.ReleaseIdentity.Annotations.Image != "listingkit.sh/api-release-image" ||
		authority.ReleaseIdentity.Annotations.Routing != "listingkit.sh/image-agent-routing-contract" {
		t.Fatalf("release policy must own the exact API release identity stamp: %#v", authority.ReleaseIdentity)
	}
	if authority.RBAC.Kustomization == "" || len(authority.RBAC.Paths) < 6 || len(authority.ProtectedResources) == 0 {
		t.Fatalf("release policy must own RBAC paths and protected resources: %#v", authority.RBAC)
	}
	if authority.ReleaseGateRunners.Manifest != "deployments/kubernetes/listingkit-workbench/release-authority/listingkit-release-gate-runners.yaml" ||
		authority.ReleaseGateRunners.HoldImage != releaseGateHoldImage || len(authority.ReleaseGateRunners.Names) != 4 {
		t.Fatalf("release policy must own the aggregate runner manifest and exact inventory: %#v", authority.ReleaseGateRunners)
	}
	if len(authority.Documentation.Paths) == 0 || len(authority.Documentation.CanonicalWorkflows) != 2 {
		t.Fatalf("release policy must own supported release docs and canonical workflow links: %#v", authority.Documentation)
	}
}

func TestListingKitReleaseAuthorityCrossFilePolicyRejectsReleaseGateInvocationOverrides(t *testing.T) {
	t.Parallel()

	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	workflow, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		from string
		to   string
	}{
		{name: "missing run ID", from: "            --run-id \"$GITHUB_RUN_ID\" \\\n", to: ""},
		{name: "static run ID override", from: "            --run-id \"$GITHUB_RUN_ID\" \\\n", to: "            --run-id \"$GITHUB_RUN_ID\" \\\n            --run-id \"424242\" \\\n"},
		{name: "duplicate run attempt", from: "            --run-attempt \"$GITHUB_RUN_ATTEMPT\" \\\n", to: "            --run-attempt \"$GITHUB_RUN_ATTEMPT\" \\\n            --run-attempt \"$GITHUB_RUN_ATTEMPT\" \\\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			mutated := strings.Replace(string(workflow), test.from, test.to, 1)
			if mutated == string(workflow) {
				t.Fatalf("test fixture did not mutate %q", test.name)
			}
			output, runErr := runReleaseAuthorityCrossFilePolicy(t, mutated, "")
			if runErr == nil {
				t.Fatalf("cross-file release-authority policy accepted %s:\n%s", test.name, output)
			}
		})
	}
}

func TestListingKitReleaseAuthorityCrossFilePolicyRejectsNativeSidecarRunner(t *testing.T) {
	t.Parallel()

	workflow, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	runnerPath := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "release-authority", "listingkit-release-gate-runners.yaml")
	runners, err := os.ReadFile(runnerPath)
	if err != nil {
		t.Fatal(err)
	}
	mutatedRunners := strings.Replace(string(runners), "        - name: release-gate\n          image:", "        - name: release-gate\n          restartPolicy: Always\n          image:", 1)
	if mutatedRunners == string(runners) {
		t.Fatal("test fixture did not add restartPolicy to release-gate init container")
	}

	output, runErr := runReleaseAuthorityCrossFilePolicy(t, string(workflow), mutatedRunners)
	if runErr == nil {
		t.Fatalf("cross-file release-authority policy accepted a native-sidecar release-gate runner:\n%s", output)
	}
}

func TestListingKitReleaseAuthorityOIDCConfiguratorFailsClosedOnClaimMismatch(t *testing.T) {
	t.Parallel()

	script, err := filepath.Abs(filepath.Join("..", "scripts", "listingkit-configure-github-oidc-kubeconfig.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(script); err != nil {
		t.Fatalf("OIDC kubeconfig configurator is missing: %v", err)
	}

	now := time.Now().UTC()
	claims := map[string]interface{}{
		"iss":          "https://token.actions.githubusercontent.com",
		"aud":          listingKitAPIOIDCAudience,
		"sub":          "repo:qq550723504/task-processor:environment:" + listingKitAPIOIDCEnvironment,
		"repository":   "qq550723504/task-processor",
		"environment":  listingKitAPIOIDCEnvironment,
		"workflow_ref": listingKitAPIWorkflowRef,
		"nbf":          now.Add(-time.Minute).Unix(),
		"exp":          now.Add(10 * time.Minute).Unix(),
	}

	run := func(t *testing.T, tokenClaims map[string]interface{}, audience, workflowFlag, workflowValue string) ([]byte, string, error) {
		t.Helper()
		dir := t.TempDir()
		tokenPath := filepath.Join(dir, "oidc-token")
		kubeconfigPath := filepath.Join(dir, "kubeconfig")
		if err := os.WriteFile(tokenPath, []byte(unsignedJWT(t, tokenClaims)), 0o600); err != nil {
			t.Fatal(err)
		}
		args := []string{filepath.ToSlash(script),
			"--token-file", filepath.ToSlash(tokenPath),
			"--kubeconfig", filepath.ToSlash(kubeconfigPath),
			"--cluster-server", "https://kubernetes.example.invalid",
			"--cluster-ca-b64", base64.StdEncoding.EncodeToString([]byte("test-ca")),
			"--issuer", "https://token.actions.githubusercontent.com",
			"--audience", audience,
			"--subject", "repo:qq550723504/task-processor:environment:" + listingKitAPIOIDCEnvironment,
			"--repository", listingKitRepository,
			"--environment", listingKitAPIOIDCEnvironment,
			workflowFlag, workflowValue,
		}
		command := exec.Command(preflightBash(t), args...)
		output, runErr := command.CombinedOutput()
		return output, kubeconfigPath, runErr
	}

	t.Run("wrong audience writes no kubeconfig", func(t *testing.T) {
		output, kubeconfigPath, runErr := run(t, claims, listingKitUIOIDCAudience, "--workflow-ref", listingKitAPIWorkflowRef)
		if runErr == nil {
			t.Fatalf("wrong audience unexpectedly configured Kubernetes credentials: %s", output)
		}
		if _, statErr := os.Stat(kubeconfigPath); !os.IsNotExist(statErr) {
			t.Fatalf("claim mismatch must fail before writing kubeconfig, statErr=%v", statErr)
		}
	})

	for _, test := range []struct {
		name        string
		workflowRef interface{}
	}{
		{name: "feature branch", workflowRef: listingKitRepository + "/.github/workflows/listingkit-deploy.yml@refs/heads/feature/release"},
		{name: "tag", workflowRef: listingKitRepository + "/.github/workflows/listingkit-deploy.yml@refs/tags/listingkit-api-v-test"},
		{name: "trusted ref prefix lookalike", workflowRef: listingKitAPIWorkflowRef + "-attacker"},
		{name: "missing ref", workflowRef: nil},
	} {
		t.Run(test.name+" writes no kubeconfig", func(t *testing.T) {
			mismatched := cloneOIDCClaims(claims)
			if test.workflowRef == nil {
				delete(mismatched, "workflow_ref")
			} else {
				mismatched["workflow_ref"] = test.workflowRef
			}
			output, kubeconfigPath, runErr := run(t, mismatched, listingKitAPIOIDCAudience, "--workflow-ref", listingKitAPIWorkflowRef)
			if runErr == nil {
				t.Fatalf("untrusted workflow ref unexpectedly configured Kubernetes credentials: %s", output)
			}
			if _, statErr := os.Stat(kubeconfigPath); !os.IsNotExist(statErr) {
				t.Fatalf("workflow ref mismatch must fail before writing kubeconfig, statErr=%v", statErr)
			}
		})
	}

	t.Run("path only configuration is rejected", func(t *testing.T) {
		output, kubeconfigPath, runErr := run(t, claims, listingKitAPIOIDCAudience, "--workflow-path", ".github/workflows/listingkit-deploy.yml")
		if runErr == nil {
			t.Fatalf("path-only OIDC configuration unexpectedly created Kubernetes credentials: %s", output)
		}
		if _, statErr := os.Stat(kubeconfigPath); !os.IsNotExist(statErr) {
			t.Fatalf("path-only configuration must fail before writing kubeconfig, statErr=%v", statErr)
		}
	})

	t.Run("exact claims create ephemeral kubeconfig without logging token", func(t *testing.T) {
		output, kubeconfigPath, runErr := run(t, claims, listingKitAPIOIDCAudience, "--workflow-ref", listingKitAPIWorkflowRef)
		if runErr != nil {
			t.Fatalf("exact OIDC claims failed: %v\n%s", runErr, output)
		}
		kubeconfig, readErr := os.ReadFile(kubeconfigPath)
		if readErr != nil {
			t.Fatalf("read ephemeral kubeconfig: %v", readErr)
		}
		token := unsignedJWT(t, claims)
		if !strings.Contains(string(kubeconfig), token) {
			t.Fatal("ephemeral kubeconfig is missing the short-lived OIDC token")
		}
		if strings.Contains(string(output), token) {
			t.Fatal("OIDC token must never be printed")
		}
	})
}

func TestListingKitReleaseAuthorityBindsWorkflowRefToMain(t *testing.T) {
	t.Parallel()

	authenticationPath := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "release-authority", "kubernetes-authentication-config.example.yaml")
	authentication, err := os.ReadFile(authenticationPath)
	if err != nil {
		t.Fatal(err)
	}
	authenticationText := string(authentication)
	for _, workflowRef := range []string{listingKitAPIWorkflowRef, listingKitUIWorkflowRef} {
		if !strings.Contains(authenticationText, workflowRef) {
			t.Errorf("Kubernetes AuthenticationConfiguration must bind exact workflow ref %q", workflowRef)
		}
	}
	if strings.Contains(authenticationText, "workflow_ref.startsWith") {
		t.Error("Kubernetes AuthenticationConfiguration must not authorize a workflow-ref prefix")
	}

	for workflow, workflowRef := range map[string]string{
		"listingkit-deploy.yml":    listingKitAPIWorkflowRef,
		"listingkit-ui-deploy.yml": listingKitUIWorkflowRef,
	} {
		parsed := loadReleaseAuthorityWorkflow(t, workflow)
		jobName := "deploy-api"
		if workflow == "listingkit-ui-deploy.yml" {
			jobName = "deploy-ui"
		}
		job := parsed.Jobs[jobName]
		var kubeconfigRun string
		for _, step := range job.Steps {
			if strings.Contains(step.Run, "listingkit-configure-github-oidc-kubeconfig.sh") {
				kubeconfigRun = step.Run
				break
			}
		}
		if !strings.Contains(kubeconfigRun, "--workflow-ref "+workflowRef) {
			t.Errorf("%s must pass exact trusted workflow ref %q", workflow, workflowRef)
		}
		if strings.Contains(kubeconfigRun, "--workflow-path") {
			t.Errorf("%s must not configure path-only OIDC trust", workflow)
		}
	}
}

func TestListingKitReleaseAuthorityNegativeFixturesAreCheckedIn(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "policy", "listingkit-release-authority", "fixtures", "negative")
	for _, name := range []string{
		"kube-config-fallback.yaml",
		"shared-environment.yaml",
		"shared-subject.yaml",
		"missing-id-token.yaml",
		"wrong-audience.yaml",
		"workflow-job-owner-drift.yaml",
		"workflow-step-owner-drift.yaml",
		"job-reusable-workflow.yaml",
		"step-composite-action.yaml",
		"with-target-bypass.yaml",
		"excess-rbac-verbs.yaml",
		"excess-rbac-resources.yaml",
		"excess-rbac-resource-names.yaml",
		"policy-drift.yaml",
		"release-identity-owner-drift.yaml",
		"workflow-ref-feature-branch.yaml",
		"workflow-ref-tag.yaml",
		"workflow-ref-path-prefix-lookalike.yaml",
		"workflow-ref-missing.yaml",
		"trusted-workflow-ref-policy-drift.yaml",
	} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("missing release-authority negative fixture %s: %v", name, err)
		}
	}
}

func cloneOIDCClaims(source map[string]interface{}) map[string]interface{} {
	clone := make(map[string]interface{}, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func unsignedJWT(t *testing.T, claims map[string]interface{}) string {
	t.Helper()
	header, err := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload) + ".test-signature"
}

func TestListingKitReleaseAuthorityHasNoLongLivedKubeconfigFallback(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"listingkit-deploy.yml", "listingkit-ui-deploy.yml"} {
		path := filepath.Join("..", ".github", "workflows", name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(content), "KUBE_CONFIG") {
			t.Errorf("%s retains a long-lived KUBE_CONFIG credential path", name)
		}
		for _, required := range []string{"core.getIDToken", "K8S_CLUSTER_SERVER", "K8S_CLUSTER_CA_B64"} {
			if !strings.Contains(string(content), required) {
				t.Errorf("%s is missing fail-closed OIDC kubeconfig input %q", name, required)
			}
		}
	}
}

func TestListingKitReleaseAuthorityBootstrapIsStandaloneAndCreateFree(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "release-authority")
	for _, name := range []string{
		"kustomization.yaml",
		"release-policy.yaml",
		"kubernetes-authentication-config.example.yaml",
		"listingkit-api-release-role.yaml",
		"listingkit-api-release-rolebinding.yaml",
		"listingkit-ui-release-role.yaml",
		"listingkit-ui-release-rolebinding.yaml",
	} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("release-authority bootstrap is missing %s: %v", name, err)
		}
	}

	for _, name := range []string{"listingkit-api-release-role.yaml", "listingkit-ui-release-role.yaml"} {
		path := filepath.Join(root, name)
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var role struct {
			Rules []struct {
				ResourceNames []string `yaml:"resourceNames"`
				Verbs         []string `yaml:"verbs"`
			} `yaml:"rules"`
		}
		if err := yaml.Unmarshal(content, &role); err != nil {
			t.Fatalf("decode %s: %v", path, err)
		}
		for i, rule := range role.Rules {
			if len(rule.ResourceNames) == 0 {
				t.Errorf("%s rule %d is not constrained by resourceNames", name, i)
			}
			for _, verb := range rule.Verbs {
				if verb == "create" || verb == "*" || verb == "deletecollection" {
					t.Errorf("%s rule %d grants forbidden top-level verb %q", name, i, verb)
				}
			}
		}
	}
}

type releaseAuthorityWorkflow struct {
	Jobs map[string]struct {
		Environment struct {
			Name string `yaml:"name"`
		} `yaml:"environment"`
		Permissions map[string]string `yaml:"permissions"`
		Steps       []struct {
			Name string            `yaml:"name"`
			Uses string            `yaml:"uses"`
			Run  string            `yaml:"run"`
			Env  map[string]string `yaml:"env"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

func runReleaseAuthorityCrossFilePolicy(t *testing.T, mutatedWorkflow, mutatedRunnerManifest string) (string, error) {
	t.Helper()

	repoRoot, err := filepath.Abs(filepath.Join(".."))
	if err != nil {
		t.Fatal(err)
	}
	fixtureRoot := t.TempDir()
	workflowPath := filepath.Join(fixtureRoot, ".github", "workflows", "listingkit-deploy.yml")
	if err := os.MkdirAll(filepath.Dir(workflowPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workflowPath, []byte(mutatedWorkflow), 0o600); err != nil {
		t.Fatal(err)
	}
	runnerManifestPath := "deployments/kubernetes/listingkit-workbench/release-authority/listingkit-release-gate-runners.yaml"
	if mutatedRunnerManifest != "" {
		runnerFixturePath := filepath.Join(fixtureRoot, "deployments", "kubernetes", "listingkit-workbench", "release-authority", "listingkit-release-gate-runners.yaml")
		if err := os.MkdirAll(filepath.Dir(runnerFixturePath), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(runnerFixturePath, []byte(mutatedRunnerManifest), 0o600); err != nil {
			t.Fatal(err)
		}
		runnerManifestPath = "/fixture/deployments/kubernetes/listingkit-workbench/release-authority/listingkit-release-gate-runners.yaml"
	}

	arguments := []string{
		"run", "--rm",
		"--volume", repoRoot + ":/project",
		"--volume", fixtureRoot + ":/fixture",
		"--workdir", "/project",
		"openpolicyagent/conftest@sha256:5fd81e332d7e4bc01daf3ef35371800a9a9720a30c0c37a78de0c5fbe4b6d622",
		"test", "--combine",
		"--namespace", "listingkit_release_authority",
		"--policy", "policy/listingkit-release-authority",
		"deployments/kubernetes/listingkit-workbench/release-authority/release-policy.yaml",
		"deployments/kubernetes/listingkit-workbench/release-authority/kubernetes-authentication-config.example.yaml",
		"deployments/kubernetes/listingkit-workbench/release-authority/listingkit-api-release-role.yaml",
		"deployments/kubernetes/listingkit-workbench/release-authority/listingkit-api-release-rolebinding.yaml",
		"deployments/kubernetes/listingkit-workbench/release-authority/listingkit-ui-release-role.yaml",
		"deployments/kubernetes/listingkit-workbench/release-authority/listingkit-ui-release-rolebinding.yaml",
		"deployments/kubernetes/listingkit-workbench/release-authority/kustomization.yaml",
		runnerManifestPath,
		"/fixture/.github/workflows/listingkit-deploy.yml",
		".github/workflows/listingkit-ui-deploy.yml",
	}
	command := exec.Command("docker", arguments...)
	command.Dir = repoRoot
	output, runErr := command.CombinedOutput()
	return string(output), runErr
}

func loadReleaseAuthorityWorkflow(t *testing.T, name string) releaseAuthorityWorkflow {
	t.Helper()
	path := filepath.Join("..", ".github", "workflows", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var workflow releaseAuthorityWorkflow
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return workflow
}

func assertOIDCDeployJob(t *testing.T, workflow releaseAuthorityWorkflow, jobName, environment, audience string) {
	t.Helper()
	job, ok := workflow.Jobs[jobName]
	if !ok {
		t.Fatalf("workflow is missing %s", jobName)
	}
	if job.Environment.Name != environment {
		t.Errorf("%s environment=%q want %q", jobName, job.Environment.Name, environment)
	}
	if got := job.Permissions["id-token"]; got != "write" {
		t.Errorf("%s id-token permission=%q want write", jobName, got)
	}

	var tokenStepFound bool
	for _, step := range job.Steps {
		if !strings.Contains(step.Run, "core.getIDToken") && !strings.Contains(step.Uses, "actions/github-script") {
			continue
		}
		tokenStepFound = true
		if got := step.Env["LISTINGKIT_OIDC_AUDIENCE"]; got != audience {
			t.Errorf("%s OIDC audience=%q want %q", jobName, got, audience)
		}
	}
	if !tokenStepFound {
		t.Errorf("%s has no official GitHub OIDC token step", jobName)
	}
}
