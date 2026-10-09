package imageagent

import (
	"context"
	"time"
)

type ImageSetRunSummary struct {
	RunID          string                 `json:"runId"`
	ContextKind    ImageSourceContextKind `json:"contextKind"`
	ContextID      string                 `json:"contextId"`
	Status         RunStatus              `json:"status"`
	TargetPlatform string                 `json:"targetPlatform"`
	CreatedAt      time.Time              `json:"createdAt"`
}
type ImageSetRunReader interface {
	ListImageSets(context.Context, ExecutionIdentity, string, string, int) ([]ImageSetRunSummary, string, error)
}
