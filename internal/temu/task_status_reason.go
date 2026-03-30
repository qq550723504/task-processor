package temu

import "errors"

const (
	temuTaskReasonDraftSaved         = "DRAFT_SAVED"
	temuTaskReasonAuthExpired        = "AUTH_EXPIRED"
	temuTaskReasonDuplicateProduct   = "DUPLICATE_PRODUCT"
	temuTaskReasonProductNotFound    = "PRODUCT_NOT_FOUND"
	temuTaskReasonProductOffline     = "PRODUCT_OFFLINE"
	temuTaskReasonInvalidASIN        = "INVALID_ASIN"
	temuTaskReasonTooManyVariants    = "TOO_MANY_VARIANTS"
	temuTaskReasonPageNotFound       = "PAGE_NOT_FOUND"
	temuTaskReasonPageElementsMissed = "PAGE_ELEMENTS_MISSING"
	temuTaskReasonRetryableFailure   = "RETRYABLE_FAILURE"
	temuTaskReasonNonRetryable       = "NON_RETRYABLE_FAILURE"

	temuTaskStageInitStore      = "init_store"
	temuTaskStageSaveDraft      = "save_draft"
	temuTaskStagePublishProduct = "publish_product"
)

func classifyTaskError(err error) (reasonCode string, stage string) {
	if err == nil {
		return "", temuTaskStagePublishProduct
	}

	stage = temuTaskStagePublishProduct
	switch {
	case IsAuthExpiredError(err):
		return temuTaskReasonAuthExpired, temuTaskStageInitStore
	case errors.Is(err, ErrDuplicateProduct):
		return temuTaskReasonDuplicateProduct, stage
	case errors.Is(err, ErrProductNotFound):
		return temuTaskReasonProductNotFound, stage
	case errors.Is(err, ErrProductOffline):
		return temuTaskReasonProductOffline, stage
	case errors.Is(err, ErrInvalidASIN):
		return temuTaskReasonInvalidASIN, stage
	case errors.Is(err, ErrTooManyVariants):
		return temuTaskReasonTooManyVariants, stage
	case errors.Is(err, ErrPageNotFound):
		return temuTaskReasonPageNotFound, stage
	case errors.Is(err, ErrMissingPageElements):
		return temuTaskReasonPageElementsMissed, stage
	case IsRetryableError(err):
		return temuTaskReasonRetryableFailure, stage
	default:
		return temuTaskReasonNonRetryable, stage
	}
}
