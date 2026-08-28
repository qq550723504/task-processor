package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	drainRunIDAnnotation      = "listingkit.sh/api-release-run-id"
	drainRunAttemptAnnotation = "listingkit.sh/api-release-run-attempt"
	drainImageAnnotation      = "listingkit.sh/api-release-image"
	drainRoutingAnnotation    = "listingkit.sh/image-agent-routing-contract"
	drainRoutingContract      = "image-agent-v3-new-starts-v1"
)

const zeroDrainSampleOutput = `api_ready_pod_count=1
db_nonterminal_run_count=0
temporal_parent_identity_count=0
open_v2_parent_count=0
open_v2_child_count=0
pending_v2_child_count=0
pending_v2_activity_count=0
pending_v2_activity_attempt_sum=0`

const zeroDrainOutput = `sample=1
` + zeroDrainSampleOutput + `
sample=2
` + zeroDrainSampleOutput + `
sample=3
` + zeroDrainSampleOutput + `
stable_sample_count=3
convergence_interval_seconds=300
first_to_final_window_seconds=600`

func TestListingKitV2DrainCheckAllowsOnlyAuthoritativeZeroInventory(t *testing.T) {
	// Temporal CLI v1.8.1 passes CountWorkflowExecutionsResponse directly to
	// PrintStructured. Its CustomJSONMarshalOptions do not EmitUnpopulated, so
	// the protobuf default count=0 is canonically rendered as {}.
	result := runListingKitV2DrainCheck(t, drainFixture{})
	if result.err != nil {
		t.Fatalf("zero drain must exit zero: %v\n%s", result.err, result.output)
	}
	if got := strings.TrimSpace(string(result.output)); got != zeroDrainOutput {
		t.Fatalf("zero drain output:\n%s\nwant:\n%s", got, zeroDrainOutput)
	}
	if strings.Contains(string(result.output), "secret-") {
		t.Fatalf("drain output must contain deterministic safe counts only: %s", result.output)
	}
	assertMatchingListAndCountQueries(t, result.commandLog, "ImageAgentWorkflow")
	assertMatchingListAndCountQueries(t, result.commandLog, "ImageSlotWorkflow")
}

func TestListingKitV2DrainCheckRequiresThreeCompleteSamplesAcrossFixedWindow(t *testing.T) {
	result := runListingKitV2DrainCheck(t, drainFixture{})
	if result.err != nil {
		t.Fatalf("stable zero drain must exit zero: %v\n%s", result.err, result.output)
	}
	if got := strings.Count(result.commandLog, "workflow list"); got != 6 {
		t.Fatalf("three samples must issue six Temporal list queries, got %d; log=%q", got, result.commandLog)
	}
	for _, workflowType := range []string{"ImageAgentWorkflow", "ImageSlotWorkflow"} {
		if got := strings.Count(result.commandLog, workflowType); got != 6 {
			t.Fatalf("three samples must freshly count and list %s, got %d commands; log=%q", workflowType, got, result.commandLog)
		}
	}
	if got, want := strings.TrimSpace(result.sleepLog), "300\n300"; got != want {
		t.Fatalf("stable drain must wait exactly twice for the fixed 300-second interval, got %q want %q", got, want)
	}
	for _, required := range []string{
		"stable_sample_count=3",
		"convergence_interval_seconds=300",
		"first_to_final_window_seconds=600",
	} {
		if !strings.Contains(string(result.output), required) {
			t.Errorf("stable drain summary is missing %q: %s", required, result.output)
		}
	}
	if got := strings.Count(result.kubectlLog, "get deployment product-listing-api"); got != 3 {
		t.Fatalf("each sample must freshly query the API Deployment, got %d; log=%q", got, result.kubectlLog)
	}
	if got := strings.Count(result.kubectlLog, "get pods -l app=product-listing-api"); got != 3 {
		t.Fatalf("each sample must freshly query serving API Pods, got %d; log=%q", got, result.kubectlLog)
	}
	if got := strings.Count(result.psqlLog, "image_agent_v2_runs"); got != 3 {
		t.Fatalf("each sample must freshly query authoritative non-terminal runs, got %d; log=%q", got, result.psqlLog)
	}
}

