# Image Review invocation admission evidence

Refs #334 / #130 / #126. This is a bounded admission report, **not an implementation or acceptance of governed Review**. Production code is unchanged. Rolling HEAD, CI and independent review evidence belong in the associated PR.

## Decision

`BLOCKED`: the real caller has no authoritative effective Organization binding. A Review-only change cannot meet the Issue's Home != effective Organization requirement. Do not populate that missing identity in a fixture and call it production evidence.

Finding: effective Organization is not resolved or durably captured by this caller.

Product requirement affected: verified effective scope, including Home != effective, must select credentials and invocation ownership without a new legacy mapping.

Classification: `BLOCKER`.

Reason: AGENTS category **core happy path cannot complete under the current design**. Requiring an effective Organization at the Review seam rejects every current activity; treating its TenantID as an effective Organization guesses a binding the command edge never established.

Action: the identity/application owner must define a separate bounded slice for command-edge organization resolution and durable execution-scope capture, including ownership of existing run/credential data. Reuse the existing workbench organization resolver; do not add IAM or tenantbridge consumers. Resume this slice once that binding reaches the actual activity, or once the coordinating owner explicitly approves the necessary revised scope. This report neither authorizes that cross-boundary work nor changes Temporal recovery.

## Actual call graph

Paths and line anchors below refer to the inspected baseline; the PR records the exact SHA.

1. `internal/imageagent/httpapi/routes.go:18`: ImageAgent routes use VerifiedIdentity without OrganizationAccessPolicy. `internal/app/httpapi/server_auth.go:91-109` therefore selects the existing identity middleware and does not run organization resolution.
2. `internal/authruntime/zitadel/verifier.go:89-104`: token resource owner becomes TenantID and HomeOrganizationID; EffectiveOrganizationID is absent. This is distinct from `internal/workbenchcontext/resolver.go:148-150`, which sets TenantID and effective Organization together after grant selection.
3. `internal/imageagent/service.go:591-597`: verifiedExecutionIdentity copies TenantID/UserID/trace. `internal/imageagent/ports.go:7` has no effective Organization or grants in ExecutionIdentity. `internal/imageagent/temporal/activities.go:100-112` restores that captured tenant/user shape.
4. Current v3 generation/review execution reaches `internal/imageagent/tools/product_image_executor.go:218` or staged review at `:301`. The Product Image review capability validates input/output and checks cancellation both before and after its backend (`internal/product/image/ports.go:168-200`). Review is model quality analysis, not approval.
5. `internal/app/worker/imageagent/capabilities.go`: buildProductionImageCapabilities constructs the routed provider with only Manager. Review resolves both image and default chat routes, compares supplied authorization metadata, gets the exact configured chat client and calls ProductImageAdapter.Review. There is no invocation recorder call here.
6. `internal/integration/openai/client_manager.go`: effective route resolution uses the credential resolver, with static configuration when no scoped override is found. GormCredentialResolver reads aiidentity TenantID/UserID, not a fresh effective Organization selection. GetClientWithRoute compares the selected configuration version before returning the concrete client.
7. `internal/integration/openai/product_image_adapter.go:166-228`: authorization check, multimodal request construction, real CreateChatCompletion, JSON decoding. Nil authorization is allowed by the shared adapter; this allowance is not the stricter #334 contract.
8. `internal/integration/openai/pool.go:128-182`: the BaseClient owns chat retries. MaxRetries defaults to 3; each attempt gets a child timeout, while the parent deadline remains the outer limit. No routed-provider fallback loop exists. A quote of one model call is not proof of one HTTP request.
9. `internal/app/worker/imageagent/dependencies.go` builds GormInvocationRecorder and passes it in imageCapabilityRuntime, but the capability builder does not consume it. `internal/aicapability/store/gorm_invocation_recorder.go:37-48` performs one INSERT and has no retries. Duplicate invocation IDs currently produce a database error, not an idempotent successful replay; no new replay semantics were introduced.

ExecutionPlanner, global policy/modes, fallback policy and generalized budgets are not connected by this route merely because their types exist. They remain in #130. Production pricing is `unpriced-v1`, CostUpperBoundKnown=false. The cost-budget-enabled caller rejects that quote (`product_image_executor.go:262`); the shared normalization/adapter alone does not. This distinction must survive any subsequent implementation.

