package task

import (
	"fmt"
	"strings"
)

func NormalizeTaskMessage(msg TaskMessage) (NormalizedTask, error) {
	source := strings.TrimSpace(msg.SourcePlatform)
	target := strings.TrimSpace(msg.TargetPlatform)
	legacyPlatform := strings.TrimSpace(msg.Platform)

	if source == "" && legacyPlatform != "" {
		source = legacyPlatform
	}
	if target == "" && legacyPlatform != "" {
		target = legacyPlatform
	}
	if target == "" {
		return NormalizedTask{}, fmt.Errorf("missing target platform")
	}
	// Historical messages used platform as the source crawler platform.
	// Newer messages split sourcePlatform and targetPlatform, so
	// amazon -> shein must be accepted.
	if legacyPlatform != "" && msg.SourcePlatform != "" && !strings.EqualFold(legacyPlatform, source) {
		return NormalizedTask{}, fmt.Errorf("platform %q conflicts with sourcePlatform %q", legacyPlatform, source)
	}

	return NormalizedTask{
		ID: strings.TrimSpace(msg.TaskID),
		Route: PlatformRoute{
			Source: SourcePlatform(source),
			Target: TargetPlatform(target),
		},
		Payload:  msg.Payload,
		TraceID:  strings.TrimSpace(msg.TraceID),
		Metadata: msg.Metadata,
	}, nil
}
