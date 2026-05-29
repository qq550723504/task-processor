package httpapi

import (
	"testing"

	kernelmodule "task-processor/internal/kernel/module"
)

func TestBuildTemporalRuntimeBuildsModuleAndWorkerService(t *testing.T) {
	t.Parallel()

	result, err := BuildTemporalRuntime(TemporalRuntimeBuildInput{
		ServiceInput: buildSuccessfulServiceInputFixture(),
	})
	if err != nil {
		t.Fatalf("BuildTemporalRuntime() error = %v", err)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	reg := kernelmodule.NewRegistry()
	if err := result.Module.Register(reg); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	workers := reg.TemporalWorkers()
	if len(workers) != 1 {
		t.Fatalf("temporal workers = %d, want 1", len(workers))
	}
	if workers[0].Name != temporalWorkerName {
		t.Fatalf("temporal worker name = %s, want %s", workers[0].Name, temporalWorkerName)
	}
	if workers[0].Start == nil {
		t.Fatal("expected temporal worker starter")
	}
}

func TestTemporalModuleRegisterSkipsMissingWorkerService(t *testing.T) {
	t.Parallel()

	reg := kernelmodule.NewRegistry()
	if err := buildTemporalModule(temporalModuleInput{}).Register(reg); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if len(reg.TemporalWorkers()) != 0 {
		t.Fatalf("temporal workers = %d, want 0", len(reg.TemporalWorkers()))
	}
}
