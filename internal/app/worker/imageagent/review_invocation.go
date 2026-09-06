package imageagentworker

import (
	"context"
	"errors"
	"time"
	"unicode"

	"github.com/google/uuid"
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
	if !ok || verified.EffectiveOrganizationID == "" || verified.EffectiveOrganizationID != identity.TenantID || verified.TenantID != identity.TenantID || verified.UserID != identity.UserID || identity.AgentRunID == "" || identity.BusinessTaskID == "" || nilDependency(settings.Recorder) || settings.Logger == nil || request.Authorization == nil {
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
	var observation openai.ProductImageReviewObservation
	observed := false
	zero := 0
	adapter, err := p.adapter(ctx, "review", &quote, func(config *openai.ProductImageAdapterConfig) {
		config.ReviewMaxRetries = &zero
		config.ReviewObserver = func(value openai.ProductImageReviewObservation) { observation, observed = value, true }
	})
	if err != nil {
		return productimage.Review{}, err
	}
	started := time.Now().UTC()
	result, providerErr := adapter.Review(ctx, request)
	if providerErr == nil {
		result, providerErr = productimage.ValidateReview(result)
	}
	if !observed {
		return result, providerErr
	}
	finished := time.Now().UTC()
	record := aicapability.InvocationRecord{
		InvocationID: uuid.NewString(), AgentRunID: identity.AgentRunID, TenantID: identity.TenantID, UserID: identity.UserID,
		BusinessTaskID: identity.BusinessTaskID, TraceID: identity.TraceID,
		Capability: aicapability.CapabilityProductImageScene, Operation: aicapability.OperationProductImageReview,
		RouteOutcome: aicapability.RouteOutcomeActive, ProviderID: quote.Provider, ModelID: quote.Model,
		RoutingKey: quote.RouteReference, CredentialReference: quote.CredentialReference, ConfigurationVersion: quote.ConfigurationVersion,
		PromptKey: "product-image-review", PromptVersion: observation.PromptVersion, PromptHash: observation.PromptHash,
		StartedAt: started, FinishedAt: finished, Attempt: 1, Outcome: aicapability.InvocationSucceeded,
		ProviderRequestID: safeReviewReference(observation.ProviderRequestID),
	}
	usage := observation.Usage
	// The current wire contract has no presence bit. Zero/missing usage stays unknown.
	if usage.PromptTokens >= 0 && usage.CompletionTokens >= 0 && usage.TotalTokens > 0 && usage.PromptTokens+usage.CompletionTokens == usage.TotalTokens {
		record.UsageKnown = true
		record.PromptTokens, record.CompletionTokens, record.TotalTokens = usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens
	}
	if providerErr != nil {
		record.Outcome, record.ErrorCategory, record.ErrorCode = aicapability.InvocationFailed, aicapability.ErrorProviderUnavailable, "review_provider_failed"
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
	}
	return result, providerErr
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
