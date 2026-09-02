package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestListingKitCommercialImageAgentWorkersUseExactSecretAndConfigScope(t *testing.T) {
	base := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench")
	sharedSecrets := map[string]string{
		"TASK_PROCESSOR_DATABASE_HOST":                  "TASK_PROCESSOR_DATABASE_HOST",
		"TASK_PROCESSOR_DATABASE_PORT":                  "TASK_PROCESSOR_DATABASE_PORT",
		"TASK_PROCESSOR_DATABASE_USER":                  "TASK_PROCESSOR_DATABASE_USER",
		"TASK_PROCESSOR_DATABASE_PASSWORD":              "TASK_PROCESSOR_DATABASE_PASSWORD",
		"TASK_PROCESSOR_DATABASE_NAME":                  "TASK_PROCESSOR_DATABASE_NAME",
		"TASK_PROCESSOR_OPENAI_API_KEY":                 "TASK_PROCESSOR_OPENAI_API_KEY",
		"TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_KEY":   "TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_KEY",
		"TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_STYLE": "TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_API_STYLE",
		"TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_BASE_URL":  "TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_BASE_URL",
		"TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_MODEL":     "TASK_PROCESSOR_OPENAI_CLIENTS_IMAGE_MODEL",
	}
	sharedConfig := map[string]string{
		"IMAGE_AGENT_TEMPORAL_ENABLED":   "IMAGE_AGENT_TEMPORAL_ENABLED",
		"IMAGE_AGENT_TEMPORAL_ADDRESS":   "IMAGE_AGENT_TEMPORAL_ADDRESS",
		"IMAGE_AGENT_TEMPORAL_NAMESPACE": "IMAGE_AGENT_TEMPORAL_NAMESPACE",
	}

	for _, test := range []struct {
		relativePath string
		wantSecrets  map[string]string
		wantConfig   map[string]string
	}{
		{relativePath: filepath.Join("base", "image-agent-temporal-worker-deployment.yaml"), wantSecrets: sharedSecrets, wantConfig: sharedConfig},
		{relativePath: filepath.Join("base", "image-agent-temporal-worker-v3-deployment.yaml"), wantSecrets: func() map[string]string {
			result := map[string]string{}
			for key, value := range sharedSecrets {
				result[key] = value
			}
			result["TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_ACCESS_KEY_ID"] = "TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_ACCESS_KEY_ID"
			result["TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_SECRET_ACCESS_KEY"] = "TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_SECRET_ACCESS_KEY"
			return result
		}(), wantConfig: func() map[string]string {
			result := map[string]string{}
			for key, value := range sharedConfig {
				result[key] = value
			}
			for _, key := range []string{
				"TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_ENABLED",
				"TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_PROVIDER",
				"TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_PUBLIC_BASE",
				"TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_BUCKET",
				"TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_REGION",
				"TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_ENDPOINT",
				"TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_USE_PATH_STYLE",
				"TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_ARTIFACT_MODE",
				"TASK_PROCESSOR_IMAGEAGENT_ARTIFACT_STORE_S3_COS_IMMUTABLE_NON_VERSIONED_BUCKET_POLICY",
			} {
				result[key] = key
			}
			return result
		}()},
	} {
		t.Run(test.relativePath, func(t *testing.T) {
			manifest := loadImageAgentWorkloadManifest(t, filepath.Join(base, test.relativePath))
			container := onlyImageAgentContainer(t, manifest)
			if len(container.EnvFrom) != 0 {
				t.Fatalf("%s must use per-key configuration and Secret references, got envFrom=%#v", test.relativePath, container.EnvFrom)
			}
			actualSecrets := map[string]string{}
			actualConfig := map[string]string{}
			for _, variable := range container.Env {
				for _, forbidden := range []string{"ZITADEL", "INVITATION", "TENCENT_SMS", "SMS_WEBHOOK"} {
					if strings.Contains(variable.Name, forbidden) {
						t.Fatalf("%s must not receive forbidden credential/config %q", test.relativePath, variable.Name)
					}
				}
				if variable.ValueFrom == nil {
					t.Fatalf("%s must not contain literal environment value %q", test.relativePath, variable.Name)
				}
				if (variable.ValueFrom.SecretKeyRef == nil) == (variable.ValueFrom.ConfigMapKeyRef == nil) {
					t.Fatalf("%s environment %q must reference exactly one approved source", test.relativePath, variable.Name)
				}
				if ref := variable.ValueFrom.SecretKeyRef; ref != nil {
					if ref.Name != listingKitSharedSecret {
						t.Fatalf("%s references unexpected Secret %q", test.relativePath, ref.Name)
					}
					actualSecrets[variable.Name] = ref.Key
				}
				if ref := variable.ValueFrom.ConfigMapKeyRef; ref != nil {
					if ref.Name != "listingkit-workbench-config" {
						t.Fatalf("%s references unexpected ConfigMap %q", test.relativePath, ref.Name)
					}
					actualConfig[variable.Name] = ref.Key
				}
			}
			if !reflect.DeepEqual(actualSecrets, test.wantSecrets) {
				t.Fatalf("%s Secret allowlist=%#v want=%#v", test.relativePath, actualSecrets, test.wantSecrets)
			}
			if !reflect.DeepEqual(actualConfig, test.wantConfig) {
				t.Fatalf("%s ConfigMap allowlist=%#v want=%#v", test.relativePath, actualConfig, test.wantConfig)
			}
		})
	}

	canaryManifest := loadImageAgentWorkloadFromMultiDoc(t, filepath.Join(base, "release-authority", "listingkit-release-gate-runners.yaml"), "image-agent-temporal-v3-canary-runner")
	if len(canaryManifest.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatal("canary runner must have one release-gate init container")
	}
	canary := canaryManifest.Spec.Template.Spec.InitContainers[0]
	if len(canary.EnvFrom) != 0 {
		t.Fatalf("canary must not use envFrom, got %#v", canary.EnvFrom)
	}
	actualCanaryConfig := map[string]string{}
	for _, variable := range canary.Env {
		if variable.ValueFrom == nil || variable.ValueFrom.ConfigMapKeyRef == nil || variable.ValueFrom.SecretKeyRef != nil {
			t.Fatalf("canary environment %q must be a Temporal ConfigMap key only", variable.Name)
		}
		if variable.ValueFrom != nil && variable.ValueFrom.SecretKeyRef != nil {
			t.Fatalf("canary must not receive Secret key %q", variable.ValueFrom.SecretKeyRef.Key)
		}
		if variable.ValueFrom != nil && variable.ValueFrom.ConfigMapKeyRef != nil {
			actualCanaryConfig[variable.Name] = variable.ValueFrom.ConfigMapKeyRef.Key
		}
	}
	wantCanaryConfig := map[string]string{
		"IMAGE_AGENT_TEMPORAL_ADDRESS":   "IMAGE_AGENT_TEMPORAL_ADDRESS",
		"IMAGE_AGENT_TEMPORAL_NAMESPACE": "IMAGE_AGENT_TEMPORAL_NAMESPACE",
	}
	if !reflect.DeepEqual(actualCanaryConfig, wantCanaryConfig) {
		t.Fatalf("canary ConfigMap allowlist=%#v want=%#v", actualCanaryConfig, wantCanaryConfig)
	}
}

