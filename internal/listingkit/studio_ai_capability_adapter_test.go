package listingkit

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"task-processor/internal/aicapability"
)

type studioCapabilityRouterStub struct {
	decision aicapability.RouteDecision
	err      error
	calls    int
	request  aicapability.RouteRequest
}

func (s *studioCapabilityRouterStub) Decide(_ context.Context, request aicapability.RouteRequest) (aicapability.RouteDecision, error) {
	s.calls++
	s.request = request
	return s.decision, s.err
}

type studioCapabilityGeneratorStub struct {
	generateCalls   int
	editCalls       int
	submitGenCalls  int
	submitEditCalls int
	queryCalls      int
	generateReq     *AIImageGenerateRequest
	editReq         *AIImageEditRequest
	submitGenReq    *AIImageGenerateRequest
	submitEditReq   *AIImageEditRequest
	queryJobID      string
	queryRoute      string
	queryIdentity   RequestIdentity
	submitIdentity  RequestIdentity
	submitContext   AIAsyncImageQueryContext
	response        *AIImageResponse
	submit          *AIImageAsyncSubmit
	query           *AIImageAsyncResult
	err             error
	supportsAsync   bool
	defaultModel    string
}

func (s *studioCapabilityGeneratorStub) GenerateImage(_ context.Context, request *AIImageGenerateRequest) (*AIImageResponse, error) {
	s.generateCalls++
	s.generateReq = request
	return s.response, s.err
}

func (s *studioCapabilityGeneratorStub) EditImage(_ context.Context, request *AIImageEditRequest) (*AIImageResponse, error) {
	s.editCalls++
	s.editReq = request
	return s.response, s.err
}

func (s *studioCapabilityGeneratorStub) GetDefaultModel() string { return s.defaultModel }

func (s *studioCapabilityGeneratorStub) SupportsAsyncImageGeneration() bool { return s.supportsAsync }

func (s *studioCapabilityGeneratorStub) SubmitImageGeneration(ctx context.Context, request *AIImageGenerateRequest) (*AIImageAsyncSubmit, error) {
	s.submitGenCalls++
	s.submitGenReq = request
	s.submitIdentity = RequestIdentityFromContext(ctx)
	s.submitContext = AIAsyncImageQueryContextFromContext(ctx)
	return s.submit, s.err
}

func (s *studioCapabilityGeneratorStub) SubmitImageEdit(ctx context.Context, request *AIImageEditRequest) (*AIImageAsyncSubmit, error) {
	s.submitEditCalls++
	s.submitEditReq = request
	s.submitIdentity = RequestIdentityFromContext(ctx)
	s.submitContext = AIAsyncImageQueryContextFromContext(ctx)
	return s.submit, s.err
}

func (s *studioCapabilityGeneratorStub) QueryImageGeneration(_ context.Context, jobID string) (*AIImageAsyncResult, error) {
	s.queryCalls++
	s.queryJobID = jobID
	return s.query, s.err
}

func (s *studioCapabilityGeneratorStub) QueryImageGenerationForRoutingKey(ctx context.Context, routingKey, jobID string) (*AIImageAsyncResult, error) {
	s.queryCalls++
	s.queryRoute = routingKey
	s.queryJobID = jobID
	s.queryIdentity = RequestIdentityFromContext(ctx)
	return s.query, s.err
}

type studioCapabilityAsyncJobStoreStub struct {
	binding        aicapability.AsyncJobBinding
	putCalls       int
	getCalls       int
	statusCalls    int
	putErr         error
	getErr         error
	statusErr      error
	status         string
	statusCategory aicapability.ErrorCategory
	putCtxErr      error
	requireLivePut bool
}

func (s *studioCapabilityAsyncJobStoreStub) PutAsyncJobBinding(ctx context.Context, binding aicapability.AsyncJobBinding) error {
	s.putCalls++
	s.putCtxErr = ctx.Err()
	if s.requireLivePut && s.putCtxErr != nil {
		return s.putCtxErr
	}
	if s.putErr != nil {
		return s.putErr
	}
	s.binding = binding
	return nil
}

