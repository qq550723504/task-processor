package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type imageAgentWorkloadManifest struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Replicas int `yaml:"replicas"`
		Selector struct {
			MatchLabels map[string]string `yaml:"matchLabels"`
		} `yaml:"selector"`
		ActiveDeadlineSeconds int `yaml:"activeDeadlineSeconds"`
		BackoffLimit          int `yaml:"backoffLimit"`
		Template              struct {
			Metadata struct {
				Labels map[string]string `yaml:"labels"`
			} `yaml:"metadata"`
			Spec struct {
				RestartPolicy  string                        `yaml:"restartPolicy"`
				InitContainers []imageAgentWorkloadContainer `yaml:"initContainers"`
				Containers     []imageAgentWorkloadContainer `yaml:"containers"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

type imageAgentWorkloadContainer struct {
	Name    string   `yaml:"name"`
	Image   string   `yaml:"image"`
	Command []string `yaml:"command"`
	Args    []string `yaml:"args"`
	EnvFrom []struct {
		ConfigMapRef *struct {
			Name string `yaml:"name"`
		} `yaml:"configMapRef"`
		SecretRef *struct {
			Name string `yaml:"name"`
		} `yaml:"secretRef"`
	} `yaml:"envFrom"`
	Env []struct {
		Name      string `yaml:"name"`
		ValueFrom *struct {
			ConfigMapKeyRef *struct {
				Name string `yaml:"name"`
				Key  string `yaml:"key"`
			} `yaml:"configMapKeyRef"`
			SecretKeyRef *struct {
				Name string `yaml:"name"`
				Key  string `yaml:"key"`
			} `yaml:"secretKeyRef"`
		} `yaml:"valueFrom"`
	} `yaml:"env"`
}

func TestListingKitImageAgentDeploymentsAndCanaryAreCapabilityIsolated(t *testing.T) {
	base := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench")
	v2 := loadImageAgentWorkloadManifest(t, filepath.Join(base, "base", "image-agent-temporal-worker-deployment.yaml"))
	v3 := loadImageAgentWorkloadManifest(t, filepath.Join(base, "base", "image-agent-temporal-worker-v3-deployment.yaml"))
	canary := loadImageAgentWorkloadFromMultiDoc(t, filepath.Join(base, "release-authority", "listingkit-release-gate-runners.yaml"), "image-agent-temporal-v3-canary-runner")

	assertImageAgentDeployment(t, v2, "image-agent-temporal-worker", "image-agent-temporal-worker", "v2", "image-agent-manual")
	assertImageAgentDeployment(t, v3, "image-agent-temporal-worker-v3", "image-agent-temporal-worker-v3", "v3", "image-agent-manual-v3")
	if v2.Spec.Selector.MatchLabels["app"] == v3.Spec.Selector.MatchLabels["app"] {
		t.Fatal("v2 and v3 image-agent workers must have distinct selectors")
	}
	if got, want := onlyImageAgentContainer(t, v3).Image, onlyImageAgentContainer(t, v2).Image; got != want {
		t.Fatalf("v2 and v3 workers must start from the same application image, v2=%q v3=%q", want, got)
	}

	if canary.Kind != "Deployment" || canary.Metadata.Name != "image-agent-temporal-v3-canary-runner" || canary.Spec.Replicas != 0 {
		t.Fatalf("v3 compatibility canary must be a fixed zero-replica Deployment, got kind=%q name=%q replicas=%d", canary.Kind, canary.Metadata.Name, canary.Spec.Replicas)
	}
	if len(canary.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatalf("canary runner must contain one release-gate init container")
	}
	canaryContainer := canary.Spec.Template.Spec.InitContainers[0]
	if !reflect.DeepEqual(canaryContainer.Command, []string{"/app/image-agent-temporal-worker"}) ||
		!reflect.DeepEqual(canaryContainer.Args, []string{"-canary", "-canary-task-queue", "image-agent-manual-v3-canary", "-log-level", "info"}) {
		t.Fatalf("canary must invoke only the side-effect-free v3 compatibility mode, command=%#v args=%#v", canaryContainer.Command, canaryContainer.Args)
	}
	for _, source := range canaryContainer.EnvFrom {
		if source.SecretRef != nil {
			t.Fatalf("compatibility canary must not import Secret %q", source.SecretRef.Name)
		}
	}
	for _, variable := range canaryContainer.Env {
		if variable.ValueFrom != nil && variable.ValueFrom.SecretKeyRef != nil {
			t.Fatalf("compatibility canary must not receive Secret key %q", variable.ValueFrom.SecretKeyRef.Key)
		}
	}
}

func loadImageAgentWorkloadFromMultiDoc(t *testing.T, path, name string) imageAgentWorkloadManifest {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read image-agent workload %s: %v", path, err)
	}
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	for {
		var manifest imageAgentWorkloadManifest
		if err := decoder.Decode(&manifest); err != nil {
			t.Fatalf("find workload %s in %s: %v", name, path, err)
		}
		if manifest.Metadata.Name == name {
			return manifest
		}
	}
}

func TestListingKitDeployOrdersImageAgentGatesBeforeAPIRouting(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml"))
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
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
	steps := workflow.Jobs["deploy-api"].Steps
	indexes := map[string]int{}
	for index, step := range steps {
		for key, fragment := range map[string]string{
			"schema":      "--deployment product-listing-api-schema-migrate-runner",
			"v2_apply":    "image-agent-temporal-worker-deployment.yaml",
			"v2_restart":  "rollout restart deployment/image-agent-temporal-worker\n",
			"v2_wait":     "rollout status deployment/image-agent-temporal-worker --timeout=5m",
			"v3_apply":    "image-agent-temporal-worker-v3-deployment.yaml",
			"v3_restart":  "rollout restart deployment/image-agent-temporal-worker-v3",
			"v3_wait":     "rollout status deployment/image-agent-temporal-worker-v3 --timeout=5m",
			"canary_gate": "--deployment image-agent-temporal-v3-canary-runner",
			"api_apply":   "product-listing-api-deployment.yaml",
			"api_stamp":   "patch deployment product-listing-api",
			"api_wait":    "rollout status deployment/product-listing-api --timeout=5m",
		} {
			if strings.Contains(step.Run, fragment) {
				indexes[key] = index
			}
		}
	}
	ordered := []string{"schema", "v2_apply", "v2_restart", "v2_wait", "canary_gate", "v3_apply", "v3_restart", "v3_wait", "api_apply", "api_stamp", "api_wait"}
	previous := -1
	for _, key := range ordered {
		index, ok := indexes[key]
		if !ok {
			t.Fatalf("deploy workflow is missing ordered image-agent gate %q", key)
		}
		if index < previous {
			t.Fatalf("deploy workflow gate %q at step %d must follow previous gate at step %d", key, index, previous)
		}
		previous = index
	}
	for _, expected := range []struct {
		key       string
		container string
	}{
		{key: "v2_apply", container: "image-agent-temporal-worker"},
		{key: "v3_apply", container: "image-agent-temporal-worker-v3"},
	} {
		run := steps[indexes[expected.key]].Run
		if !strings.Contains(run, "listingkit-apply-image-agent-worker-deployment.sh") ||
			!strings.Contains(run, "--container "+expected.container) ||
			!strings.Contains(run, "--image \"$API_CANDIDATE_IMAGE\"") {
			t.Errorf("deploy gate %q must apply its named container with the immutable candidate image, run=%q", expected.key, run)
		}
	}
	canaryRun := steps[indexes["canary_gate"]].Run
	for _, required := range []string{"listingkit-run-release-gate-deployment.sh", "--manifest .workflow-tools/deployments/kubernetes/listingkit-workbench/release-authority/listingkit-release-gate-runners.yaml", "--image \"$API_CANDIDATE_IMAGE\"", "--timeout-seconds 300"} {
		if !strings.Contains(canaryRun, required) {
			t.Errorf("canary runner must contain %q", required)
		}
	}
	for _, step := range steps {
		if !strings.Contains(step.Run, "image-agent-temporal-worker") {
			continue
		}
		if strings.Contains(step.Run, "listingkit-apply-image-agent-worker-deployment.sh") && !strings.Contains(step.Run, "--image \"$API_CANDIDATE_IMAGE\"") {
			t.Errorf("image-agent apply step %q must use the immutable API candidate image", step.Name)
		}
	}
}

func loadImageAgentWorkloadManifest(t *testing.T, path string) imageAgentWorkloadManifest {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read image-agent workload %s: %v", path, err)
	}
	var manifest imageAgentWorkloadManifest
	if err := yaml.Unmarshal(content, &manifest); err != nil {
		t.Fatalf("parse image-agent workload %s: %v", path, err)
	}
	return manifest
}

func onlyImageAgentContainer(t *testing.T, manifest imageAgentWorkloadManifest) imageAgentWorkloadContainer {
	t.Helper()
	if len(manifest.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("%s has %d containers, want one", manifest.Metadata.Name, len(manifest.Spec.Template.Spec.Containers))
	}
	return manifest.Spec.Template.Spec.Containers[0]
}

func assertImageAgentDeployment(t *testing.T, manifest imageAgentWorkloadManifest, name, containerName, wireMode, taskQueue string) {
	t.Helper()
	if manifest.Kind != "Deployment" || manifest.Metadata.Name != name {
		t.Fatalf("image-agent worker must be Deployment %q, got kind=%q name=%q", name, manifest.Kind, manifest.Metadata.Name)
	}
	if manifest.Spec.Selector.MatchLabels["app"] != name || manifest.Spec.Template.Metadata.Labels["app"] != name {
		t.Fatalf("deployment %s selector and pod label must both be %q", name, name)
	}
	container := onlyImageAgentContainer(t, manifest)
	if container.Name != containerName {
		t.Fatalf("deployment %s container=%q want=%q", name, container.Name, containerName)
	}
	wantArgs := []string{"-config", "config/config-prod.yaml", "-log-level", "info", "-wire-mode", wireMode, "-task-queue", taskQueue}
	if !reflect.DeepEqual(container.Args, wantArgs) {
		t.Fatalf("deployment %s args=%#v want=%#v", name, container.Args, wantArgs)
	}
}

func TestListingKitDeployPreflightsBeforeItsAuthorizedDeploymentMutations(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}

	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name            string `yaml:"name"`
				Run             string `yaml:"run"`
				If              string `yaml:"if"`
				ContinueOnError bool   `yaml:"continue-on-error"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatalf("parse ListingKit deploy workflow: %v", err)
	}

	deployJob, ok := workflow.Jobs["deploy-api"]
	if !ok {
		t.Fatal("ListingKit deploy workflow is missing deploy-api job")
	}

	preflightIndex := -1
	immutableDeploymentIndexes := make([]int, 0, 1)
	releaseIdentityStampIndexes := make([]int, 0, 1)
	for index, step := range deployJob.Steps {
		if strings.Contains(step.Run, "--deployment listingkit-identity-preflight-runner") {
			preflightIndex = index
			if step.ContinueOnError {
				t.Error("identity preflight step must block deployment when its caller returns failure")
			}
			if !strings.Contains(step.Run, "--image \"$PREFLIGHT_RUNNER_IMAGE\"") {
				t.Error("identity preflight must run in its distinct digest-pinned runner image")
			}
			if strings.Contains(step.Run, "--image-tag") {
				t.Error("identity preflight must not infer a fixed-registry image from a tag")
			}
		}
		if strings.Contains(step.Run, "scripts/listingkit-apply-api-deployment.sh") {
			immutableDeploymentIndexes = append(immutableDeploymentIndexes, index)
			if step.If != "" && step.If != "${{ success() }}" {
				t.Errorf("immutable deployment step must require prior success, got if: %q", step.If)
			}
			for _, required := range []string{
				"--image \"$API_CANDIDATE_IMAGE\"",
				"--manifest .workflow-tools/deployments/kubernetes/listingkit-workbench/base/product-listing-api-deployment.yaml",
			} {
				if !strings.Contains(step.Run, required) {
					t.Errorf("immutable deployment step must contain %q", required)
				}
			}
		}
		if strings.Contains(step.Run, "patch deployment product-listing-api") {
			releaseIdentityStampIndexes = append(releaseIdentityStampIndexes, index)
			for _, required := range []string{
				"listingkit.sh/api-release-run-id",
				"listingkit.sh/api-release-run-attempt",
				"listingkit.sh/api-release-image",
			} {
				if !strings.Contains(step.Run, required) {
					t.Errorf("release identity stamp must contain %q", required)
				}
			}
		}
		for _, forbidden := range []string{
			"kubectl set image",
			"kubectl -n ${{ env.K8S_NAMESPACE }} apply -f deployments/kubernetes/listingkit-workbench/base/product-listing-api-deployment.yaml",
		} {
			if strings.Contains(step.Run, forbidden) {
				t.Errorf("deploy-api step %q contains forbidden deployment mutation %q", step.Name, forbidden)
			}
		}
	}

	if preflightIndex < 0 {
		t.Fatal("deploy-api job is missing the identity preflight driver")
	}
	if len(immutableDeploymentIndexes) != 1 {
		t.Fatalf("deploy-api job must contain exactly one immutable deployment mutation, got %d", len(immutableDeploymentIndexes))
	}
	if len(releaseIdentityStampIndexes) != 1 {
		t.Fatalf("deploy-api job must contain exactly one API-owned release identity stamp, got %d", len(releaseIdentityStampIndexes))
	}
	if immutableDeploymentIndexes[0] <= preflightIndex {
		t.Fatalf("immutable deployment step %d must run after identity preflight step %d", immutableDeploymentIndexes[0], preflightIndex)
	}
	if releaseIdentityStampIndexes[0] <= immutableDeploymentIndexes[0] {
		t.Fatalf("release identity stamp step %d must run after immutable deployment step %d", releaseIdentityStampIndexes[0], immutableDeploymentIndexes[0])
	}
}