func TestListingKitV2DrainCheckRejectsEveryNonzeroRetirementClass(t *testing.T) {
	parentWorkflowID := "image-agent:tenant-a:user-a:run-v2"
	parentDBRow := "tenant-a\tuser-a\trun-v2\n"
	parent := listExecutionJSON(parentWorkflowID, "parent-run", "ImageAgentWorkflow")
	child := listExecutionJSON("child-v2", "child-run", "ImageSlotWorkflow")
	for _, test := range []struct {
		name       string
		fixture    drainFixture
		wantOutput string
	}{
		{name: "open_parent", fixture: drainFixture{parentList: parent, parentCount: workflowCountJSON(1), dbRows: parentDBRow, descriptions: map[string]string{parentWorkflowID: describeExecutionJSON("image-agent-manual", nil, nil)}}, wantOutput: "open_v2_parent_count=1"},
		{name: "open_child", fixture: drainFixture{childList: child, childCount: workflowCountJSON(1), descriptions: map[string]string{"child-v2": describeExecutionJSON("image-agent-manual", nil, nil)}}, wantOutput: "open_v2_child_count=1"},
		{name: "pending_child", fixture: drainFixture{parentList: parent, parentCount: workflowCountJSON(1), dbRows: parentDBRow, descriptions: map[string]string{parentWorkflowID: describeExecutionJSON("image-agent-manual", []string{`{"workflowId":"slot-pending","runId":"slot-run","workflowTypeName":"ImageSlotWorkflow"}`}, nil)}}, wantOutput: "pending_v2_child_count=1"},
		{name: "pending_activity", fixture: drainFixture{parentList: parent, parentCount: workflowCountJSON(1), dbRows: parentDBRow, descriptions: map[string]string{parentWorkflowID: describeExecutionJSON("image-agent-manual", nil, []string{`{"activityType":{"name":"imageagent.execute_slot.v2"},"attempt":3}`})}}, wantOutput: "pending_v2_activity_count=1\npending_v2_activity_attempt_sum=3"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := runListingKitV2DrainCheck(t, test.fixture)
			if result.err == nil {
				t.Fatalf("nonzero drain inventory must exit nonzero: %s", result.output)
			}
			if !strings.Contains(string(result.output), test.wantOutput) {
				t.Fatalf("drain output %q missing %q", result.output, test.wantOutput)
			}
			if test.name == "open_parent" {
				for _, required := range []string{"workflow describe", "--workflow-id " + parentWorkflowID, "--run-id parent-run"} {
					if !strings.Contains(result.commandLog, required) {
						t.Errorf("Temporal CLI must describe exact list identity; log %q missing %q", result.commandLog, required)
					}
				}
			}
		})
	}
}

func TestListingKitV2DrainCheckFailsClosedOnMalformedOrFailedEvidence(t *testing.T) {
	parent := listExecutionJSON("parent-v2", "parent-run", "ImageAgentWorkflow")
	for _, test := range []struct {
		name    string
		fixture drainFixture
	}{
		{name: "malformed_list", fixture: drainFixture{parentList: "{"}},
		{name: "whitespace_list_output", fixture: drainFixture{parentList: "  \n\t"}},
		{name: "malformed_count", fixture: drainFixture{parentCount: "{"}},
		{name: "whitespace_count_output", fixture: drainFixture{parentCount: "  \n\t"}},
		{name: "unknown_count_field", fixture: drainFixture{parentCount: `{"count":"0","total":"0"}`}},
		{name: "noncanonical_explicit_zero_count", fixture: drainFixture{parentCount: `{"count":"0"}`}},
		{name: "noncanonical_empty_groups", fixture: drainFixture{parentCount: `{"groups":[]}`}},
		{name: "numeric_count_is_not_official_proto_json", fixture: drainFixture{parentCount: `{"count":0}`}},
		{name: "negative_count", fixture: drainFixture{parentCount: `{"count":"-1"}`}},
		{name: "noncanonical_count", fixture: drainFixture{parentCount: `{"count":"01"}`}},
		{name: "count_groups", fixture: drainFixture{parentCount: `{"groups":[{"count":"0"}]}`}},
		{name: "noncanonical_empty_array_list", fixture: drainFixture{parentList: "[]"}},
		{name: "count_nonzero_list_empty", fixture: drainFixture{parentCount: workflowCountJSON(1)}},
		{name: "count_list_mismatch", fixture: drainFixture{parentList: parent, parentCount: workflowCountJSON(2)}},
		{name: "malformed_describe", fixture: drainFixture{parentList: parent, parentCount: workflowCountJSON(1), descriptions: map[string]string{"parent-v2": "{"}}},
		{name: "missing_queue", fixture: drainFixture{parentList: parent, parentCount: workflowCountJSON(1), descriptions: map[string]string{"parent-v2": `{"workflowExecutionInfo":{},"pendingChildren":[],"pendingActivities":[]}`}}},
		{name: "list_command_failure", fixture: drainFixture{temporalFailure: "list-parent"}},
		{name: "count_command_failure", fixture: drainFixture{temporalFailure: "count-parent"}},
		{name: "describe_command_failure", fixture: drainFixture{parentList: parent, parentCount: workflowCountJSON(1), temporalFailure: "describe"}},
		{name: "jq_failure", fixture: drainFixture{jqFailure: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := runListingKitV2DrainCheck(t, test.fixture)
			if result.err == nil {
				t.Fatalf("incomplete or malformed evidence must exit nonzero: %s", result.output)
			}
			if strings.TrimSpace(string(result.output)) == zeroDrainOutput {
				t.Fatal("failed collection must never look like explicit zero drain evidence")
			}
		})
	}
}

