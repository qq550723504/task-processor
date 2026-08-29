# Image Agent external-effect finalization design

## Goal

Make Image Agent v3 cancellation and elapsed-budget handling preserve a durable provider-effect outcome before a run becomes terminal. A provider request that may already have happened must finish as `settled`, `provider_not_dispatched`, or `provider_outcome_unknown`; it must never be hidden by a cancelled child workflow or activity timeout.

## Scope and compatibility

This change applies only to new v3 Temporal histories. A new version marker keeps all previously recorded histories on their current deterministic path. No provider reconciliation API, provider-result lookup, or production deployment is included.

The existing provider-neutral effect repository remains the durable authority for provider claim, budget reservation, settlement, proven non-dispatch, and unknown outcomes. The implementation reuses its existing transitions rather than adding provider-specific storage.

## Design

### 1. Separate provider dispatch from finalization

`DeadlineAt` is the last instant at which a provider call may be dispatched. It is not the activity deadline.

For a budgeted v3 slot, the child workflow gives `ExecuteSlotV3` an activity `StartToCloseTimeout` of `min(10 minutes, remaining dispatch time + 60 seconds)`. The 60-second reconciliation grace is a fixed, bounded interval. The provider call continues to use a context whose deadline is exactly `DeadlineAt`.

If the provider context returns a proven pre-dispatch failure, the activity records `provider_not_dispatched` and returns the existing retryable stable code. If it returns any ambiguous error, including cancellation or provider-deadline expiry after the provider boundary, the activity persists `provider_outcome_unknown` and retains an unknown budget reservation when applicable. If it returns generated output, it settles the usage before later staging steps.

All repository writes that finalize a claimed provider effect use a derived 60-second finalization context that preserves request identity but is detached from the Temporal activity's cancellation signal. This context is used only for durable effect/budget transitions; it is never used to invoke a provider, upload artifacts, or continue publication.

If the detached finalization write itself cannot complete in the bounded interval, the activity returns an error and the effect remains claimed. The parent does not write a terminal cancellation projection. A later explicit recovery path must inspect that durable claim; the implementation must not invent a successful or cancelled outcome.

### 2. Cancellation becomes a two-step command saga

An accepted cancel command records its normal pending-command receipt and sets an in-memory cancellation intent, but does not persist `RunStatusCancelled` immediately.

The parent workflow stops launching new slot children and requests cancellation of in-flight children. It then waits for every started child to complete. The v3 child activity has the reconciliation grace described above, so each activity has time to durably record its provider-effect outcome after provider cancellation or expiry.

After every in-flight child has completed, the command saga writes the terminal cancelled projection. The public cancellation update returns after Temporal accepts the command rather than waiting for completion; the workbench observes the pending command and terminal state through projection/SSE. A cancellation that cannot finish finalization remains a resumable pending command and must not report a cancelled run.

### 3. Terminal intent distinguishes reversible and irreversible work

The workflow effect owner must not treat a failed terminal projection write as a successful terminal effect. It records a terminal intent only after the associated terminal projection succeeds.

Approval has an irreversible boundary: after `PublishApproved` succeeds, the approval record is in `approval_persist_complete` and only the same approval action may resume its terminal projection write. A cancel command must be rejected at this point, because it cannot reverse the already published external effect.

A failed action that has not crossed an irreversible boundary may still be superseded by cancellation. This preserves cancellation recovery for ordinary failed commands while avoiding a local `cancelled` projection that contradicts an externally published approval.

## Error handling and invariants

- Cancellation and elapsed time stop provider dispatch; they do not erase an already claimed provider effect.
- No final run status is written while a started v3 child can still have an unrecorded provider outcome.
- A provider outcome that cannot be proved absent or settled becomes `provider_outcome_unknown`, which remains cancel-only.
- Budget accounting changes only through the existing transactional effect repository methods.
- A completed external approval is never reclassified as cancelled. Its matching approval action remains the only recovery command until the completed projection is durable.
- All new workflow decisions are version-gated; old histories retain their exact activity timeout and terminal-intent behavior.

## Tests

Tests must first reproduce each failure before implementation:

1. Cancelling an in-flight provider call still records provider-unknown/budget-unknown with a cancelled activity context, and only then permits the cancelled projection.
2. A `DeadlineAt` reached during provider execution leaves reconciliation grace for the durable unknown transition; no activity timeout may map it to `slot_execution_failed` first.
3. Publication success followed by a completed-projection failure rejects a different cancel action and permits only resume of the original approval without republishing.
4. A failed terminal projection before any irreversible external effect can still be superseded by cancellation.
5. Replay tests prove that prior histories use their pre-finalization behavior and new histories select the new protocol.

Focused activity, child-workflow, parent-workflow, memory/GORM repository, and history-replay tests are required. Full local and remote CI remain separate verification gates.