func (s *studioCapabilityAsyncJobStoreStub) GetAsyncJobBinding(_ context.Context, _ string) (aicapability.AsyncJobBinding, error) {
	s.getCalls++
	if s.getErr != nil {
		return aicapability.AsyncJobBinding{}, s.getErr
	}
	return s.binding, nil
}

func (s *studioCapabilityAsyncJobStoreStub) UpdateAsyncJobBindingStatus(_ context.Context, _ string, status string, category aicapability.ErrorCategory) error {
	s.statusCalls++
	s.status = status
	s.statusCategory = category
	return s.statusErr
}

type studioCapabilityRecorderStub struct {
	mu      sync.Mutex
	records []aicapability.InvocationRecord
	err     error
	ctxErr  error
}

func (s *studioCapabilityRecorderStub) RecordInvocation(ctx context.Context, record aicapability.InvocationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctxErr = ctx.Err()
	s.records = append(s.records, record)
	return s.err
}

func (s *studioCapabilityRecorderStub) last(t *testing.T) aicapability.InvocationRecord {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.records) == 0 {
		t.Fatal("expected an invocation record")
	}
	return s.records[len(s.records)-1]
}

func newStudioCapabilityAdapterForTest(t *testing.T, mode aicapability.RoutingMode, legacy *studioCapabilityGeneratorStub, router *studioCapabilityRouterStub, recorder *studioCapabilityRecorderStub) *studioAIImageCapabilityAdapter {
	t.Helper()
	adapter, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy:   legacy,
		Router:   router,
		Recorder: recorder,
		AsyncJobStore: func() aicapability.AsyncJobBindingStore {
			if mode == aicapability.RoutingModeActive {
				return &studioCapabilityAsyncJobStoreStub{}
			}
			return nil
		}(),
		Mode:  mode,
		Now:   func() time.Time { return time.Date(2026, 8, 6, 1, 2, 3, 0, time.UTC) },
		NewID: func() string { return "invocation-1" },
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}
	wrapped, ok := adapter.(*studioAIImageCapabilityAdapter)
	if !ok {
		t.Fatalf("adapter type = %T, want *studioAIImageCapabilityAdapter", adapter)
	}
	return wrapped
}

func routeDecision() aicapability.RouteDecision {
	return aicapability.RouteDecision{
		Capability:           aicapability.CapabilityListingKitStudioImage,
		ProviderID:           "provider-a",
		ModelID:              "model-a",
		RoutingKey:           "route-model-a",
		CredentialReference:  "credential-a",
		PolicyVersion:        "policy-v1",
		ConfigurationVersion: "config-v1",
		FallbackIndex:        1,
	}
}

func studioCapabilityContext() context.Context {
	ctx := WithRequestIdentity(context.Background(), RequestIdentity{TenantID: "tenant-a", UserID: "user-a"})
	return WithRequestTrace(ctx, RequestTrace{BatchRunID: "batch-run-a", BatchID: "trace-a", SessionID: "session-a"})
}

func TestStudioAIImageCapabilityAdapterShadowKeepsGenerateRequestUntouched(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{response: &AIImageResponse{Data: []AIImageData{{URL: "https://example.test/image.png"}}, RequestID: "provider-request", Usage: AIUsage{TotalTokens: 3}}}
	router := &studioCapabilityRouterStub{decision: routeDecision()}
	recorder := &studioCapabilityRecorderStub{}
	adapter := newStudioCapabilityAdapterForTest(t, aicapability.RoutingModeShadow, legacy, router, recorder)
	request := &AIImageGenerateRequest{Model: "original-model", Prompt: "private prompt"}

	response, err := adapter.GenerateImage(studioCapabilityContext(), request)
	if err != nil || response == nil {
		t.Fatalf("GenerateImage() = (%v, %v)", response, err)
	}
	if legacy.generateCalls != 1 || legacy.generateReq != request || request.Model != "original-model" {
		t.Fatalf("shadow must delegate original request once: calls=%d req=%p original=%p model=%q", legacy.generateCalls, legacy.generateReq, request, request.Model)
	}
	if router.calls != 1 || router.request.RequestedRoutingKey != "original-model" || router.request.Operation != aicapability.OperationImageGenerate {
		t.Fatalf("unexpected route request: %+v", router.request)
	}
	record := recorder.last(t)
	if record.RouteOutcome != aicapability.RouteOutcomeShadowDecided || record.RoutingKey != "route-model-a" || record.PromptHash == "" || record.InputHash == "" || record.OutputHash == "" {
		t.Fatalf("unexpected record: %+v", record)
	}
	if record.TenantID != "tenant-a" || record.UserID != "user-a" || record.BusinessTaskID != "batch-run-a" || record.TraceID != "trace-a" {
		t.Fatalf("unexpected request identity record: %+v", record)
	}
}

