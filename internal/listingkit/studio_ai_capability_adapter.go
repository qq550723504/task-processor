package listingkit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/aicapability"
)

const studioAIInvocationRecordTimeout = 2 * time.Second

// StudioAIImageCapabilityAdapterConfig adds capability routing and observability
// around the existing Studio image generator without changing its provider contract.
type StudioAIImageCapabilityAdapterConfig struct {
	Legacy        AIImageGenerator
	Router        aicapability.Router
	Recorder      aicapability.InvocationRecorder
	AsyncJobStore aicapability.AsyncJobBindingStore
	Mode          aicapability.RoutingMode
	OnRecordError func(aicapability.InvocationRecord, error)
	Now           func() time.Time
	NewID         func() string
}

type studioAIImageCapabilityAdapter struct {
	legacy           AIImageGenerator
	legacyAsync      AIAsyncImageGenerator
	legacyRouteAware AIAsyncImageQueryByRoutingKey
	router           aicapability.Router
	recorder         aicapability.InvocationRecorder
	asyncJobStore    aicapability.AsyncJobBindingStore
	mode             aicapability.RoutingMode
	onRecordError    func(aicapability.InvocationRecord, error)
	now              func() time.Time
	newID            func() string
}

// NewStudioAIImageCapabilityAdapter returns the original generator in legacy mode.
// Shadow and active modes require both the capability router and invocation recorder.
func NewStudioAIImageCapabilityAdapter(config StudioAIImageCapabilityAdapterConfig) (AIImageGenerator, error) {
	if config.Legacy == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, "", nil)
	}
	mode, err := aicapability.ParseRoutingMode(string(config.Mode))
	if err != nil {
		return nil, err
	}
	if mode == aicapability.RoutingModeLegacy {
		return config.Legacy, nil
	}
	if config.Router == nil || config.Recorder == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, "", nil)
	}
	if mode == aicapability.RoutingModeActive && config.AsyncJobStore == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, "", nil)
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.NewID == nil {
		config.NewID = uuid.NewString
	}
	async, _ := config.Legacy.(AIAsyncImageGenerator)
	routeAware, _ := config.Legacy.(AIAsyncImageQueryByRoutingKey)
	return &studioAIImageCapabilityAdapter{
		legacy:           config.Legacy,
		legacyAsync:      async,
		legacyRouteAware: routeAware,
		router:           config.Router,
		recorder:         config.Recorder,
		asyncJobStore:    config.AsyncJobStore,
		mode:             mode,
		onRecordError:    config.OnRecordError,
		now:              config.Now,
		newID:            config.NewID,
	}, nil
}

func (a *studioAIImageCapabilityAdapter) GenerateImage(ctx context.Context, request *AIImageGenerateRequest) (*AIImageResponse, error) {
	startedAt := a.now()
	decision, routeOutcome, routeErr := a.route(ctx, aicapability.OperationImageGenerate, requestedGenerateModel(request), []aicapability.ModelFeature{aicapability.FeatureImageGenerate})
	if routeErr != nil && a.mode == aicapability.RoutingModeActive {
		a.record(ctx, a.newRecord(ctx, startedAt, aicapability.OperationImageGenerate, requestedGenerateModel(request), routeOutcome, decision, routeErr, nil, imageInvocationResult{}, hashPrompt(generatePrompt(request)), hashGenerateInput(request)))
		return nil, routeErr
	}
	next := request
	if a.mode == aicapability.RoutingModeActive {
		next = activeGenerateRequest(request, decision.RoutingKey)
	}
	response, err := a.legacy.GenerateImage(ctx, next)
	a.record(ctx, a.newRecord(ctx, startedAt, aicapability.OperationImageGenerate, requestedGenerateModel(request), routeOutcome, decision, routeErr, err, imageResponseResult(response), hashPrompt(generatePrompt(request)), hashGenerateInput(request)))
	return response, err
}

func (a *studioAIImageCapabilityAdapter) EditImage(ctx context.Context, request *AIImageEditRequest) (*AIImageResponse, error) {
	startedAt := a.now()
	decision, routeOutcome, routeErr := a.route(ctx, aicapability.OperationImageEdit, requestedEditModel(request), []aicapability.ModelFeature{aicapability.FeatureImageEdit})
	if routeErr != nil && a.mode == aicapability.RoutingModeActive {
		a.record(ctx, a.newRecord(ctx, startedAt, aicapability.OperationImageEdit, requestedEditModel(request), routeOutcome, decision, routeErr, nil, imageInvocationResult{}, hashPrompt(editPrompt(request)), hashEditInput(request)))
		return nil, routeErr
	}
	next := request
	if a.mode == aicapability.RoutingModeActive {
		next = activeEditRequest(request, decision.RoutingKey)
	}
	response, err := a.legacy.EditImage(ctx, next)
	a.record(ctx, a.newRecord(ctx, startedAt, aicapability.OperationImageEdit, requestedEditModel(request), routeOutcome, decision, routeErr, err, imageResponseResult(response), hashPrompt(editPrompt(request)), hashEditInput(request)))
	return response, err
}

