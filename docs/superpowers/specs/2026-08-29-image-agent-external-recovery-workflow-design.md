# Image Agent External Effect Recovery Workflow Design

## Goal

Give every v3 provider, staging, or publication effect that cannot be terminalized during cancellation an independent, durable recovery owner.

## Context

The main image-agent workflow must not project `cancelled` while an external effect is still claimed or its outcome is unknown. Keeping the main workflow open forever is not sufficient: the existing Update-based retry actions are unavailable when the effect proof is corrupt or only permits `cancel`. Recovery therefore moves to a separate Temporal workflow with an idempotent workflow identity.

## Decision

When the parent detects a claimed or unproven v3 effect that cannot be terminalized inside the bounded activity grace, it automatically starts `ImageAgentEffectRecoveryWorkflow` using a stable `(tenant, owner, run, plan revision, slot, attempt)` identity. The parent persists `recovery_requested` before or atomically with exposing the blocked projection; a start failure keeps the run blocked and retryable and never permits terminal `cancelled`.

The recovery workflow owns reconciliation and terminalization only. It never invokes the provider again. It reads the durable effect record and recovery bundle, retries idempotent staging/publication operations under bounded attempts, and persists one of `published`, `provider_unknown`, `staging_unknown`, `publication_unknown`, or `recovery_blocked`. It may be retriggered by an authenticated operator/API command using the same workflow ID; duplicate starts attach to the existing execution.

## Ownership and interfaces

- Main workflow: detects unsafe cancellation, persists blocked state and `recovery_requested`, then starts the recovery workflow with a deterministic ID.
- Recovery workflow: reconciles the exact effect identity; it may call artifact-store recovery, staging transitions, publication lease renewal/finalization/completion, and effect repositories, but not provider generation.
- Worker registration: recovery workflow and its activities are registered on the existing image-agent queue first; a later isolated queue is allowed but not required by this slice.
- API/application: exposes an authenticated `RecoverEffect` command for explicit re-drive. Authorization requires the same tenant/owner scope as the run plus the existing image-agent recovery permission; the client cannot supply workflow IDs, tenant IDs, or provider inputs.

## Lifecycle

1. Activity returns a non-terminal effect proof or bounded-finalization failure.
2. Parent writes a blocked projection containing the effect identity and recovery code.
3. Parent starts the recovery workflow with `WorkflowID = image-agent-effect-recovery:<tenant>:<owner>:<run>:<revision>:<slot>:<attempt>` and `UseExisting`/idempotent conflict handling.
4. Recovery workflow loads the effect and recovery bundle, reconciles observed provider/staging/publication state, and retries only idempotent durable operations.
5. Recovery persists a terminal/unknown/blocked effect phase and emits a durable recovery result.
6. The parent projection is refreshed by query/update or a completion signal; it can then expose retry/edit/cancel actions only when the recorded phase permits them.

## Failure and safety rules

- Recovery start errors are durable `recovery_start_failed` and leave the run blocked; they do not become `cancelled`.
- Duplicate starts are successful attaches to the existing deterministic workflow; no second provider or publication attempt is created.
- A missing or corrupt effect proof is fail-closed: recovery records `recovery_blocked` and exposes only operator re-drive/cancel according to policy.
- Provider calls are forbidden in the recovery workflow. Provider `DeadlineAt` is never extended.
- Every activity has bounded `StartToClose` and retry limits; exhaustion records an unknown/blocked effect phase rather than dropping ownership.
- All state writes are scoped by the exact effect identity and are idempotent under retries.

## Testing and acceptance

- Unit/workflow tests prove automatic start uses the deterministic ID, duplicate start attaches, and provider generation is never called.
- Failure tests prove start failure persists blocked/recovery-start-failed and never persists `cancelled`.
- Reconciliation tests cover lost claim/completion responses, cancellation, worker restart, and recovery bundle rehydration.
- Authorization tests prove cross-tenant/owner recovery is rejected and client-supplied workflow/provider fields are ignored.
- A Temporal workflow replay fixture covers the new parent decision and preserves legacy histories without recovery-start commands.

## Non-goals

- No provider re-generation, new provider capability policy, or deployment rollout is included.
- No automatic merge, production deployment, or runtime acceptance is performed by this design.