func TestStudioAIImageCapabilityAdapterActiveRoutesEditWithoutMutatingCallerSlices(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{response: &AIImageResponse{}}
	router := &studioCapabilityRouterStub{decision: routeDecision()}
	recorder := &studioCapabilityRecorderStub{}
	adapter := newStudioCapabilityAdapterForTest(t, aicapability.RoutingModeActive, legacy, router, recorder)
	request := &AIImageEditRequest{Model: "original-model", Prompt: "edit", ImageData: []byte("image-bytes"), ImageURLs: []string{"https://example.test/a.png"}}

	if _, err := adapter.EditImage(studioCapabilityContext(), request); err != nil {
		t.Fatalf("EditImage() error = %v", err)
	}
	if legacy.editCalls != 1 || legacy.editReq == request || legacy.editReq.Model != "route-model-a" {
		t.Fatalf("active request = %#v, original=%#v", legacy.editReq, request)
	}
	legacy.editReq.ImageURLs[0] = "mutated-by-provider"
	if request.Model != "original-model" || request.ImageURLs[0] != "https://example.test/a.png" {
		t.Fatalf("active routing mutated caller request: %#v", request)
	}
	if record := recorder.last(t); record.Operation != aicapability.OperationImageEdit || record.RouteOutcome != aicapability.RouteOutcomeActive {
		t.Fatalf("unexpected edit record: %+v", record)
	}
}

func TestStudioAIImageCapabilityAdapterRouteFailureDiffersByMode(t *testing.T) {
	routeErr := aicapability.NewError(aicapability.ErrorCredentialUnavailable, "image_generate", errors.New("missing credential"))
	for _, tt := range []struct {
		name        string
		mode        aicapability.RoutingMode
		wantCalls   int
		wantErr     bool
		wantOutcome aicapability.RouteOutcome
	}{
		{name: "shadow", mode: aicapability.RoutingModeShadow, wantCalls: 1, wantOutcome: aicapability.RouteOutcomeShadowRouteError},
		{name: "active", mode: aicapability.RoutingModeActive, wantCalls: 0, wantErr: true, wantOutcome: aicapability.RouteOutcomeActive},
	} {
		t.Run(tt.name, func(t *testing.T) {
			legacy := &studioCapabilityGeneratorStub{response: &AIImageResponse{}}
			recorder := &studioCapabilityRecorderStub{}
			adapter := newStudioCapabilityAdapterForTest(t, tt.mode, legacy, &studioCapabilityRouterStub{err: routeErr}, recorder)
			_, err := adapter.GenerateImage(studioCapabilityContext(), &AIImageGenerateRequest{Model: "model"})
			if (err != nil) != tt.wantErr || legacy.generateCalls != tt.wantCalls {
				t.Fatalf("GenerateImage() error=%v calls=%d", err, legacy.generateCalls)
			}
			record := recorder.last(t)
			if record.RouteOutcome != tt.wantOutcome || record.RouteErrorCategory != aicapability.ErrorCredentialUnavailable {
				t.Fatalf("unexpected route error record: %+v", record)
			}
		})
	}
}