func TestListingKitV2DrainCheckFailsClosedOnServingAPIIdentityOrReadiness(t *testing.T) {
	expectedImage := drainExpectedAPIImage()
	validAnnotations := drainReleaseAnnotations("424242", "2", expectedImage)
	without := func(source map[string]string, key string) map[string]string {
		clone := cloneStringMap(source)
		delete(clone, key)
		return clone
	}
	wrong := func(source map[string]string, key, value string) map[string]string {
		clone := cloneStringMap(source)
		clone[key] = value
		return clone
	}
	for _, test := range []struct {
		name    string
		fixture drainFixture
	}{
		{name: "wrong_deployment_image", fixture: drainFixture{deploymentJSON: apiDeploymentJSON("docker.io/example/api@sha256:"+strings.Repeat("b", 64), validAnnotations, 1)}},
		{name: "mixed_pod_images", fixture: drainFixture{
			deploymentJSON: apiDeploymentJSON(expectedImage, validAnnotations, 2),
			podsJSON:       apiPodsJSON([]string{expectedImage, "docker.io/example/api@sha256:" + strings.Repeat("b", 64)}, []map[string]string{validAnnotations, validAnnotations}, []bool{true, true}),
		}},
		{name: "missing_run_id_annotation", fixture: drainFixture{deploymentJSON: apiDeploymentJSON(expectedImage, without(validAnnotations, drainRunIDAnnotation), 1)}},
		{name: "wrong_run_id_annotation", fixture: drainFixture{podsJSON: apiPodsJSON([]string{expectedImage}, []map[string]string{wrong(validAnnotations, drainRunIDAnnotation, "999")}, []bool{true})}},
		{name: "missing_run_attempt_annotation", fixture: drainFixture{deploymentJSON: apiDeploymentJSON(expectedImage, without(validAnnotations, drainRunAttemptAnnotation), 1)}},
		{name: "wrong_run_attempt_annotation", fixture: drainFixture{podsJSON: apiPodsJSON([]string{expectedImage}, []map[string]string{wrong(validAnnotations, drainRunAttemptAnnotation, "3")}, []bool{true})}},
		{name: "missing_digest_annotation", fixture: drainFixture{deploymentJSON: apiDeploymentJSON(expectedImage, without(validAnnotations, drainImageAnnotation), 1)}},
		{name: "wrong_digest_annotation", fixture: drainFixture{podsJSON: apiPodsJSON([]string{expectedImage}, []map[string]string{wrong(validAnnotations, drainImageAnnotation, "docker.io/example/api@sha256:"+strings.Repeat("c", 64))}, []bool{true})}},
		{name: "missing_routing_annotation", fixture: drainFixture{deploymentJSON: apiDeploymentJSON(expectedImage, without(validAnnotations, drainRoutingAnnotation), 1)}},
		{name: "wrong_routing_annotation", fixture: drainFixture{podsJSON: apiPodsJSON([]string{expectedImage}, []map[string]string{wrong(validAnnotations, drainRoutingAnnotation, "image-agent-v2-new-starts")}, []bool{true})}},
		{name: "no_serving_pods", fixture: drainFixture{podsJSON: apiPodsJSON(nil, nil, nil)}},
		{name: "pod_not_ready", fixture: drainFixture{podsJSON: apiPodsJSON([]string{expectedImage}, []map[string]string{validAnnotations}, []bool{false})}},
		{name: "malformed_deployment_json", fixture: drainFixture{deploymentJSON: "{"}},
		{name: "malformed_pods_json", fixture: drainFixture{podsJSON: "{"}},
		{name: "deployment_query_failure", fixture: drainFixture{kubectlFailure: "deployment-1"}},
		{name: "pods_query_failure", fixture: drainFixture{kubectlFailure: "pods-1"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := runListingKitV2DrainCheck(t, test.fixture)
			if result.err == nil {
				t.Fatalf("invalid serving API evidence must fail closed: %s", result.output)
			}
		})
	}
}

func TestListingKitV2DrainCheckRequiresV3RoutingContractForEverySample(t *testing.T) {
	expectedImage := drainExpectedAPIImage()
	validAnnotations := drainReleaseAnnotations("424242", "2", expectedImage)
	missingRouting := cloneStringMap(validAnnotations)
	delete(missingRouting, drainRoutingAnnotation)
	zero := zeroDrainSampleFixture()
	result := runListingKitV2DrainCheck(t, drainFixture{samples: []drainSampleFixture{
		zero,
		{deploymentJSON: apiDeploymentJSON(expectedImage, missingRouting, 1)},
		zero,
	}})
	if result.err == nil {
		t.Fatalf("a sample without the immutable v3 routing contract must invalidate drain: %s", result.output)
	}
}

func TestListingKitV2DrainCheckReconcilesDatabaseAndTemporalEverySample(t *testing.T) {
	workflowID := "image-agent:tenant-a:user-a:run-1"
	dbRow := "tenant-a\tuser-a\trun-1\n"
	v2Parent := drainSampleFixture{
		dbRows:      dbRow,
		parentList:  listExecutionJSON(workflowID, "temporal-run-1", "ImageAgentWorkflow"),
		parentCount: workflowCountJSON(1),
		descriptions: map[string]string{
			workflowID: describeExecutionJSON("image-agent-manual", nil, nil),
		},
	}
	v3Parent := v2Parent
	v3Parent.descriptions = map[string]string{workflowID: describeExecutionJSON("image-agent-manual-v3", nil, nil)}
	zero := zeroDrainSampleFixture()
	for _, test := range []struct {
		name    string
		fixture drainFixture
	}{
		{name: "database_only_run", fixture: drainFixture{dbRows: dbRow}},
		{name: "temporal_only_run", fixture: drainFixture{
			parentList: v3Parent.parentList, parentCount: v3Parent.parentCount, descriptions: v3Parent.descriptions,
		}},
		{name: "producer_and_database_run_appears_after_first_zero", fixture: drainFixture{samples: []drainSampleFixture{zero, v2Parent, zero}}},
		{name: "stale_first_and_second_visibility_zero_then_nonzero", fixture: drainFixture{samples: []drainSampleFixture{zero, zero, v2Parent}}},
		{name: "malformed_database_output", fixture: drainFixture{dbRows: "tenant-a\tmissing-run-id\n"}},
		{name: "psql_failure", fixture: drainFixture{psqlFailure: "psql-1"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := runListingKitV2DrainCheck(t, test.fixture)
			if result.err == nil {
				t.Fatalf("database/Temporal disagreement or failure must fail closed: %s", result.output)
			}
			if strings.Contains(string(result.output), workflowID) || strings.Contains(string(result.output), "tenant-a") {
				t.Fatalf("private full identities must not leak to stdout/stderr: %s", result.output)
			}
		})
	}
}

