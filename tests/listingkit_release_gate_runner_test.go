package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestListingKitReleaseGateRunnersArePreinstalledAndZeroReplica(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "release-authority", "listingkit-release-gate-runners.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	type container struct {
		Name    string   `yaml:"name"`
		Image   string   `yaml:"image"`
		Command []string `yaml:"command"`
		Args    []string `yaml:"args"`
	}
	type deployment struct {
		Kind     string `yaml:"kind"`
		Metadata struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
		Spec struct {
			Replicas int `yaml:"replicas"`
			Template struct {
				Spec struct {
					AutomountServiceAccountToken *bool       `yaml:"automountServiceAccountToken"`
					InitContainers               []container `yaml:"initContainers"`
					Containers                   []container `yaml:"containers"`
				} `yaml:"spec"`
			} `yaml:"template"`
		} `yaml:"spec"`
	}

	want := map[string]string{
		"product-listing-api-schema-migrate-runner": "/app/product-listing-api-schema-migrate",
		"listingkit-schema-migrate-runner":          "/app/listingkit-schema-migrate",
		"listingkit-identity-preflight-runner":      "/app/listingkit-identity-preflight",
		"image-agent-temporal-v3-canary-runner":     "/app/image-agent-temporal-worker",
	}
	seen := map[string]bool{}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	for {
		var item deployment
		if err := decoder.Decode(&item); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		command, expected := want[item.Metadata.Name]
		if !expected {
			t.Fatalf("unexpected release runner %q", item.Metadata.Name)
		}
		seen[item.Metadata.Name] = true
		if item.Kind != "Deployment" || item.Spec.Replicas != 0 {
			t.Errorf("runner %s must be a zero-replica Deployment", item.Metadata.Name)
		}
		if item.Spec.Template.Spec.AutomountServiceAccountToken == nil || *item.Spec.Template.Spec.AutomountServiceAccountToken {
			t.Errorf("runner %s must not mount a service-account token", item.Metadata.Name)
		}
		if len(item.Spec.Template.Spec.InitContainers) != 1 || len(item.Spec.Template.Spec.InitContainers[0].Command) != 1 || item.Spec.Template.Spec.InitContainers[0].Command[0] != command {
			t.Errorf("runner %s must execute its one-shot command %s", item.Metadata.Name, command)
		}
		if len(item.Spec.Template.Spec.Containers) != 1 || item.Spec.Template.Spec.Containers[0].Image != "registry.k8s.io/pause@sha256:ee6521f290b2168b6e0935a181d4cff9be1ac3f505666ef0e3c98fae8199917a" {
			t.Errorf("runner %s must hold readiness with the pinned Kubernetes pause image", item.Metadata.Name)
		}
	}
	if len(seen) != len(want) {
		t.Fatalf("release runners=%v want all %v", seen, want)
	}
}

const (
	releaseGateTestDeployment = "listingkit-schema-migrate-runner"
	releaseGateTestImage      = "docker.io/example/api@sha256:" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	releaseGateHoldImage      = "registry.k8s.io/pause@sha256:ee6521f290b2168b6e0935a181d4cff9be1ac3f505666ef0e3c98fae8199917a"
)

func TestListingKitReleaseGateRunnerCanonicalContract(t *testing.T) {
	t.Parallel()

	output, logText, err := runReleaseGateScenario(t, releaseGateScenario{})
	if err != nil {
		t.Fatalf("run canonical release gate: %v\n%s", err, output)
	}
	for _, required := range []string{
		"create --dry-run=client --validate=false",
		"get deployment/" + releaseGateTestDeployment + " -o json",
		"apply -f",
		"patch deployment/" + releaseGateTestDeployment + " --type=strategic --patch-file",
		"scale deployment/" + releaseGateTestDeployment + " --replicas=1",
	} {
		if !strings.Contains(logText, required) {
			t.Errorf("canonical runner log missing %q:\n%s", required, logText)
		}
	}
	if got := strings.Count(logText, "scale deployment/"+releaseGateTestDeployment+" --replicas=0"); got != 2 {
		t.Errorf("runner must scale down before and after execution, got %d:\n%s", got, logText)
	}

}