func TestStudioAIImageCapabilityAdapterProviderFailureIsClassifiedAndRecorderNeverRetries(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{err: context.DeadlineExceeded}
	recorder := &studioCapabilityRecorderStub{err: errors.New("ledger unavailable")}
	callbackCalls := 0
	adapter, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy: legacy, Router: &studioCapabilityRouterStub{decision: routeDecision()}, Recorder: recorder, AsyncJobStore: &studioCapabilityAsyncJobStoreStub{}, Mode: aicapability.RoutingModeActive,
		OnRecordError: func(aicapability.InvocationRecord, error) { callbackCalls++ },
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}

	_, err = adapter.GenerateImage(context.Background(), &AIImageGenerateRequest{Model: "model"})
	if !errors.Is(err, context.DeadlineExceeded) || legacy.generateCalls != 1 || callbackCalls != 1 {
		t.Fatalf("GenerateImage() error=%v calls=%d callbacks=%d", err, legacy.generateCalls, callbackCalls)
	}
	if record := recorder.last(t); record.ErrorCategory != aicapability.ErrorProviderTimeout || record.Outcome != aicapability.InvocationFailed {
		t.Fatalf("unexpected provider error record: %+v", record)
	}
}

func TestStudioAIImageCapabilityAdapterPreservesNilRequestForLegacy(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{response: &AIImageResponse{}}
	recorder := &studioCapabilityRecorderStub{}
	adapter := newStudioCapabilityAdapterForTest(t, aicapability.RoutingModeActive, legacy, &studioCapabilityRouterStub{decision: routeDecision()}, recorder)

	if _, err := adapter.GenerateImage(studioCapabilityContext(), nil); err != nil {
		t.Fatalf("GenerateImage(nil) error = %v", err)
	}
	if legacy.generateReq != nil {
		t.Fatalf("legacy received %#v, want nil", legacy.generateReq)
	}
	if record := recorder.last(t); record.RequestedRoutingKey != "" || record.PromptHash != "" || record.InputHash != "" {
		t.Fatalf("nil request should not create payload metadata: %+v", record)
	}
}

func TestStudioAIImageCapabilityAdapterClassifiesCapabilityErrors(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{err: aicapability.NewError(aicapability.ErrorProviderRejected, "image_generate", errors.New("provider detail"))}
	recorder := &studioCapabilityRecorderStub{}
	adapter := newStudioCapabilityAdapterForTest(t, aicapability.RoutingModeShadow, legacy, &studioCapabilityRouterStub{decision: routeDecision()}, recorder)

	_, err := adapter.GenerateImage(context.Background(), &AIImageGenerateRequest{Model: "model", Prompt: "prompt"})
	if aicapability.CategoryOf(err) != aicapability.ErrorProviderRejected {
		t.Fatalf("GenerateImage() category = %q", aicapability.CategoryOf(err))
	}
	if record := recorder.last(t); record.ErrorCategory != aicapability.ErrorProviderRejected || record.ErrorCode != string(aicapability.ErrorProviderRejected) {
		t.Fatalf("unexpected provider rejection record: %+v", record)
	}
}

