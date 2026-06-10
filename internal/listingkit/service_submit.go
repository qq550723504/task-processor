package listingkit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/listingkit/submission"
	sheinpub "task-processor/internal/publishing/shein"
	sheinother "task-processor/internal/shein/api/other"
	sheinproduct "task-processor/internal/shein/api/product"
)

const sheinSubmitInFlightTTL = submission.InFlightTTL

var (
	errSheinSubmitReplayExisting = errors.New("shein submit replay existing")
	errSheinSubmitRecoverRemote  = errors.New("shein submit recover remote")
	errSheinSubmitMissingPackage = errors.New("shein submit missing package")
)

func (s *service) SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*ListingKitPreview, error) {
	return s.taskSubmissionOrDefault().SubmitTask(ctx, taskID, req)
}

type sheinWorkflowSubmitOptions struct {
	platform  string
	action    string
	requestID string
	startedAt time.Time
}

type sheinDirectSubmitOptions struct {
	action    string
	requestID string
	startedAt time.Time
}

func normalizeSubmitTarget(req *SubmitTaskRequest) (platform string, action string, err error) {
	return normalizeSubmitTargetWithDefault(req, "")
}

func normalizeSubmitTargetWithDefault(req *SubmitTaskRequest, defaultAction string) (platform string, action string, err error) {
	platform = "shein"
	action = "publish"
	if value := strings.ToLower(strings.TrimSpace(defaultAction)); value != "" {
		action = value
	}
	if req != nil {
		if value := strings.ToLower(strings.TrimSpace(req.Platform)); value != "" {
			platform = value
		}
		if value := strings.ToLower(strings.TrimSpace(req.Action)); value != "" {
			action = value
		}
	}
	if platform != "shein" {
		return "", "", fmt.Errorf("%w: %s", ErrUnsupportedSubmitPlatform, platform)
	}
	if !isSupportedSubmitAction(action) {
		return "", "", unsupportedSubmitActionError(action)
	}
	return platform, action, nil
}

func (s *service) acquireSheinSubmitTask(ctx context.Context, taskID, action, requestID string, startedAt time.Time) (*Task, *ListingKitPreview, error) {
	return s.taskSubmissionRecoveryOrDefault().acquireSheinSubmitTask(ctx, taskID, action, requestID, startedAt)
}

func (s *service) shouldStartSheinPublishWorkflow(platform, action string) bool {
	return s != nil &&
		s.sheinPublishWorkflowEnabled &&
		s.sheinPublishWorkflowClient != nil &&
		platform == "shein" &&
		action == "publish"
}

func (s *service) taskSubmissionOrDefault() *taskSubmissionService {
	if s.submission.taskSubmission != nil {
		return s.submission.taskSubmission
	}
	if s.submission.sheinSubmitLocks == nil {
		s.submission.sheinSubmitLocks = submission.NewSubmitLockManager()
	}
	s.submission.taskSubmission = newTaskSubmissionService(buildTaskSubmissionServiceConfig(s))
	return s.submission.taskSubmission
}

func (s *service) taskSubmissionExecutionOrDefault() *taskSubmissionExecutionService {
	if s.submission.taskSubmissionExecution != nil {
		return s.submission.taskSubmissionExecution
	}
	s.submission.taskSubmissionExecution = newTaskSubmissionExecutionService(buildTaskSubmissionExecutionServiceConfig(s))
	return s.submission.taskSubmissionExecution
}

func (s *service) taskSubmissionStateOrDefault() *taskSubmissionStateService {
	if s.submission.taskSubmissionState != nil {
		return s.submission.taskSubmissionState
	}
	s.submission.taskSubmissionState = newTaskSubmissionStateService(taskSubmissionStateServiceConfig{
		repo:                   s.repo,
		rememberSheinSubmitted: s.rememberSheinSubmittedResolution,
	})
	return s.submission.taskSubmissionState
}

func sheinProductAttributesReadyForSubmit(attrs []sheinproduct.ProductAttribute) bool {
	if len(attrs) == 0 {
		return false
	}
	for _, attr := range attrs {
		if attr.AttributeID <= 0 {
			return false
		}
		if attr.AttributeValueID == nil && strings.TrimSpace(attr.AttributeExtraValue) == "" {
			return false
		}
	}
	return true
}