func TestListingKitReleaseGateRunnerBindsReadinessToCurrentInvocation(t *testing.T) {
	t.Parallel()

	shared := &releaseGateSharedState{}
	firstOutput, firstLog, firstErr := runReleaseGateScenario(t, releaseGateScenario{runID: "424242", runAttempt: "1", rejectPods: true, sharedState: shared})
	if firstErr != nil {
		t.Fatalf("first current-rollout invocation failed: %v\n%s", firstErr, firstOutput)
	}
	firstInvocation := "listingkit-release-gate-v1:listingkit-schema-migrate-runner:424242:1"
	secondInvocation := "listingkit-release-gate-v1:listingkit-schema-migrate-runner:424242:2"
	firstGeneration := releaseGateLatestAvailableGeneration(t, shared)

	blockedOutput, blockedLog, blockedErr := runReleaseGateScenario(t, releaseGateScenario{
		runID:           "424242",
		runAttempt:      "2",
		rejectPods:      true,
		invocationDrift: firstInvocation,
		sharedState:     shared,
	})
	if blockedErr == nil || !strings.Contains(blockedOutput, "live release-gate runner contract differs from reviewed manifest") {
		t.Fatalf("attempt 2 must not accept attempt 1 availability before its template identity changes, err=%v output=%s", blockedErr, blockedOutput)
	}
	if got := releaseGateLatestAvailableInvocation(t, shared); got != firstInvocation {
		t.Fatalf("blocked attempt 2 availability retained invocation=%q, want prior attempt %q", got, firstInvocation)
	}

	secondOutput, secondLog, secondErr := runReleaseGateScenario(t, releaseGateScenario{runID: "424242", runAttempt: "2", rejectPods: true, sharedState: shared})
	if secondErr != nil {
		t.Fatalf("second current-rollout invocation failed: %v\n%s", secondErr, secondOutput)
	}
	if got := releaseGateLatestAvailableInvocation(t, shared); got != secondInvocation {
		t.Fatalf("attempt 2 must become available only after its annotation changes, got %q want %q", got, secondInvocation)
	}
	if got := releaseGateLatestAvailableGeneration(t, shared); got <= firstGeneration {
		t.Fatalf("attempt 2 must wait for a new Deployment generation, got %d after %d", got, firstGeneration)
	}

	for _, check := range []struct {
		name       string
		log        string
		invocation string
	}{
		{name: "first", log: firstLog, invocation: firstInvocation},
		{name: "blocked second", log: blockedLog, invocation: secondInvocation},
		{name: "second", log: secondLog, invocation: secondInvocation},
	} {
		if strings.Contains(check.log, "get pods") {
			t.Errorf("%s current-rollout proof must not query Pod collections:\n%s", check.name, check.log)
		}
		if !strings.Contains(check.log, `"listingkit.sh/release-gate-invocation":"`+check.invocation+`"`) {
			t.Errorf("%s invocation must patch the trusted Pod-template annotation %q:\n%s", check.name, check.invocation, check.log)
		}
	}
}

func TestListingKitReleaseGateWorkflowUsesTrustedInvocationIdentity(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatalf("parse ListingKit deploy workflow: %v", err)
	}
	var releaseGateSteps []string
	for _, step := range workflow.Jobs["deploy-api"].Steps {
		if strings.Contains(step.Run, "listingkit-run-release-gate-deployment.sh") {
			releaseGateSteps = append(releaseGateSteps, step.Run)
		}
	}
	if len(releaseGateSteps) != 4 {
		t.Fatalf("release-gate invocation count=%d, want 4", len(releaseGateSteps))
	}
	for index, run := range releaseGateSteps {
		if strings.Count(run, `--run-id "$GITHUB_RUN_ID"`) != 1 || strings.Count(run, `--run-attempt "$GITHUB_RUN_ATTEMPT"`) != 1 {
			t.Errorf("release-gate invocation %d must pass only the trusted GitHub run identity, run=%q", index, run)
		}
	}
}

func TestListingKitReleaseGateRoleForbidsPodPermissions(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "release-authority", "listingkit-api-release-role.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var role struct {
		Rules []struct {
			Resources []string `yaml:"resources"`
		} `yaml:"rules"`
	}
	if err := yaml.Unmarshal(content, &role); err != nil {
		t.Fatalf("parse API release Role: %v", err)
	}
	for _, rule := range role.Rules {
		for _, resource := range rule.Resources {
			if resource == "pods" || resource == "pods/status" {
				t.Fatalf("API release Role must not grant Pod permissions: %#v", rule.Resources)
			}
		}
	}
}

