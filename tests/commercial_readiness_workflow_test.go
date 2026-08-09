package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommercialReadinessWorkflowCollectsPinnedReleaseEvidence(t *testing.T) {
	path := filepath.Join("..", ".github", "workflows", "commercial-readiness.yml")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read commercial-readiness workflow: %v", err)
	}

	workflow := string(content)
	for _, required := range []string{
		"workflow_dispatch:",
		"commit_sha:",
		"^[0-9a-fA-F]{40}$",
		"ref: ${{ inputs.commit_sha }}",
		"go test ./... -count=1",
		"go test -race ./internal/app/runtime/listingcontrol -run TestControlPlaneService -count=1",
		"go test -race ./internal/listingadmin -run \"TestConcurrentClaimForDispatchOnlyOneWorkerWins|TestConcurrentRollbackDispatchOnlyOriginalQueuedClaimIsRestoredOnce|TestConcurrentRecoveryOnlyUpdatesStillEligibleRowsOnce\" -count=1",
		"make build-all",
		"npm run lint",
		"npm run typecheck",
		"npm test",
		"npm run build",
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
		"sparse-checkout: scripts/validate-listingkit-invitation-secret.sh",
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
	deploymentUpdate := strings.Index(deployAPIJob, "Update API deployment image")
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
		"REPLACE_WITH_DEPLOYED_TAG",
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