func TestCommercialReadinessWorkflowCollectsPinnedReleaseEvidence(t *testing.T) {
	path := filepath.Join("..", ".github", "workflows", "commercial-readiness.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read commercial-readiness workflow: %v", err)
	}

	workflow := string(content)
	packageContent, err := os.ReadFile(filepath.Join("..", "web", "listingkit-ui", "package.json"))
	if err != nil {
		t.Fatalf("read frontend package manifest: %v", err)
	}
	var packageManifest struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal(packageContent, &packageManifest); err != nil {
		t.Fatalf("parse frontend package manifest: %v", err)
	}
	packageManager, _, found := strings.Cut(packageManifest.PackageManager, "@")
	if !found || packageManager == "" {
		t.Fatalf("frontend packageManager must pin a package manager and version, got %q", packageManifest.PackageManager)
	}

	var installCommand, lockfile string
	scriptCommand := func(script string) string { return packageManager + " " + script }
	switch packageManager {
	case "pnpm":
		installCommand = "pnpm install --frozen-lockfile"
		lockfile = "pnpm-lock.yaml"
	case "npm":
		installCommand = "npm ci"
		lockfile = "package-lock.json"
		scriptCommand = func(script string) string {
			if script == "test" {
				return "npm test"
			}
			return "npm run " + script
		}
	default:
		t.Fatalf("commercial-readiness workflow policy does not support package manager %q", packageManager)
	}

	for _, required := range []string{
		"workflow_dispatch:",
		"commit_sha:",
		"^[0-9a-fA-F]{40}$",
		"ref: ${{ inputs.commit_sha }}",
		"go test ./... -count=1",
		"go test -race ./internal/app/runtime/listingcontrol -run TestControlPlaneService -count=1",
		"go test -race ./internal/listingadmin -run \"TestConcurrentClaimForDispatchOnlyOneWorkerWins|TestConcurrentRollbackDispatchOnlyOriginalQueuedClaimIsRestoredOnce|TestConcurrentRecoveryOnlyUpdatesStillEligibleRowsOnce\" -count=1",
		"make build-all",
		"cache: " + packageManager,
		"cache-dependency-path: web/listingkit-ui/" + lockfile,
		installCommand,
		scriptCommand("lint"),
		scriptCommand("typecheck"),
		scriptCommand("test"),
		scriptCommand("build"),
		"deployments/docker/Dockerfile.product-listing-api",
		"deployments/docker/Dockerfile.listingkit-ui",
		"kustomize build deployments/kubernetes/listingkit-workbench/overlays/prod",
		"actions/upload-artifact@v4",
		"if: ${{ always() }}",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("commercial-readiness workflow must contain %q", required)
		}
	}

	if strings.Contains(workflow, "continue-on-error: true") {
		t.Fatal("commercial-readiness workflow must not hide failed validation steps")
	}
}

