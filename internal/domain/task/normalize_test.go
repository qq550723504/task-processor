package task

import "testing"

func TestNormalizeTaskMessageLegacyPlatformOnly(t *testing.T) {
	task, err := NormalizeTaskMessage(TaskMessage{
		TaskID:   "1001",
		Platform: "shein",
	})
	if err != nil {
		t.Fatalf("NormalizeTaskMessage returned error: %v", err)
	}
	if task.Route.Source != SourcePlatform("shein") {
		t.Fatalf("expected legacy platform to become source, got %q", task.Route.Source)
	}
	if task.Route.Target != TargetPlatform("shein") {
		t.Fatalf("expected legacy platform to become target, got %q", task.Route.Target)
	}
}

func TestNormalizeTaskMessageSourceAndTarget(t *testing.T) {
	task, err := NormalizeTaskMessage(TaskMessage{
		TaskID:         "1001",
		SourcePlatform: "amazon",
		TargetPlatform: "temu",
	})
	if err != nil {
		t.Fatalf("NormalizeTaskMessage returned error: %v", err)
	}
	if task.Route.Source != SourcePlatformAmazon {
		t.Fatalf("expected source amazon, got %q", task.Route.Source)
	}
	if task.Route.Target != TargetPlatformTemu {
		t.Fatalf("expected target temu, got %q", task.Route.Target)
	}
}

func TestNormalizeTaskMessageRejectsPlatformTargetConflict(t *testing.T) {
	_, err := NormalizeTaskMessage(TaskMessage{
		TaskID:         "1001",
		Platform:       "shein",
		TargetPlatform: "temu",
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestNormalizeTaskMessageRequiresTargetPlatform(t *testing.T) {
	_, err := NormalizeTaskMessage(TaskMessage{
		TaskID:         "1001",
		SourcePlatform: "amazon",
	})
	if err == nil {
		t.Fatal("expected missing target platform error")
	}
}
