package a1688

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/authidentity"
	"task-processor/internal/compatibility/listingkit/sourcehandoff"
	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
	crawler1688 "task-processor/internal/integration/crawler/a1688"
	"task-processor/internal/listingkit"
	"task-processor/internal/sourceaccount"
	"task-processor/internal/tenantbridge"
)

// CreateTaskCommand is the application-facing command shape for turning one
// already-fetched 1688 product into a ListingKit task through the existing
// CreateGenerateTask boundary.
type CreateTaskCommand struct {
	URL             string
	Product         *alibaba1688model.Product1688
	RawSnapshot     string
	SourceRunID     string
	RequestID       string
	Error           error
	SourceAccountID int64

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
	creator                      sourcehandoff.GenerateTaskCreator
	storeAccessValidator         listingkit.StoreAccessValidator
	sourceAccountAccessValidator sourceaccount.AccessValidator
}

func NewTaskCommandService(creator sourcehandoff.GenerateTaskCreator, dependencies ...any) *TaskCommandService {
	service := &TaskCommandService{creator: creator}
	for _, dependency := range dependencies {
		if value, ok := dependency.(listingkit.StoreAccessValidator); ok {
			service.storeAccessValidator = value
		}
		if value, ok := dependency.(sourceaccount.AccessValidator); ok {
			service.sourceAccountAccessValidator = value
		}
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
	if command.SourceAccountID < 0 {
		return nil, fmt.Errorf("invalid source_account_id: must not be negative")
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
		Source: crawler1688.Alibaba1688SourceEnvelopeInput{
			Request:     crawler1688.Alibaba1688CrawlRequestInput{URL: url, AccountID: command.SourceAccountID},
			Product:     crawler1688.SnapshotFromLegacyProduct(command.Product),
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
	identity, _ := authidentity.AuthenticatedIdentityFromContext(ctx)
	tenantID := strings.TrimSpace(identity.TenantID)
	legacyTenantID, err := tenantbridge.ResolveLegacyTenantID(ctx, tenantID)
	if err != nil || legacyTenantID <= 0 {
		return listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable")
	}
	if command.SheinStoreID <= 0 {
		return listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable")
	}
	if _, err := s.storeAccessValidator.ValidateStoreAccess(ctx, legacyTenantID, command.SheinStoreID, "SHEIN"); err != nil {
		return err
	}
	if command.SourceAccountID < 0 {
		return fmt.Errorf("invalid source_account_id: must not be negative")
	}
	if command.SourceAccountID > 0 {
		if s.sourceAccountAccessValidator == nil {
			return listingkit.NewStoreAccessError(sourceaccount.SourceAccountUnavailable, "source account is unavailable")
		}
		if _, err := s.sourceAccountAccessValidator.ValidateSourceAccountAccess(ctx, legacyTenantID, command.SourceAccountID); err != nil {
			if code := sourceaccount.ErrorCode(err); code != "" {
				message := "source account is unavailable"
				if code == sourceaccount.SourceAccountDisabled {
					message = "source account is disabled"
				}
				return listingkit.NewStoreAccessError(code, message)
			}
			return err
		}
	}
	return nil
}

func validateRequestIdentity(ctx context.Context, command CreateTaskCommand) error {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	tenantID := strings.TrimSpace(identity.TenantID)
	userID := strings.TrimSpace(identity.UserID)
	if !ok || tenantID == "" || userID == "" ||
		strings.TrimSpace(command.TenantID) != tenantID || strings.TrimSpace(command.UserID) != userID {
		return fmt.Errorf("verified request identity is required")
	}
	return nil
}