func (a *studioAIImageCapabilityAdapter) GetDefaultModel() string {
	return a.legacy.GetDefaultModel()
}

func (a *studioAIImageCapabilityAdapter) SupportsAsyncImageGeneration() bool {
	return a.legacyAsync != nil && a.legacyAsync.SupportsAsyncImageGeneration()
}

func (a *studioAIImageCapabilityAdapter) SubmitImageGeneration(ctx context.Context, request *AIImageGenerateRequest) (*AIImageAsyncSubmit, error) {
	startedAt := a.now()
	features := []aicapability.ModelFeature{aicapability.FeatureImageGenerate, aicapability.FeatureAsyncImageJob}
	decision, routeOutcome, routeErr := a.route(ctx, aicapability.OperationAsyncImageGenerate, requestedGenerateModel(request), features)
	if routeErr != nil && a.mode == aicapability.RoutingModeActive {
		a.record(ctx, a.newRecord(ctx, startedAt, aicapability.OperationAsyncImageGenerate, requestedGenerateModel(request), routeOutcome, decision, routeErr, nil, imageInvocationResult{}, hashPrompt(generatePrompt(request)), hashGenerateInput(request)))
		return nil, routeErr
	}
	next := request
	if a.mode == aicapability.RoutingModeActive {
		next = activeGenerateRequest(request, decision.RoutingKey)
	}
	response, err := a.submitImageGeneration(ctx, next)
	if err == nil {
		err = a.persistAsyncJobBinding(ctx, aicapability.OperationAsyncImageGenerate, decision, response)
	}
	a.record(ctx, a.newRecord(ctx, startedAt, aicapability.OperationAsyncImageGenerate, requestedGenerateModel(request), routeOutcome, decision, routeErr, err, asyncSubmitResult(response), hashPrompt(generatePrompt(request)), hashGenerateInput(request)))
	return response, err
}

func (a *studioAIImageCapabilityAdapter) SubmitImageEdit(ctx context.Context, request *AIImageEditRequest) (*AIImageAsyncSubmit, error) {
	startedAt := a.now()
	features := []aicapability.ModelFeature{aicapability.FeatureImageEdit, aicapability.FeatureAsyncImageJob}
	decision, routeOutcome, routeErr := a.route(ctx, aicapability.OperationAsyncImageEdit, requestedEditModel(request), features)
	if routeErr != nil && a.mode == aicapability.RoutingModeActive {
		a.record(ctx, a.newRecord(ctx, startedAt, aicapability.OperationAsyncImageEdit, requestedEditModel(request), routeOutcome, decision, routeErr, nil, imageInvocationResult{}, hashPrompt(editPrompt(request)), hashEditInput(request)))
		return nil, routeErr
	}
	next := request
	if a.mode == aicapability.RoutingModeActive {
		next = activeEditRequest(request, decision.RoutingKey)
	}
	response, err := a.submitImageEdit(ctx, next)
	if err == nil {
		err = a.persistAsyncJobBinding(ctx, aicapability.OperationAsyncImageEdit, decision, response)
	}
	a.record(ctx, a.newRecord(ctx, startedAt, aicapability.OperationAsyncImageEdit, requestedEditModel(request), routeOutcome, decision, routeErr, err, asyncSubmitResult(response), hashPrompt(editPrompt(request)), hashEditInput(request)))
	return response, err
}

func (a *studioAIImageCapabilityAdapter) QueryImageGeneration(ctx context.Context, jobID string) (*AIImageAsyncResult, error) {
	startedAt := a.now()
	response, err, decision, routeOutcome, routeErr := a.queryAsyncJob(ctx, jobID)
	if response != nil && a.mode == aicapability.RoutingModeActive && routeOutcome == aicapability.RouteOutcomeActive {
		_ = a.updateAsyncJobStatus(ctx, jobID, string(response.Status), "")
	}
	a.record(ctx, a.newRecord(ctx, startedAt, aicapability.OperationAsyncImageQuery, "", routeOutcome, decision, routeErr, err, asyncQueryResult(response), "", hashFields(jobID)))
	return response, err
}