func TestStudioAIImageCapabilityAdapterAsyncAndQueryPreserveLegacyContract(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{supportsAsync: true, defaultModel: "legacy-model", submit: &AIImageAsyncSubmit{JobID: "job-a", RequestID: "request-a", Provider: "provider-a", Response: &AIImageResponse{Usage: AIUsage{TotalTokens: 4}}}, query: &AIImageAsyncResult{JobID: "job-a", RequestID: "request-b", Provider: "provider-a", Usage: AIUsage{TotalTokens: 5}}}
	router := &studioCapabilityRouterStub{decision: routeDecision()}
	recorder := &studioCapabilityRecorderStub{}
	adapter := newStudioCapabilityAdapterForTest(t, aicapability.RoutingModeActive, legacy, router, recorder)

	if !adapter.SupportsAsyncImageGeneration() || adapter.GetDefaultModel() != "legacy-model" {
		t.Fatal("transparent methods did not delegate")
	}
	if _, err := adapter.SubmitImageGeneration(studioCapabilityContext(), &AIImageGenerateRequest{Model: "original", Prompt: "generate"}); err != nil {
		t.Fatalf("SubmitImageGeneration() error = %v", err)
	}
	if legacy.submitGenReq.Model != "route-model-a" || router.request.Operation != aicapability.OperationAsyncImageGenerate {
		t.Fatalf("async generate did not route: request=%+v route=%+v", legacy.submitGenReq, router.request)
	}
	if _, err := adapter.SubmitImageEdit(studioCapabilityContext(), &AIImageEditRequest{Model: "original", Prompt: "edit", ImageURLs: []string{"one"}}); err != nil {
		t.Fatalf("SubmitImageEdit() error = %v", err)
	}
	if legacy.submitEditReq.Model != "route-model-a" || router.request.Operation != aicapability.OperationAsyncImageEdit {
		t.Fatalf("async edit did not route: request=%+v route=%+v", legacy.submitEditReq, router.request)
	}
	if _, err := adapter.QueryImageGeneration(studioCapabilityContext(), "job-a"); err != nil {
		t.Fatalf("QueryImageGeneration() error = %v", err)
	}
	if legacy.queryCalls != 1 || legacy.queryJobID != "job-a" || router.calls != 2 {
		t.Fatalf("query rerouted or did not pass through: calls=%d router=%d", legacy.queryCalls, router.calls)
	}
	if legacy.queryRoute != "route-model-a" || legacy.queryJobID != "job-a" {
		t.Fatalf("query did not recover submit route: route=%q job=%q", legacy.queryRoute, legacy.queryJobID)
	}
	if record := recorder.last(t); record.Operation != aicapability.OperationAsyncImageQuery || record.ProviderID != "provider-a" || record.RoutingKey != "route-model-a" || record.RouteOutcome != aicapability.RouteOutcomeActive {
		t.Fatalf("query record did not include bound route: %+v", record)
	}
}

func TestStudioAIImageCapabilityAdapterActiveBindsSubmitAndQueriesByRoute(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{
		supportsAsync: true,
		submit:        &AIImageAsyncSubmit{JobID: "job-a", RequestID: "request-a", Provider: "provider-a"},
		query:         &AIImageAsyncResult{JobID: "job-a", RequestID: "request-b", Provider: "provider-a"},
	}
	store := &studioCapabilityAsyncJobStoreStub{}
	adapterValue, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy: legacy, Router: &studioCapabilityRouterStub{decision: routeDecision()}, Recorder: &studioCapabilityRecorderStub{},
		AsyncJobStore: store, Mode: aicapability.RoutingModeActive,
		Now: func() time.Time { return time.Date(2026, 8, 6, 1, 2, 3, 0, time.UTC) }, NewID: func() string { return "invocation-1" },
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}
	adapter := adapterValue.(*studioAIImageCapabilityAdapter)

	if _, err := adapter.SubmitImageGeneration(studioCapabilityContext(), &AIImageGenerateRequest{Model: "original", Prompt: "generate"}); err != nil {
		t.Fatalf("SubmitImageGeneration() error = %v", err)
	}
	if store.putCalls != 1 || store.binding.JobID != "job-a" || store.binding.RoutingKey != "route-model-a" || store.binding.ProviderID != "provider-a" {
		t.Fatalf("unexpected binding after submit: calls=%d binding=%+v", store.putCalls, store.binding)
	}

	if _, err := adapter.QueryImageGeneration(studioCapabilityContext(), "job-a"); err != nil {
		t.Fatalf("QueryImageGeneration() error = %v", err)
	}
	if store.getCalls != 1 || legacy.queryRoute != "route-model-a" || legacy.queryJobID != "job-a" {
		t.Fatalf("query did not use stored route: gets=%d route=%q job=%q", store.getCalls, legacy.queryRoute, legacy.queryJobID)
	}
}

func TestStudioAIImageCapabilityAdapterSubmitUsesResolvedConfigurationContext(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{
		supportsAsync: true,
		submit:        &AIImageAsyncSubmit{JobID: "job-a"},
	}
	store := &studioCapabilityAsyncJobStoreStub{}
	adapterValue, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy: legacy, Router: &studioCapabilityRouterStub{decision: routeDecision()}, Recorder: &studioCapabilityRecorderStub{},
		AsyncJobStore: store, Mode: aicapability.RoutingModeActive,
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}

	ctx := studioCapabilityContext()
	if _, err := adapterValue.(*studioAIImageCapabilityAdapter).SubmitImageGeneration(ctx, &AIImageGenerateRequest{Model: "original", Prompt: "generate"}); err != nil {
		t.Fatalf("SubmitImageGeneration() error = %v", err)
	}
	if legacy.submitIdentity != (RequestIdentity{TenantID: "tenant-a", UserID: "user-a"}) {
		t.Fatalf("submit identity = %+v", legacy.submitIdentity)
	}
	if legacy.submitContext.CredentialReference != "credential-a" || legacy.submitContext.ConfigurationVersion != "config-v1" {
		t.Fatalf("submit route context = %+v", legacy.submitContext)
	}
}

