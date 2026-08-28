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
		"patch deployment/" + releaseGateTestDeployment + " --type=json --patch-file",
		"scale deployment/" + releaseGateTestDeployment + " --replicas=1",
		"get pods -l app=" + releaseGateTestDeployment + " -o json",
	} {
		if !strings.Contains(logText, required) {
			t.Errorf("canonical runner log missing %q:\n%s", required, logText)
		}
	}
	if got := strings.Count(logText, "scale deployment/"+releaseGateTestDeployment+" --replicas=0"); got != 2 {
		t.Errorf("runner must scale down before and after execution, got %d:\n%s", got, logText)
	}

	script := readReleaseGateFile(t, filepath.Join("..", "scripts", "listingkit-run-release-gate-deployment.sh"))
	for _, required := range []string{
		"--manifest",
		"listingkit-runner-select-v1",
		"listingkit-runner-canonical-v1",
		"listingkit-runner-init-result-v1",
		"automountServiceAccountToken",
		"serviceAccountName",
		"envFrom",
		"volumeMounts",
		"securityContext",
		"resources",
		"imagePullPolicy",
	} {
		if !strings.Contains(script, required) {
			t.Errorf("release-gate helper canonical contract missing %q", required)
		}
	}
	for _, forbidden := range []string{"--container", "kubectl create -f", "kubectl delete"} {
		if strings.Contains(script, forbidden) {
			t.Errorf("release-gate helper must not contain %q", forbidden)
		}
	}

	workflow := readReleaseGateFile(t, filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml"))
	manifestArgument := "--manifest .workflow-tools/deployments/kubernetes/listingkit-workbench/release-authority/listingkit-release-gate-runners.yaml"
	if got := strings.Count(workflow, manifestArgument); got != 4 {
		t.Errorf("all four release-gate invocations must pass the reviewed aggregate manifest, got %d", got)
	}
	for _, deployment := range []string{
		"product-listing-api-schema-migrate-runner",
		"listingkit-schema-migrate-runner",
		"listingkit-identity-preflight-runner",
		"image-agent-temporal-v3-canary-runner",
	} {
		if !strings.Contains(workflow, "--deployment "+deployment) {
			t.Errorf("release workflow is missing exact reviewed runner %q", deployment)
		}
	}
	if strings.Contains(workflow, "--container release-gate") {
		t.Error("release workflow must not choose the release-gate container")
	}
}

func TestListingKitReleaseGateRunnerRejectsCommandDrift(t *testing.T) {
	t.Parallel()
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerInitContainer(deployment)["command"] = []any{"/bin/true"}
	})
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
	assertReleaseGateContractRejected(t, func(deployment map[string]any) {
		runnerInitContainer(deployment)["image"] = "docker.io/example/api@sha256:" + strings.Repeat("b", 64)
	})
}

func TestListingKitReleaseGateRunnerRequiresSuccessfulInitTermination(t *testing.T) {
	t.Parallel()

	output, logText, err := runReleaseGateScenario(t, releaseGateScenario{failedInit: true})
	if err == nil || !strings.Contains(output, "release-gate init container terminated unsuccessfully") {
		t.Fatalf("failed init termination must fail, err=%v output=%s", err, output)
	}
	if !strings.Contains(logText, "get pods -l app="+releaseGateTestDeployment+" -o json") {
		t.Fatalf("runner must inspect Pod init termination, log:\n%s", logText)
	}
}

type releaseGateScenario struct {
	missing          bool
	failedInit       bool
	mutateDeployment func(map[string]any)
}

func assertReleaseGateContractRejected(t *testing.T, mutate func(map[string]any)) {
	t.Helper()
	output, logText, err := runReleaseGateScenario(t, releaseGateScenario{mutateDeployment: mutate})
	if err == nil || !strings.Contains(output, "live release-gate runner contract differs from reviewed manifest") {
		t.Fatalf("drifted live runner must fail canonical comparison, err=%v output=%s", err, output)
	}
	for _, required := range []string{"apply -f", "patch deployment/" + releaseGateTestDeployment + " --type=json --patch-file"} {
		if !strings.Contains(logText, required) {
			t.Errorf("drift test must reach reviewed re-apply and narrow image patch %q:\n%s", required, logText)
		}
	}
}

