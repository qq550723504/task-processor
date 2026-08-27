package tests

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

type renderedObjectIdentity struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
}

func TestListingKitImageAgentDeployRendersCanaryOnlyFromJobsBoundary(t *testing.T) {
	root := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench")
	for _, test := range []struct {
		name         string
		path         string
		wantWorkers  []string
		wantCanaries int
		wantObjects  int
	}{
		{name: "base", path: filepath.Join(root, "base"), wantWorkers: []string{"image-agent-temporal-worker", "image-agent-temporal-worker-v3"}},
		{name: "prod", path: filepath.Join(root, "overlays", "prod"), wantWorkers: []string{"image-agent-temporal-worker", "image-agent-temporal-worker-v3"}},
		{name: "jobs", path: filepath.Join(root, "jobs"), wantCanaries: 1, wantObjects: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			objects := renderKustomizeObjects(t, test.path)
			if test.wantObjects > 0 && len(objects) != test.wantObjects {
				t.Fatalf("%s render has %d objects, want %d", test.name, len(objects), test.wantObjects)
			}
			workers := map[string]bool{}
			canaries := 0
			for _, object := range objects {
				if object.Kind == "Deployment" {
					workers[object.Metadata.Name] = true
				}
				if object.Kind == "Job" && object.Metadata.Name == "image-agent-temporal-v3-canary" {
					canaries++
				}
			}
			for _, worker := range test.wantWorkers {
				if !workers[worker] {
					t.Errorf("%s render is missing long-lived worker %s", test.name, worker)
				}
			}
			if canaries != test.wantCanaries {
				t.Fatalf("%s render has %d image-agent canaries, want %d", test.name, canaries, test.wantCanaries)
			}
		})
	}
}

func TestListingKitAPIDeployExclusivelyOwnsCanaryMutationLifecycle(t *testing.T) {
	workflowDir := filepath.Join("..", ".github", "workflows")
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		t.Fatal(err)
	}
	wantOwner := "listingkit-deploy.yml"
	fragments := []string{
		"delete job image-agent-temporal-v3-canary",
		"--manifest .workflow-tools/deployments/kubernetes/listingkit-workbench/jobs/image-agent-temporal-v3-canary-job.yaml",
		"wait --for=condition=complete job/image-agent-temporal-v3-canary",
	}
	ownerContent := ""
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yml") && !strings.HasSuffix(entry.Name(), ".yaml")) {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(workflowDir, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if entry.Name() == wantOwner {
			ownerContent = string(content)
			continue
		}
		for _, fragment := range fragments {
			if strings.Contains(string(content), fragment) {
				t.Errorf("%s must not own canary lifecycle fragment %q", entry.Name(), fragment)
			}
		}
	}
	for _, fragment := range fragments {
		if strings.Count(ownerContent, fragment) != 1 {
			t.Errorf("%s must own exactly one canary lifecycle fragment %q", wantOwner, fragment)
		}
	}

	runbook, err := os.ReadFile(filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(runbook), "kustomize build . | kubectl apply -f -") {
		t.Fatal("generic production-overlay instructions must not apply the release directly")
	}
}

func renderKustomizeObjects(t *testing.T, path string) []renderedObjectIdentity {
	t.Helper()
	command := exec.Command("kubectl", "kustomize", path)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("kubectl kustomize %s: %v\n%s", path, err, output)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(output))
	var objects []renderedObjectIdentity
	for {
		var object renderedObjectIdentity
		err := decoder.Decode(&object)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode kubectl kustomize %s: %v", path, err)
		}
		if object.Kind != "" {
			objects = append(objects, object)
		}
	}
	return objects
}

type releaseWorkflow struct {
	Name        string                            `yaml:"name"`
	On          map[string]map[string]interface{} `yaml:"on"`
	Permissions map[string]string                 `yaml:"permissions"`
	Jobs        map[string]releaseWorkflowJob     `yaml:"jobs"`
}

type releaseWorkflowJob struct {
	Needs       interface{}           `yaml:"needs"`
	If          string                `yaml:"if"`
	Environment interface{}           `yaml:"environment"`
	Outputs     map[string]string     `yaml:"outputs"`
	Steps       []releaseWorkflowStep `yaml:"steps"`
}

type releaseWorkflowStep struct {
	Name string                 `yaml:"name"`
	ID   string                 `yaml:"id"`
	If   string                 `yaml:"if"`
	Uses string                 `yaml:"uses"`
	Run  string                 `yaml:"run"`
	Env  map[string]string      `yaml:"env"`
	With map[string]interface{} `yaml:"with"`
}

