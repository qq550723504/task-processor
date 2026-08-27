package tests

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	Needs   interface{}           `yaml:"needs"`
	If      string                `yaml:"if"`
	Outputs map[string]string     `yaml:"outputs"`
	Steps   []releaseWorkflowStep `yaml:"steps"`
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
	workstationScriptPath := filepath.Join("..", "scripts", "build-push-deploy-listingkit-workbench.ps1")
	workstationScript, err := os.ReadFile(workstationScriptPath)
	if err != nil {
		t.Fatal(err)
	}
	workstationText := string(workstationScript)
	guardOffset := strings.Index(workstationText, `-not $SkipApply`)
	firstExternalCommandOffset := strings.Index(workstationText, "git rev-parse")
	if guardOffset < 0 || firstExternalCommandOffset < 0 || guardOffset > firstExternalCommandOffset ||
		!strings.Contains(workstationText, `[string]::Equals($Namespace.Trim(), $ProductionNamespace`) {
		t.Fatal("workstation entry point must reject production apply before resolving source or invoking external commands")
	}
	if strings.Contains(workstationText, "$ProductionIngressManifest") {
		t.Fatal("workstation entry point must not retain a production ingress mutation path")
	}

	workflowDir := filepath.Join("..", ".github", "workflows")
	entries, err := os.ReadDir(workflowDir)
	if err != nil {
		t.Fatal(err)
	}
	allowedWorkflowOwners := map[string]bool{
		"listingkit-deploy.yml":    true,
		"listingkit-ui-deploy.yml": true,
	}
	productionTargets := []string{
		"deployment/product-listing-api",
		"deployment/listingkit-ui",
		"configmap listingkit-workbench-config",
		"base/product-listing-api-deployment.yaml",
		"base/listingkit-ui-deployment.yaml",
		"overlays/prod/patch-ingress.yaml",
	}
	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yml") && !strings.HasSuffix(entry.Name(), ".yaml")) {
			continue
		}
		content, readErr := os.ReadFile(filepath.Join(workflowDir, entry.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, target := range productionTargets {
			if strings.Contains(string(content), target) && !allowedWorkflowOwners[entry.Name()] {
				t.Errorf("workflow %s advertises or owns ListingKit production mutation target %q outside the gated API/UI workflows", entry.Name(), target)
			}
		}
	}

	for _, relativePath := range []string{
		filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "README.md"),
		filepath.Join("..", "docs", "operations", "listingkit-release-candidate-runbook.md"),
	} {
		content, readErr := os.ReadFile(relativePath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(content)
		for _, forbidden := range []string{
			"kubectl apply -f deployments/kubernetes/listingkit-workbench/base/configmap.yaml",
			"kubectl -n task-processor set image deployment/product-listing-api",
			"kubectl -n task-processor set image deployment/listingkit-ui",
			"kubectl -n task-processor rollout restart deployment/product-listing-api",
			"kubectl -n task-processor rollout restart deployment/listingkit-ui",
			"scripts/listingkit-apply-api-deployment.sh",
			"scripts/listingkit-apply-ui-deployment.sh",
			"kubectl -n task-processor apply -f deployments/kubernetes/listingkit-workbench/overlays/prod/patch-ingress.yaml",
			"workstation `kubectl set image` procedure remains",
		} {
			if strings.Contains(text, forbidden) {
				t.Errorf("supported production document %s advertises forbidden bypass %q", relativePath, forbidden)
			}
		}
		for _, required := range []string{"ListingKit API Deploy", "ListingKit UI Deploy", "release_gate_run_id", "release_gate_run_attempt"} {
			if !strings.Contains(text, required) {
				t.Errorf("supported production document %s must route release/rollback through exact attempt-bound workflows; missing %q", relativePath, required)
			}
		}
	}
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