func TestListingKitV2DrainCheckCannotSkipOrOverrideFixedWaits(t *testing.T) {
	result := runListingKitV2DrainCheck(t, drainFixture{sleepFailure: "sleep-1"})
	if result.err == nil {
		t.Fatalf("a skipped or failed convergence wait must not produce drain success: %s", result.output)
	}
	if got := strings.TrimSpace(result.sleepLog); got != "300" {
		t.Fatalf("the first fixed wait must still receive 300 seconds, got %q", got)
	}
}

type drainFixture struct {
	parentList      string
	childList       string
	parentCount     string
	childCount      string
	descriptions    map[string]string
	deploymentJSON  string
	podsJSON        string
	dbRows          string
	samples         []drainSampleFixture
	temporalFailure string
	kubectlFailure  string
	psqlFailure     string
	sleepFailure    string
	jqFailure       bool
}

type drainSampleFixture struct {
	parentList     string
	childList      string
	parentCount    string
	childCount     string
	descriptions   map[string]string
	deploymentJSON string
	podsJSON       string
	dbRows         string
}

type drainResult struct {
	output     []byte
	err        error
	commandLog string
	sleepLog   string
	kubectlLog string
	psqlLog    string
}

func runListingKitV2DrainCheck(t *testing.T, fixture drainFixture) drainResult {
	t.Helper()
	fixtureDir := t.TempDir()
	binDir := t.TempDir()
	commandLogPath := filepath.Join(t.TempDir(), "temporal.log")
	sleepLogPath := filepath.Join(t.TempDir(), "sleep.log")
	kubectlLogPath := filepath.Join(t.TempDir(), "kubectl.log")
	psqlLogPath := filepath.Join(t.TempDir(), "psql.log")
	stateDir := t.TempDir()
	bashEnvPath := filepath.Join(t.TempDir(), "bash-env")
	writeDrainFixture(t, bashEnvPath, `sleep() {
  printf '%s\n' "$*" >> "$FAKE_SLEEP_LOG"
  local sample
  sample="$(( $(wc -l < "$FAKE_SLEEP_LOG") ))"
  [[ "${FAKE_SLEEP_FAILURE:-}" != "sleep-${sample}" ]] || return 9
}
`)
	for index, sample := range normalizedDrainSamples(t, fixture) {
		sampleDir := filepath.Join(fixtureDir, fmt.Sprintf("sample-%d", index+1))
		if err := os.Mkdir(sampleDir, 0o700); err != nil {
			t.Fatal(err)
		}
		writeDrainFixture(t, filepath.Join(sampleDir, "ImageAgentWorkflow.list.json"), sample.parentList)
		writeDrainFixture(t, filepath.Join(sampleDir, "ImageSlotWorkflow.list.json"), sample.childList)
		writeDrainFixture(t, filepath.Join(sampleDir, "ImageAgentWorkflow.count.json"), sample.parentCount)
		writeDrainFixture(t, filepath.Join(sampleDir, "ImageSlotWorkflow.count.json"), sample.childCount)
		writeDrainFixture(t, filepath.Join(sampleDir, "api-deployment.json"), sample.deploymentJSON)
		writeDrainFixture(t, filepath.Join(sampleDir, "api-pods.json"), sample.podsJSON)
		writeDrainFixture(t, filepath.Join(sampleDir, "database.tsv"), sample.dbRows)
		for workflowID, description := range sample.descriptions {
			writeDrainFixture(t, filepath.Join(sampleDir, "describe-"+drainFixtureID(workflowID)+".json"), description)
		}
	}
	writePreflightFake(t, filepath.Join(binDir, "temporal"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_TEMPORAL_LOG"
if [[ "${1:-}" == "--version" ]]; then
  printf 'temporal version 1.8.1\n'
  exit 0
fi
if [[ "${1:-}" != "workflow" ]]; then exit 2; fi
case "${2:-}" in
	  list)
    if [[ "$*" == *"ImageAgentWorkflow"* ]]; then
	      sample="$(( $(cat "$FAKE_DRAIN_STATE/temporal-sample" 2>/dev/null || printf 0) + 1 ))"
	      printf '%s' "$sample" > "$FAKE_DRAIN_STATE/temporal-sample"
	      [[ "${FAKE_TEMPORAL_FAILURE:-}" != "list-parent" && "${FAKE_TEMPORAL_FAILURE:-}" != "list-parent-${sample}" ]] || exit 9
	      cat "$FAKE_TEMPORAL_FIXTURES/sample-${sample}/ImageAgentWorkflow.list.json"
    elif [[ "$*" == *"ImageSlotWorkflow"* ]]; then
	      sample="$(cat "$FAKE_DRAIN_STATE/temporal-sample")"
	      [[ "${FAKE_TEMPORAL_FAILURE:-}" != "list-child" && "${FAKE_TEMPORAL_FAILURE:-}" != "list-child-${sample}" ]] || exit 9
	      cat "$FAKE_TEMPORAL_FIXTURES/sample-${sample}/ImageSlotWorkflow.list.json"
    else
      exit 2
	    fi
	    ;;
	  count)
	    sample="$(cat "$FAKE_DRAIN_STATE/temporal-sample")"
	    if [[ "$*" == *"ImageAgentWorkflow"* ]]; then
	      [[ "${FAKE_TEMPORAL_FAILURE:-}" != "count-parent" && "${FAKE_TEMPORAL_FAILURE:-}" != "count-parent-${sample}" ]] || exit 9
	      cat "$FAKE_TEMPORAL_FIXTURES/sample-${sample}/ImageAgentWorkflow.count.json"
	    elif [[ "$*" == *"ImageSlotWorkflow"* ]]; then
	      [[ "${FAKE_TEMPORAL_FAILURE:-}" != "count-child" && "${FAKE_TEMPORAL_FAILURE:-}" != "count-child-${sample}" ]] || exit 9
	      cat "$FAKE_TEMPORAL_FIXTURES/sample-${sample}/ImageSlotWorkflow.count.json"
	    else
	      exit 2
	    fi
	    ;;
  describe)
	    sample="$(cat "$FAKE_DRAIN_STATE/temporal-sample")"
	    [[ "${FAKE_TEMPORAL_FAILURE:-}" != "describe" && "${FAKE_TEMPORAL_FAILURE:-}" != "describe-${sample}" ]] || exit 9
    workflow_id=""
    while [[ $# -gt 0 ]]; do
      if [[ "$1" == "--workflow-id" ]]; then workflow_id="$2"; break; fi
      shift
	    done
	    [[ -n "$workflow_id" ]] || exit 2
	    fixture_id="${workflow_id//:/__}"
	    cat "$FAKE_TEMPORAL_FIXTURES/sample-${sample}/describe-${fixture_id}.json"
    ;;
  *) exit 2 ;;
esac
`)
	writePreflightFake(t, filepath.Join(binDir, "kubectl"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_KUBECTL_LOG"
if [[ "$*" == *"get deployment product-listing-api"* ]]; then
  sample="$(( $(cat "$FAKE_DRAIN_STATE/kubectl-sample" 2>/dev/null || printf 0) + 1 ))"
  printf '%s' "$sample" > "$FAKE_DRAIN_STATE/kubectl-sample"
  [[ "${FAKE_KUBECTL_FAILURE:-}" != "deployment-${sample}" ]] || exit 9
  cat "$FAKE_TEMPORAL_FIXTURES/sample-${sample}/api-deployment.json"
elif [[ "$*" == *"get pods -l app=product-listing-api"* ]]; then
  sample="$(cat "$FAKE_DRAIN_STATE/kubectl-sample")"
  [[ "${FAKE_KUBECTL_FAILURE:-}" != "pods-${sample}" ]] || exit 9
  cat "$FAKE_TEMPORAL_FIXTURES/sample-${sample}/api-pods.json"
else
  exit 2
fi
`)
	writePreflightFake(t, filepath.Join(binDir, "psql"), `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$FAKE_PSQL_LOG"
sample="$(( $(cat "$FAKE_DRAIN_STATE/psql-sample" 2>/dev/null || printf 0) + 1 ))"
printf '%s' "$sample" > "$FAKE_DRAIN_STATE/psql-sample"
[[ "${FAKE_PSQL_FAILURE:-}" != "psql-${sample}" ]] || exit 9
cat "$FAKE_TEMPORAL_FIXTURES/sample-${sample}/database.tsv"
`)
	writePreflightFake(t, filepath.Join(binDir, "jq"), fakeDrainJQ)

	scriptPath, err := filepath.Abs(filepath.Join("..", "scripts", "listingkit-image-agent-v2-drain-check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(preflightBash(t), filepath.ToSlash(scriptPath),
		"--expected-run-id", "424242",
		"--expected-run-attempt", "2",
		"--expected-api-image", "docker.io/xuwei190/task-processor-product-listing-api@sha256:"+strings.Repeat("a", 64),
		"--namespace", "task-processor")
	jqFailure := "false"
	if fixture.jqFailure {
		jqFailure = "true"
	}
	environment := make([]string, 0, len(os.Environ())+20)
	for _, variable := range os.Environ() {
		if strings.HasPrefix(strings.ToUpper(variable), "PATH=") {
			continue
		}
		environment = append(environment, variable)
	}
	command.Env = append(environment,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TEMPORAL_ADDRESS=secret-address",
		"TEMPORAL_NAMESPACE=secret-namespace",
		"FAKE_TEMPORAL_FIXTURES="+filepath.ToSlash(fixtureDir),
		"FAKE_DRAIN_STATE="+filepath.ToSlash(stateDir),
		"FAKE_TEMPORAL_LOG="+filepath.ToSlash(commandLogPath),
		"FAKE_TEMPORAL_FAILURE="+fixture.temporalFailure,
		"FAKE_KUBECTL_LOG="+filepath.ToSlash(kubectlLogPath),
		"FAKE_KUBECTL_FAILURE="+fixture.kubectlFailure,
		"FAKE_PSQL_LOG="+filepath.ToSlash(psqlLogPath),
		"FAKE_PSQL_FAILURE="+fixture.psqlFailure,
		"FAKE_JQ_FAILURE="+jqFailure,
		"FAKE_SLEEP_LOG="+filepath.ToSlash(sleepLogPath),
		"FAKE_SLEEP_FAILURE="+fixture.sleepFailure,
		"BASH_ENV="+filepath.ToSlash(bashEnvPath),
		"DRAIN_SAMPLE_COUNT=1",
		"DRAIN_CONVERGENCE_INTERVAL_SECONDS=0",
		"PGHOST=secret-db-host",
		"PGPORT=5432",
		"PGDATABASE=secret-db-name",
		"PGUSER=secret-db-user",
		"PGPASSWORD=secret-db-password",
	)
	output, runErr := command.CombinedOutput()
	logContent, _ := os.ReadFile(commandLogPath)
	sleepLog, _ := os.ReadFile(sleepLogPath)
	kubectlLog, _ := os.ReadFile(kubectlLogPath)
	psqlLog, _ := os.ReadFile(psqlLogPath)
	return drainResult{output: output, err: runErr, commandLog: string(logContent), sleepLog: string(sleepLog), kubectlLog: string(kubectlLog), psqlLog: string(psqlLog)}
}