func TestListingKitDeployOwnsImageAgentWorkerBuildManifestAndImmutableRollout(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	workflowBytes, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}
	workflow := string(workflowBytes)
	for _, required := range []string{
		"scripts/listingkit-apply-image-agent-worker-deployment.sh",
		"scripts/listingkit-run-release-gate-deployment.sh",
		"deployments/kubernetes/listingkit-workbench/release-authority/listingkit-release-gate-runners.yaml",
		"deployments/kubernetes/listingkit-workbench/base/image-agent-temporal-worker-deployment.yaml",
		"deployments/kubernetes/listingkit-workbench/base/image-agent-temporal-worker-v3-deployment.yaml",
		"--deployment image-agent-temporal-v3-canary-runner",
		"--image \"$API_CANDIDATE_IMAGE\"",
		"rollout restart deployment/image-agent-temporal-worker",
		"rollout status deployment/image-agent-temporal-worker --timeout=5m",
		"rollout restart deployment/image-agent-temporal-worker-v3",
		"rollout status deployment/image-agent-temporal-worker-v3 --timeout=5m",
		"--timeout-seconds 300",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("ListingKit deploy workflow missing image-agent worker ownership %q", required)
		}
	}
	v2Wait := strings.Index(workflow, "rollout status deployment/image-agent-temporal-worker --timeout=5m")
	v3Wait := strings.Index(workflow, "rollout status deployment/image-agent-temporal-worker-v3 --timeout=5m")
	canaryWait := strings.Index(workflow, "--deployment image-agent-temporal-v3-canary-runner")
	apiStamp := strings.Index(workflow, "patch deployment product-listing-api")
	apiWait := strings.Index(workflow, "rollout status deployment/product-listing-api --timeout=5m")
	if v2Wait < 0 || v3Wait < 0 || canaryWait < 0 || apiStamp < 0 || apiWait < 0 || !(v2Wait < canaryWait && canaryWait < v3Wait && v3Wait < apiStamp && apiStamp < apiWait) {
		t.Fatalf("v2, v3, and canary gates must complete before the API release identity stamp: v2Wait=%d v3Wait=%d canaryWait=%d apiStamp=%d apiWait=%d", v2Wait, v3Wait, canaryWait, apiStamp, apiWait)
	}
	dockerfileBytes, err := os.ReadFile(filepath.Join("..", "deployments", "docker", "Dockerfile.product-listing-api"))
	if err != nil {
		t.Fatalf("read API Dockerfile: %v", err)
	}
	for _, required := range []string{"./cmd/image-agent-temporal-worker", "/app/image-agent-temporal-worker"} {
		if !strings.Contains(string(dockerfileBytes), required) {
			t.Errorf("API image must own image-agent worker binary %q", required)
		}
	}
}

