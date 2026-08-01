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