func TestListingKitDeployEmitsExactReleaseAttestationAfterAPIRollout(t *testing.T) {
	workflow := loadReleaseWorkflow(t, filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml"))
	steps := workflow.Jobs["deploy-api"].Steps
	canaryWait, apiWait, emit, upload := -1, -1, -1, -1
	for index, step := range steps {
		switch step.Name {
		case "Require image agent v3 compatibility canary success":
			canaryWait = index
		case "Wait for API rollout":
			apiWait = index
		case "Emit ListingKit API release attestation":
			emit = index
			for _, required := range []string{
				"listingkit-api-release-gate/v1",
				"${{ needs.prepare.outputs.source_ref }}",
				"$API_CANDIDATE_IMAGE",
				"$GITHUB_RUN_ID",
				"$GITHUB_RUN_ATTEMPT",
				"api_workflow_run_attempt",
				"expires_at",
				"@sha256:",
			} {
				if !strings.Contains(step.Run, required) {
					t.Errorf("release attestation emitter is missing %q", required)
				}
			}
		case "Upload ListingKit API release attestation":
			upload = index
			if step.Uses != "actions/upload-artifact@v4" {
				t.Errorf("release attestation must use upload-artifact@v4, got %q", step.Uses)
			}
			if got := stringValue(step.With["name"]); got != "listingkit-api-release-gate-${{ github.run_id }}-${{ github.run_attempt }}" {
				t.Errorf("attestation artifact name must be scoped to exact API run ID and attempt, got %q", got)
			}
			if _, overwrites := step.With["overwrite"]; overwrites {
				t.Error("attempt evidence must remain immutable; upload must not overwrite prior artifacts")
			}
		}
	}
	if canaryWait < 0 || apiWait < 0 || emit < 0 || upload < 0 || !(canaryWait < apiWait && apiWait < emit && emit < upload) {
		t.Fatalf("attestation must be emitted/uploaded only after canary and API rollout: canaryWait=%d apiWait=%d emit=%d upload=%d", canaryWait, apiWait, emit, upload)
	}
}

func TestListingKitUIDeployRequiresVerifiedExactAPIReleaseGate(t *testing.T) {
	workflow := loadReleaseWorkflow(t, filepath.Join("..", ".github", "workflows", "listingkit-ui-deploy.yml"))
	if workflow.Permissions["actions"] != "read" {
		t.Fatalf("UI workflow actions permission=%q want read", workflow.Permissions["actions"])
	}
	workflowRun := workflow.On["workflow_run"]
	if !strings.Contains(stringValue(workflowRun["workflows"]), "ListingKit API Deploy") {
		t.Fatalf("UI workflow_run must name ListingKit API Deploy, got %#v", workflowRun)
	}
	dispatch := workflow.On["workflow_dispatch"]
	dispatchInputs := strings.ReplaceAll(stringValue(dispatch["inputs"]), " ", "")
	if !strings.Contains(dispatchInputs, "release_gate_run_id") || !strings.Contains(dispatchInputs, "release_gate_run_attempt") || strings.Count(dispatchInputs, "required:true") < 2 {
		t.Fatalf("manual UI production deploy must require explicit release_gate_run_id and release_gate_run_attempt, got %#v", dispatch)
	}

	gate, ok := workflow.Jobs["verify-release-gate"]
	if !ok {
		t.Fatal("UI workflow must define verify-release-gate job")
	}
	gateScript := joinWorkflowRuns(gate.Steps)
	verifier, err := os.ReadFile(filepath.Join("..", "scripts", "verify-listingkit-api-release-attestation.sh"))
	if err != nil {
		t.Fatalf("read release attestation verifier: %v", err)
	}
	gateScript += "\n" + string(verifier)
	for _, required := range []string{
		"ListingKit API Deploy",
		".github/workflows/listingkit-deploy.yml",
		"listingkit-api-release-gate/v1",
		"release_gate_run_id",
		"workflow_run.id",
		"conclusion",
		"success",
		"gh api",
		"expires_at",
		"issued_at",
		"source_sha",
		"api_candidate_image",
		"api_workflow_run_id",
		"api_workflow_run_attempt",
		"run_attempt",
		"@sha256:",
	} {
		if !strings.Contains(gateScript, required) {
			t.Errorf("release gate verifier is missing fail-closed check %q", required)
		}
	}
	if gate.Outputs["verified"] == "" || gate.Outputs["source_sha"] == "" {
		t.Fatalf("release gate must export verified and source_sha outputs, got %#v", gate.Outputs)
	}
	downloads := 0
	for _, step := range gate.Steps {
		if step.Uses != "actions/download-artifact@v5" {
			continue
		}
		downloads++
		if got := stringValue(step.With["run-id"]); got != "${{ steps.select-gate.outputs.gate_run_id }}" {
			t.Errorf("release gate artifact must come from the selected exact run ID, got %q", got)
		}
		if got := stringValue(step.With["name"]); got != "listingkit-api-release-gate-${{ steps.select-gate.outputs.gate_run_id }}-${{ steps.select-gate.outputs.gate_run_attempt }}" {
			t.Errorf("release gate artifact name must bind the selected exact run ID and attempt, got %q", got)
		}
		if got := stringValue(step.With["github-token"]); got != "${{ github.token }}" {
			t.Errorf("cross-run artifact download must use the scoped GitHub token, got %q", got)
		}
	}
	if downloads != 1 {
		t.Fatalf("verify-release-gate must contain exactly one exact-run artifact download, got %d", downloads)
	}

	build := workflow.Jobs["build-ui"]
	checkoutRef := ""
	for _, step := range build.Steps {
		if step.Name == "Checkout UI source" {
			checkoutRef = stringValue(step.With["ref"])
		}
	}
	if !strings.Contains(checkoutRef, "needs.verify-release-gate.outputs.source_sha") || !strings.Contains(checkoutRef, "github.sha") {
		t.Fatalf("UI build must select tag SHA for build-only tags and exact attested SHA for gated releases, got %q", checkoutRef)
	}
	if !strings.Contains(build.If, "github.event_name == 'push'") || !strings.Contains(build.If, "needs.verify-release-gate.outputs.verified == 'true'") {
		t.Fatalf("UI build must allow build-only tags and require the verified gate otherwise, if=%q", build.If)
	}

	deploy := workflow.Jobs["deploy-ui"]
	needs := stringValue(deploy.Needs)
	if !strings.Contains(needs, "verify-release-gate") || !strings.Contains(deploy.If, "needs.verify-release-gate.outputs.verified == 'true'") || strings.Contains(deploy.If, "github.event_name == 'push'") {
		t.Fatalf("deploy-ui must depend on verified release gate, needs=%q if=%q", needs, deploy.If)
	}
	mutations := 0
	for jobName, job := range workflow.Jobs {
		for _, step := range job.Steps {
			if strings.Contains(step.Run, "kubectl ") && (strings.Contains(step.Run, " set image ") || strings.Contains(step.Run, " rollout restart ") || strings.Contains(step.Run, " apply ") || strings.Contains(step.Run, " patch ")) {
				mutations++
				if jobName != "deploy-ui" {
					t.Errorf("production mutation %q must live only in gated deploy-ui job", step.Name)
				}
			}
		}
	}
	if mutations == 0 {
		t.Fatal("expected gated UI production mutation steps")
	}
}

func TestListingKitSupportedProductionMutationEntryPointsAreGated(t *testing.T) {
	violations := auditListingKitProductionMutationOwnership(t, "..")
	if len(violations) != 0 {
		t.Fatalf("unsupported or unclassified ListingKit production mutation entries:\n%s", strings.Join(violations, "\n"))
	}
}

func TestListingKitProductionMutationClassifierNormalizesEquivalentEntries(t *testing.T) {
	for _, test := range []struct {
		name string
		text string
		want []string
	}{
		{name: "documented API restart", text: "then restart only `product-listing-api`", want: []string{"rollout_restart|deployment/product-listing-api"}},
		{name: "alternate API rollout wording", text: "Roll product-listing-api after the configuration change.", want: []string{"rollout_restart|deployment/product-listing-api"}},
		{name: "config map prose", text: "Apply the updated ConfigMap before the API rollout.", want: []string{"apply|configmap/listingkit-workbench-config"}},
		{name: "kubectl patch variant", text: "kubectl -n task-processor patch deployment product-listing-api --type merge", want: []string{"patch|deployment/product-listing-api"}},
		{name: "kustomize apply variant", text: "kubectl apply -k deployments/kubernetes/listingkit-workbench/overlays/prod", want: []string{"apply|bundle/listingkit-production"}},
		{name: "set image variant", text: "kubectl set image deploy/listingkit-ui listingkit-ui=image@sha256:abc", want: []string{"set_image|deployment/listingkit-ui"}},
		{name: "delete API variant", text: "kubectl -n task-processor delete deployment product-listing-api", want: []string{"delete|deployment/product-listing-api"}},
		{name: "replace ingress variant", text: "kubectl -n task-processor replace -f overlays/prod/patch-ingress.yaml", want: []string{"replace|ingress/listingkit-sms-webhook"}},
		{name: "scale UI variant", text: "kubectl -n task-processor scale deployment listingkit-ui --replicas=0", want: []string{"scale|deployment/listingkit-ui"}},
		{name: "Chinese operator wording", text: "更新生产 ConfigMap，然后重启 product-listing-api。", want: []string{"apply|configmap/listingkit-workbench-config", "rollout_restart|deployment/product-listing-api"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := classifyListingKitMutationText(test.text)
			if strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("classified mutations=%v want %v", got, test.want)
			}
		})
	}
}

