package task

import "encoding/json"

type PlatformRoute struct {
	Source SourcePlatform
	Target TargetPlatform
}

type NormalizedTask struct {
	ID       string
	Route    PlatformRoute
	Payload  json.RawMessage
	TraceID  string
	Metadata map[string]string
}