func TestListingKitAPIManifestUsesDependencyReadinessProbe(t *testing.T) {
	path := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "base", "product-listing-api-deployment.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ListingKit API deployment manifest: %v", err)
	}

	manifest := string(content)
	readinessStart := strings.Index(manifest, "          readinessProbe:")
	livenessStart := strings.Index(manifest, "          livenessProbe:")
	startupStart := strings.Index(manifest, "          startupProbe:")
	if readinessStart < 0 || livenessStart < 0 || startupStart < 0 {
		t.Fatal("expected readiness, liveness, and startup probes in ListingKit API deployment manifest")
	}
	if !strings.Contains(manifest[readinessStart:livenessStart], "path: /readyz") {
		t.Fatal("ListingKit API readiness probe must use /readyz")
	}
	if !strings.Contains(manifest[livenessStart:startupStart], "path: /health") {
		t.Fatal("ListingKit API liveness probe must use /health")
	}
}

func TestListingKitGenerationUsageLedgerCanaryIsRestrictedToBillingTenant1038(t *testing.T) {
	path := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "base", "configmap.yaml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ListingKit ConfigMap: %v", err)
	}

	var configMap struct {
		Data map[string]string `yaml:"data"`
	}
	if err := yaml.Unmarshal(content, &configMap); err != nil {
		t.Fatalf("parse ListingKit ConfigMap: %v", err)
	}
	if got := configMap.Data["TASK_PROCESSOR_LISTINGKIT_GENERATION_USAGE_LEDGER_ENABLED"]; got != "true" {
		t.Errorf("ListingKit generation usage ledger must be enabled, got %q", got)
	}
	if got := configMap.Data["TASK_PROCESSOR_LISTINGKIT_GENERATION_USAGE_LEDGER_TENANT_IDS"]; got != "1038" {
		t.Errorf("ListingKit generation usage ledger canary must contain only billing tenant 1038, got %q", got)
	}
}

