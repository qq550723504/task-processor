package task

import (
	"fmt"
	"strings"
)

func NormalizeTaskEventV2(event TaskEventV2) (NormalizedTask, error) {
	if event.SchemaVersion != TaskEventSchemaVersionV2 {
		return NormalizedTask{}, fmt.Errorf("unsupported task event schema version %d", event.SchemaVersion)
	}

	taskID := strings.TrimSpace(event.TaskID)
	source := strings.TrimSpace(string(event.SourcePlatform))
	target := strings.TrimSpace(string(event.TargetPlatform))
	if taskID == "" {
		return NormalizedTask{}, fmt.Errorf("missing task ID")
	}
	if source == "" {
		return NormalizedTask{}, fmt.Errorf("missing source platform")
	}
	if target == "" {
		return NormalizedTask{}, fmt.Errorf("missing target platform")
	}

	return NormalizedTask{
		ID: taskID,
		Route: PlatformRoute{
			Source: SourcePlatform(source),
			Target: TargetPlatform(target),
		},
		Payload:  event.Payload,
		TraceID:  strings.TrimSpace(event.TraceID),
		Metadata: event.Metadata,
	}, nil
}
