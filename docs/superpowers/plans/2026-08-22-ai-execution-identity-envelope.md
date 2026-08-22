# Unified Async AI Execution Identity Envelope Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Make one validated execution identity survive authenticated entry, durable task persistence, inline child AI calls, worker retries, and governed ProductEnrich/ProductImage/Amazon execution.

**Architecture:** Extend the existing provider-neutral internal/shared/aiidentity package with a versioned durable envelope and explicit capture/restore validation. Business task rows persist the envelope; queue payloads continue to route by task ID, and workers reload the task before restoring identity. HTTP routes declare explicit verified-identity policy, while platform adapters keep their own models and project the shared envelope at boundaries.

**Tech Stack:** Go, Gin, GORM, PostgreSQL schema migration through internal/app/schema/productlisting, existing worker pool/Redis submitters, Go testing.

**Spec:** docs/superpowers/specs/2026-08-22-ai-execution-identity-envelope-design.md

## Global Constraints

- Version 1 is the initial envelope schema.
- TenantID, UserID, and BusinessTaskID are required for governed execution.
- The persisted business task is authoritative after enqueue; transport identity is never trusted over a reloaded task row.
- Missing, malformed, or mismatched identity fails closed before provider routing and is not converted into generic fallback output.
- The envelope contains identity and trace metadata only; it never contains credentials, prompts, responses, or provider secrets.
- Schema changes are additive and must be deployable before writers; pre-migration tasks remain readable but cannot enter governed execution without repair.
- This implementation does not introduce LangGraph, replace Temporal/RabbitMQ, replace ZITADEL, or migrate unrelated platform tasks.

---

### Task 1: Add the shared versioned envelope contract

Files:
- Create internal/shared/aiidentity/envelope.go
- Create internal/shared/aiidentity/envelope_test.go
- Modify internal/shared/aiidentity/context.go

Interfaces:
- Consumes aiidentity.Identity, including verified tenant/user values projected by ListingKit ZITADEL middleware.
- Produces ExecutionEnvelope, PersistedExecutionEnvelope, CaptureExecutionEnvelope, Validate, WithExecutionEnvelope, ExecutionEnvelopeFromContext, and RestoreExecutionEnvelope.

- [ ] Step 1: Write failing contract tests.

Cover missing identity, unsupported version, missing tenant/user/task, task-ID mismatch, context round trip, and preservation of trace/source metadata. The mismatch assertion must use errors.Is(err, ErrIdentityIntegrity).

- [ ] Step 2: Run go test ./internal/shared/aiidentity and verify it fails because the envelope types and errors are absent.

- [ ] Step 3: Implement the minimal contract.

Define CurrentEnvelopeVersion = 1, ErrMissingIdentity, and ErrIdentityIntegrity. ExecutionEnvelope.Validate trims values, accepts only version 1, requires tenant/user/business task for governed execution, and validates bounded source metadata. CaptureExecutionEnvelope reads verified Identity and binds the supplied task ID. RestoreExecutionEnvelope validates the envelope and supplied task ID before projecting it through WithIdentity. PersistedExecutionEnvelope has GORM-safe fields ExecutionIdentityVersion, ExecutionTenantID, ExecutionUserID, ExecutionTraceID, ExecutionSourcePlatform, and ExecutionSourceTaskType, plus conversion methods that reject zero-version partial rows.

- [ ] Step 4: Run go test ./internal/shared/aiidentity and verify PASS.

- [ ] Step 5: Commit with git add internal/shared/aiidentity && git commit -m "feat: add async AI execution identity envelope".

### Task 2: Persist and capture envelopes on the three task models

Files:
- Modify internal/productenrich/model.go
- Modify internal/productenrich/service_task.go
- Modify internal/productenrich/identity_context.go
- Modify internal/productimage/domain/model.go
- Modify internal/productimage/service_task.go
- Modify internal/productimage/identity_context.go
- Modify internal/amazonlisting/model_types.go
- Modify internal/amazonlisting/service_task.go
- Modify internal/amazonlisting/workflow_listing.go
- Modify internal/productenrich/service_task_test.go
- Modify internal/productimage/service_task_test.go
- Modify internal/amazonlisting/service_task_test.go
- Modify internal/app/schema/productlisting/runtime_test.go

Interfaces:
- Consumes aiidentity.PersistedExecutionEnvelope and the authenticated execution context.
- Produces ExecutionEnvelope and SetExecutionEnvelope adapters on ProductEnrich, ProductImage, and Amazon tasks; authenticated task creation persists source metadata.

- [ ] Step 1: Write failing persistence tests.

For each model, assert that an authenticated context creates a version-1 envelope containing tenant, user, source platform/task type, and the new task ID. Assert that a legacy context still creates a readable zero-envelope task; do not synthesize tenant/user values. Assert model-to-envelope round trips.