func TestListingKitReleaseGateRunnerRejectsStaleAvailableDeploymentGeneration(t *testing.T) {
	t.Parallel()

	output, logText, err := runReleaseGateScenario(t, releaseGateScenario{runID: "424242", runAttempt: "2", staleAvailableGeneration: true, rejectPods: true})
	if err == nil || !strings.Contains(output, "release-gate Deployment rollout did not become available") {
		t.Fatalf("a later attempt must reject the previous generation's available state, err=%v output=%s", err, output)
	}
	if strings.Contains(logText, "get pods") {
		t.Fatalf("stale-generation rejection must not fall back to Pod discovery:\n%s", logText)
	}
}

func TestListingKitReleaseGateRunnerRejectsMissingInvocationIdentityBeforeMutation(t *testing.T) {
	t.Parallel()

	for _, scenario := range []releaseGateScenario{
		{runID: "", runAttempt: "1"},
		{runID: "424242", runAttempt: ""},
		{runID: "not-a-decimal", runAttempt: "1"},
		{runID: "424242", runAttempt: "0"},
	} {
		output, logText, err := runReleaseGateScenario(t, scenario)
		if err == nil || !strings.Contains(output, "release-gate runner requires non-empty decimal --run-id and --run-attempt") {
			t.Fatalf("invalid invocation identity must fail before mutation, scenario=%+v err=%v output=%s", scenario, err, output)
		}
		for _, forbidden := range []string{" apply ", " patch ", "--replicas=1"} {
			if strings.Contains(" "+logText+" ", forbidden) {
				t.Errorf("invalid invocation identity must fail before mutation %q:\n%s", forbidden, logText)
			}
		}
	}
}

func TestListingKitReleaseGateRunnerRejectsCommandDrift(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerInitContainer(deployment)["command"] = []any{"/bin/true"}
	})
}

func TestListingKitReleaseGateRunnerRejectsArgsDrift(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerInitContainer(deployment)["args"] = []any{"--unreviewed"}
	})
}

func TestListingKitReleaseGateRunnerRejectsEnvironmentDrift(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerInitContainer(deployment)["env"] = []any{map[string]any{"name": "UNREVIEWED", "value": "true"}}
	})
}

func TestListingKitReleaseGateRunnerRejectsEnvironmentFromDrift(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerInitContainer(deployment)["envFrom"] = []any{map[string]any{"secretRef": map[string]any{"name": "unreviewed-secret"}}}
	})
}

func TestListingKitReleaseGateRunnerRejectsVolumeDrift(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerPodSpec(deployment)["volumes"] = []any{map[string]any{"name": "unreviewed", "emptyDir": map[string]any{}}}
	})
}

func TestListingKitReleaseGateRunnerRejectsVolumeMountDrift(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerInitContainer(deployment)["volumeMounts"] = []any{map[string]any{"name": "unreviewed", "mountPath": "/tmp/unreviewed"}}
	})
}

func TestListingKitReleaseGateRunnerRejectsServiceAccountDrift(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerPodSpec(deployment)["serviceAccountName"] = "unreviewed"
	})
}

func TestListingKitReleaseGateRunnerRejectsSecurityContextDrift(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerPodSpec(deployment)["securityContext"] = map[string]any{"runAsNonRoot": false}
	})
}

func TestListingKitReleaseGateRunnerRejectsResourceDrift(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerInitContainer(deployment)["resources"] = map[string]any{"requests": map[string]any{"cpu": "1"}}
	})
}

func TestListingKitReleaseGateRunnerRejectsInvocationAnnotationDrift(t *testing.T) {
	t.Parallel()
	output, logText, err := runReleaseGateScenario(t, releaseGateScenario{invocationDrift: "listingkit-release-gate-v1:listingkit-schema-migrate-runner:424242:999"})
	if err == nil || !strings.Contains(output, "live release-gate runner contract differs from reviewed manifest") {
		t.Fatalf("invocation annotation drift must fail canonical comparison, err=%v output=%s", err, output)
	}
	if strings.Contains(logText, "get pods") {
		t.Fatalf("invocation annotation drift must fail before Pod discovery:\n%s", logText)
	}
}