func (a *studioAIImageCapabilityAdapter) queryAsyncJob(ctx context.Context, jobID string) (*AIImageAsyncResult, error, aicapability.RouteDecision, aicapability.RouteOutcome, error) {
	decision := aicapability.RouteDecision{}
	if a.mode != aicapability.RoutingModeActive {
		response, err := a.queryImageGeneration(ctx, jobID)
		return response, err, decision, aicapability.RouteOutcomeLegacy, nil
	}
	binding, bindingErr := a.asyncJobStore.GetAsyncJobBinding(ctx, jobID)
	if bindingErr != nil {
		unknownErr := aicapability.NewError(aicapability.ErrorUnknownRemoteState, string(aicapability.OperationAsyncImageQuery), bindingErr)
		if errors.Is(bindingErr, aicapability.ErrAsyncJobBindingNotFound) {
			response, queryErr := a.queryImageGeneration(ctx, jobID)
			return response, queryErr, decision, aicapability.RouteOutcomeLegacy, unknownErr
		}
		return nil, unknownErr, decision, aicapability.RouteOutcomeActive, unknownErr
	}
	decision = routeDecisionFromAsyncJobBinding(binding)
	if a.legacyRouteAware == nil {
		unknownErr := aicapability.NewError(aicapability.ErrorUnknownRemoteState, string(aicapability.OperationAsyncImageQuery), errors.New("route-aware async image query is not supported"))
		return nil, unknownErr, decision, aicapability.RouteOutcomeActive, unknownErr
	}
	response, queryErr := a.legacyRouteAware.QueryImageGenerationForRoutingKey(ctx, binding.RoutingKey, jobID)
	return response, queryErr, decision, aicapability.RouteOutcomeActive, nil
}

func (a *studioAIImageCapabilityAdapter) persistAsyncJobBinding(ctx context.Context, operation aicapability.Operation, decision aicapability.RouteDecision, response *AIImageAsyncSubmit) error {
	if a.mode != aicapability.RoutingModeActive || response == nil || strings.TrimSpace(response.JobID) == "" {
		if a.mode == aicapability.RoutingModeActive && response != nil && strings.TrimSpace(response.JobID) == "" {
			return aicapability.NewError(aicapability.ErrorInvalidProviderResponse, string(operation), errors.New("async Provider response has no job ID"))
		}
		return nil
	}
	identity := requestIdentity(ctx)
	trace := requestTrace(ctx)
	err := a.asyncJobStore.PutAsyncJobBinding(ctx, aicapability.AsyncJobBinding{
		JobID: response.JobID, TenantID: identity.TenantID, UserID: identity.UserID, BusinessTaskID: businessTaskID(trace), TraceID: trace.BatchID,
		Capability: aicapability.CapabilityListingKitStudioImage, Operation: operation, ProviderID: decision.ProviderID, ModelID: decision.ModelID,
		RoutingKey: decision.RoutingKey, CredentialReference: decision.CredentialReference, PolicyVersion: decision.PolicyVersion, ConfigurationVersion: decision.ConfigurationVersion,
		SubmittedAt: a.now(), UpdatedAt: a.now(), Status: string(response.Status),
	})
	if err != nil {
		return aicapability.NewError(aicapability.ErrorUnknownRemoteState, string(operation), err)
	}
	return nil
}

func (a *studioAIImageCapabilityAdapter) updateAsyncJobStatus(ctx context.Context, jobID, status string, category aicapability.ErrorCategory) error {
	if a.asyncJobStore == nil || strings.TrimSpace(jobID) == "" {
		return nil
	}
	return a.asyncJobStore.UpdateAsyncJobBindingStatus(ctx, jobID, status, category)
}

func routeDecisionFromAsyncJobBinding(binding aicapability.AsyncJobBinding) aicapability.RouteDecision {
	return aicapability.RouteDecision{
		Capability: binding.Capability, Operation: binding.Operation, ProviderID: binding.ProviderID, ModelID: binding.ModelID,
		RoutingKey: binding.RoutingKey, CredentialReference: binding.CredentialReference, PolicyVersion: binding.PolicyVersion, ConfigurationVersion: binding.ConfigurationVersion,
	}
}

func (a *studioAIImageCapabilityAdapter) submitImageGeneration(ctx context.Context, request *AIImageGenerateRequest) (*AIImageAsyncSubmit, error) {
	if a.legacyAsync == nil {
		return nil, ErrAsyncImageGenerationNotSupported
	}
	return a.legacyAsync.SubmitImageGeneration(ctx, request)
}