func TestStudioAIImageCapabilityAdapterQueryRecordsProviderFailureCategory(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{
		supportsAsync: true,
		query:         &AIImageAsyncResult{JobID: "job-a", Status: AIImageAsyncResultFailed, Error: "content rejected"},
	}
	store := &studioCapabilityAsyncJobStoreStub{binding: aicapability.AsyncJobBinding{
		JobID: "job-a", TenantID: "tenant-a", UserID: "user-a", Capability: aicapability.CapabilityListingKitStudioImage,
		Operation: aicapability.OperationAsyncImageGenerate, ProviderID: "provider-a", ModelID: "model-a", RoutingKey: "route-model-a", Status: "running",
	}}
	adapterValue, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy: legacy, Router: &studioCapabilityRouterStub{decision: routeDecision()}, Recorder: &studioCapabilityRecorderStub{},
		AsyncJobStore: store, Mode: aicapability.RoutingModeActive,
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}
	if _, err := adapterValue.(*studioAIImageCapabilityAdapter).QueryImageGeneration(context.Background(), "job-a"); err != nil {
		t.Fatalf("QueryImageGeneration() error = %v", err)
	}
	if store.statusCalls != 1 || store.status != string(AIImageAsyncResultFailed) || store.statusCategory != aicapability.ErrorProviderRejected {
		t.Fatalf("status update = calls=%d status=%q category=%q", store.statusCalls, store.status, store.statusCategory)
	}
}

func TestStudioAIImageCapabilityAdapterQueryErrorRecordsCategoryWithoutResponse(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{
		supportsAsync: true,
		err:           aicapability.NewError(aicapability.ErrorProviderUnavailable, string(aicapability.OperationAsyncImageQuery), errors.New("provider unavailable")),
	}
	store := &studioCapabilityAsyncJobStoreStub{binding: aicapability.AsyncJobBinding{
		JobID: "job-a", TenantID: "tenant-a", UserID: "user-a", Capability: aicapability.CapabilityListingKitStudioImage,
		Operation: aicapability.OperationAsyncImageGenerate, ProviderID: "provider-a", ModelID: "model-a", RoutingKey: "route-model-a", Status: "running",
	}}
	adapterValue, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy: legacy, Router: &studioCapabilityRouterStub{decision: routeDecision()}, Recorder: &studioCapabilityRecorderStub{},
		AsyncJobStore: store, Mode: aicapability.RoutingModeActive,
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}
	if _, err := adapterValue.(*studioAIImageCapabilityAdapter).QueryImageGeneration(context.Background(), "job-a"); err == nil {
		t.Fatal("expected query error")
	}
	if store.statusCalls != 1 || store.status != "running" || store.statusCategory != aicapability.ErrorProviderUnavailable {
		t.Fatalf("status update = calls=%d status=%q category=%q", store.statusCalls, store.status, store.statusCategory)
	}
}