func TestListingKitProductionMutationClassifierIgnoresReadOnlyAndUnrelatedText(t *testing.T) {
	for _, text := range []string{
		"kubectl -n task-processor rollout status deployment/product-listing-api --timeout=5m",
		"kubectl -n task-processor get deployment listingkit-ui -o wide",
		"Inspect the ConfigMap before the API rollout.",
		"Apply the updated ConfigMap for an unrelated service.",
		"The ListingKit API Deploy workflow reports the API rollout status.",
	} {
		t.Run(text, func(t *testing.T) {
			if got := classifyListingKitMutationText(text); len(got) != 0 {
				t.Fatalf("read-only or unrelated text classified as production mutation: %v", got)
			}
		})
	}
}

func TestListingKitProductionMutationOwnerCannotCoverDirectFallbackInSameParagraph(t *testing.T) {
	paragraph := "Run ListingKit API Deploy for the candidate. Alternatively, restart product-listing-api from the workstation."
	mutations := classifyListingKitMutationText(listingKitPositiveInstructionText(paragraph))
	if len(mutations) == 0 {
		t.Fatal("fixture must contain a classified production mutation")
	}
	if listingKitParagraphUsesGatedOwner(paragraph, mutations) {
		t.Fatal("a gated workflow name must not cover a separate direct production fallback")
	}
}

func TestListingKitWorkflowMutationClassifierResolvesStepLocalTargets(t *testing.T) {
	for _, step := range []releaseWorkflowStep{
		{Run: "target=product-listing-api\nkubectl -n task-processor patch deployment \"$target\" --type merge"},
		{Env: map[string]string{"TARGET": "product-listing-api"}, Run: `kubectl -n task-processor patch deployment "$TARGET" --type merge`},
	} {
		if got := classifyListingKitWorkflowStep(step); strings.Join(got, ",") != "patch|deployment/product-listing-api" {
			t.Fatalf("step-local target must be paired with its mutation intent, got %v", got)
		}
	}
}

