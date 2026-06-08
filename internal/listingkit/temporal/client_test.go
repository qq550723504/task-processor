package temporal

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	sdkclient "go.temporal.io/sdk/client"

	"task-processor/internal/listingkit"
	"task-processor/internal/listingkit/core"
	"task-processor/internal/listingkit/submission"
)

func TestClientStartSheinPublishUsesStableWorkflowID(t *testing.T) {
	t.Parallel()

	raw := &stubTemporalClient{}
	client := NewClient(raw)

	in := listingkit.SheinPublishWorkflowStartInput{
		TaskID:      "task-123",
		Platform:    "shein",
		Action:      "publish",
		RequestID:   "request-123",
		RequestedAt: time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC),
	}
	if err := client.StartSheinPublish(context.Background(), in); err != nil {
		t.Fatalf("start shein publish: %v", err)
	}

	if raw.options.ID != "shein-submit:task-123:publish" {
		t.Fatalf("workflow id = %q, want stable shein publish id", raw.options.ID)
	}
	if raw.options.TaskQueue != TaskQueueSheinSubmitPublish {
		t.Fatalf("task queue = %q, want %q", raw.options.TaskQueue, TaskQueueSheinSubmitPublish)
	}
	if raw.options.WorkflowExecutionErrorWhenAlreadyStarted != true {
		t.Fatalf("already-started error flag = false, want true")
	}
	if raw.options.WorkflowIDConflictPolicy != enumspb.WORKFLOW_ID_CONFLICT_POLICY_FAIL {
		t.Fatalf("conflict policy = %v, want fail", raw.options.WorkflowIDConflictPolicy)
	}
	if raw.options.WorkflowIDReusePolicy != enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE {
		t.Fatalf("reuse policy = %v, want allow duplicate", raw.options.WorkflowIDReusePolicy)
	}
	if reflect.ValueOf(raw.workflow).Pointer() != reflect.ValueOf(PublishWorkflow).Pointer() {
		t.Fatalf("workflow = %T, want PublishWorkflow", raw.workflow)
	}
	got, ok := raw.args[0].(SheinPublishWorkflowInput)
	if !ok {
		t.Fatalf("workflow input = %T, want SheinPublishWorkflowInput", raw.args[0])
	}
	if got.TaskID != in.TaskID || got.Platform != "shein" || got.Action != "publish" || got.RequestID != in.RequestID {
		t.Fatalf("workflow input = %+v, want request fields from start input", got)
	}
}

func TestClientStartStandardProductUsesStableWorkflowIDAndAllowsManualRerun(t *testing.T) {
	t.Parallel()

	raw := &stubTemporalClient{}
	client := NewClient(raw)

	in := listingkit.StandardProductWorkflowStartInput{
		TaskID:      "task-123",
		RequestedAt: time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
	}
	if err := client.StartStandardProduct(context.Background(), in); err != nil {
		t.Fatalf("start standard product: %v", err)
	}

	if raw.options.ID != "listingkit-standard:task-123" {
		t.Fatalf("workflow id = %q, want stable standard workflow id", raw.options.ID)
	}
	if raw.options.TaskQueue != TaskQueueStandardProduct {
		t.Fatalf("task queue = %q, want %q", raw.options.TaskQueue, TaskQueueStandardProduct)
	}
	if raw.options.WorkflowIDReusePolicy != enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE {
		t.Fatalf("reuse policy = %v, want allow duplicate", raw.options.WorkflowIDReusePolicy)
	}
}

func TestClientStartPlatformAdaptationUsesStableWorkflowIDAndAllowsManualRerun(t *testing.T) {
	t.Parallel()

	raw := &stubTemporalClient{}
	client := NewClient(raw)

	in := listingkit.PlatformAdaptWorkflowStartInput{
		TaskID:      "task-123",
		Platform:    "all",
		RequestedAt: time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC),
	}
	if err := client.StartPlatformAdaptation(context.Background(), in); err != nil {
		t.Fatalf("start platform adaptation: %v", err)
	}

	if raw.options.ID != "listingkit-platform:all:task-123" {
		t.Fatalf("workflow id = %q, want stable platform workflow id", raw.options.ID)
	}
	if raw.options.TaskQueue != TaskQueuePlatformAdapt {
		t.Fatalf("task queue = %q, want %q", raw.options.TaskQueue, TaskQueuePlatformAdapt)
	}
	if raw.options.WorkflowIDReusePolicy != enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE {
		t.Fatalf("reuse policy = %v, want allow duplicate", raw.options.WorkflowIDReusePolicy)
	}
}