func (a *studioAIImageCapabilityAdapter) submitImageEdit(ctx context.Context, request *AIImageEditRequest) (*AIImageAsyncSubmit, error) {
	if a.legacyAsync == nil {
		return nil, ErrAsyncImageGenerationNotSupported
	}
	return a.legacyAsync.SubmitImageEdit(ctx, request)
}

func (a *studioAIImageCapabilityAdapter) queryImageGeneration(ctx context.Context, jobID string) (*AIImageAsyncResult, error) {
	if a.legacyAsync == nil {
		return nil, ErrAsyncImageGenerationNotSupported
	}
	return a.legacyAsync.QueryImageGeneration(ctx, jobID)
}

func (a *studioAIImageCapabilityAdapter) route(ctx context.Context, operation aicapability.Operation, requestedRoutingKey string, features []aicapability.ModelFeature) (aicapability.RouteDecision, aicapability.RouteOutcome, error) {
	identity := requestIdentity(ctx)
	trace := requestTrace(ctx)
	decision, err := a.router.Decide(ctx, aicapability.RouteRequest{
		TenantID:            identity.TenantID,
		UserID:              identity.UserID,
		Capability:          aicapability.CapabilityListingKitStudioImage,
		Operation:           operation,
		RequestedRoutingKey: requestedRoutingKey,
		RequiredFeatures:    features,
		TraceID:             trace.BatchID,
	})
	if err != nil {
		if a.mode == aicapability.RoutingModeShadow {
			return decision, aicapability.RouteOutcomeShadowRouteError, err
		}
		return decision, aicapability.RouteOutcomeActive, err
	}
	if a.mode == aicapability.RoutingModeShadow {
		return decision, aicapability.RouteOutcomeShadowDecided, nil
	}
	return decision, aicapability.RouteOutcomeActive, nil
}

func (a *studioAIImageCapabilityAdapter) newRecord(ctx context.Context, startedAt time.Time, operation aicapability.Operation, requestedRoutingKey string, routeOutcome aicapability.RouteOutcome, decision aicapability.RouteDecision, routeErr error, providerErr error, result imageInvocationResult, promptHash string, inputHash string) aicapability.InvocationRecord {
	finishedAt := a.now()
	identity := requestIdentity(ctx)
	trace := requestTrace(ctx)
	record := aicapability.InvocationRecord{
		InvocationID:         a.newID(),
		TenantID:             identity.TenantID,
		UserID:               identity.UserID,
		BusinessTaskID:       businessTaskID(trace),
		TraceID:              trace.BatchID,
		Capability:           aicapability.CapabilityListingKitStudioImage,
		Operation:            operation,
		RouteMode:            a.mode,
		RouteOutcome:         routeOutcome,
		ProviderID:           decision.ProviderID,
		ModelID:              decision.ModelID,
		RequestedRoutingKey:  requestedRoutingKey,
		RoutingKey:           decision.RoutingKey,
		CredentialReference:  decision.CredentialReference,
		PolicyVersion:        decision.PolicyVersion,
		ConfigurationVersion: decision.ConfigurationVersion,
		PromptHash:           promptHash,
		StartedAt:            startedAt,
		FinishedAt:           finishedAt,
		LatencyMilliseconds:  finishedAt.Sub(startedAt).Milliseconds(),
		Attempt:              1,
		FallbackIndex:        decision.FallbackIndex,
		PromptTokens:         result.usage.PromptTokens,
		CompletionTokens:     result.usage.CompletionTokens,
		TotalTokens:          result.usage.TotalTokens,
		ImageCount:           result.imageCount,
		Currency:             "",
		ProviderRequestID:    result.requestID,
		UpstreamJobID:        result.upstreamJobID,
		InputHash:            inputHash,
		OutputHash:           result.outputHash,
	}
	if routeErr != nil {
		record.RouteErrorCategory = classifyInvocationError(routeErr)
	}
	if providerErr != nil || (routeErr != nil && a.mode == aicapability.RoutingModeActive) {
		err := providerErr
		if err == nil {
			err = routeErr
		}
		record.Outcome = aicapability.InvocationFailed
		record.ErrorCategory = classifyInvocationError(err)
		record.ErrorCode = string(record.ErrorCategory)
		return record
	}
	record.Outcome = aicapability.InvocationSucceeded
	return record
}

func (a *studioAIImageCapabilityAdapter) record(ctx context.Context, record aicapability.InvocationRecord) {
	if ctx == nil {
		ctx = context.Background()
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), studioAIInvocationRecordTimeout)
	defer cancel()
	if err := a.recorder.RecordInvocation(recordCtx, record); err != nil && a.onRecordError != nil {
		a.onRecordError(record, err)
	}
}

