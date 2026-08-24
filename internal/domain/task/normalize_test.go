package task

import "testing"

func TestNormalizeTaskEventV2RequiresExplicitRouting(t *testing.T) {
	task, err := NormalizeTaskEventV2(TaskEventV2{
		SchemaVersion:  TaskEventSchemaVersionV2,
		TaskID:         "1001",
		SourcePlatform: SourcePlatformAmazon,
		TargetPlatform: TargetPlatformTemu,
	})
	if err != nil {
		t.Fatalf("NormalizeTaskEventV2 returned error: %v", err)
	}
	if task.Route.Source != SourcePlatformAmazon {
		t.Fatalf("expected source amazon, got %q", task.Route.Source)
	}
	if task.Route.Target != TargetPlatformTemu {
		t.Fatalf("expected target temu, got %q", task.Route.Target)
	}
}

func TestNormalizeTaskEventV2RejectsMissingRequiredFields(t *testing.T) {
	_, err := NormalizeTaskEventV2(TaskEventV2{
		SchemaVersion:  TaskEventSchemaVersionV2,
		TaskID:         "1001",
		SourcePlatform: SourcePlatformAmazon,
	})
	if err == nil {
		t.Fatal("expected missing target platform error")
	}
}

func TestNormalizeTaskEventV2RejectsUnsupportedSchemaVersion(t *testing.T) {
	_, err := NormalizeTaskEventV2(TaskEventV2{
		SchemaVersion:  1,
		TaskID:         "1001",
		SourcePlatform: SourcePlatformAmazon,
		TargetPlatform: TargetPlatformTemu,
	})
	if err == nil {
		t.Fatal("expected schema version error")
	}
}
