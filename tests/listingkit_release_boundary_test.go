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

func TestListingKitImageAgentSteadyStateRendersNoFiniteCanary(t *testing.T) {
	root := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench")
	for _, test := range []struct {
		name        string
		path        string
		wantWorkers []string
	}{
		{name: "base", path: filepath.Join(root, "base"), wantWorkers: []string{"image-agent-temporal-worker", "image-agent-temporal-worker-v3"}},
		{name: "prod", path: filepath.Join(root, "overlays", "prod"), wantWorkers: []string{"image-agent-temporal-worker", "image-agent-temporal-worker-v3"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			objects := renderKustomizeObjects(t, test.path)
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
			if canaries != 0 {
				t.Fatalf("%s steady-state render has %d finite image-agent canaries, want zero", test.name, canaries)
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
		"Run finite image agent v3 compatibility canary",
		"--deployment image-agent-temporal-v3-canary-runner",
		"--timeout-seconds 300",
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
		case "Run finite image agent v3 compatibility canary":
			canaryWait = index
		case "Wait for API rollout":
			apiWait = index
		case "Emit ListingKit API release attestation":
			emit = index
			for _, required := range []string{
				"listingkit-api-release-gate/v2",
				"${{ needs.prepare.outputs.source_ref }}",
				"$API_CANDIDATE_IMAGE",
				"$GITHUB_RUN_ID",
				"$GITHUB_RUN_ATTEMPT",
				"api_workflow_run_attempt",
				"workflow_ref",
				"routing_contract",
				"worker_wire_contract",
				"worker_replay_contract",
				"schema_contract",
				"@sha256:",
			} {
				if !strings.Contains(step.Run, required) {
					t.Errorf("release attestation emitter is missing %q", required)
				}
			}
			if strings.Contains(step.Run, "expires_at") {
				t.Error("immutable v2 release attestations must not reintroduce an expiry field")
			}
		case "Upload ListingKit API release attestation":
			upload = index
			if step.Uses != "actions/upload-artifact@ea165f8d65b6e75b540449e92b4886f43607fa02" {
				t.Errorf("release attestation must use the official upload-artifact v4.6.2 commit, got %q", step.Uses)
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

func TestListingKitDeployStampsExactAPIReleaseIdentityBeforeRolloutAcceptance(t *testing.T) {
	workflow := loadReleaseWorkflow(t, filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml"))
	steps := workflow.Jobs["deploy-api"].Steps
	apply, stamp, wait, attest := -1, -1, -1, -1
	for index, step := range steps {
		switch step.Name {
		case "Apply immutable API deployment after image agent compatibility gates":
			apply = index
		case "Stamp API release identity and restart Pods":
			stamp = index
			for _, required := range []string{
				"listingkit.sh/api-release-run-id",
				"listingkit.sh/api-release-run-attempt",
				"listingkit.sh/api-release-image",
				"$GITHUB_RUN_ID",
				"$GITHUB_RUN_ATTEMPT",
				"$API_CANDIDATE_IMAGE",
				"metadata",
				"spec",
				"template",
				"kubectl",
				"patch",
			} {
				if !strings.Contains(step.Run, required) {
					t.Errorf("API release identity stamp is missing %q", required)
				}
			}
			for _, validation := range []string{"^[1-9][0-9]*$", "@sha256:[0-9a-f]{64}"} {
				if !strings.Contains(step.Run, validation) {
					t.Errorf("API release identity stamp is missing validation %q", validation)
				}
			}
		case "Wait for API rollout":
			wait = index
		case "Emit ListingKit API release attestation":
			attest = index
		}
	}
	if apply < 0 || stamp < 0 || wait < 0 || attest < 0 || !(apply < stamp && stamp < wait && wait < attest) {
		t.Fatalf("release identity must be stamped after immutable apply and before rollout acceptance/attestation: apply=%d stamp=%d wait=%d attest=%d", apply, stamp, wait, attest)
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
		".github/workflows/listingkit-deploy.yml@refs/heads/main",
		"listingkit-api-release-gate/v2",
		"release_gate_run_id",
		"workflow_run.id",
		"conclusion",
		"success",
		"gh api",
		"issued_at",
		"source_sha",
		"api_candidate_image",
		"api_workflow_run_id",
		"api_workflow_run_attempt",
		"run_attempt",
		"workflow_ref",
		"routing_contract",
		"worker_wire_contract",
		"worker_replay_contract",
		"schema_contract",
		"workflow head SHA does not match attested source",
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
		if step.Uses != "actions/download-artifact@634f93cb2916e3fdff6788551b99b062d0335ce0" {
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

func TestListingKitReleaseAttestationRequiresExactWorkflowHeadSource(t *testing.T) {
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
	if err == nil {
		t.Fatalf("attestation source %q unexpectedly passed with workflow head %q: %s", attestedSource, runHead, output)
	}
	if !strings.Contains(string(output), "workflow head SHA does not match attested source") {
		t.Fatalf("source mismatch must fail for the exact workflow-head contract, got: %s", output)
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
		scenario.runHead = scenario.attestedSource
	}
	binDir := t.TempDir()
	apiDigest := strings.Repeat("c", 64)
	now := time.Now().UTC()
	runJSON := fmt.Sprintf(`{"id":424242,"run_attempt":%s,"repository":{"full_name":"octo/task-processor"},"name":"ListingKit API Deploy","path":".github/workflows/listingkit-deploy.yml@refs/heads/main","conclusion":"success","head_sha":%q}`, scenario.runAttempt, scenario.runHead)
	attestationJSON := fmt.Sprintf(`{"gate_version":"listingkit-api-release-gate/v2","repository":"octo/task-processor","workflow_name":"ListingKit API Deploy","workflow_ref":"octo/task-processor/.github/workflows/listingkit-deploy.yml@refs/heads/main","source_sha":%q,"api_candidate_image":"docker.io/xuwei190/task-processor-product-listing-api@sha256:%s","api_workflow_run_id":424242,"api_workflow_run_attempt":%s,"issued_at":%q,"routing_contract":"image-agent-v3-new-starts-v1","worker_wire_contract":"image-agent-workers-v2-v3","worker_replay_contract":"image-agent-replay-v2-v3","schema_contract":"listingkit-schema-additive-v1"}`,
		scenario.attestedSource, apiDigest, scenario.attestedAttempt, now.Add(-time.Minute).Format(time.RFC3339))
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
  *'.gate_version | select'*) printf 'listingkit-api-release-gate/v2\n' ;;
  *'.repository | select'*) printf 'octo/task-processor\n' ;;
  *'.workflow_name | select'*) printf 'ListingKit API Deploy\n' ;;
  *'.workflow_ref | select'*) printf 'octo/task-processor/.github/workflows/listingkit-deploy.yml@refs/heads/main\n' ;;
  *'.source_sha | select'*) printf '%s\n' ;;
  *'.api_candidate_image | select'*) printf 'docker.io/xuwei190/task-processor-product-listing-api@sha256:%s\n' ;;
  *'.api_workflow_run_id | select'*) printf '424242\n' ;;
  *'.api_workflow_run_attempt | select'*) printf '%s\n' ;;
  *'.issued_at | select'*) printf '%s\n' ;;
  *'.routing_contract | select'*) printf 'image-agent-v3-new-starts-v1\n' ;;
  *'.worker_wire_contract | select'*) printf 'image-agent-workers-v2-v3\n' ;;
  *'.worker_replay_contract | select'*) printf 'image-agent-replay-v2-v3\n' ;;
  *'.schema_contract | select'*) printf 'listingkit-schema-additive-v1\n' ;;
  *) exit 1 ;;
