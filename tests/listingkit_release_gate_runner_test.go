package tests

import (
	"bytes"
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

func TestListingKitReleaseGateDriverUsesOnlyNamedDeploymentAndAlwaysScalesDown(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "kubectl.log")
	writePreflightFake(t, filepath.Join(dir, "kubectl"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_KUBECTL_LOG"
if [[ "$*" == *" get deployment/listingkit-schema-migrate-runner "* ]]; then
  printf '3 3 1 1 1 '
fi
`)
	script, err := filepath.Abs(filepath.Join("..", "scripts", "listingkit-run-release-gate-deployment.sh"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(preflightBash(t), filepath.ToSlash(script),
		"--namespace", "task-processor",
		"--deployment", "listingkit-schema-migrate-runner",
		"--container", "release-gate",
		"--image", "docker.io/example/api@sha256:"+strings.Repeat("a", 64),
		"--timeout-seconds", "30")
	command.Env = append(os.Environ(),
		"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_KUBECTL_LOG="+logPath,
		"GITHUB_RUN_ID=123",
		"GITHUB_RUN_ATTEMPT=2")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run release gate: %v\n%s", err, output)
	}
	logBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	logText := string(logBytes)
	for _, required := range []string{
		"scale deployment/listingkit-schema-migrate-runner --replicas=0",
		"set image deployment/listingkit-schema-migrate-runner release-gate=docker.io/example/api@sha256:",
		"patch deployment/listingkit-schema-migrate-runner --type=merge",
		"scale deployment/listingkit-schema-migrate-runner --replicas=1",
		"get deployment/listingkit-schema-migrate-runner",
	} {
		if !strings.Contains(logText, required) {
			t.Errorf("release gate log missing %q:\n%s", required, logText)
		}
	}
	if strings.Count(logText, "scale deployment/listingkit-schema-migrate-runner --replicas=0") != 2 {
		t.Errorf("runner must scale down before and after execution:\n%s", logText)
	}
}
