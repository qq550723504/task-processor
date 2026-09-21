package imageagentworker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	goopenai "github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"task-processor/internal/aicapability"
	"task-processor/internal/authidentity"
	openai "task-processor/internal/integration/openai"
	productimage "task-processor/internal/product/image"
	"task-processor/internal/shared/aiidentity"
)

// ReviewPricing is an explicitly supplied conservative authorization bound,
// never evidence of actual provider cost. Nil keeps the existing unpriced route.
type ReviewPricing struct {
	Version           string
	MaximumCostMicros int64
}
type OrganizationReviewOptions struct {
	Recorder aicapability.InvocationRecorder
	Logger   *logrus.Logger
	Pricing  *ReviewPricing
}

func (p *routedOpenAIProductImageProvider) recordedReview(ctx context.Context, request productimage.ReviewRequest) (productimage.Review, error) {
	identity := aiidentity.FromContext(ctx)
	verified, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	settings := p.reviewGovernance
	if !ok || verified.EffectiveOrganizationID == "" || verified.EffectiveOrganizationID != identity.TenantID || verified.TenantID != identity.TenantID || verified.UserID != identity.UserID || verified.EffectiveMemberID == "" || identity.AgentRunID == "" || identity.BusinessTaskID == "" || nilDependency(settings.Recorder) || settings.Logger == nil || request.Authorization == nil {
		return productimage.Review{}, productimage.ErrInputInvalid
	}
	if err := ctx.Err(); err != nil {
		return productimage.Review{}, err
	}
	quote, err := productimage.NormalizeUsageAuthorization(*request.Authorization, "review")
	if err != nil {
		return productimage.Review{}, err
	}
	known, maximumCost := settings.Pricing != nil, int64(0)
	if known {
		maximumCost = settings.Pricing.MaximumCostMicros
	}
	if quote.MaximumModelCalls != 1 || quote.CostUpperBoundKnown != known || quote.MaximumCostMicros != maximumCost {
		return productimage.Review{}, productimage.ErrCapabilityUnsupported
	}
	if quote.MaximumTokens <= 0 {
		return productimage.Review{}, productimage.ErrCapabilityUnsupported
	}
	invocationID, inputHash := stableReviewInvocationIdentity(identity, request, quote.Fingerprint)
	reservation, ok := settings.Recorder.(aicapability.InvocationUsageReservation)
	if !ok {
		return productimage.Review{}, productimage.ErrExternalCapabilityUnavailable
	}
	lookup, hasLookup := settings.Recorder.(aicapability.InvocationReplayReader)
	if hasLookup {
		existing, found, lookupErr := lookup.FindInvocation(ctx, identity.TenantID, verified.EffectiveMemberID, invocationID, inputHash)
		if lookupErr != nil {
			return productimage.Review{}, lookupErr
		}
		if found && existing.Outcome == aicapability.InvocationSucceeded {
			replayCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
			replayErr := settings.Recorder.RecordInvocation(replayCtx, existing)
			cancel()
			if replayErr != nil {
				return productimage.Review{}, fmt.Errorf("image review usage settlement failed: %w", replayErr)
			}
			return productimage.Review{Score: existing.ReviewScore, NeedsHumanReview: existing.ReviewNeedsHumanReview, Reasons: append([]string(nil), existing.ReviewReasons...)}, nil
		}
		if found {
			return productimage.Review{}, fmt.Errorf("image review invocation is already durably dispatched and cannot be replayed safely: %w", productimage.ErrExternalCapabilityUnavailable)
		}
	}
	started := time.Now().UTC()
	if err := reservation.ReserveAIInvocationUsage(ctx, identity.TenantID, verified.EffectiveMemberID, invocationID, quote.MaximumTokens, started); err != nil {
		return productimage.Review{}, err
	}
	reservationHeld := true
	defer func() {
		if reservationHeld {
			_ = reservation.ReleaseAIInvocationUsage(context.WithoutCancel(ctx), identity.TenantID, invocationID)
		}
	}()
	record := aicapability.InvocationRecord{
		InvocationID: invocationID, AgentRunID: identity.AgentRunID, TenantID: identity.TenantID, UserID: identity.UserID, MemberID: verified.EffectiveMemberID,
		BusinessTaskID: identity.BusinessTaskID, TraceID: identity.TraceID,
		Capability: aicapability.CapabilityProductImageScene, Operation: aicapability.OperationProductImageReview,
		RouteOutcome: aicapability.RouteOutcomeActive, ProviderID: quote.Provider, ModelID: quote.Model,
		RoutingKey: quote.RouteReference, CredentialReference: quote.CredentialReference, ConfigurationVersion: quote.ConfigurationVersion,
		PromptKey: "product-image-review", StartedAt: started, Attempt: 1, Outcome: aicapability.InvocationDispatched, InputHash: inputHash,
	}
	recordCtx, recordCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	if err := settings.Recorder.RecordInvocation(recordCtx, record); err != nil {
		recordCancel()
		return productimage.Review{}, fmt.Errorf("image review dispatch boundary failed: %w", err)
	}
	recordCancel()
	// The durable dispatched row now owns the reservation. A retry must observe
	// that row and fail closed instead of issuing another provider request.
	reservationHeld = false
	var observation openai.ProductImageReviewObservation
	observed := false
	zero := 0
	adapter, err := p.adapter(ctx, "review", &quote, func(config *openai.ProductImageAdapterConfig) {
		config.ReviewMaxRetries = &zero
		config.ReviewObserver = func(value openai.ProductImageReviewObservation) { observation, observed = value, true }
	})
	if err != nil {
		record.FinishedAt = time.Now().UTC()
		record.Outcome, record.ErrorCategory, record.ErrorCode = aicapability.InvocationFailed, reviewProviderErrorCategory(err), "review_adapter_failed"
		failureCtx, failureCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		failureErr := settings.Recorder.RecordInvocation(failureCtx, record)
		failureCancel()
		if failureErr != nil {
			return productimage.Review{}, fmt.Errorf("image review adapter failure recording failed: %w", failureErr)
		}
		return productimage.Review{}, err
	}
	result, providerErr := adapter.Review(ctx, request)
	if providerErr == nil {
		result, providerErr = productimage.ValidateReview(result)
	}
	if !observed {
		if providerErr == nil {
			return result, fmt.Errorf("image review provider result was not observed: %w", productimage.ErrExternalCapabilityUnavailable)
		}
		record.FinishedAt = time.Now().UTC()
		record.Outcome, record.ErrorCategory, record.ErrorCode = aicapability.InvocationFailed, reviewProviderErrorCategory(providerErr), "review_provider_failed"
		failureCtx, failureCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		failureErr := settings.Recorder.RecordInvocation(failureCtx, record)
		failureCancel()
		if failureErr != nil {
			return result, fmt.Errorf("image review provider failure recording failed: %w", failureErr)
		}
		return result, providerErr
	}
	finished := time.Now().UTC()
	record.FinishedAt, record.Outcome = finished, aicapability.InvocationSucceeded
	record.PromptVersion, record.PromptHash = observation.PromptVersion, observation.PromptHash
	record.ProviderRequestID = safeReviewReference(observation.ProviderRequestID)
	record.ReviewScore, record.ReviewNeedsHumanReview, record.ReviewReasons = result.Score, result.NeedsHumanReview, append([]string(nil), result.Reasons...)
	usage := observation.Usage
	// The current wire contract has no presence bit. Zero/missing usage stays unknown.
	if usage.PromptTokens >= 0 && usage.CompletionTokens >= 0 && usage.TotalTokens > 0 && usage.PromptTokens+usage.CompletionTokens == usage.TotalTokens {
		record.UsageKnown = true
		record.PromptTokens, record.CompletionTokens, record.TotalTokens = usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens
	}
	if providerErr != nil {
		record.Outcome, record.ErrorCategory, record.ErrorCode = aicapability.InvocationFailed, reviewProviderErrorCategory(providerErr), "review_provider_failed"
		if errors.Is(providerErr, context.Canceled) {
			record.ErrorCode = "canceled"
		}
		if errors.Is(providerErr, context.DeadlineExceeded) {
			record.ErrorCategory, record.ErrorCode = aicapability.ErrorProviderTimeout, "deadline_exceeded"
		}
		if errors.Is(providerErr, productimage.ErrOutputValidation) {
			record.ErrorCategory, record.ErrorCode = aicapability.ErrorInvalidProviderResponse, "invalid_review_output"
		}
	}
	// No retry and no provider re-execution: cancellation cannot erase a completed call fact.
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	if err := settings.Recorder.RecordInvocation(recordCtx, record); err != nil {
		settings.Logger.WithFields(logrus.Fields{"event": "image_review_record_degraded", "invocation_id": record.InvocationID, "outcome": record.Outcome, "record_status": "failed"}).Error("image review invocation recording failed")
		if providerErr == nil {
			return productimage.Review{}, fmt.Errorf("image review usage settlement failed: %w", err)
		}
	} else {
		reservationHeld = false
	}
	return result, providerErr
}

