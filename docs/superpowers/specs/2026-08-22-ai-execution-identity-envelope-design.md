# Unified Async AI Execution Identity/Envelope

## Status

Proposed design for review. This document is intentionally limited to the
contract and rollout boundary; implementation starts only after approval.

## Goal

Define one provider-neutral execution identity that survives the complete
async AI lifecycle: authenticated HTTP entry, inline child workflows, queue
submission, worker retries, durable task reload, and downstream governed AI
calls. The first adopters are ProductEnrich, Amazon listing, and ProductImage;
the contract must also be usable by future listing platforms without copying
provider- or platform-specific identity helpers.

## Why the current boundary is insufficient

The repository already has `internal/shared/aiidentity.Identity`, but it is a
context-only value. It is projected from verified ZITADEL identity at some HTTP
boundaries and restored by ProductEnrich/ProductImage in selected paths. Queue
jobs and Redis fallback entries generally carry only a task ID. Amazon listing
tasks do not persist the caller identity even though their workflow reuses the
governed ProductEnrich service.

Consequently, a worker or inline child can reach a governed AI call without a
verifiable tenant/user identity. Current ProductEnrich extraction and
generation stages may log an error and fall back to generic analysis. That is
an unacceptable ambiguity for a governed call: missing identity must be a
distinct failure, not an output-quality fallback. Separately, route
authorization is currently based on a hard-coded module allowlist, which does
not make the products and Amazon routes' identity requirement explicit.

## Design principles

1. The persisted business task is authoritative after enqueue; queue payloads
   are transport hints, not an authority for identity.
2. One provider-neutral contract is shared by all platforms. Platform adapters
   map their task model to it; they do not create alternate envelopes.
3. Governed AI execution fails closed when required identity is absent or
   inconsistent.
4. Adoption is backward-compatible at the boundary, but old tasks are never
   silently assigned a guessed tenant or user.
5. The envelope contains identity and trace metadata only. It never contains
   credentials, prompts, model responses, or provider secrets.

## Proposed contract

Add a shared async execution package (name to be finalized during the
implementation plan) with a serializable envelope:

```go
type ExecutionEnvelope struct {
	Version         int    `json:"version"`
	TenantID        string `json:"tenant_id"`
	UserID          string `json:"user_id"`
	BusinessTaskID  string `json:"business_task_id"`
	TraceID         string `json:"trace_id,omitempty"`
	SourcePlatform  string `json:"source_platform"`
	SourceTaskType  string `json:"source_task_type"`
}
```

Version `1` is the initial schema. `TenantID`, `UserID`, and
`BusinessTaskID` are required for governed execution. `TraceID` is optional
and is used only for correlation. `SourcePlatform` and `SourceTaskType` are
operational metadata and must come from a bounded platform/task registry, not
arbitrary request input.

The package exposes these conceptual operations:

```go
CaptureExecutionEnvelope(ctx context.Context, taskID, platform, taskType string) (ExecutionEnvelope, error)
Validate() error
WithExecutionEnvelope(ctx context.Context, envelope ExecutionEnvelope) context.Context
ExecutionEnvelopeFromContext(ctx context.Context) (ExecutionEnvelope, bool)
RestoreExecutionEnvelope(ctx context.Context, envelope ExecutionEnvelope, taskID string) context.Context
```

`Capture` requires a verified shared identity in the context and binds the
business task ID. `Restore` validates that the persisted task ID and envelope
task ID agree before projecting the envelope into context. A mismatch is a
security/integrity failure and is never repaired by overwriting one side.

The existing `internal/shared/aiidentity.Identity` remains the small runtime
projection used by governed services. The new envelope is its durable and
transport-safe counterpart; conversion is explicit at capture/restore
boundaries.

## Persistence and transport

Persist the envelope on each business task that can directly or indirectly
invoke governed AI. Use a normalized, prefixed field group to avoid ORM field
collisions:

```go
type PersistedExecutionEnvelope struct {
	ExecutionIdentityVersion int
	ExecutionTenantID        string
	ExecutionUserID          string
	ExecutionTraceID         string
	ExecutionSourcePlatform  string
	ExecutionSourceTaskType  string
}
```

The task ID itself remains the business task key and is not duplicated as a
second authority. A small carrier interface (for example,
`GetExecutionEnvelope`/`SetExecutionEnvelope`) lets platform models embed or
adapt this field group without coupling the shared package to GORM models.

The first persistence adapters are ProductEnrich, Amazon listing, and
ProductImage. ListingKit/Studio tasks should project the verified identity
when they invoke a governed child, and persist it when their task itself is
the durable retry boundary. Other platforms are migrated when they actually
enter governed AI execution, not merely because they have a generic task
table.