- [ ] Step 2: Run go test ./internal/productenrich ./internal/productimage ./internal/amazonlisting and verify the new assertions fail.

- [ ] Step 3: Add additive model fields and capture adapters.

Embed aiidentity.PersistedExecutionEnvelope with the GORM embedded tag. In each create service, preserve existing ID generation, capture when a verified identity exists, and persist before repository creation. Use source values productenrich/product, productimage/image, and amazon/listing. Legacy creation remains readable but cannot later pass worker validation.

- [ ] Step 4: Run the product-listing AutoMigrate test against a legacy task table and assert all six additive columns exist while old rows remain readable. Use the existing internal/app/schema/productlisting/runtime.go AutoMigrate set.

- [ ] Step 5: Run go test ./internal/shared/aiidentity ./internal/productenrich ./internal/productimage ./internal/amazonlisting ./internal/app/schema/productlisting and verify PASS.

- [ ] Step 6: Commit with git add internal/shared/aiidentity internal/productenrich internal/productimage internal/amazonlisting internal/app/schema/productlisting && git commit -m "feat: persist async AI execution identity on task rows".

### Task 3: Restore identity in workers and inline child workflows

Files:
- Modify internal/productenrich/pipeline/processor.go
- Modify internal/productimage/pipeline/processor.go
- Modify internal/amazonlisting/workflow_processor.go
- Modify internal/amazonlisting/workflow_listing.go
- Modify internal/productenrich/service_process.go
- Modify internal/productimage/service_process.go
- Modify internal/amazonlisting/workflow_process_service.go
- Modify internal/productenrich/pipeline/processor_test.go
- Modify internal/productimage/pipeline/processor_test.go
- Modify internal/amazonlisting/workflow_test.go
- Create internal/shared/aiidentity/lifecycle_test.go

Interfaces:
- Consumes task ExecutionEnvelope adapters and RestoreExecutionEnvelope.
- Produces fail-closed worker entry and identity-preserving Amazon to ProductEnrich/ProductImage inline calls.

- [ ] Step 1: Write failing lifecycle tests.

Load a persisted Amazon task, restore tenant-a/user-a, and observe the same identity in ProductEnrich and ProductImage child services. Add retry tests proving a second worker load restores the same envelope. Add missing-envelope and task-ID-mismatch tests asserting ErrIdentityIntegrity, a failed task state, and zero provider calls.

- [ ] Step 2: Run go test ./internal/productenrich/pipeline ./internal/productimage/pipeline ./internal/amazonlisting and verify it fails because workers currently pass raw context.

- [ ] Step 3: Restore at every worker boundary.

After each processor reloads its task and before MarkProcessing or any governed call, invoke aiidentity.RestoreExecutionEnvelope(ctx, task.ExecutionEnvelope(), task.ID). On ErrIdentityIntegrity, mark the task failed with the stable identity_integrity: prefix and return the original error; do not retry this classification.

- [ ] Step 4: Propagate identity to inline children.

Call ProductEnrich and ProductImage Create...Task with the restored Amazon context so each child captures the same tenant/user, its own child task ID, and source metadata. Call ProcessProduct/ProcessImages with the restored child context. Reused completed child results retain their child ID and envelope metadata.

- [ ] Step 5: Guard governed service entry.

At the start of ProductEnrich and ProductImage processing, compare the task envelope with aiidentity.ExecutionEnvelopeFromContext. A missing or mismatched identity returns ErrIdentityIntegrity before runPipeline, image generation, or provider routing. Valid provider errors retain their existing retry/fallback behavior.

- [ ] Step 6: Run go test ./internal/shared/aiidentity ./internal/productenrich/pipeline ./internal/productimage/pipeline ./internal/amazonlisting and commit with git add internal/shared/aiidentity internal/productenrich internal/productimage internal/amazonlisting && git commit -m "feat: restore AI execution identity across workers".

### Task 4: Replace implicit AI route authorization with explicit policy metadata

Files:
- Modify internal/httproute/descriptor.go
- Modify internal/app/httpapi/server_auth.go
- Modify internal/listingkit/httpapi/zitadel_auth_route_authorization.go
- Modify internal/listingkit/httpapi/zitadel_auth_test.go
- Modify internal/productenrich/httpapi/routes.go
- Modify internal/amazonlisting/httpapi/routes.go
- Modify all internal/listingkit/httpapi/routes_descriptor_*.go files that declare protected modules
- Modify internal/sds/httpapi/http_module.go
- Modify internal/sdslogin/http_module.go
- Modify internal/sheinlogin/http_module.go
- Create internal/httproute/descriptor_test.go

Interfaces:
- Consumes explicit httproute.AuthPolicyVerifiedIdentity metadata.
- Produces route middleware decisions independent of a module-name allowlist; products, Amazon, and images are protected while health/readiness routes remain public.

- [ ] Step 1: Write failing policy tests.

