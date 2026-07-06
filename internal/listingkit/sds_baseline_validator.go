package listingkit

import (
	"context"
	"strings"
	"time"

	sdsdesign "task-processor/internal/sds/design"
	sdstemplate "task-processor/internal/sds/template"
	"task-processor/internal/sdslogin"
)

type SDSLoginStatusProvider interface {
	Status(ctx context.Context) (*sdslogin.Status, error)
}

type SDSBaselineRemoteProvider interface {
	GetProductDetail(ctx context.Context, parentProductID int64) (*sdstemplate.ProductDetail, error)
	GetDesignProduct(ctx context.Context, variantID int64) (*sdsdesign.DesignProductPage, error)
	GetPrototypeGroups(ctx context.Context, parentProductID int64) ([]sdsdesign.PrototypeGroup, error)
}

type sdsBaselinePrototypeGroupDesignProvider interface {
	GetDesignProductForPrototypeGroup(ctx context.Context, variantID, prototypeGroupID int64) (*sdsdesign.DesignProductPage, error)
}

type sdsBaselineValidationResult struct {
	Status     string
	ReasonCode string
	Reason     string
}

func (s *service) validateSDSBaseline(ctx context.Context, options *SDSSyncOptions) sdsBaselineValidationResult {
	if result := validateSDSBaselineRequiredFields(options); result.Status != "" {
		return result
	}
	loginStatusProvider := resolveSDSLoginStatusProvider(s)
	if s == nil || loginStatusProvider == nil {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusUnknown,
			ReasonCode: SDSBaselineReasonCodeLoginUnavailable,
			Reason:     "SDS login validation is unavailable.",
		}
	}
	status, err := loginStatusProvider.Status(ctx)
	if err != nil {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusFailed,
			ReasonCode: SDSBaselineReasonCodeLoginStatusCheckFailed,
			Reason:     "SDS login status check failed: " + err.Error(),
		}
	}
	if status == nil {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusUnknown,
			ReasonCode: SDSBaselineReasonCodeLoginUnavailable,
			Reason:     "SDS login status is unavailable.",
		}
	}
	if status.LoginInProgress {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeLoginInProgress,
			Reason:     "SDS login is still in progress.",
		}
	}
	if strings.TrimSpace(status.LastError) != "" {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeLoginUnavailable,
			Reason:     "SDS login state is unavailable: " + strings.TrimSpace(status.LastError),
		}
	}
	if !status.HasAccessToken {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeLoginMissingCredentials,
			Reason:     "SDS login state is missing access token.",
		}
	}
	if resolveSDSBaselineRemoteProvider(s) != nil {
		if result := s.validateSDSBaselineRemote(ctx, options); result.Status != SDSBaselineValidationStatusReady {
			return result
		}
	}
	return sdsBaselineValidationResult{
		Status: SDSBaselineValidationStatusReady,
	}
}

func validateSDSBaselineRequiredFields(options *SDSSyncOptions) sdsBaselineValidationResult {
	if options == nil {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeMissingOptions,
			Reason:     "SDS baseline options are missing.",
		}
	}
	if options.ParentProductID <= 0 {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeMissingParentProduct,
			Reason:     "SDS parent product is missing.",
		}
	}
	if options.PrototypeGroupID <= 0 {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeMissingPrototypeGroup,
			Reason:     "SDS prototype group is missing.",
		}
	}
	if options.VariantID <= 0 {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeMissingVariant,
			Reason:     "SDS variant is missing.",
		}
	}
	if strings.TrimSpace(options.DesignType) == "" {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeMissingDesignType,
			Reason:     "SDS design type is missing.",
		}
	}
	if options.PrintableWidth <= 0 || options.PrintableHeight <= 0 {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeMissingPrintableSize,
			Reason:     "SDS printable size is incomplete.",
		}
	}
	if strings.TrimSpace(options.LayerID) == "" {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeMissingLayer,
			Reason:     "SDS layer selection is missing.",
		}
	}
	return sdsBaselineValidationResult{}
}

func (s *service) persistSDSBaselineValidation(ctx context.Context, task *Task) error {
	cacheRepo, ok := s.repo.(SDSBaselineCacheRepository)
	if !ok || task == nil || task.Request == nil || task.Request.Options == nil || task.Request.Options.SDS == nil {
		return nil
	}
	tenantID := strings.TrimSpace(task.Request.TenantID)
	if tenantID == "" {
		tenantID = strings.TrimSpace(task.TenantID)
	}
	baselineKey := sdsBaselineKey(tenantID, task.Request.Options.SDS)
	if baselineKey == "" {
		return nil
	}
	entry, err := cacheRepo.GetSDSBaselineCache(ctx, tenantID, baselineKey)
	if err != nil || entry == nil {
		return err
	}
	result := s.validateSDSBaseline(ctx, task.Request.Options.SDS)
	entry.ValidationStatus = result.Status
	entry.ValidationReasonCode = result.ReasonCode
	entry.ValidationReason = result.Reason
	now := time.Now().UTC()
	entry.ValidatedAt = &now
	return cacheRepo.SaveSDSBaselineCache(ctx, entry)
}

