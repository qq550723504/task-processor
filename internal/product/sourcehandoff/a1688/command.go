package a1688

import (
	"context"
	"fmt"
	"strings"

	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/listingkit"
	"task-processor/internal/product/sourcehandoff"
	"task-processor/internal/product/sourcing"
)

// CreateTaskCommand is the application-facing command shape for turning one
// already-fetched 1688 product into a ListingKit task through the existing
// CreateGenerateTask boundary.
type CreateTaskCommand struct {
	URL         string
	Product     *alibaba1688model.Product1688
	RawSnapshot string
	SourceRunID string
	RequestID   string
	Error       error

	TenantID           string
	UserID             string
	Platforms          []string
	Country            string
	Language           string
	SheinStoreID       int64
	TargetCategoryHint string
	Options            *listingkit.GenerateOptions
}

// CreateTaskResult exposes the created task plus the prepared handoff details so
// callers can inspect source identity, warnings, and generated request details.
type CreateTaskResult struct {
	Task    *listingkit.Task
	Handoff *ListingKitTaskHandoff
}

// TaskCommandService is the narrow application service for 1688 -> ListingKit
// task creation. It depends only on the existing ListingKit task creator
// boundary and does not fetch, crawl, or submit marketplace payloads.
type TaskCommandService struct {
	creator sourcehandoff.GenerateTaskCreator
}

func NewTaskCommandService(creator sourcehandoff.GenerateTaskCreator) *TaskCommandService {
	return &TaskCommandService{creator: creator}
}

// CreateTask prepares a 1688 source envelope and delegates to the existing
// ListingKit task create boundary. The command expects caller-owned crawler data;
// URL-only crawling is intentionally outside this service.
func (s *TaskCommandService) CreateTask(ctx context.Context, command CreateTaskCommand) (*CreateTaskResult, error) {
	if s == nil || s.creator == nil {
		return nil, fmt.Errorf("listingkit generate task creator is required")
	}
	url := strings.TrimSpace(command.URL)
	if url == "" && command.Product != nil {
		url = strings.TrimSpace(command.Product.URL)
	}
	if url == "" {
		return nil, fmt.Errorf("1688 source url is required")
	}

	task, handoff, err := CreateListingKitTask(ctx, s.creator, ListingKitTaskInput{
		Source: sourcing.Alibaba1688SourceEnvelopeInput{
			Request:     sourcing.Alibaba1688CrawlRequestInput{URL: url, StoreID: command.SheinStoreID},
			Product:     command.Product,
			RawSnapshot: command.RawSnapshot,
			SourceRunID: command.SourceRunID,
			RequestID:   command.RequestID,
			Error:       command.Error,
		},
		TenantID:           command.TenantID,
		UserID:             command.UserID,
		Platforms:          command.Platforms,
		Country:            command.Country,
		Language:           command.Language,
		SheinStoreID:       command.SheinStoreID,
		TargetCategoryHint: command.TargetCategoryHint,
		Options:            command.Options,
	})
	if err != nil {
		return &CreateTaskResult{Handoff: handoff}, err
	}
	return &CreateTaskResult{Task: task, Handoff: handoff}, nil
}
