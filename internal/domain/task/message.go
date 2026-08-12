package task

import "encoding/json"

const TaskEventSchemaVersionV2 = 2

// TaskEventV2 is the versioned RabbitMQ event for complete task payloads.
// Listing Control dispatch signals carry only an ID and intentionally do not
// use this contract.
type TaskEventV2 struct {
	SchemaVersion  int               `json:"schemaVersion"`
	TaskID         string            `json:"taskId"`
	SourcePlatform SourcePlatform    `json:"sourcePlatform"`
	TargetPlatform TargetPlatform    `json:"targetPlatform"`
	Payload        json.RawMessage   `json:"payload"`
	TraceID        string            `json:"traceId,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}