func (s *service) validateSDSBaselineRemote(ctx context.Context, options *SDSSyncOptions) sdsBaselineValidationResult {
	remoteProvider := resolveSDSBaselineRemoteProvider(s)
	if s == nil || remoteProvider == nil || options == nil {
		return sdsBaselineValidationResult{Status: SDSBaselineValidationStatusUnknown}
	}
	detail, err := remoteProvider.GetProductDetail(ctx, options.ParentProductID)
	if err != nil {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusFailed,
			ReasonCode: SDSBaselineReasonCodeProductDetailCheckFailed,
			Reason:     "SDS product detail check failed: " + err.Error(),
		}
	}
	if detail == nil {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeProductDetailUnavailable,
			Reason:     "SDS product detail is unavailable.",
		}
	}
	page, err := getSDSBaselineDesignProduct(ctx, remoteProvider, options)
	if err != nil {
		if isSDSBaselineCredentialBootstrapError(err) {
			return sdsBaselineValidationResult{
				Status:     SDSBaselineValidationStatusBlocked,
				ReasonCode: SDSBaselineReasonCodeLoginMissingCredentials,
				Reason:     "SDS credential bootstrap is missing merchant credentials.",
			}
		}
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusFailed,
			ReasonCode: SDSBaselineReasonCodeDesignSurfaceCheckFailed,
			Reason:     "SDS design surface check failed: " + err.Error(),
		}
	}
	if page == nil {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeDesignSurfaceUnavailable,
			Reason:     "SDS design surface is unavailable.",
		}
	}
	if page.Product.ID > 0 && page.Product.ID != options.VariantID {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeVariantMismatch,
			Reason:     "SDS design surface resolved to a different variant.",
		}
	}
	if options.PrototypeGroupID > 0 && page.PrototypeGroup.ID > 0 && page.PrototypeGroup.ID != options.PrototypeGroupID {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodePrototypeGroupMismatch,
			Reason:     "SDS design surface resolved to a different prototype group.",
		}
	}
	if strings.TrimSpace(options.LayerID) != "" && !sdsBaselineLayerExists(page, options.LayerID) {
		return sdsBaselineValidationResult{
			Status:     SDSBaselineValidationStatusBlocked,
			ReasonCode: SDSBaselineReasonCodeLayerMissing,
			Reason:     "SDS design surface does not include the selected layer.",
		}
	}
	if options.ParentProductID > 0 {
		groups, groupErr := remoteProvider.GetPrototypeGroups(ctx, options.ParentProductID)
		if groupErr != nil {
			return sdsBaselineValidationResult{
				Status:     SDSBaselineValidationStatusFailed,
				ReasonCode: SDSBaselineReasonCodePrototypeGroupCheckFailed,
				Reason:     "SDS prototype group check failed: " + groupErr.Error(),
			}
		}
		if options.PrototypeGroupID > 0 && !sdsBaselinePrototypeGroupExists(groups, options.PrototypeGroupID) {
			return sdsBaselineValidationResult{
				Status:     SDSBaselineValidationStatusBlocked,
				ReasonCode: SDSBaselineReasonCodePrototypeGroupUnavailable,
				Reason:     "SDS parent product does not expose the selected prototype group.",
			}
		}
	}
	return sdsBaselineValidationResult{Status: SDSBaselineValidationStatusReady}
}

func getSDSBaselineDesignProduct(ctx context.Context, remoteProvider SDSBaselineRemoteProvider, options *SDSSyncOptions) (*sdsdesign.DesignProductPage, error) {
	if options != nil && options.PrototypeGroupID > 0 {
		if groupProvider, ok := remoteProvider.(sdsBaselinePrototypeGroupDesignProvider); ok {
			return groupProvider.GetDesignProductForPrototypeGroup(ctx, options.VariantID, options.PrototypeGroupID)
		}
	}
	return remoteProvider.GetDesignProduct(ctx, options.VariantID)
}

func isSDSBaselineCredentialBootstrapError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "merchant_name, username and password are required")
}

func sdsBaselineLayerExists(page *sdsdesign.DesignProductPage, layerID string) bool {
	if page == nil || strings.TrimSpace(layerID) == "" {
		return false
	}
	for _, layer := range page.Layers {
		if string(layer.ID) == strings.TrimSpace(layerID) {
			return true
		}
	}
	return false
}

func sdsBaselinePrototypeGroupExists(groups []sdsdesign.PrototypeGroup, prototypeGroupID int64) bool {
	if prototypeGroupID <= 0 {
		return false
	}
	for _, group := range groups {
		if group.ID == prototypeGroupID {
			return true
		}
	}
	return false
}
