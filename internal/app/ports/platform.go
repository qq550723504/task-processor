package ports

import (
	"context"

	"task-processor/internal/model"
)

// ProductSource defines the low-level crawl-source capability used by product fetchers.
type ProductSource interface {
	Process(url, zipcode string) (*model.Product, error)
	ProcessWithContext(ctx context.Context, url, zipcode string) (*model.Product, error)
	Shutdown()
}

// CrawlSource is the preferred neutral name for product crawl capabilities.
// Keep ProductSource as the compatibility name while the rest of the codebase is migrated.
type CrawlSource = ProductSource

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
