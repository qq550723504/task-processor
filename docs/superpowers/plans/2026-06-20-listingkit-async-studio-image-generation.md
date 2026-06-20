# ListingKit Async Studio Image Generation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add provider-capability-based async image generation to ListingKit batch studio generation so batch attempts can submit, persist provider job metadata, resume polling after process restarts, and materialize or fail safely without getting stuck in long-running synchronous waits.

**Architecture:** Keep existing synchronous provider support intact, but introduce a second async capability path in the image client contract. Batch generation becomes a small state machine: build request, submit async when supported, persist attempt metadata, poll later via recovery/worker logic, then materialize on success or mark failed/retryable on terminal failure. Providers that do not expose async capabilities continue to use the existing synchronous path.

**Tech Stack:** Go, existing ListingKit batch generation services, GORM-backed persistence, provider clients under `internal/infra/clients/openai` and `internal/infra/clients/nanobanana`, existing studio recovery support, Go tests.

---

## File Structure

**Modify**
- `internal/listingkit/ai_contracts.go`
  Why: extend image-generation contracts with explicit async submit/query capability and normalized async result types.
- `internal/listingkit/model_request_studio_support.go`
  Why: extend studio response models with async lifecycle metadata needed by batch execution and diagnostics.
- `internal/listingkit/studio_batch_model.go`
  Why: add attempt-level persistence fields/state needed for submit-vs-poll lifecycle.
- `internal/listingkit/studio_batch_generation.go`
  Why: split current synchronous “generate-and-materialize” flow into capability-aware submit/poll/materialize logic.
- `internal/listingkit/studio_batch_generation_recovery_support.go`
  Why: teach recovery to poll async attempts instead of only timing out stale `running` work.
- `internal/listingkit/task_studio_media_service.go`
  Why: preserve synchronous path and normalize provider metadata into shared response types.
- `internal/listingkit/httpapi/ai_image_generator_adapter.go`
  Why: adapt infra client async capabilities into ListingKit contracts.
- `internal/infra/clients/openai/types.go`
  Why: define provider-facing async submit/query request-response models.
- `internal/infra/clients/openai/images.go`
  Why: implement default “sync-only capability” behavior for official OpenAI-compatible image APIs.
- `internal/infra/clients/nanobanana/client.go`
  Why: expose true async submit/query operations instead of hiding polling entirely inside the sync call.
- `internal/listingkit/studio_batch_repository_gorm_update_support.go`
  Why: persist newly-added async attempt fields during updates.
- `internal/listingkit/studio_batch_repository_mem_support.go`
  Why: keep in-memory test repository behavior aligned with persisted attempt model.

**Test**
- `internal/listingkit/studio_batch_generation_test.go`
  Why: primary behavior coverage for batch submit/poll/recovery lifecycle.
- `internal/infra/clients/nanobanana/client_test.go`
  Why: provider async submit/query contract tests.
- `internal/infra/clients/openai/images_test.go`
  Why: verify sync-only providers still behave correctly and advertise no async support.

**Optional follow-up, not in first pass unless needed**
- `internal/listingkit/task_studio_batch_run_executor.go`
  Why: only if batch-run orchestration needs explicit handling for submitted-vs-polled attempts beyond existing continue/recover flow.

---

### Task 1: Add Async Image Capability Contracts

**Files:**
- Modify: `internal/listingkit/ai_contracts.go`
- Modify: `internal/infra/clients/openai/types.go`
- Test: `internal/infra/clients/openai/images_test.go`

- [ ] **Step 1: Write the failing test for sync-only provider capability**

Add a test in `internal/infra/clients/openai/images_test.go` asserting the OpenAI-compatible image client reports no async support and returns a typed “unsupported” error if async methods are called.

```go
func TestClientDoesNotSupportAsyncImageGenerationByDefault(t *testing.T) {
	client := NewClient(&ClientConfig{
		APIKey:     "test-key",
		Model:      "gpt-image-2",
		BaseURL:    "https://example.invalid",
		Timeout:    time.Second,
		MaxRetries: 0,
	})

	if client.SupportsAsyncImageGeneration() {
		t.Fatal("SupportsAsyncImageGeneration() = true, want false")
	}

	_, err := client.SubmitImageGeneration(context.Background(), &ImageGenerateRequest{
		Prompt: "flat artwork",
	})
	if !errors.Is(err, ErrAsyncImageGenerationNotSupported) {
		t.Fatalf("SubmitImageGeneration() error = %v, want ErrAsyncImageGenerationNotSupported", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/infra/clients/openai -run TestClientDoesNotSupportAsyncImageGenerationByDefault`

