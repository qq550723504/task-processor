package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"task-processor/internal/aicapability"
	productimage "task-processor/internal/productimage"
	"task-processor/internal/prompt"
)

const governedModelRecordTimeout = 2 * time.Second

type governedFaithfulEditor struct {
	inner    productimage.FaithfulEditor
	router   aicapability.Router
	recorder aicapability.InvocationRecorder
	logger   *logrus.Logger
}

func (e *governedFaithfulEditor) Edit(ctx context.Context, req *productimage.FaithfulEditRequest) (*productimage.FaithfulEditResult, error) {
	if e == nil || e.inner == nil || req == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductImageSubjectExtract), nil)
	}
	identity := productimage.AIIdentityFromContext(ctx)
	if strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.UserID) == "" {
		return nil, productimage.NewTenantModelAccessDeniedError(identity.TenantID)
	}
	operation := aicapability.OperationProductImageSubjectExtract
	if req.Operation == "render_white_background" {
		operation = aicapability.OperationProductImageWhiteBackground
	}
	startedAt := time.Now()
	inputHash := hashGovernedValue(req)
	promptHash := hashGovernedValue(req.PromptRef)
	decision, err := e.router.Decide(ctx, aicapability.RouteRequest{
		TenantID: identity.TenantID, UserID: identity.UserID,
		Capability: aicapability.CapabilityProductImageScene, Operation: operation,
		RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureImageEdit}, TraceID: identity.TraceID,
	})
	if err != nil {
		e.record(ctx, identity, operation, startedAt, inputHash, promptHash, decision, nil, err, true)
		return nil, err
	}
	if !validGovernedDecision(decision, operation) {
		err = aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(operation), nil)
		e.record(ctx, identity, operation, startedAt, inputHash, promptHash, decision, nil, err, true)
		return nil, err
	}
	result, providerErr := e.inner.Edit(ctx, req)
	if providerErr != nil {
		wrapped := aicapability.NewError(classifyGovernedModelError(providerErr), string(operation), providerErr)
		e.record(ctx, identity, operation, startedAt, inputHash, promptHash, decision, result, wrapped, false)
		return result, wrapped
	}
	e.record(ctx, identity, operation, startedAt, inputHash, promptHash, decision, result, nil, false)
	return result, nil
}

type governedReviewModel struct {
	inner    productimage.ImageReviewModel
	router   aicapability.Router
	recorder aicapability.InvocationRecorder
	logger   *logrus.Logger
}

func (m *governedReviewModel) Review(ctx context.Context, req *productimage.ReviewModelRequest) (*productimage.ReviewModelResult, error) {
	if m == nil || m.inner == nil || req == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductImageReview), nil)
	}
	identity := productimage.AIIdentityFromContext(ctx)
	if strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.UserID) == "" {
		return nil, productimage.NewTenantModelAccessDeniedError(identity.TenantID)
	}
	operation := aicapability.OperationProductImageReview
	startedAt := time.Now()
	inputHash := hashGovernedValue(req)
	promptHash := hashGovernedValue(prompt.KProductImageReviewDefault)
	decision, err := m.router.Decide(ctx, aicapability.RouteRequest{
		TenantID: identity.TenantID, UserID: identity.UserID,
		Capability: aicapability.CapabilityProductImageScene, Operation: operation,
		TraceID: identity.TraceID,
	})
	if err != nil {
		m.record(ctx, identity, operation, startedAt, inputHash, promptHash, decision, nil, err, true)
		return nil, err
	}
	if !validGovernedDecision(decision, operation) {
		err = aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(operation), nil)
		m.record(ctx, identity, operation, startedAt, inputHash, promptHash, decision, nil, err, true)
		return nil, err
	}
	result, providerErr := m.inner.Review(ctx, req)
	if providerErr != nil {
		wrapped := aicapability.NewError(classifyGovernedModelError(providerErr), string(operation), providerErr)
		m.record(ctx, identity, operation, startedAt, inputHash, promptHash, decision, result, wrapped, false)
		return result, wrapped
	}
	m.record(ctx, identity, operation, startedAt, inputHash, promptHash, decision, result, nil, false)
	return result, nil
}