func stableReviewInvocationIdentity(identity aiidentity.Identity, request productimage.ReviewRequest, quoteFingerprint string) (string, string) {
	parts := []string{identity.TenantID, identity.AgentRunID, identity.BusinessTaskID, quoteFingerprint, request.Product.ProductKey}
	for _, asset := range request.Sources {
		parts = append(parts, "source", asset.SourceAssetID, asset.URL)
	}
	for _, candidate := range request.Candidates {
		parts = append(parts, "candidate", candidate.Asset.SourceAssetID, candidate.Asset.URL)
	}
	payload := strings.Join(parts, "\x00")
	digest := sha256.Sum256([]byte(payload))
	hash := hex.EncodeToString(digest[:])
	// uuid.NewSHA1 gives the existing invocation schema a stable UUID while
	// retaining a deterministic identity across Temporal activity retries.
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("account-center-review:"+hash)).String(), hash
}

func reviewProviderErrorCategory(err error) aicapability.ErrorCategory {
	status := reviewProviderHTTPStatus(err)
	switch {
	case status == http.StatusTooManyRequests:
		return aicapability.ErrorRateLimited
	case status >= http.StatusBadRequest && status < http.StatusInternalServerError:
		return aicapability.ErrorProviderRejected
	default:
		return aicapability.ErrorProviderUnavailable
	}
}

