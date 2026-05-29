package listingkit

import (
	"os"
	"strings"
	"testing"
)

func TestServiceProcessFilesKeepTerminalizationInsideProcessFlowSeam(t *testing.T) {
	t.Parallel()

	serviceProcessSrc, err := os.ReadFile("service_process.go")
	if err != nil {
		t.Fatalf("ReadFile(service_process.go) error = %v", err)
	}
	serviceProcessContent := string(serviceProcessSrc)

	for _, needle := range []string{
		"return buildListingKitProcessFlow(s).run(ctx, task, log)",
	} {
		if !strings.Contains(serviceProcessContent, needle) {
			t.Fatalf("service_process.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"s.repo.MarkProcessing(",
		"s.repo.MarkCompleted(",
		"s.repo.MarkNeedsReview(",
		"s.repo.MarkFailed(",
		"s.persistProcessFailure(",
		"s.persistProcessSuccess(",
	} {
		if strings.Contains(serviceProcessContent, needle) {
			t.Fatalf("service_process.go should not contain %q", needle)
		}
	}

	flowSrc, err := os.ReadFile("service_process_flow.go")
	if err != nil {
		t.Fatalf("ReadFile(service_process_flow.go) error = %v", err)
	}
	flowContent := string(flowSrc)
	for _, needle := range []string{
		"func buildListingKitProcessFlow(s *service) *listingKitProcessFlow {",
		"func (f *listingKitProcessFlow) run(ctx context.Context, task *Task, log *logrus.Entry) (*ListingKitResult, error) {",
		"f.service.persistProcessFailure(",
		"f.service.persistProcessSuccess(",
	} {
		if !strings.Contains(flowContent, needle) {
			t.Fatalf("service_process_flow.go should contain %q", needle)
		}
	}
}

func TestProcessorFilesKeepSkipAndRetryDecisionsInsideStateHelper(t *testing.T) {
	t.Parallel()

	processorSrc, err := os.ReadFile("processor.go")
	if err != nil {
		t.Fatalf("ReadFile(processor.go) error = %v", err)
	}
	processorContent := string(processorSrc)

	for _, needle := range []string{
		"stateMachine  *ProcessorStateMachine",
		"stateMachine:  NewProcessorStateMachine(maxRetries),",
		"if err := p.stateMachine.CanProcess(task); err != nil {",
		"if p.stateMachine.ShouldRetry(task) {",
	} {
		if !strings.Contains(processorContent, needle) {
			t.Fatalf("processor.go should contain %q", needle)
		}
	}

	for _, needle := range []string{
		"if task.Status != TaskStatusPending {",
		"if task.RetryCount < p.maxRetries {",
	} {
		if strings.Contains(processorContent, needle) {
			t.Fatalf("processor.go should not contain %q", needle)
		}
	}

	stateMachineSrc, err := os.ReadFile("processor_state_machine.go")
	if err != nil {
		t.Fatalf("ReadFile(processor_state_machine.go) error = %v", err)
	}
	stateMachineContent := string(stateMachineSrc)
	for _, needle := range []string{
		"func NewProcessorStateMachine(maxRetries int) *ProcessorStateMachine {",
		"func (m *ProcessorStateMachine) CanProcess(task *Task) error {",
		"func (m *ProcessorStateMachine) ShouldRetry(task *Task) bool {",
	} {
		if !strings.Contains(stateMachineContent, needle) {
			t.Fatalf("processor_state_machine.go should contain %q", needle)
		}
	}
}
