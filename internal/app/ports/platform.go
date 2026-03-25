package ports

import (
	"context"

	"task-processor/internal/model"
)

// ProductSource defines the source-platform capability required by target platforms.
type ProductSource interface {
	Process(url, zipcode string) (*model.Product, error)
	ProcessWithContext(ctx context.Context, url, zipcode string) (*model.Product, error)
	Shutdown()
}

// TaskPublisher defines the ability to publish task payloads asynchronously.
type TaskPublisher interface {
	Publish(ctx context.Context, topic string, payload []byte) error
}

// TaskReporter defines the ability to report task results back to an upstream system.
type TaskReporter interface {
	ReportResult(ctx context.Context, taskID int64, status string, payload any) error
}

// PromptRegistry defines prompt initialization and lookup behavior required by app services.
type PromptRegistry interface {
	Initialize(ctx context.Context, dir string, hotReload bool) error
}