func validGovernedDecision(decision aicapability.RouteDecision, operation aicapability.Operation) bool {
	return decision.Capability == aicapability.CapabilityProductImageScene &&
		decision.Operation == operation &&
		strings.TrimSpace(decision.ProviderID) != "" &&
		strings.TrimSpace(decision.ModelID) != "" &&
		strings.TrimSpace(decision.RoutingKey) != "" &&
		strings.TrimSpace(decision.CredentialReference) != ""
}

func classifyGovernedModelError(err error) aicapability.ErrorCategory {
	if errors.Is(err, context.DeadlineExceeded) {
		return aicapability.ErrorProviderTimeout
	}
	if category := aicapability.CategoryOf(err); category != aicapability.ErrorUnknown {
		return category
	}
	return aicapability.ErrorProviderUnavailable
}

func hashGovernedValue(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (e *governedFaithfulEditor) record(ctx context.Context, identity productimage.AIIdentity, operation aicapability.Operation, startedAt time.Time, inputHash, promptHash string, decision aicapability.RouteDecision, result *productimage.FaithfulEditResult, callErr error, routeErr bool) {
	recordGovernedInvocation(ctx, e.recorder, e.logger, identity, operation, startedAt, inputHash, promptHash, decision, governedImageCount(result), hashGovernedValue(result), callErr, routeErr)
}

func (m *governedReviewModel) record(ctx context.Context, identity productimage.AIIdentity, operation aicapability.Operation, startedAt time.Time, inputHash, promptHash string, decision aicapability.RouteDecision, result *productimage.ReviewModelResult, callErr error, routeErr bool) {
	recordGovernedInvocation(ctx, m.recorder, m.logger, identity, operation, startedAt, inputHash, promptHash, decision, 0, hashGovernedValue(result), callErr, routeErr)
}

func recordGovernedInvocation(ctx context.Context, recorder aicapability.InvocationRecorder, logger *logrus.Logger, identity productimage.AIIdentity, operation aicapability.Operation, startedAt time.Time, inputHash, promptHash string, decision aicapability.RouteDecision, imageCount int, outputHash string, callErr error, routeErr bool) {
	if recorder == nil {
		return
	}
	finishedAt := time.Now()
	record := aicapability.InvocationRecord{
		InvocationID: uuid.NewString(), TenantID: identity.TenantID, UserID: identity.UserID,
		BusinessTaskID: identity.BusinessTaskID, TraceID: identity.TraceID,
		Capability: aicapability.CapabilityProductImageScene, Operation: operation,
		RouteMode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
		ProviderID: decision.ProviderID, ModelID: decision.ModelID, RoutingKey: decision.RoutingKey,
		CredentialReference: decision.CredentialReference, PolicyVersion: decision.PolicyVersion,
		ConfigurationVersion: decision.ConfigurationVersion, PromptHash: promptHash,
		StartedAt: startedAt, FinishedAt: finishedAt, LatencyMilliseconds: finishedAt.Sub(startedAt).Milliseconds(),
		Attempt: 1, FallbackIndex: decision.FallbackIndex, ImageCount: imageCount,
		InputHash: inputHash, OutputHash: outputHash, Outcome: aicapability.InvocationSucceeded,
	}
	if callErr != nil {
		record.Outcome = aicapability.InvocationFailed
		record.ErrorCategory = classifyGovernedModelError(callErr)
		record.ErrorCode = string(record.ErrorCategory)
		if routeErr && aicapability.CategoryOf(callErr) != aicapability.ErrorUnknown {
			record.RouteErrorCategory = aicapability.CategoryOf(callErr)
		}
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), governedModelRecordTimeout)
	defer cancel()
	if err := recorder.RecordInvocation(recordCtx, record); err != nil && logger != nil {
		logger.WithError(err).WithFields(logrus.Fields{"invocation_id": record.InvocationID, "capability": string(record.Capability), "operation": string(record.Operation)}).Warn("ai invocation ledger write failed")
	}
}

func governedImageCount(result *productimage.FaithfulEditResult) int {
	if result != nil && result.Asset != nil {
		return 1
	}
	return 0
}
