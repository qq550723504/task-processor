package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const zeroDrainOutput = `open_v2_parent_count=0
open_v2_child_count=0
pending_v2_child_count=0
pending_v2_activity_count=0
pending_v2_activity_attempt_sum=0`

func TestListingKitV2DrainCheckAllowsOnlyExplicitZeroInventory(t *testing.T) {
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
}

func TestListingKitV2DrainCheckRejectsEveryNonzeroRetirementClass(t *testing.T) {
	parent := listExecutionJSON("parent-v2", "parent-run", "ImageAgentWorkflow")
	child := listExecutionJSON("child-v2", "child-run", "ImageSlotWorkflow")
	for _, test := range []struct {
		name       string
		fixture    drainFixture
		wantOutput string
	}{
		{name: "open_parent", fixture: drainFixture{parentList: parent, descriptions: map[string]string{"parent-v2": describeExecutionJSON("image-agent-manual", nil, nil)}}, wantOutput: "open_v2_parent_count=1"},
		{name: "open_child", fixture: drainFixture{childList: child, descriptions: map[string]string{"child-v2": describeExecutionJSON("image-agent-manual", nil, nil)}}, wantOutput: "open_v2_child_count=1"},
		{name: "pending_child", fixture: drainFixture{parentList: parent, descriptions: map[string]string{"parent-v2": describeExecutionJSON("image-agent-manual", []string{`{"workflowId":"slot-pending","runId":"slot-run","workflowTypeName":"ImageSlotWorkflow"}`}, nil)}}, wantOutput: "pending_v2_child_count=1"},
		{name: "pending_activity", fixture: drainFixture{parentList: parent, descriptions: map[string]string{"parent-v2": describeExecutionJSON("image-agent-manual", nil, []string{`{"activityType":{"name":"imageagent.execute_slot.v2"},"attempt":3}`})}}, wantOutput: "pending_v2_activity_count=1\npending_v2_activity_attempt_sum=3"},
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
				for _, required := range []string{"workflow describe", "--workflow-id parent-v2", "--run-id parent-run"} {
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
		{name: "empty_list_output", fixture: drainFixture{parentListProvided: true}},
		{name: "whitespace_list_output", fixture: drainFixture{parentList: "  \n\t", parentListProvided: true}},
		{name: "malformed_describe", fixture: drainFixture{parentList: parent, descriptions: map[string]string{"parent-v2": "{"}}},
		{name: "missing_queue", fixture: drainFixture{parentList: parent, descriptions: map[string]string{"parent-v2": `{"workflowExecutionInfo":{},"pendingChildren":[],"pendingActivities":[]}`}}},
		{name: "list_command_failure", fixture: drainFixture{temporalFailure: "list-parent"}},
		{name: "describe_command_failure", fixture: drainFixture{parentList: parent, temporalFailure: "describe"}},
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

type drainFixture struct {
	parentList         string
	parentListProvided bool
	childList          string
	descriptions       map[string]string
	temporalFailure    string
	jqFailure          bool
}

type drainResult struct {
	output     []byte
	err        error
	commandLog string
}

func runListingKitV2DrainCheck(t *testing.T, fixture drainFixture) drainResult {
	t.Helper()
	fixtureDir := t.TempDir()
	binDir := t.TempDir()
	commandLogPath := filepath.Join(t.TempDir(), "temporal.log")
	if fixture.parentList == "" && !fixture.parentListProvided {
		fixture.parentList = "[]"
	}
	if fixture.childList == "" {
		fixture.childList = "[]"
	}
	writeDrainFixture(t, filepath.Join(fixtureDir, "ImageAgentWorkflow.list.json"), fixture.parentList)
	writeDrainFixture(t, filepath.Join(fixtureDir, "ImageSlotWorkflow.list.json"), fixture.childList)
	for workflowID, description := range fixture.descriptions {
		writeDrainFixture(t, filepath.Join(fixtureDir, "describe-"+workflowID+".json"), description)
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
      [[ "${FAKE_TEMPORAL_FAILURE:-}" != "list-parent" ]] || exit 9
      cat "$FAKE_TEMPORAL_FIXTURES/ImageAgentWorkflow.list.json"
    elif [[ "$*" == *"ImageSlotWorkflow"* ]]; then
      [[ "${FAKE_TEMPORAL_FAILURE:-}" != "list-child" ]] || exit 9
      cat "$FAKE_TEMPORAL_FIXTURES/ImageSlotWorkflow.list.json"
    else
      exit 2
    fi
    ;;
  describe)
    [[ "${FAKE_TEMPORAL_FAILURE:-}" != "describe" ]] || exit 9
    workflow_id=""
    while [[ $# -gt 0 ]]; do
      if [[ "$1" == "--workflow-id" ]]; then workflow_id="$2"; break; fi
      shift
    done
    [[ -n "$workflow_id" ]] || exit 2
    cat "$FAKE_TEMPORAL_FIXTURES/describe-${workflow_id}.json"
    ;;
  *) exit 2 ;;
esac
`)
	writePreflightFake(t, filepath.Join(binDir, "jq"), fakeDrainJQ)

	scriptPath, err := filepath.Abs(filepath.Join("..", "scripts", "listingkit-image-agent-v2-drain-check.sh"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(preflightBash(t), filepath.ToSlash(scriptPath))
	jqFailure := "false"
	if fixture.jqFailure {
		jqFailure = "true"
	}
	command.Env = append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TEMPORAL_ADDRESS=secret-address",
		"TEMPORAL_NAMESPACE=secret-namespace",
		"FAKE_TEMPORAL_FIXTURES="+filepath.ToSlash(fixtureDir),
		"FAKE_TEMPORAL_LOG="+filepath.ToSlash(commandLogPath),
		"FAKE_TEMPORAL_FAILURE="+fixture.temporalFailure,
		"FAKE_JQ_FAILURE="+jqFailure,
	)
	output, runErr := command.CombinedOutput()
	logContent, _ := os.ReadFile(commandLogPath)
	return drainResult{output: output, err: runErr, commandLog: string(logContent)}
}

func listExecutionJSON(workflowID, runID, workflowType string) string {
	return fmt.Sprintf(`[{"execution":{"workflowId":%q,"runId":%q},"type":{"name":%q}}]`, workflowID, runID, workflowType)
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
    if "listingkit-list" in marker:
        decoder = json.JSONDecoder()
        docs, index = [], 0
        while index < len(raw):
            while index < len(raw) and raw[index].isspace(): index += 1
            if index >= len(raw): break
            value, index = decoder.raw_decode(raw, index)
            docs.append(value)
        records = []
        for doc in docs:
            records.extend(doc if isinstance(doc, list) else [doc])
        valid = len(docs) > 0 and all(isinstance(doc, (list, dict)) for doc in docs)
        valid = valid and all(isinstance(r, dict) and isinstance(r.get("execution"), dict)
                    and isinstance(r["execution"].get("workflowId"), str) and r["execution"]["workflowId"]
                    and isinstance(r["execution"].get("runId"), str) and r["execution"]["runId"]
                    and isinstance(r.get("type"), dict) and isinstance(r["type"].get("name"), str) and r["type"]["name"]
                    for r in records)
        if "shape" in marker:
            if not valid: sys.exit(1)
            print("true")
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