func TestListingKitReleaseGateRunnerRejectsExtraContainer(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		spec := runnerPodSpec(deployment)
		spec["containers"] = append(spec["containers"].([]any), map[string]any{"name": "unexpected", "image": releaseGateHoldImage})
	})
}

func TestListingKitReleaseGateRunnerRejectsServiceAccountTokenEnablement(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerPodSpec(deployment)["automountServiceAccountToken"] = true
	})
}

func TestListingKitReleaseGateRunnerRejectsCredentialScopeDrift(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		env := runnerInitContainer(deployment)["env"].([]any)
		valueFrom := env[0].(map[string]any)["valueFrom"].(map[string]any)
		valueFrom["secretKeyRef"].(map[string]any)["name"] = "unreviewed-secret"
	})
}

func TestListingKitReleaseGateRunnerRejectsHoldImageDrift(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerHoldContainer(deployment)["image"] = "registry.k8s.io/pause@sha256:" + strings.Repeat("b", 64)
	})
}

func TestListingKitReleaseGateRunnerFailsWhenReviewedDeploymentIsMissing(t *testing.T) {
	t.Parallel()

	output, logText, err := runReleaseGateScenario(t, releaseGateScenario{missing: true})
	if err == nil || !strings.Contains(output, "preinstalled release-gate Deployment is missing") {
		t.Fatalf("missing reviewed Deployment must fail closed, err=%v output=%s", err, output)
	}
	for _, forbidden := range []string{" apply ", " patch ", "--replicas=1"} {
		if strings.Contains(" "+logText+" ", forbidden) {
			t.Errorf("missing object must fail before mutation %q:\n%s", forbidden, logText)
		}
	}
	if got := strings.Count(logText, "scale deployment/"+releaseGateTestDeployment+" --replicas=0"); got != 1 {
		t.Errorf("missing object must still trigger unconditional cleanup, got %d:\n%s", got, logText)
	}
}

func TestListingKitReleaseGateRunnerRejectsImagePatchDrift(t *testing.T) {
	t.Parallel()
	output, logText, err := runReleaseGateScenario(t, releaseGateScenario{initImageDrift: "docker.io/example/api@sha256:" + strings.Repeat("b", 64)})
	if err == nil || !strings.Contains(output, "live release-gate runner contract differs from reviewed manifest") {
		t.Fatalf("post-patch init image drift must fail canonical comparison, err=%v output=%s", err, output)
	}
	if strings.Contains(logText, "get pods") {
		t.Fatalf("post-patch init image drift must fail before Pod discovery:\n%s", logText)
	}
}

func TestListingKitReleaseGateRunnerProvesCurrentRolloutWithoutPodAccess(t *testing.T) {
	t.Parallel()

	output, logText, err := runReleaseGateScenario(t, releaseGateScenario{runID: "424242", runAttempt: "1", rejectPods: true})
	if err != nil {
		t.Fatalf("Deployment availability must prove init completion without Pod access, err=%v output=%s", err, output)
	}
	if strings.Contains(logText, "get pods") {
		t.Fatalf("runner must not inspect Pod init termination directly, log:\n%s", logText)
	}
}

type releaseGateScenario struct {
	missing                  bool
	runID                    string
	runAttempt               string
	rejectPods               bool
	staleAvailableGeneration bool
	invocationDrift          string
	initImageDrift           string
	mutateDeployment         func(map[string]any)
	sharedState              *releaseGateSharedState
}

type releaseGateSharedState struct {
	livePath    string
	historyPath string
}

func assertReleaseGateContractRejected(t *testing.T, mutate func(map[string]any)) {
	t.Helper()
	output, logText, err := runReleaseGateScenario(t, releaseGateScenario{mutateDeployment: mutate})
	if err == nil || !strings.Contains(output, "live release-gate runner contract differs from reviewed manifest") {
		t.Fatalf("drifted live runner must fail canonical comparison, err=%v output=%s", err, output)
	}
	for _, required := range []string{"apply -f", "patch deployment/" + releaseGateTestDeployment + " --type=strategic --patch-file"} {
		if !strings.Contains(logText, required) {
			t.Errorf("drift test must reach reviewed re-apply and narrow image patch %q:\n%s", required, logText)
		}
	}
}