func TestListingKitDeployPublishesProductionSMSWebhookIngressAfterAPIRollout(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}

	workflow := string(content)
	var parsedWorkflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string            `yaml:"name"`
				With map[string]string `yaml:"with"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(content, &parsedWorkflow); err != nil {
		t.Fatalf("parse ListingKit deploy workflow: %v", err)
	}
	deployJob := listingKitDeployAPIJob(t, workflow)
	const productionIngress = "deployments/kubernetes/listingkit-workbench/overlays/prod/patch-ingress.yaml"
	checkoutSparsePaths := ""
	for _, step := range parsedWorkflow.Jobs["deploy-api"].Steps {
		if step.Name == "Checkout workflow tooling" {
			checkoutSparsePaths = step.With["sparse-checkout"]
			break
		}
	}
	if !strings.Contains(checkoutSparsePaths, productionIngress) {
		t.Fatalf("ListingKit deploy workflow must check out the production SMS webhook ingress %q", productionIngress)
	}

	apiApplyIndex := strings.Index(deployJob, "scripts/listingkit-apply-api-deployment.sh")
	stampIndex := strings.Index(deployJob, "kubectl -n \"${{ env.K8S_NAMESPACE }}\" patch deployment product-listing-api")
	rolloutIndex := strings.Index(deployJob, "kubectl -n ${{ env.K8S_NAMESPACE }} rollout status deployment/product-listing-api --timeout=5m")
	ingressApplyIndex := strings.Index(deployJob, "kubectl -n ${{ env.K8S_NAMESPACE }} apply -f .workflow-tools/"+productionIngress)
	if apiApplyIndex < 0 || stampIndex < 0 || rolloutIndex < 0 || ingressApplyIndex < 0 {
		t.Fatalf("ListingKit deploy workflow must apply the API, stamp its release identity, wait for its rollout, and then apply the production SMS webhook ingress, api=%d stamp=%d rollout=%d ingress=%d", apiApplyIndex, stampIndex, rolloutIndex, ingressApplyIndex)
	}
	if !(apiApplyIndex < stampIndex && stampIndex < rolloutIndex && rolloutIndex < ingressApplyIndex) {
		t.Fatal("ListingKit deploy workflow must stamp API release identity after the immutable apply, wait for the rollout, then publish the SMS webhook ingress")
	}

	ingressPath := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "overlays", "prod", "patch-ingress.yaml")
	ingressContent, err := os.ReadFile(ingressPath)
	if err != nil {
		t.Fatalf("read production ListingKit ingress: %v", err)
	}
	var ingress struct {
		Metadata struct {
			Annotations map[string]string `yaml:"annotations"`
		} `yaml:"metadata"`
		Spec struct {
			IngressClassName string `yaml:"ingressClassName"`
			Rules            []struct {
				Host string `yaml:"host"`
			} `yaml:"rules"`
			TLS []struct {
				Hosts      []string `yaml:"hosts"`
				SecretName string   `yaml:"secretName"`
			} `yaml:"tls"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(ingressContent, &ingress); err != nil {
		t.Fatalf("parse production ListingKit ingress: %v", err)
	}
	if ingress.Spec.IngressClassName != "traefik" {
		t.Fatalf("production ListingKit ingress must be independently applicable with ingressClassName traefik, got %q", ingress.Spec.IngressClassName)
	}
	if ingress.Metadata.Annotations["cert-manager.io/cluster-issuer"] != "letsencrypt-prod" {
		t.Fatal("production ListingKit ingress must retain its TLS issuer when applied independently")
	}
	if ingress.Metadata.Annotations["ingress.kubernetes.io/ssl-redirect"] != "true" {
		t.Fatal("production ListingKit ingress must redirect HTTP to HTTPS when applied independently")
	}
	if ingress.Metadata.Annotations["traefik.ingress.kubernetes.io/router.entrypoints"] != "web,websecure" {
		t.Fatal("production ListingKit ingress must retain its Traefik entrypoints when applied independently")
	}
	if len(ingress.Spec.Rules) != 1 || ingress.Spec.Rules[0].Host != "pod.shuomiai.com" {
		t.Fatalf("production ListingKit ingress must retain its public host, got %#v", ingress.Spec.Rules)
	}
	if len(ingress.Spec.TLS) != 1 || len(ingress.Spec.TLS[0].Hosts) != 1 || ingress.Spec.TLS[0].Hosts[0] != "pod.shuomiai.com" || ingress.Spec.TLS[0].SecretName != "pod-shuomiai-com-tls" {
		t.Fatalf("production ListingKit ingress must retain its TLS host and certificate Secret, got %#v", ingress.Spec.TLS)
	}
}