Expected: FAIL because async capability types/methods do not exist yet.

- [ ] **Step 3: Add normalized async contract types**

In `internal/listingkit/ai_contracts.go`, add:

```go
var ErrAsyncImageGenerationNotSupported = errors.New("async image generation is not supported")

type AIImageAsyncSubmit struct {
	JobID              string
	RequestID          string
	Provider           string
	RawSubmitResponse  string
	AcceptedAt         time.Time
}

type AIImageAsyncResultStatus string

const (
	AIImageAsyncResultQueued     AIImageAsyncResultStatus = "queued"
	AIImageAsyncResultRunning    AIImageAsyncResultStatus = "running"
	AIImageAsyncResultSucceeded  AIImageAsyncResultStatus = "succeeded"
	AIImageAsyncResultFailed     AIImageAsyncResultStatus = "failed"
)

type AIImageAsyncResult struct {
	JobID               string
	RequestID           string
	Provider            string
	Status              AIImageAsyncResultStatus
	RawResultResponse   string
	Error               string
	Usage               AIUsage
	Response            *AIImageResponse
}

type AIAsyncImageGenerator interface {
	SupportsAsyncImageGeneration() bool
	SubmitImageGeneration(ctx context.Context, req *AIImageGenerateRequest) (*AIImageAsyncSubmit, error)
	QueryImageGeneration(ctx context.Context, jobID string) (*AIImageAsyncResult, error)
}
```

In `internal/infra/clients/openai/types.go`, add the provider-facing equivalents:

```go
var ErrAsyncImageGenerationNotSupported = errors.New("async image generation is not supported")

type ImageAsyncSubmitResponse struct {
	JobID             string
	RequestID         string
	Provider          string
	RawSubmitResponse string
	AcceptedAt        time.Time
}

type ImageAsyncQueryResponse struct {
	JobID             string
	RequestID         string
	Provider          string
	Status            string
	RawResultResponse string
	Error             string
	Usage             Usage
	Data              []ImageData
}
```

- [ ] **Step 4: Add sync-only defaults to the OpenAI-compatible client**

In `internal/infra/clients/openai/images.go`, implement:

```go
func (c *Client) SupportsAsyncImageGeneration() bool {
	return false
}

func (c *Client) SubmitImageGeneration(context.Context, *ImageGenerateRequest) (*ImageAsyncSubmitResponse, error) {
	return nil, ErrAsyncImageGenerationNotSupported
}

func (c *Client) QueryImageGeneration(context.Context, string) (*ImageAsyncQueryResponse, error) {
	return nil, ErrAsyncImageGenerationNotSupported
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/infra/clients/openai -run TestClientDoesNotSupportAsyncImageGenerationByDefault`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/ai_contracts.go internal/infra/clients/openai/types.go internal/infra/clients/openai/images.go internal/infra/clients/openai/images_test.go
git commit -m "feat: add async image generation contracts"
```

### Task 2: Expose True Async Submit/Query in Nano Banana Client

**Files:**
- Modify: `internal/infra/clients/nanobanana/client.go`
- Test: `internal/infra/clients/nanobanana/client_test.go`

- [ ] **Step 1: Write the failing test for async submit**

Add a test proving Nano Banana can submit a job and return a provider job id without waiting for image download.

```go
func TestClientSubmitImageGenerationReturnsAsyncJobMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/api/generate" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		w.Header().Set("X-Request-Id", "req-submit-1")
		_ = json.NewEncoder(w).Encode(submitResponse{
			ID:     "job-async-1",
			Status: "running",
		})
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:    "test-key",
		Model:     "nano-banana-fast",
		SubmitURL: server.URL + "/v1",
		Timeout:   time.Second,
		HTTPClient: server.Client(),
	})

	result, err := client.SubmitImageGeneration(context.Background(), &openaiclient.ImageGenerateRequest{
		Prompt: "flat artwork",
	})
	if err != nil {
		t.Fatalf("SubmitImageGeneration() error = %v", err)
	}
	if result.JobID != "job-async-1" {
		t.Fatalf("job id = %q, want job-async-1", result.JobID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/infra/clients/nanobanana -run TestClientSubmitImageGenerationReturnsAsyncJobMetadata`

Expected: FAIL because no public async submit method exists yet.

- [ ] **Step 3: Implement async capability methods in Nano Banana**

In `internal/infra/clients/nanobanana/client.go`, expose:

```go
func (c *Client) SupportsAsyncImageGeneration() bool {
	return true
}

func (c *Client) SubmitImageGeneration(ctx context.Context, req *openaiclient.ImageGenerateRequest) (*openaiclient.ImageAsyncSubmitResponse, error) {
	// build submitRequest
	// call generate endpoint once
	// parse provider status
	// return JobID + RequestID + raw submit response
}

func (c *Client) QueryImageGeneration(ctx context.Context, jobID string) (*openaiclient.ImageAsyncQueryResponse, error) {
	// call /v1/api/result?id=<jobID>
	// normalize running/succeeded/failed response
	// do not download images until success
}
```

On `succeeded`, `QueryImageGeneration` should still download image bytes and fill `Data` so upper layers can reuse existing materialization logic with minimal changes.

- [ ] **Step 4: Keep existing synchronous API by composing async methods**

Refactor current synchronous `GenerateImage` and `EditImage` to:

```go
submit, err := c.SubmitImageGeneration(ctx, req)
if err != nil { ... }
for {
	result, err := c.QueryImageGeneration(ctx, submit.JobID)
	...
}
```

This preserves behavior for current callers while enabling batch async adoption.

- [ ] **Step 5: Add the failing test for async query success normalization**

Add:

```go
func TestClientQueryImageGenerationReturnsSucceededResultPayload(t *testing.T) {
	// fake /result returns succeeded with result url
	// assert QueryImageGeneration returns status=succeeded and populated Data
}
```

- [ ] **Step 6: Run focused tests**

Run: `go test ./internal/infra/clients/nanobanana -run "TestClientSubmitImageGenerationReturnsAsyncJobMetadata|TestClientQueryImageGenerationReturnsSucceededResultPayload"`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/infra/clients/nanobanana/client.go internal/infra/clients/nanobanana/client_test.go
git commit -m "feat: expose async nanobanana image generation"
```

### Task 3: Persist Async Attempt State in Batch Models

**Files:**
- Modify: `internal/listingkit/studio_batch_model.go`
- Modify: `internal/listingkit/studio_batch_repository_gorm_update_support.go`
- Modify: `internal/listingkit/studio_batch_repository_mem_support.go`
- Test: `internal/listingkit/studio_batch_generation_test.go`

- [ ] **Step 1: Write the failing test for async attempt persistence**

Add a test that updates an attempt with async submit metadata and expects the repository to retain it.

```go
func TestStudioBatchRepositoryPersistsAsyncAttemptMetadata(t *testing.T) {
	repo := NewMemStudioBatchRepository()
	ctx := WithTenantID(context.Background(), "tenant-a")
	now := time.Date(2026, 6, 20, 10, 0, 0, 0, time.UTC)

	seedStudioBatchGenerationGraph(t, repo, ctx, studioBatchGenerationSeed{
		batch: StudioBatchRecord{ID: "batch-1", Status: StudioBatchStatusGenerating},
		items: []StudioBatchItemRecord{{ID: "item-1", BatchID: "batch-1", Status: StudioBatchItemStatusGenerating}},
		attempts: []StudioGenerationAttemptRecord{{
			ID: "attempt-1", ItemID: "item-1", AttemptNo: 1, Status: StudioGenerationAttemptStatusRunning,
		}},
	})

	err := repo.UpdateStudioGenerationAttempt(ctx, &StudioGenerationAttemptRecord{
		ID: "attempt-1",
		ItemID: "item-1",
		AttemptNo: 1,
		Status: StudioGenerationAttemptStatusQueued,
		UpstreamJobID: "job-1",
		RequestPayload: "{\"prompt\":\"x\"}",
		ResultPayload: "{\"status\":\"running\"}",
		ErrorMessage: "",
	})
	if err != nil {
		t.Fatalf("UpdateStudioGenerationAttempt() error = %v", err)
	}

	detail, _ := repo.GetStudioBatchDetail(ctx, "batch-1")
	if detail.AttemptsByItem["item-1"][0].UpstreamJobID != "job-1" {
		t.Fatalf("upstream job id mismatch")
	}
}
```

- [ ] **Step 2: Run test to verify it fails if new state is missing**

Run: `go test ./internal/listingkit -run TestStudioBatchRepositoryPersistsAsyncAttemptMetadata`

Expected: FAIL if repository/model state is insufficient.

- [ ] **Step 3: Extend attempt status model**

In `internal/listingkit/studio_batch_model.go`, add explicit async lifecycle statuses:

```go
const (
	StudioGenerationAttemptStatusQueued      StudioGenerationAttemptStatus = "queued"
	StudioGenerationAttemptStatusRunning     StudioGenerationAttemptStatus = "running"
	StudioGenerationAttemptStatusSubmitted   StudioGenerationAttemptStatus = "submitted"
	StudioGenerationAttemptStatusPolling     StudioGenerationAttemptStatus = "polling"
	StudioGenerationAttemptStatusSucceeded   StudioGenerationAttemptStatus = "succeeded"
	StudioGenerationAttemptStatusMaterialized StudioGenerationAttemptStatus = "materialized"
	StudioGenerationAttemptStatusFailed      StudioGenerationAttemptStatus = "failed"
	StudioGenerationAttemptStatusCancelled   StudioGenerationAttemptStatus = "cancelled"
)
```

Also add:

```go
Provider              string     `json:"provider,omitempty" gorm:"type:varchar(64)"`
SubmitResponsePayload string     `json:"submit_response_payload,omitempty" gorm:"type:text"`
RequestID             string     `json:"request_id,omitempty" gorm:"type:varchar(128);index"`
ResultCheckedAt       *time.Time `json:"result_checked_at,omitempty"`
QueryAttempts         int        `json:"query_attempts" gorm:"not null;default:0"`
```

- [ ] **Step 4: Update repository persistence code**

Modify GORM and in-memory update paths to read/write the new fields inside `UpdateStudioGenerationAttempt`.

- [ ] **Step 5: Run focused repository test**

Run: `go test ./internal/listingkit -run TestStudioBatchRepositoryPersistsAsyncAttemptMetadata`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/listingkit/studio_batch_model.go internal/listingkit/studio_batch_repository_gorm_update_support.go internal/listingkit/studio_batch_repository_mem_support.go internal/listingkit/studio_batch_generation_test.go
git commit -m "feat: persist async studio batch attempt state"
```

### Task 4: Teach the HTTP Adapter to Expose Async Capability

**Files:**
- Modify: `internal/listingkit/httpapi/ai_image_generator_adapter.go`
- Test: `internal/listingkit/studio_batch_generation_test.go`

- [ ] **Step 1: Write the failing test for adapter async delegation**

Add a small adapter-focused test stub inside `internal/listingkit/studio_batch_generation_test.go` or a nearby test file asserting that a provider with async support gets surfaced through the ListingKit interface.

```go
func TestListingKitAIImageGeneratorAdapterDelegatesAsyncCapability(t *testing.T) {
	// stub provider advertises async support and returns job-1
	// assert adapted generator.SupportsAsyncImageGeneration() == true
	// assert SubmitImageGeneration returns job-1
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestListingKitAIImageGeneratorAdapterDelegatesAsyncCapability`

Expected: FAIL because adapter only exposes sync methods.

- [ ] **Step 3: Extend adapter with async methods**

In `internal/listingkit/httpapi/ai_image_generator_adapter.go`, add:

```go
func (g listingKitAIImageGenerator) SupportsAsyncImageGeneration() bool
func (g listingKitAIImageGenerator) SubmitImageGeneration(ctx context.Context, req *listingkit.AIImageGenerateRequest) (*listingkit.AIImageAsyncSubmit, error)
func (g listingKitAIImageGenerator) QueryImageGeneration(ctx context.Context, jobID string) (*listingkit.AIImageAsyncResult, error)
```

Map provider response fields into ListingKit async types without losing `request_id`, `raw_*_response`, `usage`, or normalized result data.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/listingkit -run TestListingKitAIImageGeneratorAdapterDelegatesAsyncCapability`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/httpapi/ai_image_generator_adapter.go internal/listingkit/studio_batch_generation_test.go
git commit -m "feat: expose async image capability through listingkit adapter"
```

### Task 5: Split Batch Generation Into Submit and Materialize Phases

**Files:**
- Modify: `internal/listingkit/studio_batch_generation.go`
- Test: `internal/listingkit/studio_batch_generation_test.go`

- [ ] **Step 1: Write the failing test for async submit path**

Add a test proving a batch item with an async-capable generator stores submit metadata and remains in a non-terminal state without immediate materialization.

```go
func TestRunPendingStudioBatchItemsSubmitsAsyncAttemptWithoutImmediateMaterialization(t *testing.T) {
	// arrange item pending
	// async-capable image generator returns job-1
	// run generator
	// assert attempt status=submitted
	// assert item status=generating
	// assert no materialized designs yet
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestRunPendingStudioBatchItemsSubmitsAsyncAttemptWithoutImmediateMaterialization`

Expected: FAIL because current logic always waits for final response.

- [ ] **Step 3: Refactor batch execution flow**

In `internal/listingkit/studio_batch_generation.go`, split the current item execution path into:

```go
func (g *studioBatchGenerationService) runItemAttempt(...) error
func (g *studioBatchGenerationService) submitAsyncItemAttempt(...) error
func (g *studioBatchGenerationService) executeSyncItemAttempt(...) error
func (g *studioBatchGenerationService) finalizeSuccessfulAttempt(...) error
```

Decision rule:

```go
if asyncGenerator, ok := g.executeAsyncCapable(); ok && asyncGenerator.SupportsAsyncImageGeneration() {
	return g.submitAsyncItemAttempt(...)
}
return g.executeSyncItemAttempt(...)
```

`submitAsyncItemAttempt` must:
- create the attempt row
- submit to provider
- persist `provider`, `request_id`, `upstream_job_id`, `submit_response_payload`
- set attempt status to `submitted`
- leave item in `generating`

- [ ] **Step 4: Run focused test**

Run: `go test ./internal/listingkit -run TestRunPendingStudioBatchItemsSubmitsAsyncAttemptWithoutImmediateMaterialization`

Expected: PASS

- [ ] **Step 5: Add the failing test for sync fallback**

Add:

```go
func TestRunPendingStudioBatchItemsFallsBackToSyncWhenAsyncUnsupported(t *testing.T) {
	// use sync-only generator
	// assert designs materialize immediately as before
}
```

- [ ] **Step 6: Run both tests**

Run: `go test ./internal/listingkit -run "TestRunPendingStudioBatchItemsSubmitsAsyncAttemptWithoutImmediateMaterialization|TestRunPendingStudioBatchItemsFallsBackToSyncWhenAsyncUnsupported"`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/listingkit/studio_batch_generation.go internal/listingkit/studio_batch_generation_test.go
git commit -m "feat: split studio batch submit and materialize phases"
```

### Task 6: Add Async Polling Recovery for Submitted Attempts

**Files:**
- Modify: `internal/listingkit/studio_batch_generation_recovery_support.go`
- Modify: `internal/listingkit/studio_batch_generation.go`
- Test: `internal/listingkit/studio_batch_generation_test.go`

- [ ] **Step 1: Write the failing test for polling a submitted attempt to success**

```go
func TestRecoverGeneratingAsyncAttemptPollsProviderAndMaterializesOnSuccess(t *testing.T) {
	// arrange attempt status=submitted, upstream_job_id=job-1
	// async query returns succeeded with one image
	// call RecoverStudioBatchMaterialization
	// assert item -> review_ready and design persisted
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestRecoverGeneratingAsyncAttemptPollsProviderAndMaterializesOnSuccess`

Expected: FAIL because recovery only knows stale timeout and stored result-payload flows.

- [ ] **Step 3: Implement polling-aware recovery**

In `internal/listingkit/studio_batch_generation_recovery_support.go`, add logic before stale-timeout fallback:

```go
case StudioGenerationAttemptStatusSubmitted, StudioGenerationAttemptStatusPolling:
	return g.pollAsyncAttemptResult(ctx, batch, item, attempt)
```

Implement:

```go
func (g *studioBatchGenerationService) pollAsyncAttemptResult(...) error {
	// query provider by upstream_job_id
	// update query_attempts/result_checked_at
	// if queued/running -> keep generating and return nil
	// if failed -> fail item and attempt
	// if succeeded -> persist result payload and materialize
}
```

- [ ] **Step 4: Run success-path test**

Run: `go test ./internal/listingkit -run TestRecoverGeneratingAsyncAttemptPollsProviderAndMaterializesOnSuccess`

Expected: PASS

- [ ] **Step 5: Add the failing test for terminal async failure**

```go
func TestRecoverGeneratingAsyncAttemptMarksFailedOnProviderFailure(t *testing.T) {
	// query returns failed + error
	// assert item failed, attempt failed, error_message recorded
}
```

- [ ] **Step 6: Run both polling tests**

Run: `go test ./internal/listingkit -run "TestRecoverGeneratingAsyncAttemptPollsProviderAndMaterializesOnSuccess|TestRecoverGeneratingAsyncAttemptMarksFailedOnProviderFailure"`

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add internal/listingkit/studio_batch_generation_recovery_support.go internal/listingkit/studio_batch_generation.go internal/listingkit/studio_batch_generation_test.go
git commit -m "feat: recover async studio generation attempts"
```

### Task 7: Protect Against Stale Async Attempts With No Job ID

**Files:**
- Modify: `internal/listingkit/studio_batch_generation_recovery_support.go`
- Test: `internal/listingkit/studio_batch_generation_test.go`

- [ ] **Step 1: Write the failing test for stuck submitted attempt without job id**

```go
func TestRecoverSubmittedAttemptWithoutJobIDFailsCleanly(t *testing.T) {
	// submitted attempt with blank upstream_job_id
	// recovery should fail item with explicit diagnostic
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/listingkit -run TestRecoverSubmittedAttemptWithoutJobIDFailsCleanly`

Expected: FAIL because no dedicated branch exists.

- [ ] **Step 3: Add guardrail**

Implement:

```go
if strings.TrimSpace(attempt.UpstreamJobID) == "" {
	return g.failItemAndAttempt(ctx, item, attempt, "async generation attempt missing upstream job id")
}
```

This prevents the exact “running forever but cannot query upstream” failure mode from surviving future restarts.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/listingkit -run TestRecoverSubmittedAttemptWithoutJobIDFailsCleanly`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/listingkit/studio_batch_generation_recovery_support.go internal/listingkit/studio_batch_generation_test.go
git commit -m "fix: fail async attempts missing upstream job ids"
```

### Task 8: Full Verification

**Files:**
- Modify: none
- Test: `internal/infra/clients/openai/images_test.go`
- Test: `internal/infra/clients/nanobanana/client_test.go`
- Test: `internal/listingkit/studio_batch_generation_test.go`

- [ ] **Step 1: Run targeted package suites**

Run:

```bash
go test ./internal/infra/clients/openai ./internal/infra/clients/nanobanana ./internal/listingkit
```

Expected: PASS

- [ ] **Step 2: Sanity-check no accidental API break in local backend build**

Run:

```bash
go build ./cmd/product-listing-api
```

Expected: build succeeds with no compile errors.

- [ ] **Step 3: Review diff for schema/compatibility risk**

Run:

```bash
git diff --stat HEAD~8..HEAD
```

Expected: only planned files changed; no unrelated work included.

- [ ] **Step 4: Commit any final cleanup**

```bash
git add internal/infra/clients/openai internal/infra/clients/nanobanana internal/listingkit
git commit -m "chore: verify async studio image generation support"
```

---

## Self-Review

- Spec coverage: this plan covers capability abstraction, provider implementations, persistence, batch submit/materialize split, async recovery, and guardrails for missing upstream job ids. No product/UI changes are included because the requested scope was backend capability support.
- Placeholder scan: no `TODO`/`TBD` placeholders remain; each task names exact files and commands.
- Type consistency: normalized names use `AIImageAsyncSubmit`, `AIImageAsyncResult`, `ImageAsyncSubmitResponse`, `ImageAsyncQueryResponse`, `SubmitImageGeneration`, and `QueryImageGeneration` consistently through all tasks.