func runReleaseGateScenario(t *testing.T, scenario releaseGateScenario) (string, string, error) {
	t.Helper()

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
	if scenario.mutateDeployment != nil {
		scenario.mutateDeployment(reviewed)
	}
	livePath := writeReleaseGateJSON(t, dir, "live-deployment.json", reviewed)

	exitCode := 0
	reason := "Completed"
	if scenario.failedInit {
		exitCode = 17
		reason = "Error"
	}
	pods := map[string]any{"items": []any{map[string]any{
		"metadata": map[string]any{"name": "release-gate-pod"},
		"spec": map[string]any{
			"initContainers": []any{map[string]any{"name": "release-gate", "image": releaseGateTestImage}},
			"containers":     []any{map[string]any{"name": "hold-after-gate", "image": releaseGateHoldImage}},
		},
		"status": map[string]any{
			"initContainerStatuses": []any{map[string]any{
				"name":  "release-gate",
				"state": map[string]any{"terminated": map[string]any{"exitCode": exitCode, "reason": reason}},
			}},
			"containerStatuses": []any{map[string]any{"name": "hold-after-gate", "ready": true}},
		},
	}}}
	podsPath := writeReleaseGateJSON(t, dir, "pods.json", pods)

	logPath := filepath.Join(dir, "kubectl.log")
	writePreflightFake(t, filepath.Join(dir, "kubectl"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_KUBECTL_LOG"
case "$*" in
  *" create --dry-run=client --validate=false "*) cat "$FAKE_RENDERED_JSON" ;;
  *" get deployment/$FAKE_DEPLOYMENT -o json"*)
    if [[ "$FAKE_MISSING" == "true" ]]; then exit 1; fi
    cat "$FAKE_LIVE_DEPLOYMENT"
    ;;
  *" get pods -l "*" -o json"*) cat "$FAKE_PODS" ;;
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
		"--timeout-seconds", "30")
	command.Env = append(os.Environ(),
		"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_KUBECTL_LOG="+logPath,
		"FAKE_RENDERED_JSON="+renderedPath,
		"FAKE_LIVE_DEPLOYMENT="+livePath,
		"FAKE_PODS="+podsPath,
		"FAKE_DEPLOYMENT="+releaseGateTestDeployment,
		fmt.Sprintf("FAKE_MISSING=%t", scenario.missing),
		"FAKE_PYTHON="+pythonPath,
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
                "metadata": {"labels": template.get("metadata", {}).get("labels", {})},
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
elif "listingkit-runner-image-patch-v1" in filter_text:
    write([{"op": "replace", "path": "/spec/template/spec/initContainers/0/image", "value": values["image"]}])
elif "listingkit-runner-expected-v1" in filter_text:
    selected = json.loads(read_text())
    selected.setdefault("metadata", {})["namespace"] = values["namespace"]
    selected["spec"]["replicas"] = 1
    selected["spec"]["template"]["spec"]["initContainers"][0]["image"] = values["image"]
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
elif "listingkit-runner-selector-v1" in filter_text:
    obj = json.loads(read_text())
    labels = obj["spec"]["selector"]["matchLabels"]
    write(",".join(f"{key}={labels[key]}" for key in sorted(labels)))
elif "listingkit-runner-init-result-v1" in filter_text:
    items = json.loads(read_text()).get("items", [])
    result = "pending"
    if len(items) == 1 and items[0].get("metadata", {}).get("deletionTimestamp") is None:
        pod = items[0]
        init_spec = pod.get("spec", {}).get("initContainers", [])
        hold_spec = pod.get("spec", {}).get("containers", [])
        init_status = pod.get("status", {}).get("initContainerStatuses", [])
        hold_status = pod.get("status", {}).get("containerStatuses", [])
        if (len(init_spec) == 1 and init_spec[0].get("name") == "release-gate"
                and init_spec[0].get("image") == values.get("image")
                and len(hold_spec) == 1 and hold_spec[0].get("name") == "hold-after-gate"
                and hold_spec[0].get("image") == values.get("hold_image")
                and len(init_status) == 1 and init_status[0].get("name") == "release-gate"):
            terminated = init_status[0].get("state", {}).get("terminated")
            if terminated:
                if (terminated.get("exitCode") == 0 and terminated.get("reason") == "Completed"
                        and len(hold_status) == 1 and hold_status[0].get("name") == "hold-after-gate"
                        and hold_status[0].get("ready") is True):
                    result = "success"
                else:
                    result = "failed"
    write(result)
else:
    raise SystemExit("unsupported jq operation")
`