type imageInvocationResult struct {
	usage         AIUsage
	imageCount    int
	requestID     string
	upstreamJobID string
	outputHash    string
}

func imageResponseResult(response *AIImageResponse) imageInvocationResult {
	if response == nil {
		return imageInvocationResult{}
	}
	return imageInvocationResult{
		usage: response.Usage, imageCount: len(response.Data), requestID: response.RequestID, upstreamJobID: response.UpstreamJobID,
		outputHash: hashImageResponse(response),
	}
}

func asyncSubmitResult(response *AIImageAsyncSubmit) imageInvocationResult {
	if response == nil {
		return imageInvocationResult{}
	}
	result := imageResponseResult(response.Response)
	result.requestID = response.RequestID
	result.upstreamJobID = response.JobID
	result.outputHash = hashFields(response.JobID, response.RequestID, response.Provider, string(response.Status), result.outputHash)
	return result
}

func asyncQueryResult(response *AIImageAsyncResult) imageInvocationResult {
	if response == nil {
		return imageInvocationResult{}
	}
	result := imageResponseResult(response.Response)
	result.usage = response.Usage
	result.requestID = response.RequestID
	result.upstreamJobID = response.JobID
	result.outputHash = hashFields(response.JobID, response.RequestID, response.Provider, string(response.Status), result.outputHash)
	return result
}

func requestedGenerateModel(request *AIImageGenerateRequest) string {
	if request == nil {
		return ""
	}
	return strings.TrimSpace(request.Model)
}

func generatePrompt(request *AIImageGenerateRequest) string {
	if request == nil {
		return ""
	}
	return request.Prompt
}

func requestedEditModel(request *AIImageEditRequest) string {
	if request == nil {
		return ""
	}
	return strings.TrimSpace(request.Model)
}

func editPrompt(request *AIImageEditRequest) string {
	if request == nil {
		return ""
	}
	return request.Prompt
}

func activeGenerateRequest(request *AIImageGenerateRequest, routingKey string) *AIImageGenerateRequest {
	if request == nil {
		return nil
	}
	copy := *request
	copy.Model = routingKey
	return &copy
}

func activeEditRequest(request *AIImageEditRequest, routingKey string) *AIImageEditRequest {
	if request == nil {
		return nil
	}
	copy := *request
	copy.Model = routingKey
	if request.ImageURLs != nil {
		copy.ImageURLs = append([]string(nil), request.ImageURLs...)
	}
	return &copy
}

func requestIdentity(ctx context.Context) RequestIdentity {
	if ctx == nil {
		return RequestIdentity{}
	}
	identity := RequestIdentityFromContext(ctx)
	if identity.TenantID == "" {
		identity.TenantID = strings.TrimSpace(TenantIDFromContext(ctx))
	}
	return identity
}

func requestTrace(ctx context.Context) RequestTrace {
	if ctx == nil {
		return RequestTrace{}
	}
	return RequestTraceFromContext(ctx)
}

func businessTaskID(trace RequestTrace) string {
	if trace.BatchRunID != "" {
		return trace.BatchRunID
	}
	return trace.SessionID
}

func classifyInvocationError(err error) aicapability.ErrorCategory {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return aicapability.ErrorProviderTimeout
	}
	if category := aicapability.CategoryOf(err); category != aicapability.ErrorUnknown {
		return category
	}
	return aicapability.ErrorUnknown
}

func hashGenerateInput(request *AIImageGenerateRequest) string {
	if request == nil {
		return ""
	}
	return hashFields(request.Model, request.Prompt, request.Size, request.ResponseFormat, strconv.Itoa(request.N))
}

func hashPrompt(prompt string) string {
	if prompt == "" {
		return ""
	}
	return hashFields(prompt)
}

func hashEditInput(request *AIImageEditRequest) string {
	if request == nil {
		return ""
	}
	fields := []string{request.Model, request.Prompt, request.ImageContentType, request.ImageURL, request.Size, request.ResponseFormat, strconv.Itoa(request.N), string(request.ImageData)}
	fields = append(fields, request.ImageURLs...)
	return hashFields(fields...)
}

func hashImageResponse(response *AIImageResponse) string {
	if response == nil {
		return ""
	}
	fields := make([]string, 0, 1+len(response.Data)*3)
	fields = append(fields, response.RequestID)
	for _, item := range response.Data {
		fields = append(fields, item.URL, item.B64JSON, item.RevisedPrompt)
	}
	return hashFields(fields...)
}

func hashFields(fields ...string) string {
	hash := sha256.New()
	for _, field := range fields {
		_, _ = hash.Write([]byte(strconv.Itoa(len(field))))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(field))
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil))
}