func TestListingKitMemberInvitationTokenIsAPIScoped(t *testing.T) {
	base := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "base")
	sharedSecret, err := os.ReadFile(filepath.Join(base, "secret.example.yaml"))
	if err != nil {
		t.Fatalf("read shared ListingKit Secret: %v", err)
	}
	for _, key := range []string{
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID",
	} {
		if strings.Contains(string(sharedSecret), key) {
			t.Fatalf("shared ListingKit Secret must not contain %s", key)
		}
	}

	invitationSecret, err := os.ReadFile(filepath.Join(base, "member-invitation-secret.example.yaml"))
	if err != nil {
		t.Fatalf("read member invitation Secret: %v", err)
	}
	for _, required := range []string{
		"name: listingkit-member-invitation-secret",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID",
	} {
		if !strings.Contains(string(invitationSecret), required) {
			t.Errorf("member invitation Secret must contain %q", required)
		}
	}

	apiManifest, err := os.ReadFile(filepath.Join(base, "product-listing-api-deployment.yaml"))
	if err != nil {
		t.Fatalf("read ListingKit API deployment: %v", err)
	}
	for _, required := range []string{
		"name: TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN",
		"name: listingkit-member-invitation-secret",
		"key: TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN",
	} {
		if !strings.Contains(string(apiManifest), required) {
			t.Errorf("ListingKit API deployment must contain %q", required)
		}
	}
	if strings.Contains(string(apiManifest), "key: TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN\n                  optional: true") {
		t.Fatal("ListingKit API must require the dedicated member invitation Secret to prevent fallback to a legacy shared token")
	}

	for _, name := range []string{
		"listingkit-ui-deployment.yaml",
		"imgproxy-deployment.yaml",
		"shein-login-worker-deployment.yaml",
		filepath.Join("..", "jobs", "product-listing-api-schema-migrate-job.yaml"),
		filepath.Join("..", "jobs", "listingkit-schema-migrate-job.yaml"),
		filepath.Join("..", "jobs", "pod-image-index-backfill-job.yaml"),
	} {
		manifest, err := os.ReadFile(filepath.Join(base, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(manifest), "listingkit-member-invitation-secret") {
			t.Errorf("%s must not consume the member invitation Secret", name)
		}
	}

	configMap, err := os.ReadFile(filepath.Join(base, "configmap.yaml"))
	if err != nil {
		t.Fatalf("read ListingKit ConfigMap: %v", err)
	}
	if strings.Contains(string(configMap), "TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID") {
		t.Fatal("ListingKit base ConfigMap must not overwrite the manually provisioned invitation project id during deploy")
	}
	if !strings.Contains(string(apiManifest), "key: TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID") {
		t.Fatal("ListingKit API deployment must read the invitation project id from the API-only Secret")
	}

	deployWorkflow, err := os.ReadFile(filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml"))
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}
	deployAPIJob := listingKitDeployAPIJob(t, string(deployWorkflow))
	for _, required := range []string{
		"Reject legacy invitation credentials in shared Secret",
		"Validate dedicated member invitation Secret",
		"path: .workflow-tools",
		"ref: ${{ github.workflow_sha }}",
		"scripts/validate-listingkit-invitation-secret.sh",
		"bash .workflow-tools/scripts/validate-listingkit-invitation-secret.sh",
		"listingkit-workbench-secret",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN",
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_PROJECT_ID",
		"NotFound",
		"jq -e --arg key \"$key\"",
	} {
		if !strings.Contains(deployAPIJob, required) {
			t.Errorf("ListingKit deploy workflow must contain %q", required)
		}
	}
	preflight := strings.Index(deployAPIJob, "Validate dedicated member invitation Secret")
	deploymentUpdate := strings.Index(deployAPIJob, ".workflow-tools/scripts/listingkit-apply-api-deployment.sh")
	if preflight == -1 || deploymentUpdate == -1 || preflight > deploymentUpdate {
		t.Fatal("ListingKit invitation Secret preflight must run before the API Deployment is updated")
	}
}

func listingKitDeployAPIJob(t *testing.T, workflow string) string {
	t.Helper()
	workflow = strings.ReplaceAll(workflow, "\r\n", "\n")
	const marker = "\n  deploy-api:\n"
	start := strings.Index(workflow, marker)
	if start == -1 {
		t.Fatal("ListingKit deploy workflow must define deploy-api job")
	}
	job := workflow[start+len(marker):]
	offset := 0
	for _, line := range strings.SplitAfter(job, "\n") {
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			return job[:offset]
		}
		offset += len(line)
	}
	return job
}