func TestListingKitSchemaMigrationRunsBeforeIdentityPreflight(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
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
	deployJob := workflow.Jobs["deploy-api"]
	schemaIndex, productSchemaIndex, preflightIndex := -1, -1, -1
	for index, step := range deployJob.Steps {
		if strings.Contains(step.Run, "scripts/listingkit-run-release-gate-deployment.sh") {
			if strings.Contains(step.Run, "--deployment product-listing-api-schema-migrate-runner") {
				productSchemaIndex = index
				if !strings.Contains(step.Run, "--image \"$API_CANDIDATE_IMAGE\"") {
					t.Errorf("product-listing schema migration must use the immutable API candidate")
				}
			} else if strings.Contains(step.Run, "--deployment listingkit-schema-migrate-runner") {
				schemaIndex = index
				if !strings.Contains(step.Run, "--image \"$API_CANDIDATE_IMAGE\"") {
					t.Errorf("ListingKit schema migration must use the immutable API candidate")
				}
			}
		}
		if strings.Contains(step.Run, "--deployment listingkit-identity-preflight-runner") {
			preflightIndex = index
		}
	}
	if schemaIndex < 0 || productSchemaIndex < 0 || preflightIndex < 0 || schemaIndex >= preflightIndex || productSchemaIndex >= preflightIndex {
		t.Fatalf("schema migrations must run before identity preflight, product=%d listingkit=%d preflight=%d", productSchemaIndex, schemaIndex, preflightIndex)
	}
}