func normalizedDrainSamples(t *testing.T, fixture drainFixture) []drainSampleFixture {
	t.Helper()
	base := drainSampleFixture{
		parentList: fixture.parentList, childList: fixture.childList,
		parentCount: fixture.parentCount, childCount: fixture.childCount,
		descriptions: fixture.descriptions, deploymentJSON: fixture.deploymentJSON,
		podsJSON: fixture.podsJSON, dbRows: fixture.dbRows,
	}
	if len(fixture.samples) == 0 {
		base = withDrainSampleDefaults(base)
		return []drainSampleFixture{base, base, base}
	}
	if len(fixture.samples) != 3 {
		t.Fatalf("drain fixture must model exactly three samples, got %d", len(fixture.samples))
	}
	samples := make([]drainSampleFixture, 3)
	for index, sample := range fixture.samples {
		samples[index] = withDrainSampleDefaults(sample)
	}
	return samples
}

func zeroDrainSampleFixture() drainSampleFixture {
	return withDrainSampleDefaults(drainSampleFixture{})
}

func withDrainSampleDefaults(sample drainSampleFixture) drainSampleFixture {
	if sample.parentCount == "" {
		sample.parentCount = "{}"
	}
	if sample.childCount == "" {
		sample.childCount = "{}"
	}
	expectedImage := drainExpectedAPIImage()
	annotations := drainReleaseAnnotations("424242", "2", expectedImage)
	if sample.deploymentJSON == "" {
		sample.deploymentJSON = apiDeploymentJSON(expectedImage, annotations, 1)
	}
	if sample.podsJSON == "" {
		sample.podsJSON = apiPodsJSON([]string{expectedImage}, []map[string]string{annotations}, []bool{true})
	}
	return sample
}

