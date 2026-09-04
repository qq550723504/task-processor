package httpapi

import (
	"testing"

	kernelmodule "task-processor/internal/kernel/module"
)

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
