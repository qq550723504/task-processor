package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestListingKitDeployPreflightsBeforeItsOnlyDeploymentMutation(t *testing.T) {
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
	deploymentMutationIndexes := make([]int, 0, 1)
	for index, step := range deployJob.Steps {
		if strings.Contains(step.Run, "scripts/listingkit-identity-preflight-job.sh") {
			preflightIndex = index
			if step.ContinueOnError {
				t.Error("identity preflight step must block deployment when its caller returns failure")
			}
			if !strings.Contains(step.Run, "--image \"$API_CANDIDATE_IMAGE\"") {
				t.Error("identity preflight must receive the exact immutable API image that will be deployed")
			}
			if !strings.Contains(step.Run, "--runner-image \"$PREFLIGHT_RUNNER_IMAGE\"") {
				t.Error("identity preflight must run in its distinct digest-pinned runner image")
			}
			if strings.Contains(step.Run, "--image-tag") {
				t.Error("identity preflight must not infer a fixed-registry image from a tag")
			}
		}
		if strings.Contains(step.Run, "scripts/listingkit-apply-api-deployment.sh") {
			deploymentMutationIndexes = append(deploymentMutationIndexes, index)
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
	if len(deploymentMutationIndexes) != 1 {
		t.Fatalf("deploy-api job must contain exactly one immutable deployment mutation, got %d", len(deploymentMutationIndexes))
	}
	if deploymentMutationIndexes[0] <= preflightIndex {
		t.Fatalf("deployment mutation step %d must run after identity preflight step %d", deploymentMutationIndexes[0], preflightIndex)
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
	schemaIndex, preflightIndex := -1, -1
	for index, step := range deployJob.Steps {
		if strings.Contains(step.Run, "scripts/listingkit-schema-migrate-job.sh") {
			schemaIndex = index
			if !strings.Contains(step.Run, "listingkit-schema-migrate-job.yaml") || !strings.Contains(step.Run, "--image \"$API_CANDIDATE_IMAGE\"") {
				t.Errorf("schema migration step must use the immutable API candidate and ListingKit schema manifest")
			}
		}
		if strings.Contains(step.Run, "scripts/listingkit-identity-preflight-job.sh") {
			preflightIndex = index
		}
	}
	if schemaIndex < 0 || preflightIndex < 0 || schemaIndex >= preflightIndex {
		t.Fatalf("schema migration must run before identity preflight, schema=%d preflight=%d", schemaIndex, preflightIndex)
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

func TestListingKitPreflightDocumentationUsesDigestPinnedCandidateAndRunner(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", "README.md"),
		filepath.Join("..", "docs", "development", "listingkit-local-debug.md"),
	} {
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

func TestListingKitDeployWorkflowSupportsDigestPinnedRollbackWithoutRebuild(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}
	workflow := string(content)
	for _, required := range []string{
		"api_image_digest:",
		"candidate_api_digest",
		"expected_api_repository=\"${REGISTRY}/${DOCKERHUB_NAMESPACE}/${API_IMAGE_NAME}\"",
		"api_image_digest must reference $expected_api_repository",
		"api_image_digest cannot be combined with source or build-image inputs",
		"needs.prepare.outputs.candidate_api_digest == ''",
		"needs.prepare.outputs.candidate_api_digest || needs.build-api.outputs.api_digest",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("ListingKit workflow must contain digest rollback behavior %q", required)
		}
	}
}

func TestListingKitDeployWorkflowPassesDispatchInputsThroughStepEnvironment(t *testing.T) {
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
			"RELEASE_SOURCE_REF":             "${{ inputs.source_ref }}",
			"RELEASE_IMAGE_TAG":              "${{ inputs.image_tag }}",
			"RELEASE_API_IMAGE_DIGEST":       "${{ inputs.api_image_digest }}",
			"RELEASE_API_RUNTIME_BASE_IMAGE": "${{ inputs.api_runtime_base_image }}",
			"RELEASE_PUBLISH_LATEST":         "${{ inputs.publish_latest }}",
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
		if !strings.Contains(step.Run, "scripts/listingkit-identity-preflight-job.sh") {
			continue
		}
		if got, want := step.Env["API_CANDIDATE_DIGEST"], "${{ needs.prepare.outputs.candidate_api_digest || needs.build-api.outputs.api_digest }}"; got != want {
			t.Errorf("preflight API digest environment = %q, want %q", got, want)
		}
		if got, want := step.Env["PREFLIGHT_RUNNER_DIGEST"], "${{ needs.build-preflight-runner.outputs.runner_digest }}"; got != want {
			t.Errorf("preflight runner digest environment = %q, want %q", got, want)
		}
		for _, required := range []string{
			"listingkit_compose_immutable_image",
			"--image \"$API_CANDIDATE_IMAGE\"",
			"--runner-image \"$PREFLIGHT_RUNNER_IMAGE\"",
		} {
			if !strings.Contains(step.Run, required) {
				t.Errorf("preflight step must contain %q", required)
			}
		}
		return
	}
	t.Fatal("ListingKit deploy workflow is missing the identity preflight driver")
}

func TestListingKitDeployWorkflowDerivesDefaultTagFromResolvedCommit(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}
	workflow := string(content)
	for _, required := range []string{
		"GH_TOKEN: ${{ github.token }}",
		"source_ref=\"$(gh api \"repos/${GITHUB_REPOSITORY}/commits/$source_ref\" --jq .sha)\"",
		"runner_source_ref=\"$source_ref\"",
		"tag=\"${source_ref:0:12}\"",
		"^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("ListingKit deploy workflow must contain %q", required)
		}
	}
	if strings.Contains(workflow, "tag=\"${source_ref:0:8}\"") {
		t.Error("ListingKit deploy workflow must not derive a Docker tag directly from an arbitrary ref")
	}
}

func TestListingKitPreflightRunnerUsesCandidateSourceExceptDigestRollback(t *testing.T) {
	workflowPath := filepath.Join("..", ".github", "workflows", "listingkit-deploy.yml")
	content, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read ListingKit deploy workflow: %v", err)
	}
	workflow := string(content)
	for _, required := range []string{
		"runner_source_ref: ${{ steps.meta.outputs.runner_source_ref }}",
		"runner_source_ref=\"$source_ref\"",
		"runner_source_ref=\"$WORKFLOW_SOURCE_REF\"",
		"ref: ${{ needs.prepare.outputs.runner_source_ref }}",
	} {
		if !strings.Contains(workflow, required) {
			t.Errorf("ListingKit runner source selection is missing %q", required)
		}
	}
}