func runReleaseGateScenario(t *testing.T, scenario releaseGateScenario) (string, string, error) {
	t.Helper()
	if scenario.runID == "" && scenario.runAttempt == "" {
		scenario.runID = "424242"
		scenario.runAttempt = "1"
	}

	dir := t.TempDir()
	manifestPath, err := filepath.Abs(filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "release-authority", "listingkit-release-gate-runners.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	documents := decodeReleaseGateRunnerDocuments(t, manifestPath)
	renderedPath := filepath.Join(dir, "rendered.json")
	var rendered bytes.Buffer
	encoder := json.NewEncoder(&rendered)
	for _, document := range documents {
		if err := encoder.Encode(document); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(renderedPath, rendered.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	var reviewed map[string]any
	for _, document := range documents {
		if document["metadata"].(map[string]any)["name"] == releaseGateTestDeployment {
			reviewed = cloneJSONMap(t, document)
			break
		}
	}
	if reviewed == nil {
		t.Fatal("reviewed runner fixture is missing")
	}
	metadata := reviewed["metadata"].(map[string]any)
	metadata["namespace"] = "task-processor"
	metadata["generation"] = 3
	reviewed["spec"].(map[string]any)["replicas"] = 1
	runnerInitContainer(reviewed)["image"] = releaseGateTestImage
	reviewed["status"] = map[string]any{
		"observedGeneration":  3,
		"updatedReplicas":     1,
		"availableReplicas":   1,
		"unavailableReplicas": 0,
	}
	if scenario.staleAvailableGeneration {
		reviewed["status"].(map[string]any)["observedGeneration"] = 2
	}
	if scenario.mutateDeployment != nil {
		scenario.mutateDeployment(reviewed)
	}
	livePath := writeReleaseGateJSON(t, dir, "live-deployment.json", reviewed)
	historyPath := filepath.Join(dir, "available-generation-history.jsonl")
	if scenario.sharedState != nil {
		if scenario.sharedState.livePath == "" {
			scenario.sharedState.livePath = livePath
			scenario.sharedState.historyPath = historyPath
		} else {
			livePath = scenario.sharedState.livePath
			historyPath = scenario.sharedState.historyPath
		}
	}

	logPath := filepath.Join(dir, "kubectl.log")
	stateProgramPath := filepath.Join(dir, "fake-kubectl-state.py")
	if err := os.WriteFile(stateProgramPath, []byte(fakeReleaseGateKubectlState), 0o600); err != nil {
		t.Fatal(err)
	}
	writePreflightFake(t, filepath.Join(dir, "kubectl"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_KUBECTL_LOG"
case "$*" in
  *" create --dry-run=client --validate=false "*) cat "$FAKE_RENDERED_JSON" ;;
  *" get deployment/$FAKE_DEPLOYMENT -o json"*)
    if [[ "$FAKE_MISSING" == "true" ]]; then exit 1; fi
    cat "$FAKE_LIVE_DEPLOYMENT"
    ;;
  *" get pods "*)
    if [[ "$FAKE_REJECT_PODS" == "true" ]]; then exit 91; fi
    ;;
  *" patch deployment/"*" --type=strategic --patch-file "*)
    cat "${!#}" >> "$FAKE_KUBECTL_LOG"
    "$FAKE_PYTHON" "$FAKE_KUBECTL_STATE" patch "$FAKE_LIVE_DEPLOYMENT" "${!#}"
    ;;
  *" scale deployment/"*" --replicas="*)
    "$FAKE_PYTHON" "$FAKE_KUBECTL_STATE" scale "$FAKE_LIVE_DEPLOYMENT" "${!#}" "$FAKE_STALE_AVAILABLE_GENERATION"
    ;;
  *) : ;;
esac
`)
	jqProgramPath := filepath.Join(dir, "fake-jq.py")
	if err := os.WriteFile(jqProgramPath, []byte(fakeReleaseGateJQ), 0o600); err != nil {
		t.Fatal(err)
	}
	writePreflightFake(t, filepath.Join(dir, "jq"), `#!/usr/bin/env bash