func drainExpectedAPIImage() string {
	return "docker.io/xuwei190/task-processor-product-listing-api@sha256:" + strings.Repeat("a", 64)
}

func drainReleaseAnnotations(runID, runAttempt, image string) map[string]string {
	return map[string]string{
		drainRunIDAnnotation: runID, drainRunAttemptAnnotation: runAttempt, drainImageAnnotation: image,
		drainRoutingAnnotation: drainRoutingContract,
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func apiDeploymentJSON(image string, annotations map[string]string, replicas int) string {
	document := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "product-listing-api", "generation": 7, "annotations": annotations},
		"spec": map[string]interface{}{
			"replicas": replicas,
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{"annotations": annotations},
				"spec":     map[string]interface{}{"containers": []interface{}{map[string]interface{}{"name": "product-listing-api", "image": image}}},
			},
		},
		"status": map[string]interface{}{"observedGeneration": 7, "readyReplicas": replicas, "updatedReplicas": replicas, "availableReplicas": replicas},
	}
	encoded, _ := json.Marshal(document)
	return string(encoded)
}

func apiPodsJSON(images []string, annotations []map[string]string, ready []bool) string {
	items := make([]interface{}, 0, len(images))
	for index, image := range images {
		podAnnotations := map[string]string{}
		if index < len(annotations) {
			podAnnotations = annotations[index]
		}
		isReady := false
		if index < len(ready) {
			isReady = ready[index]
		}
		conditionStatus := "False"
		if isReady {
			conditionStatus = "True"
		}
		items = append(items, map[string]interface{}{
			"metadata": map[string]interface{}{"name": fmt.Sprintf("product-listing-api-%d", index+1), "annotations": podAnnotations},
			"spec":     map[string]interface{}{"containers": []interface{}{map[string]interface{}{"name": "product-listing-api", "image": image}}},
			"status": map[string]interface{}{
				"phase": "Running", "conditions": []interface{}{map[string]interface{}{"type": "Ready", "status": conditionStatus}},
				"containerStatuses": []interface{}{map[string]interface{}{"name": "product-listing-api", "ready": isReady, "state": map[string]interface{}{"running": map[string]interface{}{"startedAt": "2026-08-28T00:00:00Z"}}}},
			},
		})
	}
	encoded, _ := json.Marshal(map[string]interface{}{"items": items})
	return string(encoded)
}