func TestClientStartSheinPublishCarriesConfirmedFinal(t *testing.T) {
	t.Parallel()

	raw := &stubTemporalClient{}
	client := NewClient(raw)

	if err := client.StartSheinPublish(context.Background(), listingkit.SheinPublishWorkflowStartInput{
		TaskID:         "task-123",
		RequestID:      "request-123",
		ConfirmedFinal: true,
	}); err != nil {
		t.Fatalf("start shein publish: %v", err)
	}

	got, ok := raw.args[0].(SheinPublishWorkflowInput)
	if !ok {
		t.Fatalf("workflow input = %T, want SheinPublishWorkflowInput", raw.args[0])
	}
	if !got.ConfirmedFinal {
		t.Fatalf("workflow input = %+v, want ConfirmedFinal=true", got)
	}
}

func TestClientQuerySheinPublishStateUsesCurrentStateQuery(t *testing.T) {
	t.Parallel()

	want := listingkit.SheinPublishWorkflowState{
		TaskID:          "task-123",
		Action:          "publish",
		RequestID:       "request-123",
		CurrentPhase:    "submit_remote",
		WorkflowRunning: true,
	}
	raw := &stubTemporalClient{queryValue: SheinPublishStateQueryResult(want)}
	client := NewClient(raw)

	got, err := client.QuerySheinPublishState(context.Background(), "task-123")
	if err != nil {
		t.Fatalf("query shein publish state: %v", err)
	}

	if raw.queryWorkflowID != "shein-submit:task-123:publish" {
		t.Fatalf("query workflow id = %q, want stable shein publish id", raw.queryWorkflowID)
	}
	if raw.queryType != SheinPublishQueryCurrentState {
		t.Fatalf("query type = %q, want %q", raw.queryType, SheinPublishQueryCurrentState)
	}
	if got == nil || *got != want {
		t.Fatalf("query state = %+v, want %+v", got, want)
	}
}

func TestClientMapsAlreadyStartedToSubmitInProgress(t *testing.T) {
	t.Parallel()

	raw := &stubTemporalClient{
		startErr: serviceerror.NewWorkflowExecutionAlreadyStarted("already started", "", "run-123"),
	}
	client := NewClient(raw)

	err := client.StartSheinPublish(context.Background(), listingkit.SheinPublishWorkflowStartInput{
		TaskID: "task-123",
	})

	if !errors.Is(err, core.ErrSubmitInProgress) {
		t.Fatalf("start err = %v, want ErrSubmitInProgress", err)
	}
}

func TestClientMapsAlreadyStartedToDetailedSubmitInProgressWhenStateQuerySucceeds(t *testing.T) {
	t.Parallel()

	raw := &stubTemporalClient{
		startErr:   serviceerror.NewWorkflowExecutionAlreadyStarted("already started", "", "run-123"),
		queryValue: SheinPublishStateQueryResult{TaskID: "task-123", Action: "publish", RequestID: "request-123", CurrentPhase: "validate", WorkflowRunning: true},
	}
	client := NewClient(raw)

	err := client.StartSheinPublish(context.Background(), listingkit.SheinPublishWorkflowStartInput{
		TaskID: "task-123",
		Action: "publish",
	})

	var inProgress *submission.SubmitInProgressError
	if !errors.As(err, &inProgress) {
		t.Fatalf("start err = %v, want SubmitInProgressError", err)
	}
	if inProgress.RequestID != "request-123" || inProgress.Phase != "validate" {
		t.Fatalf("submit in progress err = %+v, want request-123/validate", inProgress)
	}
}

type stubTemporalClient struct {
	options  sdkclient.StartWorkflowOptions
	workflow interface{}
	args     []interface{}
	startErr error

	queryWorkflowID string
	queryType       string
	queryValue      SheinPublishStateQueryResult
	queryErr        error
}

func (s *stubTemporalClient) ExecuteWorkflow(ctx context.Context, options sdkclient.StartWorkflowOptions, workflow interface{}, args ...interface{}) (sdkclient.WorkflowRun, error) {
	s.options = options
	s.workflow = workflow
	s.args = append([]interface{}(nil), args...)
	return nil, s.startErr
}

func (s *stubTemporalClient) QueryWorkflow(ctx context.Context, workflowID string, runID string, queryType string, args ...interface{}) (encodedValue, error) {
	s.queryWorkflowID = workflowID
	s.queryType = queryType
	return stubEncodedValue{value: s.queryValue}, s.queryErr
}

type stubEncodedValue struct {
	value SheinPublishStateQueryResult
}

func (v stubEncodedValue) HasValue() bool {
	return true
}

func (v stubEncodedValue) Get(valuePtr interface{}) error {
	out, ok := valuePtr.(*SheinPublishStateQueryResult)
	if !ok {
		return errors.New("unexpected query target")
	}
	*out = v.value
	return nil
}