set -euo pipefail
exec "$FAKE_PYTHON" "$FAKE_JQ_PROGRAM" "$@"
`)
	writePreflightFake(t, filepath.Join(dir, "sleep"), `#!/usr/bin/env bash
set -euo pipefail
"$FAKE_PYTHON" -c 'import time; time.sleep(1)'
`)
	pythonPath, err := exec.LookPath("python3")
	if err != nil {
		pythonPath, err = exec.LookPath("python")
	}
	if err != nil {
		t.Skip("Python is required for the faithful jq test double")
	}

	script, err := filepath.Abs(filepath.Join("..", "scripts", "listingkit-run-release-gate-deployment.sh"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(preflightBash(t), filepath.ToSlash(script),
		"--namespace", "task-processor",
		"--manifest", filepath.ToSlash(manifestPath),
		"--deployment", releaseGateTestDeployment,
		"--image", releaseGateTestImage,
		"--run-id", scenario.runID,
		"--run-attempt", scenario.runAttempt,
		"--timeout-seconds", "1")
	command.Env = append(os.Environ(),
		"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_KUBECTL_LOG="+logPath,
		"FAKE_RENDERED_JSON="+renderedPath,
		"FAKE_LIVE_DEPLOYMENT="+livePath,
		"FAKE_DEPLOYMENT="+releaseGateTestDeployment,
		fmt.Sprintf("FAKE_MISSING=%t", scenario.missing),
		fmt.Sprintf("FAKE_REJECT_PODS=%t", scenario.rejectPods),
		fmt.Sprintf("FAKE_STALE_AVAILABLE_GENERATION=%t", scenario.staleAvailableGeneration),
		"FAKE_INVOCATION_DRIFT="+scenario.invocationDrift,
		"FAKE_INIT_IMAGE_DRIFT="+scenario.initImageDrift,
		"FAKE_STATE_HISTORY="+historyPath,
		"FAKE_PYTHON="+pythonPath,
		"FAKE_KUBECTL_STATE="+stateProgramPath,
		"FAKE_JQ_PROGRAM="+jqProgramPath)
	output, runErr := command.CombinedOutput()
	logBytes, readErr := os.ReadFile(logPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatal(readErr)
	}
	return string(output), string(logBytes), runErr
}

func decodeReleaseGateRunnerDocuments(t *testing.T, path string) []map[string]any {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var documents []map[string]any
	for {
		var document map[string]any
		if err := decoder.Decode(&document); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if len(document) > 0 {
			documents = append(documents, document)
		}
	}
	return documents
}

func cloneJSONMap(t *testing.T, value map[string]any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var clone map[string]any
	if err := json.Unmarshal(encoded, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func writeReleaseGateJSON(t *testing.T, dir, name string, value any) string {
	t.Helper()
	path := filepath.Join(dir, name)
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func releaseGateLatestAvailableInvocation(t *testing.T, shared *releaseGateSharedState) string {
	t.Helper()
	entry := releaseGateLatestAvailableState(t, shared)
	return entry["invocation"].(string)
}

func releaseGateLatestAvailableGeneration(t *testing.T, shared *releaseGateSharedState) int {
	t.Helper()
	entry := releaseGateLatestAvailableState(t, shared)
	return int(entry["generation"].(float64))
}

func releaseGateLatestAvailableState(t *testing.T, shared *releaseGateSharedState) map[string]any {
	t.Helper()
	content, err := os.ReadFile(shared.historyPath)
	if err != nil {
		t.Fatalf("read release-gate state history: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) == 0 {
		t.Fatal("release-gate state history has no available Deployment generation")
	}
	var entry map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("decode latest release-gate state: %v", err)
	}
	return entry
}

func runnerPodSpec(deployment map[string]any) map[string]any {
	return deployment["spec"].(map[string]any)["template"].(map[string]any)["spec"].(map[string]any)
}

func runnerInitContainer(deployment map[string]any) map[string]any {
	return runnerPodSpec(deployment)["initContainers"].([]any)[0].(map[string]any)
}

func runnerHoldContainer(deployment map[string]any) map[string]any {
	return runnerPodSpec(deployment)["containers"].([]any)[0].(map[string]any)
}

func readReleaseGateFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}

const fakeReleaseGateJQ = `import json
import sys

