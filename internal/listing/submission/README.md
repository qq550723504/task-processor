# Listing Submission

Owns submit, retry, recovery, state, and submission orchestration that is generic to listing flows.

Current stable ownership:

- generic submit attempt domain model for identity, target, action, status, phase, idempotency, remote ids, errors, and timing fields
- generic submission refresh orchestration seam (`RefreshStatus` style load/resolve/finish flow)
- generic task requeue orchestration seam (`RequeueTasks` style load/check/submit flow)
- generic immediate recovery orchestration seam (`RecoverNow` style load/recover/submit/reload flow)
- generic batch recovery orchestration seam (`RecoverBatch` style list/recover/submit/aggregate flow)
- generic recovered-submission route seam (`accepted/local completion vs remote confirmation` dispatch)
- generic lease-acquire seam (`begin lease -> replay preview / remote recovery / blocked mapping / task handoff`)
- generic workflow-start failure seam (`record failure -> clear lease -> return-priority resolution`)
- generic direct-submit phase orchestration seam (`DirectSubmit` style build/prepare/upload/pre-validate/submit flow)
- generic prepared-payload phase seam (`Prepare/Upload/PreValidate` stage flow)
- generic remote-submit attempt seam (`prepare state -> execute attempt -> shape result`)
- generic post-success persistence seam (`persist result/phase -> complete attempt -> remember -> persist success`)
- generic failure-record persistence seam (`record failure event/state`)
- generic remote refresh orchestration seam (`persist phase -> execute remote refresh -> record event -> finish`)
- source-facts readiness policy for 1688-derived facts
- in-process submit lock manager
- enqueue retry/backoff policy for queue-full submit retries
- response outcome policy for save-draft success and publish response errors
- phase detail mapping policy for submission phase events
- failure-state fallback policy for submission failure records
- remote-recovery lease expiry policy
- request-scoped remote-recovery predicate for same-request lease confirmation handoff
- active attempt lease policy
- in-flight clearing match policy
- submit-in-progress error shape and unwrap behavior
- submission event history policy: default event ID and recent-event retention
- attempt result status policy for success, failure, and unknown completion states
- submission event outcome policy: record metadata carry-over, response-note selection, and submit-error override
- submission projection policy: latest outcome selection, submit-phase event skipping, workflow status fallback, primary action record selection, and remote record summary projection
- generic readiness projection skeleton: carry readiness, checklist, submit-state, and status-overview assembly through one reusable projection bundle while platform packages supply the concrete builders
- phase event policy: default running status, default detail fallback, and error-message propagation
- remote record id normalization policy for confirm-remote event/result sync
- confirm-remote state policy: checked-at, message, and event remote-record-id normalization
- refresh mutation guard policy: action consistency and request-id consistency checks
- refresh selection policy: action fallback priority and supplier-code fallback
- refresh request-id normalization policy
- refresh remote policy: default-confirmed flag and fallback-message defaults
- action-record state policy: action slot selection and last-submission state synchronization
- action-record query policy: success-state checks plus generic selected-slot, status-scoped, and completed-record lookup by request id
- action-record query fallback policy: request-scoped started-at lookup and last-result fallback by action
- action-record mutation policy: request-id-guarded slot mutation for record updates
- remote-sync policy: always sync report remote status/check time before guarded record mutation
- attempt-record fallback policy: reuse matching request records or synthesize timing/attempt seeds from in-flight state
- in-flight state policy: begin/advance attempt state updates for current action, phase, lease, and attempt count
- attempt finalize policy: resolve final status, error message, and finished-at state for completion/failure writes
- attempt record draft policy: shape minimal record drafts from generic outcome/finalize state
- event draft policy: shape generic attempt/phase event fields before marketplace DTO assembly
- generic submit lock manager ownership for service-level coordination primitives
- generic source-facts, enqueue-retry, response-error, and in-flight TTL primitives for service-level submission coordination
- generic requeue task-id normalization for dedupe/trim request shaping
- generic submit-in-progress error ownership for non-SHEIN-specific API/service/Temporal callsites
- preferred submit-action selection policy for choosing the first supported action from ordered candidates
- exported retryable failure reason-code and default task recovery-scope constants for durable retry metadata
- root ListingKit retryable-block compatibility now consumes those exported reason-code and recovery-scope constants directly instead of keeping root-side retry metadata aliases

Does not own yet:

- full submit orchestration and platform routing
- SHEIN-specific submit package loading and remote confirmation details
- Temporal-facing submit workflow adapters beyond generic submit-in-progress error shaping
- durable retry/reblock persistence policies