esac
`, scenario.runAttempt, scenario.runHead, scenario.attestedSource, apiDigest, scenario.attestedAttempt, now.Add(-time.Minute).Format(time.RFC3339)))

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
		"--expected-run-id",
		"--expected-run-attempt",
		"--expected-api-image",
		"--namespace",
		"PGHOST",
		"PGPORT",
		"PGDATABASE",
		"PGUSER",
		"exactly three complete samples",
		"300 seconds",
		"10-minute first-to-final window",
		"ListingKit operational policy, not a Temporal SLA",
		"listingkit.sh/api-release-run-id",
		"listingkit.sh/api-release-run-attempt",
		"listingkit.sh/api-release-image",
		"listingkit.sh/image-agent-routing-contract",
		"image-agent-v3-new-starts-v1",
		"status NOT IN ('completed', 'failed', 'cancelled')",
		"Local test evidence is not live acceptance",
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

func TestListingKitRunbookRejectsV2ProducingRollback(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	runbook := string(content)
	for _, required := range []string{
		"Only a prior successful immutable API release attestation carrying",
		"image-agent-v3-new-starts-v1",
		"A v2-producing API rollback is unsupported",
		"invalidates the v2 drain evidence",
		"separately designed recovery procedure",
	} {
		if !strings.Contains(runbook, required) {
			t.Errorf("rollback contract is missing %q", required)
		}
	}
	if strings.Contains(runbook, "A rollback stops new v3 starts") {
		t.Error("supported rollback prose must not reopen v2 starts")
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