func drainFixtureID(workflowID string) string {
	return strings.ReplaceAll(workflowID, ":", "__")
}

func listExecutionJSON(workflowID, runID, workflowType string) string {
	return fmt.Sprintf(`{"execution":{"workflowId":%q,"runId":%q},"type":{"name":%q}}`, workflowID, runID, workflowType)
}

func workflowCountJSON(count int) string {
	return fmt.Sprintf(`{"count":%q}`, fmt.Sprint(count))
}

func assertMatchingListAndCountQueries(t *testing.T, commandLog, workflowType string) {
	t.Helper()
	var listQuery, countQuery string
	for _, line := range strings.Split(commandLog, "\n") {
		if !strings.Contains(line, workflowType) {
			continue
		}
		queryOffset := strings.Index(line, "--query ")
		if queryOffset < 0 {
			continue
		}
		query := strings.TrimSpace(line[queryOffset+len("--query "):])
		switch {
		case strings.HasPrefix(line, "workflow list "):
			listQuery = query
		case strings.HasPrefix(line, "workflow count "):
			countQuery = query
		}
	}
	if listQuery == "" || countQuery == "" || listQuery != countQuery {
		t.Fatalf("%s list/count queries must be present and identical; list=%q count=%q log=%q", workflowType, listQuery, countQuery, commandLog)
	}
}

func describeExecutionJSON(queue string, children, activities []string) string {
	if children == nil {
		children = []string{}
	}
	if activities == nil {
		activities = []string{}
	}
	return fmt.Sprintf(`{"workflowExecutionInfo":{"taskQueue":%q},"pendingChildren":[%s],"pendingActivities":[%s]}`,
		queue, strings.Join(children, ","), strings.Join(activities, ","))
}

func writeDrainFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