## Failure contract and narrow review

The recorder wiring gap, nil authorization at this caller, and mismatch between quoted call count and underlying retry count are `IMPLEMENTATION_TEST` work after the scope blocker is resolved. Missing current-slice Must tests would still block merge.

For a future record failure after a successful model response, returning the original Review with an explicit safe structured degradation log can preserve the successful decision when the context is still active. Log record persistence failure distinctly; do not claim a write succeeded, retry the model, or fall back to another model to repair the record. For provider error plus record failure, preserve the provider outcome and report record degradation separately. This is a candidate contract, not delivered behavior.

There is a separate existing response-after-cancellation boundary: Product Image's post-backend context check replaces success with cancellation (`ports.go:180-183`). The executor classifies it as ProviderDispatchedUnknown (`product_image_executor.go:685-690`). The initial generation path retains staged candidates rather than regenerating them (`activities_execution_v3.go:205-228`). Staged Review can return a retryable ApplicationError (`activities_review_v3.go:178-179`). With budget authorization, reservation/outcome guards at `:103-126` constrain replay; without it, that guard does not run. Thus neither unconditional repeat nor unconditional no-repeat is established. This slice must not quietly rewrite that Temporal owner to claim otherwise.

## Controlled baseline probe

`baseline_probe_test.go.txt` is an opt-in **diagnostic for the inspected baseline**, stored as text so known defects do not become permanent desired-behavior regression assertions. Copy it into the imageagent worker package only in an isolated worktree to reproduce, then remove the temporary copy. It invokes buildProductionImageCapabilities -> real Manager -> real GormCredentialResolver -> actual ProductImageAdapter -> loopback HTTP, and configures the real GormInvocationRecorder. No fake Review result substitutes for the adapter. Its HTTP server returns a synthetic chat response and never downloads, generates, uploads or approves an image.

Use a disposable task-owned PostgreSQL container/database only. The probe creates credential/ledger tables and deliberately drops ai_invocations. **Never point ISSUE334_TEST_DSN at a shared or real database.** Credentials are synthetic; only loopback endpoints are used. First insert/count/delete a control invocation to prove the recorder works; then count actual invocation rows after each call.

```powershell
# Create a task-owned postgres:17.2-alpine container on a random loopback port.
# Set ISSUE334_TEST_DSN to that disposable database, with sslmode=disable.
Copy-Item docs/engineering/evidence/issue334/baseline_probe_test.go.txt internal/app/worker/imageagent/issue334_probe_test.go
go test ./internal/app/worker/imageagent -run '^TestIssue334BaselineProbe$' -count=1 -v -timeout=60s
Remove-Item -LiteralPath internal/app/worker/imageagent/issue334_probe_test.go
```

Observed baseline results (all counts are bottom-level HTTP requests):

| Diagnostic case | HTTP | Invocation rows | Result |
| --- | ---: | ---: | --- |
| Captured tenant/user, nil authorization | 1 | 0 | Success; no effective Organization proof |
| Empty scope, nil authorization | 1 | 0 | Success through configured static route |
| Unknown-cost authorization, matching unpriced route | 1 | 0 | Adapter accepts; not a cost-budget-enabled caller acceptance |
| Stale configuration authorization | 0 | 0 | Capability unsupported |
| Provider 503, MaxRetries=3 | 4 | 0 | Capability unavailable |
| Parent deadline | 1 | 0 | Deadline exceeded |
| Inbound cancellation | 0 | 0 | Canceled |
| Real recorder table dropped | 1 | Table absent | Review succeeds; direct recorder write fails |

The last case proves the recorder is unused, **not** that a newly connected recorder handles post-model write failure safely. No approved known-price full-caller happy path, response-after-cancel runtime proof, concurrent organization isolation, post-dispatch persistence-failure contract, sensitive-error log validation or full production regression is claimed. These remain uncompleted #334 acceptance items. Paid provider, real IAM, production and business data operations are NOT_RUN. No neighboring Product/Catalog, QA sample, BFF, frontend, customer billing or Resource Ledger changes were made.