func TestTencentSMSSecretIsAPIScopedAndWebhookIngressIsExact(t *testing.T) {
	base := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "base")
	const secretName = "listingkit-tencent-sms-secret"
	requiredKeys := []string{
		"TASK_PROCESSOR_LISTINGKIT_ZITADEL_SMS_SIGNING_KEY",
		"TASK_PROCESSOR_LISTINGKIT_TENCENT_SMS_SECRET_ID",
		"TASK_PROCESSOR_LISTINGKIT_TENCENT_SMS_SECRET_KEY",
		"TASK_PROCESSOR_LISTINGKIT_TENCENT_SMS_APP_ID",
		"TASK_PROCESSOR_LISTINGKIT_TENCENT_SMS_SIGN_NAME",
		"TASK_PROCESSOR_LISTINGKIT_TENCENT_SMS_TEMPLATE_ID",
	}

	example, err := os.ReadFile(filepath.Join(base, "tencent-sms-secret.example.yaml"))
	if err != nil {
		t.Fatalf("read Tencent SMS Secret example: %v", err)
	}
	for _, required := range append([]string{"name: " + secretName}, requiredKeys...) {
		if !strings.Contains(string(example), required) {
			t.Errorf("Tencent SMS Secret example must contain %q", required)
		}
	}
	for _, key := range requiredKeys {
		if !strings.Contains(string(example), key+`: ""`) {
			t.Errorf("Tencent SMS Secret example must keep %s blank", key)
		}
	}

	sharedSecret, err := os.ReadFile(filepath.Join(base, "secret.example.yaml"))
	if err != nil {
		t.Fatalf("read shared ListingKit Secret: %v", err)
	}
	for _, key := range requiredKeys {
		if strings.Contains(string(sharedSecret), key) {
			t.Errorf("shared ListingKit Secret must not contain %s", key)
		}
	}
	configMap, err := os.ReadFile(filepath.Join(base, "configmap.yaml"))
	if err != nil {
		t.Fatalf("read ListingKit ConfigMap: %v", err)
	}
	for _, key := range requiredKeys {
		if strings.Contains(string(configMap), key) {
			t.Errorf("ListingKit ConfigMap must not contain %s", key)
		}
	}

	apiContainer := loadOnlyContainer(t, "base/product-listing-api-deployment.yaml")
	for _, key := range requiredKeys {
		found := false
		for _, variable := range apiContainer.Env {
			if variable.Name != key || variable.ValueFrom == nil || variable.ValueFrom.SecretKeyRef == nil {
				continue
			}
			found = true
			ref := variable.ValueFrom.SecretKeyRef
			if ref.Name != secretName {
				t.Errorf("ListingKit API must require %s from %s, got %s", key, secretName, ref.Name)
			}
			if ref.Key != key {
				t.Errorf("ListingKit API must map %s to its same Secret key, got %s", key, ref.Key)
			}
			if ref.Optional != nil && *ref.Optional {
				t.Errorf("ListingKit API must not make %s optional", key)
			}
		}
		if !found {
			t.Errorf("ListingKit API deployment must require %s from %s", key, secretName)
		}
	}

	for _, name := range []string{
		"listingkit-ui-deployment.yaml",
		"imgproxy-deployment.yaml",
		"shein-login-worker-deployment.yaml",
		filepath.Join("..", "jobs", "product-listing-api-schema-migrate-job.yaml"),
		filepath.Join("..", "jobs", "listingkit-schema-migrate-job.yaml"),
		filepath.Join("..", "jobs", "pod-image-index-backfill-job.yaml"),
	} {
		manifest, err := os.ReadFile(filepath.Join(base, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if strings.Contains(string(manifest), secretName) {
			t.Errorf("%s must not consume the Tencent SMS Secret", name)
		}
	}

	const webhookPath = "/api/v1/listing-kits/integrations/zitadel/sms"
	webhookRoute := "- path: " + webhookPath + "\n            pathType: Prefix\n            backend:\n              service:\n                name: product-listing-api\n                port:\n                  name: http"
	for _, ingressPath := range []string{
		filepath.Join(base, "ingress.yaml"),
		filepath.Join(base, "..", "overlays", "prod", "patch-ingress.yaml"),
	} {
		ingress, err := os.ReadFile(ingressPath)
		if err != nil {
			t.Fatalf("read ListingKit Ingress %s: %v", ingressPath, err)
		}
		manifest := strings.ReplaceAll(string(ingress), "\r\n", "\n")
		if !strings.Contains(manifest, webhookRoute) {
			t.Fatalf("Ingress %s must route only the signed Zitadel SMS webhook to product-listing-api", ingressPath)
		}
		if strings.Index(manifest, webhookRoute) > strings.Index(manifest, "- path: /\n            pathType: Prefix") {
			t.Fatalf("Ingress %s must route the SMS webhook before the UI catch-all", ingressPath)
		}
	}
}

func TestListingKitSchemaMigrationJobUsesTheReleaseImage(t *testing.T) {
	dockerfilePath := filepath.Join("..", "deployments", "docker", "Dockerfile.product-listing-api")
	dockerfile, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("read product-listing API Dockerfile: %v", err)
	}
	for _, required := range []string{
		"cmd/listingkit-schema-migrate/",
		"/out/listingkit-schema-migrate",
		"/app/listingkit-schema-migrate",
	} {
		if !strings.Contains(string(dockerfile), required) {
			t.Errorf("product-listing API image must contain %q", required)
		}
	}

	jobPath := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "jobs", "listingkit-schema-migrate-job.yaml")
	job, err := os.ReadFile(jobPath)
	if err != nil {
		t.Fatalf("read ListingKit schema migration Job: %v", err)
	}
	for _, required := range []string{
		"kind: Job",
		"REPLACE_WITH_API_IMAGE",
		"/app/listingkit-schema-migrate",
		"listingkit-workbench-config",
		"listingkit-workbench-secret",
		"-scope", "all",
	} {
		if !strings.Contains(string(job), required) {
			t.Errorf("schema migration Job must contain %q", required)
		}
	}
}
