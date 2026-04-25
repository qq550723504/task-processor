package task

import "encoding/json"

type TaskMessage struct {
	TaskID         string          `json:"taskId"`
	Platform       string          `json:"platform"`
	SourcePlatform string          `json:"sourcePlatform"`
	TargetPlatform string          `json:"targetPlatform"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	TraceID        string          `json:"traceId,omitempty"`
	Metadata       map[string]string
}