func TestListingKitDeployRemovesDeprecatedIdentityKeysBeforePreflight(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				Run  string `yaml:"run"`
				If   string `yaml:"if"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatalf("parse ListingKit deploy workflow: %v", err)
	}
	steps := workflow.Jobs["deploy-api"].Steps
	cleanupIndex, preflightIndex := -1, -1
	for index, step := range steps {
		if strings.Contains(step.Run, "scripts/listingkit-clean-legacy-identity-secret.sh") {
			cleanupIndex = index
			if got, want := step.If, "${{ needs.prepare.outputs.mode == 'normal' && steps.candidate-identity-compatibility.outputs.cleanup_legacy_identity == 'true' }}"; got != want {
				t.Errorf("legacy identity Secret cleanup must require candidate compatibility, got if: %q", got)
			}
			if !strings.Contains(step.Run, "listingkit-workbench-secret") {
				t.Fatal("legacy identity Secret cleanup must target the shared ListingKit Secret")
			}
		}
		if strings.Contains(step.Run, "--deployment listingkit-identity-preflight-runner") {
			preflightIndex = index
		}
	}
	if cleanupIndex < 0 || preflightIndex < 0 || cleanupIndex <= preflightIndex {
		t.Fatalf("legacy identity Secret cleanup must run after identity preflight, cleanup=%d preflight=%d", cleanupIndex, preflightIndex)
	}

	scriptPath := filepath.Join("..", "scripts", "listingkit-clean-legacy-identity-secret.sh")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read legacy identity Secret cleanup script: %v", err)
	}
	for _, key := range []string{
		"LISTINGKIT_ZITADEL_ALLOWED_USERNAMES",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USERNAMES",
		"LISTINGKIT_ZITADEL_ALLOWED_ROLES",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES",
	} {
		if !strings.Contains(string(script), key) {
			t.Errorf("cleanup script must remove deprecated key %q", key)
		}
	}
	if !strings.Contains(string(content), "scripts/listingkit-clean-legacy-identity-secret.sh") {
		t.Error("deploy-api sparse checkout must include the legacy identity Secret cleanup driver")
	}
}

func TestListingKitDeployInspectsCandidateCompatibilityBeforeSecretCleanup(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string `yaml:"name"`
				ID   string `yaml:"id"`
				Run  string `yaml:"run"`
				If   string `yaml:"if"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatalf("parse ListingKit deploy workflow: %v", err)
	}
	steps := workflow.Jobs["deploy-api"].Steps
	preflightIndex, compatibilityIndex, cleanupIndex, applyIndex := -1, -1, -1, -1
	for index, step := range steps {
		if strings.Contains(step.Run, "--deployment listingkit-identity-preflight-runner") {
			preflightIndex = index
		}
		if step.ID == "candidate-identity-compatibility" {
			compatibilityIndex = index
			for _, required := range []string{
				"docker pull \"$API_CANDIDATE_IMAGE\"",
				"docker image inspect",
				"org.opencontainers.image.listingkit.identity",
			} {
				if !strings.Contains(step.Run, required) {
					t.Errorf("candidate compatibility step must contain %q", required)
				}
			}
		}
		if strings.Contains(step.Run, "scripts/listingkit-clean-legacy-identity-secret.sh") {
			cleanupIndex = index
		}
		if strings.Contains(step.Run, "scripts/listingkit-apply-api-deployment.sh") {
			applyIndex = index
		}
	}
	if preflightIndex < 0 || compatibilityIndex < 0 || cleanupIndex < 0 || applyIndex < 0 || !(compatibilityIndex < preflightIndex && preflightIndex < cleanupIndex && cleanupIndex < applyIndex) {
		t.Fatalf("candidate compatibility, cleanup, and apply ordering invalid: preflight=%d compatibility=%d cleanup=%d apply=%d", preflightIndex, compatibilityIndex, cleanupIndex, applyIndex)
	}
}

func TestListingKitDeployUsesCanonicalCandidateCompatibilityForIdentityCleanup(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}
	workflow := string(content)
	for _, required := range []string{
		"org.opencontainers.image.listingkit.identity",
		"canonical-subject-v1",
		"cleanup_legacy_identity=true",
		"needs.prepare.outputs.mode == 'normal' && steps.candidate-identity-compatibility.outputs.cleanup_legacy_identity == 'true'",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("ListingKit deploy workflow must contain canonical candidate compatibility check %q", required)
		}
	}
	for _, forbidden := range []string{"source_identity_compatibility", "inputs.source_ref", "internal/core/config/validator_listingkit.go"} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("canonical workflow-bound builds must not restore caller/source compatibility authority %q", forbidden)
		}
	}
}

func TestListingKitAPIImageDeclaresCandidateIdentityCompatibilityLabel(t *testing.T) {
	dockerfilePath := filepath.Join("..", "deployments", "docker", "Dockerfile.product-listing-api")
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("read ListingKit API Dockerfile: %v", err)
	}
	if !strings.Contains(string(content), `org.opencontainers.image.listingkit.identity="canonical-subject-v1"`) {
		t.Fatal("ListingKit API image must declare the canonical-subject compatibility label")
	}
}

func TestListingKitManualDeployPreservesNonProductionGateOrderingWithoutProductionIngress(t *testing.T) {
	scriptPath := filepath.Join("..", "scripts", "build-push-deploy-listingkit-workbench.ps1")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read ListingKit manual deploy script: %v", err)
	}
	text := string(content)
	preflightCall := strings.Index(text, "& $BashExecutable $IdentityPreflightDriver")
	cleanupCall := strings.Index(text, "& $BashExecutable $LegacyIdentitySecretCleanupDriver")
	apiApplyCall := strings.Index(text, "& $BashExecutable $ImmutableApiApplyDriver")
	apiRestart := strings.Index(text, "kubectl -n $Namespace rollout restart deployment/product-listing-api")
	apiRollout := strings.Index(text, "kubectl -n $Namespace rollout status deployment/product-listing-api --timeout=5m")
	uiRollout := strings.Index(text, "kubectl -n $Namespace rollout status deployment/listingkit-ui --timeout=5m")
	if preflightCall < 0 || cleanupCall < 0 || apiApplyCall < 0 || apiRestart < 0 || apiRollout < 0 || uiRollout < 0 {
		t.Fatalf("non-production deploy must invoke preflight, cleanup, API apply, API restart, API rollout, and UI rollout: preflight=%d cleanup=%d api=%d apiRestart=%d apiRollout=%d uiRollout=%d", preflightCall, cleanupCall, apiApplyCall, apiRestart, apiRollout, uiRollout)
	}
	if !(preflightCall < cleanupCall && cleanupCall < apiApplyCall) {
		t.Fatalf("manual deploy must clean the Secret after preflight and before API apply: preflight=%d cleanup=%d api=%d", preflightCall, cleanupCall, apiApplyCall)
	}
	if !(apiApplyCall < apiRestart && apiRestart < apiRollout && apiRollout < uiRollout) {
		t.Fatalf("non-production deploy must restart API Pods after the immutable apply and wait before the UI: api=%d apiRestart=%d apiRollout=%d uiRollout=%d", apiApplyCall, apiRestart, apiRollout, uiRollout)
	}
	if !strings.Contains(text, "listingkit-workbench-secret") {
		t.Error("manual deploy cleanup must target the shared ListingKit Secret")
	}
	if strings.Contains(text, "$ProductionIngressManifest") || strings.Contains(text, "overlays/prod/patch-ingress.yaml") {
		t.Fatal("workstation deploy must not own or advertise the production ingress mutation")
	}
}

