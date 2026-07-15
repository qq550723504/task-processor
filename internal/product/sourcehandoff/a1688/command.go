package a1688

import (
	"context"
	"fmt"
	"strings"

	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/listingkit"
	"task-processor/internal/product/sourcehandoff"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/tenantbridge"
)

// CreateTaskCommand is the application-facing command shape for turning one
// already-fetched 1688 product into a ListingKit task through the existing
// CreateGenerateTask boundary.
type CreateTaskCommand struct {
	URL           string
	Product       *alibaba1688model.Product1688
	RawSnapshot   string
	SourceRunID   string
	RequestID     string
	Error         error
	SourceStoreID int64

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
	creator              sourcehandoff.GenerateTaskCreator
	storeAccessValidator listingkit.StoreAccessValidator
}

func NewTaskCommandService(creator sourcehandoff.GenerateTaskCreator, validators ...listingkit.StoreAccessValidator) *TaskCommandService {
	service := &TaskCommandService{creator: creator}
	if len(validators) > 0 {
		service.storeAccessValidator = validators[0]
	}
	return service
}

// CreateTask prepares a 1688 source envelope and delegates to the existing
// ListingKit task create boundary. The command expects caller-owned crawler data;
// URL-only crawling is intentionally outside this service.
func (s *TaskCommandService) CreateTask(ctx context.Context, command CreateTaskCommand) (*CreateTaskResult, error) {
	if s == nil || s.creator == nil {
		return nil, fmt.Errorf("listingkit generate task creator is required")
	}
	if err := validateRequestIdentity(ctx, command); err != nil {
		return nil, err
	}
	if err := s.validateStores(ctx, command); err != nil {
		return nil, err
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
			Request:     sourcing.Alibaba1688CrawlRequestInput{URL: url, StoreID: command.SourceStoreID},
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

func (s *TaskCommandService) validateStores(ctx context.Context, command CreateTaskCommand) error {
	if s.storeAccessValidator == nil {
		return listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable")
	}
	tenantID := strings.TrimSpace(listingkit.TenantIDFromContext(ctx))
	legacyTenantID, err := tenantbridge.ResolveLegacyTenantID(ctx, tenantID)
	if err != nil || legacyTenantID <= 0 {
		return listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable")
	}
	for _, item := range []struct {
		id       int64
		platform string
	}{{command.SourceStoreID, "1688"}, {command.SheinStoreID, "SHEIN"}} {
		if item.id <= 0 {
			return listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable")
		}
		if _, err := s.storeAccessValidator.ValidateStoreAccess(ctx, legacyTenantID, item.id, item.platform); err != nil {
			return err
		}
	}
	return nil
}

func validateRequestIdentity(ctx context.Context, command CreateTaskCommand) error {
	tenantID := strings.TrimSpace(listingkit.TenantIDFromContext(ctx))
	identity := listingkit.RequestIdentityFromContext(ctx)
	if tenantID == "" || identity.TenantID != tenantID || identity.UserID == "" ||
		strings.TrimSpace(command.TenantID) != tenantID || strings.TrimSpace(command.UserID) != identity.UserID {
		return fmt.Errorf("verified request identity is required")
	}
	return nil
}