func normalizedSubmitIdempotencyKey(req *SubmitTaskRequest) string {
	if req == nil {
		return ""
	}
	if value := strings.TrimSpace(req.IdempotencyKey); value != "" {
		return value
	}
	return strings.TrimSpace(req.RequestID)
}

func derivedSheinSubmitRequestID(taskID, action string, requestedAt time.Time) string {
	taskID = strings.TrimSpace(taskID)
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "publish"
	}
	timestamp := requestedAt.UTC().Format("20060102T150405.000000000Z")
	if taskID == "" {
		taskID = "unknown-task"
	}
	return fmt.Sprintf("temporal:%s:%s:%s", taskID, action, timestamp)
}

func shouldReplayStartedTemporalSubmit(err error, requestID string) bool {
	var inProgress *submission.SubmitInProgressError
	return errors.As(err, &inProgress) &&
		inProgress != nil &&
		strings.TrimSpace(inProgress.RequestID) != "" &&
		inProgress.RequestID == strings.TrimSpace(requestID)
}

func repairSheinSubmitSaleAttributes(pkg *SheinPackage) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if !sheinSubmitSaleAttributesNeedRepair(pkg) {
		return
	}
	sheinpub.ApplySaleAttributeResolution(pkg, pkg.SaleAttributeResolution)
	preview := sheinpub.BuildPreviewProduct(pkg)
	sheinpub.SetPreviewPayload(pkg, preview)
	sheinpub.NormalizePackageSemanticFields(pkg)
}

func sheinSubmitSaleAttributesNeedRepair(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || pkg.SaleAttributeResolution == nil {
		return false
	}
	if strings.TrimSpace(pkg.SaleAttributeResolution.Status) != "resolved" {
		return false
	}
	if len(pkg.DraftPayload.SKCList) == 0 {
		return false
	}
	if len(pkg.SaleAttributeResolution.SKCAttributes) == 0 && len(pkg.SaleAttributeResolution.SKUAttributes) == 0 {
		return false
	}
	for _, skc := range pkg.DraftPayload.SKCList {
		if skc.SaleAttribute == nil || skc.SaleAttribute.AttributeID <= 0 || skc.SaleAttribute.AttributeValueID == nil || *skc.SaleAttribute.AttributeValueID <= 0 {
			return true
		}
		for _, sku := range skc.SKUList {
			if len(pkg.SaleAttributeResolution.SKUAttributes) == 0 {
				continue
			}
			if len(sku.SaleAttributes) == 0 {
				return true
			}
			for _, attr := range sku.SaleAttributes {
				if attr.AttributeID <= 0 || attr.AttributeValueID == nil || *attr.AttributeValueID <= 0 {
					return true
				}
			}
		}
	}
	return false
}

func (s *service) buildSheinSubmitOtherAPI(ctx context.Context, task *Task) (sheinother.OtherAPI, error) {
	resolver := buildSubmitRuntimeContextResolver(s)
	apiClient, storeID, err := resolver.newAPIClient(ctx, task)
	if err != nil {
		return nil, err
	}
	if !apiClient.HasCookies() {
		if err := apiClient.ForceRefreshCookies(); err != nil {
			return nil, fmt.Errorf("shein other api auth unavailable: %w", err)
		}
	}
	if !apiClient.HasCookies() {
		return nil, fmt.Errorf("shein other api auth unavailable")
	}
	baseAPI := NewSheinRuntimeBaseAPIClient(apiClient, storeID)
	return sheinother.NewClient(baseAPI), nil
}

func isSupportedSubmitAction(action string) bool {
	return action == "publish" || action == "save_draft"
}

func unsupportedSubmitActionError(action string) error {
	return fmt.Errorf("unsupported submit action: %s", action)
}

func sheinSubmitSupplierCode(product *sheinproduct.Product, pkg *SheinPackage) string {
	if product != nil {
		if value := strings.TrimSpace(product.SupplierCode); value != "" {
			return value
		}
		for i := range product.SKCList {
			if product.SKCList[i].SupplierCode == nil {
				continue
			}
			if value := strings.TrimSpace(*product.SKCList[i].SupplierCode); value != "" {
				return value
			}
		}
	}
	if pkg != nil {
		for _, skc := range pkg.SkcList {
			if value := strings.TrimSpace(skc.SupplierCode); value != "" {
				return value
			}
		}
	}
	if product != nil && strings.TrimSpace(product.SPUName) != "" {
		return strings.TrimSpace(product.SPUName)
	}
	return ""
}