func TestListingKitManualDeployFailsClosedForProductionBeforeExternalCommands(t *testing.T) {
	scriptPath := filepath.Join("..", "scripts", "build-push-deploy-listingkit-workbench.ps1")
	content, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("read ListingKit manual deploy script: %v", err)
	}

	text := string(content)
	guardIndex := strings.Index(text, "workstation deployment to production namespace task-processor is forbidden")
	firstExternalIndex := strings.Index(text, "git rev-parse")
	if guardIndex < 0 || firstExternalIndex < 0 {
		t.Fatalf("manual deploy must contain the production fail-closed guard before tag resolution: guard=%d firstExternal=%d", guardIndex, firstExternalIndex)
	}
	if guardIndex >= firstExternalIndex {
		t.Fatalf("production guard must run before any external command: guard=%d firstExternal=%d", guardIndex, firstExternalIndex)
	}
}

func TestListingKitIdentityPreflightJobDeadlineMatchesDriverWait(t *testing.T) {
	manifestPath := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "jobs", "listingkit-identity-preflight-job.yaml")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read ListingKit identity preflight Job manifest: %v", err)
	}

	var job struct {
		Spec struct {
			ActiveDeadlineSeconds int `yaml:"activeDeadlineSeconds"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(content, &job); err != nil {
		t.Fatalf("parse ListingKit identity preflight Job manifest: %v", err)
	}

	if job.Spec.ActiveDeadlineSeconds != 15*60 {
		t.Fatalf("identity preflight Job deadline must match the driver's 15-minute wait, got %d seconds", job.Spec.ActiveDeadlineSeconds)
	}
}

func TestListingKitSchemaMigrationJobDeadlineMatchesDriverWait(t *testing.T) {
	manifestPath := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "jobs", "listingkit-schema-migrate-job.yaml")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read ListingKit schema migration Job manifest: %v", err)
	}

	var job struct {
		Spec struct {
			ActiveDeadlineSeconds int `yaml:"activeDeadlineSeconds"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(content, &job); err != nil {
		t.Fatalf("parse ListingKit schema migration Job: %v", err)
	}

	if job.Spec.ActiveDeadlineSeconds != 15*60 {
		t.Fatalf("schema migration Job deadline must match the driver's 15-minute wait, got %d seconds", job.Spec.ActiveDeadlineSeconds)
	}
}

func TestProductListingAPISchemaMigrationJobDeadlineMatchesDriverWait(t *testing.T) {
	manifestPath := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "jobs", "product-listing-api-schema-migrate-job.yaml")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read product-listing API schema migration Job manifest: %v", err)
	}

	var job struct {
		Spec struct {
			ActiveDeadlineSeconds int `yaml:"activeDeadlineSeconds"`
		} `yaml:"spec"`
	}
	if err := yaml.Unmarshal(content, &job); err != nil {
		t.Fatalf("parse product-listing API schema migration Job: %v", err)
	}

	if job.Spec.ActiveDeadlineSeconds != 15*60 {
		t.Fatalf("product-listing API schema migration Job deadline must match the driver's 15-minute wait, got %d seconds", job.Spec.ActiveDeadlineSeconds)
	}
}

func TestListingKitFirstControlledDeploymentRoutesProductionMigrationsThroughWorkflow(t *testing.T) {
	readmePath := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "README.md")
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("read ListingKit deployment README: %v", err)
	}
	text := string(content)
	start := strings.Index(text, "### First controlled deployment")
	end := strings.Index(text, "### Identity preflight release gate")
	if start < 0 || end <= start {
		t.Fatal("could not isolate the first controlled deployment procedure")
	}
	procedure := text[start:end]
	for _, required := range []string{
		"ListingKit API Deploy",
		"ListingKit UI Deploy",
		"release_gate_run_id",
		"release_gate_run_attempt",
		"both schema migrations",
	} {
		if !strings.Contains(procedure, required) {
			t.Errorf("first controlled deployment must route production release through the exact attempt-bound workflows; missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"kubectl apply -f deployments/kubernetes/listingkit-workbench/base/configmap.yaml",
		"bash scripts/listingkit-schema-migrate-job.sh",
		"kubectl create -n task-processor",
	} {
		if strings.Contains(procedure, forbidden) {
			t.Errorf("first controlled deployment must not advertise workstation production mutation %q", forbidden)
		}
	}
}

func TestListingKitPreflightDocumentationUsesDigestPinnedCandidateAndRunner(t *testing.T) {
	readmePath := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "README.md")
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"listingkit-identity-preflight-runner", "exact digest-pinned preflight image", "no top-level `create`"} {
		if !strings.Contains(string(readme), required) {
			t.Errorf("ListingKit production preflight documentation must contain %q", required)
		}
	}
	if strings.Contains(string(readme), "listingkit-identity-preflight-job.sh") {
		t.Error("production documentation must not advertise the legacy direct Job driver")
	}

	for _, path := range []string{filepath.Join("..", "docs", "development", "listingkit-local-debug.md")} {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read ListingKit preflight documentation %q: %v", path, err)
		}
		text := string(content)
		for _, required := range []string{
			"API_CANDIDATE_IMAGE=\"docker.io/xuwei190/task-processor-product-listing-api@sha256:<64-hex-api-digest>\"",
			"PREFLIGHT_RUNNER_IMAGE=\"docker.io/xuwei190/task-processor-listingkit-identity-preflight@sha256:<64-hex-runner-digest>\"",
			"--image \"$API_CANDIDATE_IMAGE\"",
			"--runner-image \"$PREFLIGHT_RUNNER_IMAGE\"",
		} {
			if !strings.Contains(text, required) {
				t.Errorf("ListingKit preflight documentation %q must contain %q", path, required)
			}
		}
		if strings.Contains(text, "--image \"docker.io/xuwei190/task-processor-product-listing-api:<immutable-release-tag>\"") {
			t.Errorf("ListingKit preflight documentation %q must not pass a mutable image tag to the driver", path)
		}
	}
}