args = sys.argv[1:]
values = {}
i = 0
while i < len(args):
    if args[i] in ("--arg", "--argjson") and i + 2 < len(args):
        values[args[i + 1]] = json.loads(args[i + 2]) if args[i] == "--argjson" else args[i + 2]
        i += 3
    else:
        i += 1

marker_index = next((i for i, arg in enumerate(args) if "listingkit-runner-" in arg), -1)
if marker_index < 0:
    raise SystemExit("unsupported jq filter")
filter_text = args[marker_index]
input_path = args[marker_index + 1] if marker_index + 1 < len(args) else None

def read_text():
    if input_path:
        with open(input_path, "r", encoding="utf-8") as source:
            return source.read()
    return sys.stdin.read()

def write(value):
    if isinstance(value, str):
        sys.stdout.write(value + "\n")
    else:
        json.dump(value, sys.stdout, sort_keys=True, separators=(",", ":"))
        sys.stdout.write("\n")

def parse_stream(text):
    decoder = json.JSONDecoder()
    result = []
    offset = 0
    while offset < len(text):
        while offset < len(text) and text[offset].isspace():
            offset += 1
        if offset >= len(text):
            break
        item, offset = decoder.raw_decode(text, offset)
        result.append(item)
    return result

def canonical_container(container):
    return {
        "name": container.get("name"),
        "image": container.get("image"),
        "imagePullPolicy": container.get("imagePullPolicy"),
        "command": container.get("command", []),
        "args": container.get("args", []),
        "env": container.get("env", []),
        "envFrom": container.get("envFrom", []),
        "volumeMounts": container.get("volumeMounts", []),
        "securityContext": container.get("securityContext", {}),
        "resources": container.get("resources", {}),
    }

def canonical(obj):
    spec = obj["spec"]
    template = spec["template"]
    pod = template["spec"]
    return {
        "apiVersion": obj.get("apiVersion"),
        "kind": obj.get("kind"),
        "metadata": {
            "name": obj.get("metadata", {}).get("name"),
            "namespace": obj.get("metadata", {}).get("namespace", values.get("namespace")),
            "labels": obj.get("metadata", {}).get("labels", {}),
        },
        "spec": {
            "replicas": spec.get("replicas"),
            "revisionHistoryLimit": spec.get("revisionHistoryLimit"),
            "progressDeadlineSeconds": spec.get("progressDeadlineSeconds"),
            "strategy": spec.get("strategy"),
            "selector": spec.get("selector"),
            "template": {
                "metadata": {
                    "labels": template.get("metadata", {}).get("labels", {}),
                    "annotations": template.get("metadata", {}).get("annotations", {}),
                },
                "spec": {
                    "automountServiceAccountToken": pod.get("automountServiceAccountToken"),
                    "serviceAccountName": pod.get("serviceAccountName", "default"),
                    "securityContext": pod.get("securityContext", {}),
                    "imagePullSecrets": pod.get("imagePullSecrets", []),
                    "initContainers": [canonical_container(item) for item in pod.get("initContainers", [])],
                    "containers": [canonical_container(item) for item in pod.get("containers", [])],
                    "volumes": pod.get("volumes", []),
                },
            },
        },
    }

if "listingkit-runner-select-v1" in filter_text:
    matches = [item for item in parse_stream(read_text())
               if item.get("apiVersion") == "apps/v1" and item.get("kind") == "Deployment"
               and item.get("metadata", {}).get("name") == values.get("deployment")]
    if len(matches) != 1:
        raise SystemExit(5)
    selected = matches[0]
    pod = selected.get("spec", {}).get("template", {}).get("spec", {})
    init = pod.get("initContainers", [])
    hold = pod.get("containers", [])
    valid = (selected.get("spec", {}).get("replicas") == 0
             and pod.get("automountServiceAccountToken") is False
             and len(init) == 1 and init[0].get("name") == "release-gate"
             and isinstance(init[0].get("command"), list) and len(init[0]["command"]) > 0
             and len(hold) == 1 and hold[0].get("name") == "hold-after-gate"
             and hold[0].get("image") == values.get("hold_image"))
    if not valid:
        raise SystemExit(6)
    write(selected)