Assert AuthPolicyVerifiedIdentity on ProductEnrich generate/result routes, all Amazon listing routes, and all ProductImage routes. Assert that public health/readiness routes do not receive ZITADEL middleware and that an untrusted tenant/user header cannot satisfy a verified route.

- [ ] Step 2: Run go test ./internal/httproute ./internal/listingkit/httpapi ./internal/app/httpapi ./internal/productenrich/httpapi ./internal/amazonlisting/httpapi and verify it fails because descriptors lack policy metadata and products/Amazon are absent from the current allowlist.

- [ ] Step 3: Add AuthPolicy and route declarations.

Add AuthPolicyPublic and AuthPolicyVerifiedIdentity to httproute.Descriptor. Mark every existing ZITADEL-protected ListingKit, SDS, Shein, crawler, product-sourcing, ProductEnrich, ProductImage, and Amazon descriptor explicitly. Mark health, readiness, system, management, and jobs public. RouteRequiresZitadelAuth returns only the explicit policy value; it no longer decides from module names.

- [ ] Step 4: Keep zitadel_auth_middleware.go as the only HTTP projection from verified ZITADEL identity into listingkit.AuthenticatedIdentity and shared aiidentity. Add the middleware projection test.

- [ ] Step 5: Run the focused route tests and commit with git add internal/httproute internal/app/httpapi internal/listingkit/httpapi internal/productenrich/httpapi internal/amazonlisting/httpapi internal/sds/httpapi internal/sdslogin internal/sheinlogin && git commit -m "feat: declare verified identity policy on AI routes".

### Task 5: Add governance observability and end-to-end acceptance coverage

Files:
- Modify the internal/aicapability governance recording path used by ProductEnrich/ProductImage
- Modify internal/productenrich/understanding.go
- Modify internal/productenrich/generator_json.go
- Modify internal/productenrich/service_process_test.go
- Modify internal/productimage/governed_scene_generator_test.go
- Modify internal/app/schema/productlisting/runtime_test.go
- Create internal/shared/aiidentity/acceptance_test.go

Interfaces:
- Consumes ErrIdentityIntegrity, task envelope restore, and the existing AI invocation ledger.
- Produces stable identity-integrity classification and proof that missing identity cannot become generic fallback output.

- [ ] Step 1: Write failing governance tests.

Assert ProductEnrich missing identity fails before its provider adapter is called, ProductImage missing identity fails before scene generation, and the invocation ledger records no successful provider invocation for either case. Assert valid-envelope provider timeout/error still follows the existing provider retry policy.

- [ ] Step 2: Run go test ./internal/aicapability/... ./internal/productenrich/... ./internal/productimage/... ./internal/shared/aiidentity and verify it fails because identity-integrity is not yet a stable failure category.

- [ ] Step 3: Implement the stable failure path.

Use one redacted error classification string, identity_integrity. Log only task ID, source platform/task type, and outcome. Update ProductEnrich extraction/generation handling so ErrIdentityIntegrity is returned rather than converted into generic analysis. Keep normal provider failures on their existing fallback/retry path. Reuse the existing AI invocation recorder for valid calls and add a counter/event for rejected identity-integrity attempts without writing secrets, prompts, or responses.

- [ ] Step 4: Add a full lifecycle acceptance test.

Start with one shared identity, execute an Amazon inline ProductEnrich call, retry a ProductImage worker, and compare tenant/user/business-task identity at every boundary. Run the product-listing AutoMigrate test against legacy task rows.

- [ ] Step 5: Run final verification.

    go test ./internal/shared/aiidentity ./internal/aicapability/... ./internal/productenrich/... ./internal/productimage/... ./internal/amazonlisting/... ./internal/httproute ./internal/listingkit/httpapi ./internal/app/httpapi ./internal/app/schema/productlisting
    go vet ./internal/shared/aiidentity ./internal/productenrich/... ./internal/productimage/... ./internal/amazonlisting/...
    git diff --check

Expected: all commands exit successfully; missing or mismatched envelopes make zero provider calls; the worktree contains only intended changes.

- [ ] Step 6: Commit with git add internal/shared/aiidentity internal/aicapability internal/productenrich internal/productimage internal/amazonlisting internal/httproute internal/listingkit/httpapi internal/app/httpapi internal/app/schema/productlisting && git commit -m "test: verify unified AI identity lifecycle".

## Self-review checklist

- Spec coverage: contract/versioning (Task 1), additive persistence and old-task behavior (Task 2), worker/retry/inline propagation (Task 3), explicit route policy (Task 4), fail-closed governance/metrics and acceptance (Task 5).
- No provider-specific identity contract is introduced; all three platforms adapt the existing shared package.
- Task ID remains the queue routing key and durable task row remains authoritative.
- LangGraph/Temporal remain unchanged and unrelated platform tasks are not migrated.