func TestListingKitDeployWorkflowSupportsAttestedRollbackWithoutRebuild(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}
	workflow := string(content)
	for _, required := range []string{
		"rollback_run_id:",
		"rollback_run_attempt:",
		"verify-rollback-attestation:",
		"actions/download-artifact@634f93cb2916e3fdff6788551b99b062d0335ce0",
		"verify-listingkit-api-release-attestation.sh",
		"needs.prepare.outputs.mode == 'rollback'",
		"needs.verify-rollback-attestation.outputs.api_image",
		"needs.prepare.outputs.mode == 'normal'",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("ListingKit workflow must contain exact-attestation rollback behavior %q", required)
		}
	}
	for _, forbidden := range []string{"api_image_digest:", "inputs.api_image_digest", "candidate_api_digest"} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("ListingKit rollback must not restore direct digest authority %q", forbidden)
		}
	}
}

func TestListingKitProductListingAPIImageCarriesV3NewStartsContract(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("..", "deployments", "docker", "Dockerfile.product-listing-api"))
	if err != nil {
		t.Fatal(err)
	}
	const label = `org.opencontainers.image.listingkit.image-agent-routing="image-agent-v3-new-starts-v1"`
	if !strings.Contains(string(content), label) {
		t.Fatalf("product-listing API image must carry immutable v3 new-start routing label %q", label)
	}
}

