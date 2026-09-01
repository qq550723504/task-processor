package rabbitmq

import (
	"errors"
	"testing"
)

func TestConsumerStateRunningClearsLastError(t *testing.T) {
	sm := NewConsumerStateManager()

	sm.SetError(errors.New("temporary worker stop"), "shein.tasks.bucket.0")
	if sm.IsHealthy() {
		t.Fatal("expected errored consumer to be unhealthy")
	}

	sm.SetState(ConsumerStateRunning, "shein.tasks.bucket.0")

	info := sm.GetStateInfo()
	if info.LastError != nil {
		t.Fatalf("expected LastError to be cleared after recovery, got %v", info.LastError)
	}
	if !sm.IsHealthy() {
		t.Fatal("expected recovered running consumer to be healthy")
	}
}
