package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// StandardProductWorkflowClient starts the standard-product layer as a
// standalone Temporal workflow without forcing platform adaptation to run.
type StandardProductWorkflowClient interface {
	StartStandardProduct(ctx context.Context, in StandardProductWorkflowStartInput) error
}

// PlatformAdaptWorkflowClient starts the platform adaptation layer from a
// persisted standard-product snapshot.
type PlatformAdaptWorkflowClient interface {
	StartPlatformAdaptation(ctx context.Context, in PlatformAdaptWorkflowStartInput) error
}

type StandardProductWorkflowStartInput struct {
	TaskID          string    `json:"task_id"`
	RequestedAt     time.Time `json:"requested_at"`
	TriggeredByUser string    `json:"triggered_by_user,omitempty"`
}

type PlatformAdaptWorkflowStartInput struct {
	TaskID          string    `json:"task_id"`
	Platform        string    `json:"platform"`
	RequestedAt     time.Time `json:"requested_at"`
	TriggeredByUser string    `json:"triggered_by_user,omitempty"`
}

// LayerWorkflowActivityHost exposes the listingkit service methods that
// Temporal activities use for the standard-product and platform-adaptation
// workflow layers.
type LayerWorkflowActivityHost interface {
	ProcessStandardProductLayer(ctx context.Context, taskID string) (*StandardProductSnapshot, error)
	ProcessPlatformAdaptationLayer(ctx context.Context, taskID string, platform string) (*ListingKitResult, error)
}

type LayerWorkflowActivityHostSource interface {
	LayerWorkflowActivityHost
}

func NewLayerWorkflowActivityHost(svc any) (LayerWorkflowActivityHost, error) {
	if svc == nil {
		return nil, fmt.Errorf("listingkit service is nil")
	}
	host, ok := svc.(LayerWorkflowActivityHost)
	if !ok {
		return nil, fmt.Errorf("listingkit service does not implement LayerWorkflowActivityHost")
	}
	return host, nil
}

func normalizeStandardProductWorkflowStartInput(in StandardProductWorkflowStartInput) StandardProductWorkflowStartInput {
	in.TaskID = strings.TrimSpace(in.TaskID)
	in.TriggeredByUser = strings.TrimSpace(in.TriggeredByUser)
	return in
}

func normalizePlatformAdaptWorkflowStartInput(in PlatformAdaptWorkflowStartInput) PlatformAdaptWorkflowStartInput {
	in.TaskID = strings.TrimSpace(in.TaskID)
	in.Platform = strings.ToLower(strings.TrimSpace(in.Platform))
	in.TriggeredByUser = strings.TrimSpace(in.TriggeredByUser)
	if in.Platform == "" {
		in.Platform = "all"
	}
	return in
}

func NormalizeStandardProductWorkflowStartInputForTemporal(in StandardProductWorkflowStartInput) StandardProductWorkflowStartInput {
	return normalizeStandardProductWorkflowStartInput(in)
}

func NormalizePlatformAdaptWorkflowStartInputForTemporal(in PlatformAdaptWorkflowStartInput) PlatformAdaptWorkflowStartInput {
	return normalizePlatformAdaptWorkflowStartInput(in)
}