func reviewProviderHTTPStatus(err error) int {
	var apiErr *goopenai.APIError
	if errors.As(err, &apiErr) && apiErr != nil {
		return apiErr.HTTPStatusCode
	}
	var requestErr *goopenai.RequestError
	if errors.As(err, &requestErr) && requestErr != nil {
		return requestErr.HTTPStatusCode
	}
	return 0
}

func safeReviewReference(value string) string {
	if len(value) > 128 {
		return ""
	}
	for _, char := range value {
		if !(unicode.IsLetter(char) || unicode.IsDigit(char) || char == '-' || char == '_' || char == '.') {
			return ""
		}
	}
	return value
}

// Resolved clients and their pools share this sink. Never forward provider
// error text, configuration values or arbitrary SDK fields into application logs.
type safeReviewTransportLogger struct{ logger *logrus.Logger }

func (l safeReviewTransportLogger) Debug(string, map[string]any) {}
func (l safeReviewTransportLogger) Info(string, map[string]any)  {}
func (l safeReviewTransportLogger) Warn(string, map[string]any) {
	l.logger.WithField("event", "organization_provider_transport_warning").Warn("organization provider transport warning")
}
func (l safeReviewTransportLogger) Error(string, map[string]any) {
	l.logger.WithField("event", "organization_provider_transport_error").Error("organization provider transport error")
}