Queue and Redis payloads continue to carry a task ID as the minimal routing
key. The worker loads the durable task, validates the envelope, and restores
the runtime context before calling the service. If a transport needs a
self-contained payload later, it may serialize the same envelope, but the
durable task remains the authority and the worker must compare both copies.
Retries and redeliveries therefore preserve the same envelope without
trusting mutable caller headers.

Inline child workflows create a new business task ID and capture the parent
verified identity into the child envelope. They may inherit the trace ID (or
create a child span), but must not reuse the parent business task ID.

## Route authorization

Replace the implicit module-name allowlist with explicit route policy metadata:

```go
type AuthPolicy int

const (
	AuthPolicyPublic AuthPolicy = iota
	AuthPolicyVerifiedIdentity
)
```

Products generation/result routes, Amazon listing routes, and ProductImage
generation routes declare `AuthPolicyVerifiedIdentity`. The middleware still
performs ZITADEL verification and projects `aiidentity`; the policy only makes
the requirement explicit and testable. Public health/readiness endpoints stay
public. Authorization remains tenant-scoped for task reads and mutations; the
envelope does not replace access control.

## Governance and failure semantics

Before provider routing, the governed AI execution boundary validates the
envelope. Missing, malformed, or mismatched identity returns a dedicated
identity-integrity error with stable classification and metrics. It must not
fall through to generic ProductEnrich analysis, synthetic text, or an
unattributed provider call.

Task processors should mark the task failed or needs-review according to the
existing task state model, retain the error classification, and allow an
operator to repair/retry only after the durable identity is corrected.
Provider errors may continue to use the existing retry/fallback policy, but
that policy is separate from identity-integrity failures.

## Adoption matrix

| Platform/path | Required change |
| --- | --- |
| ProductEnrich | Replace ad-hoc task identity restoration with capture/persist/restore adapters; make governed stages reject missing identity before fallback. |
| ProductImage | Adapt existing tenant/user task fields to the envelope and use the same worker restore path. |
| Amazon listing | Add envelope fields to its task, capture the verified caller identity, restore it in worker/inline ProductEnrich calls, and declare verified route policy. |
| ListingKit/Studio | Add projection adapters at governed child boundaries; persist only where the parent task is a retry boundary. |
| Shein/Temu/other platforms | Adopt when they invoke governed AI; no speculative migration of unrelated tasks. |

## Security and compatibility

- Never trust `X-Tenant-ID`, `X-User-ID`, queue payload identity, or client
  supplied source metadata as verified identity.
- Do not put secrets, access tokens, prompts, provider responses, or raw
  personal data in the envelope.
- Existing tasks created before migration have no inferred identity. They may
  be read for diagnosis, but governed execution must stop with the dedicated
  missing-identity classification until repaired or explicitly re-created
  through an authenticated path.
- Schema migration must be additive and deployable before code that writes the
  new fields; rollback leaves old task data readable and disables the new
  governed path rather than guessing identity.

## Verification strategy

The implementation must include:

1. Contract tests for validation, version handling, capture/restore, task-ID
   mismatch, and redacted serialization.
2. Lifecycle tests for ProductEnrich, ProductImage, and Amazon covering HTTP
   entry → durable task → queue/worker → governed child call.
3. Route-policy tests proving products/Amazon/ProductImage are rejected without
   verified ZITADEL identity while public health routes remain available.
4. Governance tests proving missing identity never invokes a provider and is
   not converted into generic fallback output.
5. Additive migration and old-task behavior tests.
6. Existing repository unit/integration checks plus focused race/static checks
   for the changed packages.

## Alternatives considered

**Queue-only envelope.** Rejected: retries and inline workflows can bypass the
queue, and a queue payload is not an authoritative source of identity.

**Per-platform identity fields and helpers.** Rejected: this is the current
drift pattern and would make every new platform another special case.

**Context-only propagation.** Rejected: context does not survive durable
reload, worker redelivery, or process boundaries.

**Recommended: shared envelope plus task-row authority and explicit route
policy.** This solves the lifecycle gap while keeping provider-specific
construction inside provider adapters and keeping platform models decoupled
from the shared contract.

## Non-goals

- This change does not introduce LangGraph or replace Temporal/RabbitMQ.
- It does not migrate every platform or redesign provider/model management.
- It does not replace ZITADEL authentication or tenant authorization.
- It does not persist prompts, responses, credentials, or model state.

## Rollout and acceptance

1. Land the shared contract, additive schema, and contract tests.
2. Add explicit route policies and identity-integrity metrics.
3. Adapt ProductEnrich, ProductImage, and Amazon worker/inline paths.
4. Enable each platform behind its existing runtime flags and verify no
   identity-missing execution reaches a provider.
5. Observe task failure/retry, identity-integrity, and fallback metrics before
   expanding to additional platforms.

Acceptance requires one traceable identity to remain intact from authenticated
request through an Amazon inline ProductEnrich call and through a retried
ProductImage worker, with missing identity failing closed in both cases.