func TestStudioAIImageCapabilityAdapterQueryRestoresSubmitIdentity(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{
		supportsAsync: true,
		query:         &AIImageAsyncResult{JobID: "job-a"},
	}
	store := &studioCapabilityAsyncJobStoreStub{binding: aicapability.AsyncJobBinding{
		JobID: "job-a", TenantID: "tenant-submit", UserID: "user-submit", Capability: aicapability.CapabilityListingKitStudioImage,
		Operation: aicapability.OperationAsyncImageGenerate, ProviderID: "provider-a", ModelID: "model-a", RoutingKey: "route-model-a",
		CredentialReference: "credential-a", ConfigurationVersion: "config-submit",
	}}
	adapterValue, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy: legacy, Router: &studioCapabilityRouterStub{decision: routeDecision()}, Recorder: &studioCapabilityRecorderStub{},
		AsyncJobStore: store, Mode: aicapability.RoutingModeActive,
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}
	adapter := adapterValue.(*studioAIImageCapabilityAdapter)
	currentCtx := WithRequestIdentity(context.Background(), RequestIdentity{TenantID: "tenant-current", UserID: "user-current"})

	if _, err := adapter.QueryImageGeneration(currentCtx, "job-a"); err != nil {
		t.Fatalf("QueryImageGeneration() error = %v", err)
	}
	if legacy.queryIdentity.TenantID != "tenant-submit" || legacy.queryIdentity.UserID != "user-submit" {
		t.Fatalf("query used current identity %+v, want submit identity", legacy.queryIdentity)
	}
}

func TestStudioAIImageCapabilityAdapterActiveMissingBindingFallsBackAndRecordsUnknownState(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{supportsAsync: true, query: &AIImageAsyncResult{JobID: "legacy-job"}}
	recorder := &studioCapabilityRecorderStub{}
	store := &studioCapabilityAsyncJobStoreStub{getErr: aicapability.ErrAsyncJobBindingNotFound}
	adapterValue, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy: legacy, Router: &studioCapabilityRouterStub{decision: routeDecision()}, Recorder: recorder,
		AsyncJobStore: store, Mode: aicapability.RoutingModeActive,
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}
	adapter := adapterValue.(*studioAIImageCapabilityAdapter)

	if _, err := adapter.QueryImageGeneration(studioCapabilityContext(), "legacy-job"); err != nil {
		t.Fatalf("QueryImageGeneration() error = %v", err)
	}
	if legacy.queryCalls != 1 || legacy.queryJobID != "legacy-job" {
		t.Fatalf("legacy fallback query calls=%d job=%q", legacy.queryCalls, legacy.queryJobID)
	}
	record := recorder.last(t)
	if record.ErrorCategory != aicapability.ErrorUnknownRemoteState || record.RouteOutcome != aicapability.RouteOutcomeLegacy {
		t.Fatalf("unexpected missing binding record: %+v", record)
	}
}

func TestStudioAIImageCapabilityAdapterActiveBindingFailureReturnsUnknownState(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{supportsAsync: true, submit: &AIImageAsyncSubmit{JobID: "job-a"}}
	recorder := &studioCapabilityRecorderStub{}
	store := &studioCapabilityAsyncJobStoreStub{putErr: errors.New("binding database unavailable")}
	adapterValue, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy: legacy, Router: &studioCapabilityRouterStub{decision: routeDecision()}, Recorder: recorder,
		AsyncJobStore: store, Mode: aicapability.RoutingModeActive,
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}
	adapter := adapterValue.(*studioAIImageCapabilityAdapter)

	response, err := adapter.SubmitImageGeneration(studioCapabilityContext(), &AIImageGenerateRequest{Model: "original", Prompt: "generate"})
	if response == nil || aicapability.CategoryOf(err) != aicapability.ErrorUnknownRemoteState {
		t.Fatalf("SubmitImageGeneration() = (%+v, %v), want remote-unknown error", response, err)
	}
	if record := recorder.last(t); record.ErrorCategory != aicapability.ErrorUnknownRemoteState || record.Outcome != aicapability.InvocationFailed {
		t.Fatalf("unexpected binding failure record: %+v", record)
	}
}

func TestStudioAIImageCapabilityAdapterActiveRejectsEmptyAsyncJobID(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{supportsAsync: true, submit: &AIImageAsyncSubmit{RequestID: "request-a"}}
	recorder := &studioCapabilityRecorderStub{}
	adapterValue, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy: legacy, Router: &studioCapabilityRouterStub{decision: routeDecision()}, Recorder: recorder,
		AsyncJobStore: &studioCapabilityAsyncJobStoreStub{}, Mode: aicapability.RoutingModeActive,
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}
	adapter := adapterValue.(*studioAIImageCapabilityAdapter)

	_, err = adapter.SubmitImageGeneration(studioCapabilityContext(), &AIImageGenerateRequest{Model: "original", Prompt: "generate"})
	if aicapability.CategoryOf(err) != aicapability.ErrorInvalidProviderResponse {
		t.Fatalf("SubmitImageGeneration() category = %q, want %q", aicapability.CategoryOf(err), aicapability.ErrorInvalidProviderResponse)
	}
}