const fakeDrainJQ = `#!/usr/bin/env python
import json, os, sys
sys.stdout.reconfigure(newline="\n")
if os.environ.get("FAKE_JQ_FAILURE") == "true":
    sys.exit(7)
marker = next((arg for arg in sys.argv[1:] if "listingkit-" in arg), "")
path = sys.argv[-1]
try:
    raw = open(path, encoding="utf-8").read()
    args = sys.argv[1:]
    values = {}
    for index, arg in enumerate(args):
        if arg == "--arg" and index + 2 < len(args):
            values[args[index + 1]] = args[index + 2]
    def annotations_valid(annotations):
        return isinstance(annotations, dict) \
            and annotations.get(values.get("run_id_key")) == values.get("run_id") \
            and annotations.get(values.get("run_attempt_key")) == values.get("run_attempt") \
            and annotations.get(values.get("image_key")) == values.get("image") \
            and annotations.get(values.get("routing_key")) == values.get("routing_contract")
    if "listingkit-api-deployment-shape" in marker:
        data = json.loads(raw)
        metadata = data.get("metadata") if isinstance(data, dict) else None
        spec = data.get("spec") if isinstance(data, dict) else None
        status = data.get("status") if isinstance(data, dict) else None
        template = spec.get("template") if isinstance(spec, dict) else None
        template_metadata = template.get("metadata") if isinstance(template, dict) else None
        template_spec = template.get("spec") if isinstance(template, dict) else None
        containers = template_spec.get("containers") if isinstance(template_spec, dict) else None
        api = [c for c in containers or [] if isinstance(c, dict) and c.get("name") == "product-listing-api"]
        replicas = spec.get("replicas") if isinstance(spec, dict) else None
        generation = metadata.get("generation") if isinstance(metadata, dict) else None
        valid = isinstance(metadata, dict) and metadata.get("name") == "product-listing-api" \
            and annotations_valid(metadata.get("annotations")) \
            and isinstance(template_metadata, dict) and annotations_valid(template_metadata.get("annotations")) \
            and type(generation) is int and generation >= 1 \
            and type(replicas) is int and replicas >= 1 and isinstance(status, dict) \
            and status.get("observedGeneration") == generation \
            and status.get("readyReplicas") == replicas \
            and status.get("updatedReplicas") == replicas \
            and status.get("availableReplicas") == replicas \
            and len(api) == 1 and api[0].get("image") == values.get("image")
        if not valid: sys.exit(1)
        print("true")
    elif "listingkit-api-deployment-ready-count" in marker:
        data = json.loads(raw)
        value = data.get("status", {}).get("readyReplicas") if isinstance(data, dict) else None
        if type(value) is not int or value < 1: sys.exit(1)
        print(value)
    elif "listingkit-api-pods-shape" in marker:
        data = json.loads(raw)
        items = data.get("items") if isinstance(data, dict) else None
        valid = isinstance(items, list) and len(items) > 0
        for pod in items or []:
            metadata = pod.get("metadata") if isinstance(pod, dict) else None
            spec = pod.get("spec") if isinstance(pod, dict) else None
            status = pod.get("status") if isinstance(pod, dict) else None
            containers = spec.get("containers") if isinstance(spec, dict) else None
            statuses = status.get("containerStatuses") if isinstance(status, dict) else None
            api = [c for c in containers or [] if isinstance(c, dict) and c.get("name") == "product-listing-api"]
            api_status = [c for c in statuses or [] if isinstance(c, dict) and c.get("name") == "product-listing-api"]
            conditions = status.get("conditions") if isinstance(status, dict) else None
            ready_condition = any(isinstance(c, dict) and c.get("type") == "Ready" and c.get("status") == "True" for c in conditions or [])
            running = len(api_status) == 1 and isinstance(api_status[0].get("state"), dict) \
                and isinstance(api_status[0]["state"].get("running"), dict)
            valid = valid and isinstance(metadata, dict) and metadata.get("deletionTimestamp") is None \
                and annotations_valid(metadata.get("annotations")) and isinstance(status, dict) \
                and status.get("phase") == "Running" and ready_condition \
                and len(api) == 1 and api[0].get("image") == values.get("image") \
                and len(api_status) == 1 and api_status[0].get("ready") is True and running
        if not valid: sys.exit(1)
        print("true")
    elif "listingkit-api-pods-count" in marker:
        data = json.loads(raw)
        items = data.get("items") if isinstance(data, dict) else None
        if not isinstance(items, list): sys.exit(1)
        print(len(items))
    elif "listingkit-count" in marker:
        data = json.loads(raw)
        if data == {}:
            value = "0"
        elif isinstance(data, dict) and set(data) == {"count"}:
            value = data["count"]
            if not isinstance(value, str) or not value.isdigit() or value == "0" or value.startswith("0"):
                sys.exit(1)
        else:
            sys.exit(1)
        print(value)
    elif "listingkit-list" in marker:
        decoder = json.JSONDecoder()
        docs, index = [], 0
        while index < len(raw):
            while index < len(raw) and raw[index].isspace(): index += 1
            if index >= len(raw): break
            value, index = decoder.raw_decode(raw, index)
            docs.append(value)
        records = docs
        valid = len(docs) > 0 and all(isinstance(doc, dict) for doc in docs)
        valid = valid and all(isinstance(r, dict) and isinstance(r.get("execution"), dict)
                    and isinstance(r["execution"].get("workflowId"), str) and r["execution"]["workflowId"]
                    and isinstance(r["execution"].get("runId"), str) and r["execution"]["runId"]
                    and isinstance(r.get("type"), dict) and isinstance(r["type"].get("name"), str) and r["type"]["name"]
                    for r in records)
        if "shape" in marker:
            if not valid: sys.exit(1)
            print("true")
        elif "count" in marker:
            if not valid: sys.exit(1)
            print(len(records))
        else:
            if not valid: sys.exit(1)
            for r in records:
                print("\t".join([r["execution"]["workflowId"], r["execution"]["runId"], r["type"]["name"]]))
    else:
        data = json.loads(raw)
        info = data.get("workflowExecutionInfo") if isinstance(data, dict) else None
        queue = info.get("taskQueue") if isinstance(info, dict) else None
        children = data.get("pendingChildren") if isinstance(data, dict) else None
        activities = data.get("pendingActivities") if isinstance(data, dict) else None
        child_valid = isinstance(children, list) and all(isinstance(c, dict)
            and isinstance(c.get("workflowId"), str) and c["workflowId"]
            and isinstance(c.get("runId"), str) and c["runId"]
            and (c.get("workflowTypeName") == "ImageSlotWorkflow"
                 or isinstance(c.get("workflowType"), dict) and c["workflowType"].get("name") == "ImageSlotWorkflow")
            for c in children)
        activity_valid = isinstance(activities, list) and all(isinstance(a, dict)
            and isinstance(a.get("activityType"), dict)
            and isinstance(a["activityType"].get("name"), str) and a["activityType"]["name"]
            and isinstance(a.get("attempt"), int) and a["attempt"] >= 1
            for a in activities)
        valid = isinstance(queue, str) and bool(queue) and child_valid and activity_valid
        if "shape" in marker:
            if not valid: sys.exit(1)
            print("true")
        elif not valid:
            sys.exit(1)
        elif "queue" in marker:
            print(queue)
        elif "child-rows" in marker:
            for c in children:
                name = c.get("workflowTypeName") or c["workflowType"]["name"]
                print("\t".join([c["workflowId"], c["runId"], name]))
        elif "activity-rows" in marker:
            for a in activities:
                print("\t".join([a["activityType"]["name"], str(a["attempt"])]))
        else:
            sys.exit(2)
except Exception:
    sys.exit(1)
`