elif "listingkit-runner-release-gate-patch-v2" in filter_text:
    write({"spec": {"template": {"metadata": {"annotations": {
        "listingkit.sh/release-gate-invocation": values["invocation"]
    }}, "spec": {"initContainers": [{"name": "release-gate", "image": values["image"]}]}}}})
elif "listingkit-runner-expected-v2" in filter_text:
    selected = json.loads(read_text())
    selected.setdefault("metadata", {})["namespace"] = values["namespace"]
    selected["spec"]["replicas"] = 1
    selected["spec"]["template"]["spec"]["initContainers"][0]["image"] = values["image"]
    selected["spec"]["template"].setdefault("metadata", {}).setdefault("annotations", {})["listingkit.sh/release-gate-invocation"] = values["invocation"]
    write(selected)
elif "listingkit-runner-canonical-v1" in filter_text:
    write(canonical(json.loads(read_text())))
elif "listingkit-runner-available-v1" in filter_text:
    obj = json.loads(read_text())
    status = obj.get("status", {})
    ok = (status.get("observedGeneration", -1) >= obj.get("metadata", {}).get("generation", 0)
          and obj.get("spec", {}).get("replicas") == 1
          and status.get("updatedReplicas") == 1
          and status.get("availableReplicas") == 1
          and status.get("unavailableReplicas", 0) == 0)
    if not ok:
        raise SystemExit(1)
    write(True)
else:
    raise SystemExit("unsupported jq operation")
`

const fakeReleaseGateKubectlState = `import json
import os
import sys

mode, state_path, *rest = sys.argv[1:]

with open(state_path, "r", encoding="utf-8") as source:
    deployment = json.load(source)

def write_state():
    with open(state_path, "w", encoding="utf-8") as destination:
        json.dump(deployment, destination, sort_keys=True, separators=(",", ":"))

if mode == "patch":
    patch_path, = rest
    with open(patch_path, "r", encoding="utf-8") as source:
        patch = json.load(source)
    template = deployment["spec"]["template"]
    annotations = template.setdefault("metadata", {}).setdefault("annotations", {})
    annotations.update(patch["spec"]["template"]["metadata"]["annotations"])
    if os.environ.get("FAKE_INVOCATION_DRIFT"):
        annotations["listingkit.sh/release-gate-invocation"] = os.environ["FAKE_INVOCATION_DRIFT"]
    requested = patch["spec"]["template"]["spec"]["initContainers"]
    for requested_container in requested:
        for live_container in template["spec"]["initContainers"]:
            if live_container.get("name") == requested_container.get("name"):
                live_container.update(requested_container)
                if os.environ.get("FAKE_INIT_IMAGE_DRIFT"):
                    live_container["image"] = os.environ["FAKE_INIT_IMAGE_DRIFT"]
                break
        else:
            raise SystemExit("requested init container was not found")
    deployment["metadata"]["generation"] = deployment["metadata"].get("generation", 0) + 1
elif mode == "scale":
    replicas_arg, stale = rest
    replicas = int(replicas_arg.removeprefix("--replicas="))
    deployment["spec"]["replicas"] = replicas
    deployment["metadata"]["generation"] = deployment["metadata"].get("generation", 0) + 1
    if replicas == 1:
        generation = deployment["metadata"]["generation"]
        deployment["status"] = {
            "observedGeneration": generation - 1 if stale == "true" else generation,
            "updatedReplicas": 1,
            "availableReplicas": 1,
            "unavailableReplicas": 0,
        }
        history_path = os.environ.get("FAKE_STATE_HISTORY")
        if history_path:
            annotation = deployment["spec"]["template"].get("metadata", {}).get("annotations", {}).get("listingkit.sh/release-gate-invocation")
            with open(history_path, "a", encoding="utf-8") as history:
                history.write(json.dumps({"generation": generation, "invocation": annotation}, sort_keys=True) + "\n")
    else:
        deployment["status"] = {
            "observedGeneration": deployment["metadata"]["generation"],
            "updatedReplicas": 0,
            "availableReplicas": 0,
            "unavailableReplicas": 0,
        }
else:
    raise SystemExit("unsupported kubectl state operation")

write_state()
`