func TestListingKitSupportedDocumentInventoryDiscoversNewRunbooks(t *testing.T) {
	repoRoot := t.TempDir()
	paths := []string{
		"deployments/kubernetes/listingkit-workbench/README.md",
		"deployments/kubernetes/listingkit-workbench/nested/operator.md",
		"docs/operations/listingkit-new-release.md",
	}
	for _, relativePath := range paths {
		absolutePath := filepath.Join(repoRoot, filepath.FromSlash(relativePath))
		if err := os.MkdirAll(filepath.Dir(absolutePath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolutePath, []byte("supported"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := listingKitSupportedDocumentPaths(t, repoRoot)
	if strings.Join(got, ",") != strings.Join(paths, ",") {
		t.Fatalf("supported document inventory=%v want %v", got, paths)
	}
}

func TestListingKitLegacyIdentityCleanupProductionMisuseGuard(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "kubectl.log")
	writePreflightFake(t, filepath.Join(binDir, "kubectl"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_KUBECTL_LOG"
exit 9
`)
	scriptPath, err := filepath.Abs(filepath.Join("..", "scripts", "listingkit-clean-legacy-identity-secret.sh"))
	if err != nil {
		t.Fatal(err)
	}
	run := func(namespace, workflowRef, job, runID, runAttempt string) ([]byte, error) {
		command := exec.Command(preflightBash(t), filepath.ToSlash(scriptPath), namespace, "listingkit-workbench-secret", "product-listing-api")
		command.Env = append(os.Environ(),
			"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
			"FAKE_KUBECTL_LOG="+filepath.ToSlash(logPath),
			"GITHUB_ACTIONS=true",
			"GITHUB_WORKFLOW_REF="+workflowRef,
			"GITHUB_JOB="+job,
			"GITHUB_RUN_ID="+runID,
			"GITHUB_RUN_ATTEMPT="+runAttempt,
		)
		return command.CombinedOutput()
	}

	for _, test := range []struct {
		name        string
		workflowRef string
		job         string
		runID       string
		runAttempt  string
	}{
		{name: "direct caller"},
		{name: "wrong workflow", workflowRef: "octo/task-processor/.github/workflows/other.yml@refs/heads/main", job: "deploy-api", runID: "424242", runAttempt: "2"},
		{name: "missing exact attempt", workflowRef: "octo/task-processor/.github/workflows/listingkit-deploy.yml@refs/heads/main", job: "deploy-api", runID: "424242"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_ = os.Remove(logPath)
			if output, runErr := run("task-processor", test.workflowRef, test.job, test.runID, test.runAttempt); runErr == nil {
				t.Fatalf("unsupported production cleanup unexpectedly passed: %s", output)
			}
			if content, readErr := os.ReadFile(logPath); readErr == nil && len(content) != 0 {
				t.Fatalf("unsupported production caller reached kubectl before the misuse guard: %s", content)
			}
		})
	}

	_ = os.Remove(logPath)
	if _, runErr := run(
		"task-processor",
		"octo/task-processor/.github/workflows/listingkit-deploy.yml@refs/tags/listingkit-api-v-test",
		"deploy-api",
		"424242",
		"2",
	); runErr == nil {
		t.Fatal("workflow-shaped production probe unexpectedly completed against the failing fake kubectl")
	}
	if content, readErr := os.ReadFile(logPath); readErr != nil || len(content) == 0 {
		t.Fatalf("exact API workflow run and attempt must retain the internal helper path; readErr=%v content=%q", readErr, content)
	}

	_ = os.Remove(logPath)
	if _, runErr := run("listingkit-nonprod", "", "", "", ""); runErr == nil {
		t.Fatal("non-production probe unexpectedly completed against the failing fake kubectl")
	}
	if content, readErr := os.ReadFile(logPath); readErr != nil || len(content) == 0 {
		t.Fatalf("explicit non-production caller must retain the helper path; readErr=%v content=%q", readErr, content)
	}
}

func TestListingKitKubectlMutationIntentClassifierIgnoresObservations(t *testing.T) {
	for _, test := range []struct {
		name string
		text string
		want []string
	}{
		{name: "apply", text: `kubectl -n "$ns" apply -f "$manifest"`, want: []string{"apply"}},
		{name: "patch", text: `kubectl -n "$ns" patch deployment "$name"`, want: []string{"patch"}},
		{name: "restart", text: `kubectl -n "$ns" rollout restart deployment/api`, want: []string{"rollout_restart"}},
		{name: "create", text: `kubectl create -n "$ns" -f "$manifest"`, want: []string{"create"}},
		{name: "client dry run", text: `kubectl create --dry-run=client -f manifest.yaml`, want: nil},
		{name: "rollout status", text: `kubectl -n "$ns" rollout status deployment/api`, want: nil},
		{name: "get", text: `kubectl -n "$ns" get deployment api`, want: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := listingKitKubectlMutationIntents(test.text); strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Fatalf("kubectl mutation intents=%v want %v", got, test.want)
			}
		})
	}
}

func TestListingKitMutationHelperInventoryFailsClosedOnNewWriter(t *testing.T) {
	scriptDir := t.TempDir()
	path := filepath.Join(scriptDir, "listingkit-direct-production-bypass.sh")
	if err := os.WriteFile(path, []byte("#!/usr/bin/env bash\nkubectl -n task-processor patch deployment product-listing-api\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	violations := unclassifiedListingKitMutationHelpers(t, scriptDir, map[string]bool{})
	if len(violations) != 1 || !strings.Contains(violations[0], "listingkit-direct-production-bypass.sh") {
		t.Fatalf("new mutation helper must fail closed, got %v", violations)
	}
}

var listingKitMutationWhitespace = regexp.MustCompile(`\s+`)

func classifyListingKitMutationText(text string) []string {
	normalized := strings.ToLower(text)
	normalized = strings.NewReplacer("`", "", "_", " ", "\\", "/", "\r", " ", "\n", " ").Replace(normalized)
	normalized = listingKitMutationWhitespace.ReplaceAllString(normalized, " ")

	targets := map[string]bool{
		"deployment/product-listing-api": strings.Contains(normalized, "product-listing-api") ||
			regexp.MustCompile(`\bapi (pods?|deployment|rollout)\b`).MatchString(normalized),
		"deployment/listingkit-ui": strings.Contains(normalized, "listingkit-ui") ||
			regexp.MustCompile(`\bui (pods?|deployment)\b`).MatchString(normalized),
		"configmap/listingkit-workbench-config": strings.Contains(normalized, "listingkit-workbench-config") ||
			(strings.Contains(normalized, "configmap") &&
				(strings.Contains(normalized, "listingkit") || strings.Contains(normalized, "product-listing-api") ||
					strings.Contains(normalized, "api rollout") || strings.Contains(normalized, "task-processor"))),
		"ingress/listingkit-sms-webhook": strings.Contains(normalized, "ingress"),
		"bundle/listingkit-production":   regexp.MustCompile(`\bapply\s+-k\b.*(?:overlays/prod|listingkit-production)`).MatchString(normalized),
		"secret/listingkit-workbench-secret": strings.Contains(normalized, "listingkit-workbench-secret") ||
			strings.Contains(normalized, "shared secret"),
	}

	entries := map[string]bool{}
	add := func(intent, target string) {
		if targets[target] {
			entries[intent+"|"+target] = true
		}
	}

	setImage := regexp.MustCompile(`\bkubectl\b.*\bset[- ]image\b`).MatchString(normalized)
	patch := regexp.MustCompile(`\bpatch\s+(?:deployment|deploy|configmap|ingress|secret)\b`).MatchString(normalized)
	deleteResource := regexp.MustCompile(`\bkubectl\b.*\bdelete\s+(?:deployment|deploy|configmap|ingress|secret)\b`).MatchString(normalized)
	replaceResource := regexp.MustCompile(`\bkubectl\b.*\breplace\b`).MatchString(normalized)
	scaleResource := regexp.MustCompile(`\bkubectl\b.*\bscale\s+(?:deployment|deploy)\b`).MatchString(normalized)
	apply := regexp.MustCompile(`\bapply\s+(?:the\b|an?\b|updated\b|immutable\b|production\b|-[fk]\b)`).MatchString(normalized) ||
		regexp.MustCompile(`\bkubectl\b.*\bapply\b`).MatchString(normalized) ||
		strings.Contains(normalized, "应用") || strings.Contains(normalized, "更新")
	applyAPI := regexp.MustCompile(`\bapply(?:ing)?\s+(?:the\s+)?(?:api|product-listing-api)(?:\s+deployment)?\b`).MatchString(normalized) ||
		regexp.MustCompile(`\bkubectl\b.*\bapply\b.*product-listing-api`).MatchString(normalized)
	applyUI := regexp.MustCompile(`\bapply(?:ing)?\s+(?:the\s+)?(?:ui|listingkit-ui)(?:\s+deployment)?\b`).MatchString(normalized) ||
		regexp.MustCompile(`\bkubectl\b.*\bapply\b.*listingkit-ui`).MatchString(normalized)
	restart := strings.Contains(normalized, "rollout restart") || strings.Contains(normalized, "restart") ||
		strings.Contains(normalized, "重启") || regexp.MustCompile(`(?:^|[ .:;])roll\s+(?:only\s+)?`).MatchString(normalized) ||
		strings.Contains(normalized, "roll the api") || strings.Contains(normalized, "roll the ui")

	if setImage {
		add("set_image", "deployment/product-listing-api")
		add("set_image", "deployment/listingkit-ui")
	}
	if patch && !setImage {
		add("patch", "deployment/product-listing-api")
		add("patch", "deployment/listingkit-ui")
		add("patch", "configmap/listingkit-workbench-config")
		add("patch", "ingress/listingkit-sms-webhook")
		add("patch", "secret/listingkit-workbench-secret")
	}
	if deleteResource {
		add("delete", "deployment/product-listing-api")
		add("delete", "deployment/listingkit-ui")
		add("delete", "configmap/listingkit-workbench-config")
		add("delete", "ingress/listingkit-sms-webhook")
		add("delete", "secret/listingkit-workbench-secret")
	}
	if replaceResource {
		add("replace", "deployment/product-listing-api")
		add("replace", "deployment/listingkit-ui")
		add("replace", "configmap/listingkit-workbench-config")
		add("replace", "ingress/listingkit-sms-webhook")
		add("replace", "secret/listingkit-workbench-secret")
	}
	if scaleResource {
		add("scale", "deployment/product-listing-api")
		add("scale", "deployment/listingkit-ui")
	}
	if apply && !setImage {
		if applyAPI {
			add("apply", "deployment/product-listing-api")
		}
		if applyUI {
			add("apply", "deployment/listingkit-ui")
		}
		add("apply", "configmap/listingkit-workbench-config")
		add("apply", "ingress/listingkit-sms-webhook")
		add("apply", "bundle/listingkit-production")
	}
	if restart {
		add("rollout_restart", "deployment/product-listing-api")
		add("rollout_restart", "deployment/listingkit-ui")
	}

	result := make([]string, 0, len(entries))
	for entry := range entries {
		result = append(result, entry)
	}
	sort.Strings(result)
	return result
}

func listingKitKubectlMutationIntents(text string) []string {
	intents := map[string]bool{}
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		normalized := strings.ToLower(listingKitMutationWhitespace.ReplaceAllString(line, " "))
		if !regexp.MustCompile(`\bkubectl(?:\.exe)?\b`).MatchString(normalized) {
			continue
		}
		switch {
		case regexp.MustCompile(`\brollout\s+restart\b`).MatchString(normalized):
			intents["rollout_restart"] = true
		case regexp.MustCompile(`\bset[- ]image\b`).MatchString(normalized):
			intents["set_image"] = true
		case regexp.MustCompile(`\bapply\b`).MatchString(normalized):
			intents["apply"] = true
		case regexp.MustCompile(`\bpatch\b`).MatchString(normalized):
			intents["patch"] = true
		case regexp.MustCompile(`\breplace\b`).MatchString(normalized):
			intents["replace"] = true
		case regexp.MustCompile(`\bdelete\b`).MatchString(normalized):
			intents["delete"] = true
		case regexp.MustCompile(`\bscale\b`).MatchString(normalized):
			intents["scale"] = true
		case regexp.MustCompile(`\bcreate\b`).MatchString(normalized) &&
			!strings.Contains(normalized, "--dry-run=client") && !strings.Contains(normalized, "--dry-run client"):
			intents["create"] = true
		}
	}
	result := make([]string, 0, len(intents))
	for intent := range intents {
		result = append(result, intent)
	}
	sort.Strings(result)
	return result
}

func unclassifiedListingKitMutationHelpers(t *testing.T, scriptDir string, known map[string]bool) []string {
	t.Helper()
	entries, err := os.ReadDir(scriptDir)
	if err != nil {
		t.Fatal(err)
	}
	var violations []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || (!strings.HasSuffix(name, ".sh") && !strings.HasSuffix(name, ".ps1")) ||
			(!strings.HasPrefix(name, "listingkit-") && name != "build-push-deploy-listingkit-workbench.ps1") || known[name] {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(scriptDir, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if intents := listingKitKubectlMutationIntents(string(content)); len(intents) > 0 {
			violations = append(violations, fmt.Sprintf("unclassified ListingKit mutation helper %s has intents %v", name, intents))
		}
	}
	sort.Strings(violations)
	return violations
}

func classifyListingKitWorkflowStep(step releaseWorkflowStep) []string {
	context := step.Run
	envKeys := make([]string, 0, len(step.Env))
	for key := range step.Env {
		envKeys = append(envKeys, key)
	}
	sort.Strings(envKeys)
	for _, key := range envKeys {
		context += "\n" + key + "=" + step.Env[key]
	}
	entries := classifyListingKitMutationText(context)
	add := func(entry string) {
		for _, existing := range entries {
			if existing == entry {
				return
			}
		}
		entries = append(entries, entry)
	}
	if strings.Contains(step.Run, ".workflow-tools/scripts/listingkit-clean-legacy-identity-secret.sh") {
		add("patch|secret/listingkit-workbench-secret")
		add("rollout_restart|deployment/product-listing-api")
	}
	if strings.Contains(step.Run, ".workflow-tools/scripts/listingkit-apply-api-deployment.sh") &&
		strings.Contains(step.Run, "product-listing-api-deployment.yaml") {
		add("apply|deployment/product-listing-api")
	}
	return uniqueSortedListingKitMutations(entries)
}

func auditListingKitProductionMutationOwnership(t *testing.T, repoRoot string) []string {
	t.Helper()
	allowedOwners := map[string]bool{
		".github/workflows/listingkit-deploy.yml|deploy-api|Remove deprecated ListingKit identity keys from shared Secret|patch|secret/listingkit-workbench-secret":       true,
		".github/workflows/listingkit-deploy.yml|deploy-api|Remove deprecated ListingKit identity keys from shared Secret|rollout_restart|deployment/product-listing-api": true,
		".github/workflows/listingkit-deploy.yml|deploy-api|Apply ListingKit runtime configuration for candidate|apply|configmap/listingkit-workbench-config":             true,
		".github/workflows/listingkit-deploy.yml|deploy-api|Apply immutable API deployment after image agent compatibility gates|apply|deployment/product-listing-api":    true,
		".github/workflows/listingkit-deploy.yml|deploy-api|Restart API Pods for Secret changes and v3 new-start routing|rollout_restart|deployment/product-listing-api":  true,
		".github/workflows/listingkit-deploy.yml|deploy-api|Apply production ListingKit SMS webhook ingress|apply|ingress/listingkit-sms-webhook":                         true,
		".github/workflows/listingkit-ui-deploy.yml|deploy-ui|Apply ListingKit UI authorization scopes|patch|configmap/listingkit-workbench-config":                       true,
		".github/workflows/listingkit-ui-deploy.yml|deploy-ui|Update UI deployment image|set_image|deployment/listingkit-ui":                                              true,
		".github/workflows/listingkit-ui-deploy.yml|deploy-ui|Restart UI pods for authorization configuration|rollout_restart|deployment/listingkit-ui":                   true,
	}

	var violations []string
	workflowDir := filepath.Join(repoRoot, ".github", "workflows")
	workflowEntries, err := os.ReadDir(workflowDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range workflowEntries {
		if file.IsDir() || (!strings.HasSuffix(file.Name(), ".yml") && !strings.HasSuffix(file.Name(), ".yaml")) {
			continue
		}
		relativePath := filepath.ToSlash(filepath.Join(".github", "workflows", file.Name()))
		workflow := loadReleaseWorkflow(t, filepath.Join(workflowDir, file.Name()))
		for jobName, job := range workflow.Jobs {
			for _, step := range job.Steps {
				for _, mutation := range classifyListingKitWorkflowStep(step) {
					owner := strings.Join([]string{relativePath, jobName, step.Name, mutation}, "|")
					if !allowedOwners[owner] {
						violations = append(violations, "workflow mutation has no exact gated owner: "+owner)
					}
				}
			}
		}
	}

	for _, relativePath := range listingKitSupportedDocumentPaths(t, repoRoot) {
		content, readErr := os.ReadFile(filepath.Join(repoRoot, relativePath))
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, paragraph := range listingKitMarkdownParagraphs(string(content)) {
			positiveText := listingKitPositiveInstructionText(paragraph.text)
			mutations := classifyListingKitMutationText(positiveText)
			if len(mutations) == 0 {
				continue
			}
			if listingKitParagraphUsesGatedOwner(paragraph.text, mutations) {
				continue
			}
			violations = append(violations, fmt.Sprintf("supported document %s:%d has mutation %v without exact gated call path", filepath.ToSlash(relativePath), paragraph.line, mutations))
		}
	}

	violations = append(violations, auditListingKitMutationHelpers(t, repoRoot)...)
	sort.Strings(violations)
	return violations
}

func listingKitSupportedDocumentPaths(t *testing.T, repoRoot string) []string {
	t.Helper()
	var paths []string
	for _, relativeRoot := range []string{
		filepath.Join("deployments", "kubernetes", "listingkit-workbench"),
		filepath.Join("docs", "operations"),
	} {
		absoluteRoot := filepath.Join(repoRoot, relativeRoot)
		if err := filepath.WalkDir(absoluteRoot, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
				return nil
			}
			relativePath, relativeErr := filepath.Rel(repoRoot, path)
			if relativeErr != nil {
				return relativeErr
			}
			paths = append(paths, filepath.ToSlash(relativePath))
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(paths)
	return paths
}

type listingKitMarkdownParagraph struct {
	line int
	text string
}

func listingKitMarkdownParagraphs(document string) []listingKitMarkdownParagraph {
	lines := strings.Split(strings.ReplaceAll(document, "\r\n", "\n"), "\n")
	var paragraphs []listingKitMarkdownParagraph
	start := 0
	var current []string
	flush := func() {
		if len(current) == 0 {
			return
		}
		paragraphs = append(paragraphs, listingKitMarkdownParagraph{line: start, text: strings.Join(current, "\n")})
		current = nil
	}
	for index, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if len(current) == 0 {
			start = index + 1
		}
		current = append(current, line)
	}
	flush()
	return paragraphs
}

func listingKitPositiveInstructionText(paragraph string) string {
	var positive []string
	for _, sentence := range listingKitInstructionClauses(paragraph) {
		if listingKitClauseIsNegated(sentence) {
			continue
		}
		positive = append(positive, sentence)
	}
	return strings.Join(positive, "\n")
}

func listingKitParagraphUsesGatedOwner(paragraph string, mutations []string) bool {
	if len(mutations) == 0 {
		return true
	}
	activeOwner := ""
	for _, clause := range listingKitInstructionClauses(paragraph) {
		if listingKitClauseIsNegated(clause) {
			continue
		}
		normalized := listingKitMutationWhitespace.ReplaceAllString(strings.ReplaceAll(clause, "**", ""), " ")
		lower := strings.ToLower(normalized)
		mentionsAPIOwner := strings.Contains(normalized, "ListingKit API Deploy")
		mentionsUIOwner := strings.Contains(normalized, "ListingKit UI Deploy")
		clauseMutations := classifyListingKitMutationText(clause)
		if len(clauseMutations) == 0 {
			if mentionsAPIOwner {
				activeOwner = "ListingKit API Deploy"
			}
			if mentionsUIOwner {
				activeOwner = "ListingKit UI Deploy"
			}
			continue
		}
		directFallback := strings.Contains(lower, "alternatively") || strings.Contains(lower, "fallback") ||
			strings.Contains(lower, "from the workstation") || strings.Contains(lower, "standalone helper") ||
			regexp.MustCompile(`\bkubectl\b.*\b(apply|patch|rollout\s+restart|set[- ]image)\b`).MatchString(lower)
		if directFallback {
			return false
		}
		for _, mutation := range clauseMutations {
			requiredOwner := "ListingKit API Deploy"
			if strings.HasSuffix(mutation, "|deployment/listingkit-ui") {
				requiredOwner = "ListingKit UI Deploy"
			}
			exactOwner := strings.Contains(normalized, requiredOwner)
			actorReference := strings.Contains(lower, "workflow") || strings.Contains(lower, "gate") ||
				strings.Contains(lower, "its run") || strings.Contains(lower, "that run") ||
				strings.Contains(lower, "that exact") || strings.Contains(lower, "it applies") ||
				strings.Contains(lower, "it restarts") || strings.Contains(lower, "alone")
			if !exactOwner && !(activeOwner == requiredOwner && actorReference) {
				return false
			}
			if requiredOwner == "ListingKit UI Deploy" &&
				!(strings.Contains(normalized, "workflow_run") ||
					(strings.Contains(normalized, "release_gate_run_id") && strings.Contains(normalized, "release_gate_run_attempt"))) {
				return false
			}
		}
		if mentionsAPIOwner {
			activeOwner = "ListingKit API Deploy"
		}
		if mentionsUIOwner {
			activeOwner = "ListingKit UI Deploy"
		}
	}
	return true
}

func listingKitInstructionClauses(paragraph string) []string {
	normalized := strings.ReplaceAll(paragraph, "\r", "")
	normalized = listingKitMutationWhitespace.ReplaceAllString(normalized, " ")
	return regexp.MustCompile(`[.!?。！？]\s+`).Split(normalized, -1)
}

func listingKitClauseIsNegated(clause string) bool {
	lower := strings.ToLower(clause)
	return strings.Contains(lower, "must not") || strings.Contains(lower, "do not") || strings.Contains(lower, "never") ||
		strings.Contains(lower, "not supported") || strings.Contains(lower, "not a release") || strings.Contains(clause, "不得") || strings.Contains(clause, "不可") ||
		strings.Contains(clause, "禁止") || strings.Contains(clause, "不能")
}

func auditListingKitMutationHelpers(t *testing.T, repoRoot string) []string {
	t.Helper()
	directIntentPolicies := map[string][]string{
		"listingkit-clean-legacy-identity-secret.sh": {"patch", "rollout_restart"},
		"listingkit-apply-api-deployment.sh":         {"apply", "patch"},
		"listingkit-schema-migrate-job.sh":           {"create"},
		"listingkit-identity-preflight-job.sh":       {"create"},
		"build-push-deploy-listingkit-workbench.ps1": {"rollout_restart"},
	}
	scriptDir := filepath.Join(repoRoot, "scripts")
	known := make(map[string]bool, len(directIntentPolicies))
	for name := range directIntentPolicies {
		known[name] = true
	}
	violations := unclassifiedListingKitMutationHelpers(t, scriptDir, known)
	for name, want := range directIntentPolicies {
		content, readErr := os.ReadFile(filepath.Join(scriptDir, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		got := listingKitKubectlMutationIntents(string(content))
		want = uniqueSortedListingKitMutations(want)
		if strings.Join(got, ",") != strings.Join(want, ",") {
			violations = append(violations, fmt.Sprintf("helper scripts/%s direct mutation intents=%v want %v", name, got, want))
		}
	}

	cleanHelper := "listingkit-clean-legacy-identity-secret.sh"
	allowedCallers := map[string]bool{
		".github/workflows/listingkit-deploy.yml|deploy-api|Remove deprecated ListingKit identity keys from shared Secret": true,
		"scripts/build-push-deploy-listingkit-workbench.ps1|nonproduction-workstation":                                     true,
	}
	workflow := loadReleaseWorkflow(t, filepath.Join(repoRoot, ".github", "workflows", "listingkit-deploy.yml"))
	for jobName, job := range workflow.Jobs {
		for _, step := range job.Steps {
			if strings.Contains(step.Run, cleanHelper) {
				caller := ".github/workflows/listingkit-deploy.yml|" + jobName + "|" + step.Name
				if !allowedCallers[caller] {
					violations = append(violations, "legacy cleanup helper has unsupported workflow caller: "+caller)
				}
			}
		}
	}
	scriptEntries, err := os.ReadDir(scriptDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range scriptEntries {
		if entry.IsDir() || entry.Name() == cleanHelper {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(scriptDir, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(content), cleanHelper) {
			continue
		}
		caller := filepath.ToSlash(filepath.Join("scripts", entry.Name())) + "|nonproduction-workstation"
		if !allowedCallers[caller] {
			violations = append(violations, "legacy cleanup helper has unsupported script caller: "+caller)
		}
	}
	for _, relativePath := range listingKitSupportedDocumentPaths(t, repoRoot) {
		content, readErr := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(relativePath)))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Contains(string(content), cleanHelper) {
			violations = append(violations, "internal legacy cleanup helper is advertised by supported document: "+relativePath)
		}
	}
	sort.Strings(violations)
	return violations
}

func uniqueSortedListingKitMutations(entries []string) []string {
	set := map[string]bool{}
	for _, entry := range entries {
		set[entry] = true
	}
	result := make([]string, 0, len(set))
	for entry := range set {
		result = append(result, entry)
	}
	sort.Strings(result)
	return result
}

func TestListingKitReleaseAttestationAllowsResolvedSourceDifferentFromWorkflowHead(t *testing.T) {
	attestedSource := strings.Repeat("a", 40)
	runHead := strings.Repeat("b", 40)
	output, err := runListingKitReleaseAttestationVerifier(t, releaseAttestationScenario{
		selectedAttempt: "2",
		runAttempt:      "2",
		attestedAttempt: "2",
		attestedSource:  attestedSource,
		runHead:         runHead,
		includeAttempt:  true,
	})
	if err != nil {
		t.Fatalf("verify attestation with independent resolved source: %v\n%s", err, output)
	}
	if got := strings.TrimSpace(string(output)); got != attestedSource {
		t.Fatalf("verified source=%q want attested source %q (workflow head was %q)", got, attestedSource, runHead)
	}
}

func TestListingKitReleaseAttestationRequiresExactRunAttempt(t *testing.T) {
	for _, test := range []struct {
		name     string
		scenario releaseAttestationScenario
		wantPass bool
	}{
		{name: "attempt_1", scenario: releaseAttestationScenario{selectedAttempt: "1", runAttempt: "1", attestedAttempt: "1", includeAttempt: true}, wantPass: true},
		{name: "attempt_2", scenario: releaseAttestationScenario{selectedAttempt: "2", runAttempt: "2", attestedAttempt: "2", includeAttempt: true}, wantPass: true},
		{name: "REST_attempt_does_not_match_selection", scenario: releaseAttestationScenario{selectedAttempt: "2", runAttempt: "1", attestedAttempt: "2", includeAttempt: true}},
		{name: "same_run_artifact_from_another_attempt", scenario: releaseAttestationScenario{selectedAttempt: "2", runAttempt: "2", attestedAttempt: "1", includeAttempt: true}},
		{name: "missing_attempt_selection", scenario: releaseAttestationScenario{runAttempt: "1", attestedAttempt: "1", includeAttempt: false}},
	} {
		t.Run(test.name, func(t *testing.T) {
			output, err := runListingKitReleaseAttestationVerifier(t, test.scenario)
			if test.wantPass && err != nil {
				t.Fatalf("exact attempt verification failed: %v\n%s", err, output)
			}
			if !test.wantPass && err == nil {
				t.Fatalf("attempt mismatch unexpectedly passed: %s", output)
			}
		})
	}
}

type releaseAttestationScenario struct {
	selectedAttempt string
	runAttempt      string
	attestedAttempt string
	attestedSource  string
	runHead         string
	includeAttempt  bool
}

func runListingKitReleaseAttestationVerifier(t *testing.T, scenario releaseAttestationScenario) ([]byte, error) {
	t.Helper()
	if scenario.attestedSource == "" {
		scenario.attestedSource = strings.Repeat("a", 40)
	}
	if scenario.runHead == "" {
		scenario.runHead = strings.Repeat("b", 40)
	}
	binDir := t.TempDir()
	apiDigest := strings.Repeat("c", 64)
	now := time.Now().UTC()
	runJSON := fmt.Sprintf(`{"id":424242,"run_attempt":%s,"repository":{"full_name":"octo/task-processor"},"name":"ListingKit API Deploy","path":".github/workflows/listingkit-deploy.yml@refs/heads/main","conclusion":"success","head_sha":%q}`, scenario.runAttempt, scenario.runHead)
	attestationJSON := fmt.Sprintf(`{"gate_version":"listingkit-api-release-gate/v1","repository":"octo/task-processor","workflow_name":"ListingKit API Deploy","workflow_path":".github/workflows/listingkit-deploy.yml","source_sha":%q,"api_candidate_image":"docker.io/xuwei190/task-processor-product-listing-api@sha256:%s","api_workflow_run_id":424242,"api_workflow_run_attempt":%s,"issued_at":%q,"expires_at":%q}`,
		scenario.attestedSource, apiDigest, scenario.attestedAttempt, now.Add(-time.Minute).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339))
	runPath := filepath.Join(t.TempDir(), "run.json")
	attestationPath := filepath.Join(t.TempDir(), "attestation.json")
	if err := os.WriteFile(runPath, []byte(runJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attestationPath, []byte(attestationJSON), 0o600); err != nil {
		t.Fatal(err)
	}
	writePreflightFake(t, filepath.Join(binDir, "gh"), fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
if [[ "$*" == *"repos/octo/task-processor/commits/%s"* ]]; then
  printf '%s\n'
  exit 0
fi
exit 1
`, scenario.attestedSource, scenario.attestedSource))
	writePreflightFake(t, filepath.Join(binDir, "jq"), fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
filter="${2:-}"
case "$filter" in
  *'keys == '*) exit 0 ;;
  *'.repository.full_name'*) printf 'octo/task-processor\n' ;;
  *'.id | select'*) printf '424242\n' ;;
  *'.run_attempt | select'*) printf '%s\n' ;;
  *'.name | select'*) printf 'ListingKit API Deploy\n' ;;
  *'.path | select'*) printf '.github/workflows/listingkit-deploy.yml@refs/heads/main\n' ;;
  *'.conclusion | select'*) printf 'success\n' ;;
  *'.head_sha | select'*) printf '%s\n' ;;
  *'.gate_version | select'*) printf 'listingkit-api-release-gate/v1\n' ;;
  *'.repository | select'*) printf 'octo/task-processor\n' ;;
  *'.workflow_name | select'*) printf 'ListingKit API Deploy\n' ;;
  *'.workflow_path | select'*) printf '.github/workflows/listingkit-deploy.yml\n' ;;
  *'.source_sha | select'*) printf '%s\n' ;;
  *'.api_candidate_image | select'*) printf 'docker.io/xuwei190/task-processor-product-listing-api@sha256:%s\n' ;;
  *'.api_workflow_run_id | select'*) printf '424242\n' ;;
  *'.api_workflow_run_attempt | select'*) printf '%s\n' ;;
  *'.issued_at | select'*) printf '%s\n' ;;
  *'.expires_at | select'*) printf '%s\n' ;;
  *) exit 1 ;;
esac
`, scenario.runAttempt, scenario.runHead, scenario.attestedSource, apiDigest, scenario.attestedAttempt, now.Add(-time.Minute).Format(time.RFC3339), now.Add(time.Hour).Format(time.RFC3339)))

	verifierPath, err := filepath.Abs(filepath.Join("..", "scripts", "verify-listingkit-api-release-attestation.sh"))
	if err != nil {
		t.Fatal(err)
	}
	args := []string{filepath.ToSlash(verifierPath),
		"--attestation", attestationPath,
		"--run-json", runPath,
		"--run-id", "424242"}
	if scenario.includeAttempt {
		args = append(args, "--run-attempt", scenario.selectedAttempt)
	}
	args = append(args,
		"--repository", "octo/task-processor",
		"--api-repository", "docker.io/xuwei190/task-processor-product-listing-api")
	command := exec.Command(preflightBash(t), args...)
	command.Env = append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return command.CombinedOutput()
}

func TestListingKitImageAgentDrainRunbookDefinesCompleteSafeInventoryAndRecoveryHorizon(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "README.md"))
	if err != nil {
		t.Fatalf("read ListingKit runbook: %v", err)
	}
	runbook := string(content)
	for _, required := range []string{
		"ImageAgentWorkflow",
		"ImageSlotWorkflow",
		"listingkit-image-agent-v2-drain-check.sh",
		"Temporal CLI 1.8.1",
		"pendingChildren",
		"pendingActivities",
		"imageagent.execute_slot",
		"imageagent.execute_slot.v2",
		"open_v2_parent_count",
		"open_v2_child_count",
		"pending_v2_child_count",
		"pending_v2_activity_count",
		"pending_v2_activity_attempt_sum",
		"tenant_id, owner_user_id, id, created_at",
		"created_at >=",
		"created_at <",
		"image-agent:${tenant_id}:${owner_user_id}:${run_id}",
		"30 days",
		"7 days",
		"37 days",
		"45 days",
		"WorkflowExecutionTimeout",
	} {
		if !strings.Contains(runbook, required) {
			t.Errorf("image-agent recovery runbook is missing %q", required)
		}
	}
	sectionStart := strings.Index(runbook, "## Image-agent Temporal v2/v3 recovery rollout")
	if sectionStart < 0 {
		t.Fatal("image-agent recovery rollout section is missing")
	}
	section := runbook[sectionStart:]
	blocks := strings.Split(section, "```bash\n")
	if len(blocks) < 3 {
		t.Fatalf("image-agent recovery runbook must contain executable inventory and lifecycle bash blocks, got %d", len(blocks)-1)
	}
	for index, remainder := range blocks[1:] {
		end := strings.Index(remainder, "\n```")
		if end < 0 {
			t.Fatalf("bash block %d is not closed", index+1)
		}
		command := exec.Command(preflightBash(t), "-n")
		command.Stdin = strings.NewReader(remainder[:end])
		if output, syntaxErr := command.CombinedOutput(); syntaxErr != nil {
			t.Fatalf("bash block %d is not syntactically executable: %v\n%s", index+1, syntaxErr, output)
		}
	}
}

func loadReleaseWorkflow(t *testing.T, path string) releaseWorkflow {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read workflow %s: %v", path, err)
	}
	var workflow releaseWorkflow
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatalf("parse workflow %s: %v", path, err)
	}
	return workflow
}

func joinWorkflowRuns(steps []releaseWorkflowStep) string {
	var runs []string
	for _, step := range steps {
		runs = append(runs, step.Run, fmt.Sprint(step.Env))
	}
	return strings.Join(runs, "\n")
}

func stringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.ReplaceAll(strings.TrimSpace(fmt.Sprint(value)), "\n", "")
}