func TestStudioAIImageCapabilityAdapterActiveRejectsNilAsyncSubmit(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{supportsAsync: true}
	recorder := &studioCapabilityRecorderStub{}
	adapterValue, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy: legacy, Router: &studioCapabilityRouterStub{decision: routeDecision()}, Recorder: recorder,
		AsyncJobStore: &studioCapabilityAsyncJobStoreStub{}, Mode: aicapability.RoutingModeActive,
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}

	_, err = adapterValue.(*studioAIImageCapabilityAdapter).SubmitImageGeneration(studioCapabilityContext(), &AIImageGenerateRequest{Model: "original", Prompt: "generate"})
	if aicapability.CategoryOf(err) != aicapability.ErrorInvalidProviderResponse {
		t.Fatalf("SubmitImageGeneration() category = %q, want %q", aicapability.CategoryOf(err), aicapability.ErrorInvalidProviderResponse)
	}
}

func TestStudioAIImageCapabilityAdapterAsyncBindingWriteSurvivesRequestCancellation(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{supportsAsync: true, submit: &AIImageAsyncSubmit{JobID: "job-a"}}
	store := &studioCapabilityAsyncJobStoreStub{requireLivePut: true}
	adapterValue, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy: legacy, Router: &studioCapabilityRouterStub{decision: routeDecision()}, Recorder: &studioCapabilityRecorderStub{},
		AsyncJobStore: store, Mode: aicapability.RoutingModeActive,
	})
	if err != nil {
		t.Fatalf("NewStudioAIImageCapabilityAdapter() error = %v", err)
	}

	ctx, cancel := context.WithCancel(studioCapabilityContext())
	cancel()
	if _, err := adapterValue.(*studioAIImageCapabilityAdapter).SubmitImageGeneration(ctx, &AIImageGenerateRequest{Model: "original", Prompt: "generate"}); err != nil {
		t.Fatalf("SubmitImageGeneration() error = %v", err)
	}
	if store.putCalls != 1 || store.putCtxErr != nil {
		t.Fatalf("binding write context = %v, calls=%d; want live context and one write", store.putCtxErr, store.putCalls)
	}
}

func TestStudioAIImageCapabilityAdapterUsesCancellationSafeRecordingContext(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{response: &AIImageResponse{}}
	recorder := &studioCapabilityRecorderStub{}
	adapter := newStudioCapabilityAdapterForTest(t, aicapability.RoutingModeShadow, legacy, &studioCapabilityRouterStub{decision: routeDecision()}, recorder)
	ctx, cancel := context.WithCancel(studioCapabilityContext())
	cancel()

	_, _ = adapter.GenerateImage(ctx, &AIImageGenerateRequest{Model: "model"})
	if recorder.ctxErr != nil {
		t.Fatalf("recording inherited cancelled context: %v", recorder.ctxErr)
	}
}

func TestNewStudioAIImageCapabilityAdapterLegacyAndValidation(t *testing.T) {
	legacy := &studioCapabilityGeneratorStub{}
	wrapped, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{Legacy: legacy, Mode: aicapability.RoutingModeLegacy})
	if err != nil || wrapped != legacy {
		t.Fatalf("legacy adapter = (%T, %v), want original generator", wrapped, err)
	}
	if _, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{}); aicapability.CategoryOf(err) != aicapability.ErrorInvalidInput {
		t.Fatalf("missing legacy error category = %q", aicapability.CategoryOf(err))
	}
	if _, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{Legacy: legacy, Mode: aicapability.RoutingModeActive}); aicapability.CategoryOf(err) != aicapability.ErrorInvalidInput {
		t.Fatalf("missing active dependencies error category = %q", aicapability.CategoryOf(err))
	}
}