func TestListingKitDeployRequiresV3NewStartsLabelBeforeAnyProductionMutation(t *testing.T) {
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
				If   string `yaml:"if"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatal(err)
	}
	steps := workflow.Jobs["deploy-api"].Steps
	inspectionIndex := -1
	firstMutationIndex := len(steps)
	for index, step := range steps {
		if step.Name == "Inspect candidate identity compatibility" {
			inspectionIndex = index
			if step.If != "" {
				t.Fatalf("candidate routing inspection must apply to built and supplied digests, got if=%q", step.If)
			}
			for _, required := range []string{
				`org.opencontainers.image.listingkit.image-agent-routing`,
				`image-agent-v3-new-starts-v1`,
				`docker image inspect`,
			} {
				if !strings.Contains(step.Run, required) {
					t.Errorf("candidate routing inspection is missing %q", required)
				}
			}
		}
		if containsAny(step.Run,
			"listingkit-run-release-gate-deployment.sh",
			"listingkit-apply-image-agent-worker-deployment.sh",
			"listingkit-apply-api-deployment.sh",
			"kubectl -n ${{ env.K8S_NAMESPACE }} apply",
			"patch deployment product-listing-api") && index < firstMutationIndex {
			firstMutationIndex = index
		}
	}
	if inspectionIndex < 0 || inspectionIndex >= firstMutationIndex {
		t.Fatalf("candidate v3 routing label must be required before every production mutation, inspect=%d mutation=%d", inspectionIndex, firstMutationIndex)
	}
}

func TestListingKitDeployValidatesAttestedRollbackV3NewStartsContract(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(content)
	for _, required := range []string{
		`verify-listingkit-api-release-attestation.sh`,
		`ROLLBACK_API_IMAGE`,
		`needs.verify-rollback-attestation.outputs.api_image`,
		`docker pull "$API_CANDIDATE_IMAGE"`,
		`org.opencontainers.image.listingkit.image-agent-routing`,
		`image-agent-v3-new-starts-v1`,
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("built and attested rollback candidates must share routing-contract inspection %q", required)
		}
	}
	if strings.Contains(workflow, "inputs.api_image_digest") {
		t.Error("rollback routing validation must not accept a caller-provided digest")
	}
}

func TestListingKitDeployStampsV3RoutingContractOnDeploymentAndPodTemplate(t *testing.T) {
	t.Parallel()

	content, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml"))
	if err != nil {
		t.Fatal(err)
	}
	deployJob := listingKitDeployAPIJob(t, string(content))
	stampStart := strings.Index(deployJob, "- name: Stamp API release identity and restart Pods")
	if stampStart < 0 {
		t.Fatal("API release identity stamp step is missing")
	}
	stamp := deployJob[stampStart:]
	if next := strings.Index(stamp[1:], "\n      - name:"); next >= 0 {
		stamp = stamp[:next+1]
	}
	for _, required := range []string{
		`listingkit.sh/image-agent-routing-contract`,
		`--arg routing_contract "image-agent-v3-new-starts-v1"`,
	} {
		if !strings.Contains(stamp, required) {
			t.Errorf("API release stamp is missing %q", required)
		}
	}
	if got := strings.Count(stamp, "($routing_contract_key):$routing_contract"); got != 2 {
		t.Fatalf("routing contract must be stamped on Deployment and Pod template exactly once each, got %d", got)
	}
}

func TestListingKitReleasePathHasNoCallerControlledRoutingContract(t *testing.T) {
	t.Parallel()

	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	workflow := loadReleaseWorkflow(t, workflowPath)
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(content))
	dispatchInputs := strings.ToLower(fmt.Sprint(workflow.On["workflow_dispatch"]["inputs"]))
	for _, forbiddenInput := range []string{"routing_contract", "route_contract", "image_agent_routing", "release_routing_contract"} {
		if strings.Contains(dispatchInputs, forbiddenInput) {
			t.Errorf("production workflow exposes caller-controlled routing input %q", forbiddenInput)
		}
	}
	for _, forbiddenExpression := range []string{
		"inputs.image_agent_routing",
		"inputs.routing_contract",
		"release_routing_contract",
	} {
		if strings.Contains(lower, forbiddenExpression) {
			t.Errorf("production workflow exposes caller-controlled routing expression %q", forbiddenExpression)
		}
	}
	drain, err := os.ReadFile(filepath.Join("..", "scripts", "listingkit-image-agent-v2-drain-check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"--routing-contract", "--expected-routing-contract", "ROUTING_CONTRACT:-"} {
		if strings.Contains(string(drain), forbidden) {
			t.Errorf("drain executable exposes routing-contract override %q", forbidden)
		}
	}
}

func TestListingKitDeployWorkflowPassesRollbackInputsThroughStepEnvironment(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}

	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				ID  string            `yaml:"id"`
				Run string            `yaml:"run"`
				Env map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatalf("parse ListingKit deploy workflow: %v", err)
	}

	prepareJob, ok := workflow.Jobs["prepare"]
	if !ok {
		t.Fatal("ListingKit deploy workflow is missing prepare job")
	}
	for _, step := range prepareJob.Steps {
		if step.ID != "meta" {
			continue
		}
		for name, want := range map[string]string{
			"ROLLBACK_RUN_ID":      "${{ inputs.rollback_run_id }}",
			"ROLLBACK_RUN_ATTEMPT": "${{ inputs.rollback_run_attempt }}",
		} {
			if got := step.Env[name]; got != want {
				t.Errorf("prepare metadata step environment %s = %q, want %q", name, got, want)
			}
		}
		if strings.Contains(step.Run, "${{ inputs.") {
			t.Error("prepare metadata shell must not interpolate workflow_dispatch inputs directly")
		}
		return
	}
	t.Fatal("ListingKit deploy workflow is missing prepare metadata step")
}

func TestListingKitDeployWorkflowPassesOnlyDigestsAcrossBuildJobBoundaries(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}

	var workflow struct {
		Jobs map[string]struct {
			Outputs map[string]string `yaml:"outputs"`
			Steps   []struct {
				Name string            `yaml:"name"`
				Run  string            `yaml:"run"`
				Env  map[string]string `yaml:"env"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(content, &workflow); err != nil {
		t.Fatalf("parse ListingKit deploy workflow: %v", err)
	}

	for jobName, outputName := range map[string]string{
		"build-api":              "api_digest",
		"build-preflight-runner": "runner_digest",
	} {
		job, ok := workflow.Jobs[jobName]
		if !ok {
			t.Fatalf("ListingKit deploy workflow is missing %s job", jobName)
		}
		if _, ok := job.Outputs[outputName]; !ok {
			t.Errorf("%s must expose only its immutable digest as %s", jobName, outputName)
		}
		for name, value := range job.Outputs {
			if strings.Contains(value, "docker.io/") {
				t.Errorf("%s output %s must not contain a full registry image reference", jobName, name)
			}
		}
	}
	if _, ok := workflow.Jobs["prepare"].Outputs["api_tags"]; ok {
		t.Error("prepare must not pass full API image tags across a job boundary")
	}

	deployJob := workflow.Jobs["deploy-api"]
	for _, step := range deployJob.Steps {
		if !strings.Contains(step.Run, "--deployment listingkit-identity-preflight-runner") {
			continue
		}
		if got, want := step.Env["PREFLIGHT_RUNNER_DIGEST"], "${{ needs.build-preflight-runner.outputs.runner_digest }}"; got != want {
			t.Errorf("preflight runner digest environment = %q, want %q", got, want)
		}
		for _, required := range []string{
			"listingkit_compose_immutable_image",
			"--image \"$PREFLIGHT_RUNNER_IMAGE\"",
			"--manifest .workflow-tools/deployments/kubernetes/listingkit-workbench/release-authority/listingkit-release-gate-runners.yaml",
		} {
			if !strings.Contains(step.Run, required) {
				t.Errorf("preflight step must contain %q", required)
			}
		}
		return
	}
	t.Fatal("ListingKit deploy workflow is missing the identity preflight driver")
}

func TestListingKitDeployWorkflowDerivesDefaultTagFromCanonicalWorkflowCommit(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}
	workflow := string(content)
	for _, required := range []string{
		"WORKFLOW_SOURCE_REF: ${{ github.workflow_sha }}",
		"GITHUB_REF\" == \"refs/heads/main",
		"GITHUB_WORKFLOW_REF\" == \"$CANONICAL_WORKFLOW_REF",
		"source_ref=\"$WORKFLOW_SOURCE_REF\"",
		"runner_source_ref=\"$source_ref\"",
		"tag=\"${source_ref:0:12}\"",
		"^[0-9a-f]{40}$",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("ListingKit canonical source binding must contain %q", required)
		}
	}
	for _, forbidden := range []string{"inputs.source_ref", "repos/${GITHUB_REPOSITORY}/commits/$source_ref"} {
		if strings.Contains(workflow, forbidden) {
			t.Errorf("ListingKit deploy workflow must not resolve caller-selected source %q", forbidden)
		}
	}
}

func TestListingKitPreflightRunnerUsesCanonicalSourceOnlyForNormalRelease(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}
	workflow := string(content)
	for _, required := range []string{
		"runner_source_ref: ${{ steps.meta.outputs.runner_source_ref }}",
		"runner_source_ref=\"$source_ref\"",
		"ref: ${{ needs.prepare.outputs.runner_source_ref }}",
		"build-preflight-runner:",
		"if: ${{ needs.prepare.outputs.mode == 'normal' }}",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("ListingKit runner source selection is missing %q", required)
		}
	}
}
