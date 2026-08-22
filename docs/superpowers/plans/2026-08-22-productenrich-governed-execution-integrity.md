# ProductEnrich Governed Execution Integrity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make ProductEnrich rollout, routing, legacy fallback, caching, audit, tenant scoping, and persisted identity obey one set of fail-closed invariants.

**Architecture:** Add a provider-neutral execution plan and cache status contract, then make ProductEnrich prepare one bound execution before cache lookup or provider invocation. Legacy and active calls share the same recorder path; score caches use the bound tenant/route/prompt identity; repository and persisted-envelope invariants move into shared helpers used by every implementation.

**Tech Stack:** Go 1.x, GORM, Redis-compatible cache interface, Kubernetes Job YAML, GitHub Actions workflow tests, PowerShell/Pester, shell static tests.

**Spec:** `docs/superpowers/specs/2026-08-22-productenrich-governed-execution-integrity-design.md`

## Global Constraints

- Rollout exclusion is `legacy`, never `policy_denied`.
- Missing or malformed identity fails before planning, cache lookup, or provider calls.
- Every executed or cached path emits one truthful record; no false failed-active record precedes legacy success.
- Provider/client-manager types remain outside `internal/aicapability`.
- Governed cache keys include tenant, capability, operation, route, policy/configuration, prompt, base score, and input identity.
- Verified-tenant repository access returns not-found for cross-tenant tasks.
- Legacy tasks remain readable without guessed identity but cannot enter governed execution.
- No LangGraph, Temporal, RabbitMQ, Redis, or provider-adapter replacement is part of this plan.

---

### Task 1: Add provider-neutral execution-plan and cache-audit contracts

**Files:**
- Create: `internal/aicapability/execution_plan.go`
- Create: `internal/aicapability/execution_plan_test.go`
- Modify: `internal/aicapability/invocation.go`
- Modify: `internal/aicapability/store/gorm_invocation_recorder.go`
- Modify: `internal/aicapability/store/gorm_invocation_recorder_test.go`
- Modify: `internal/app/schema/productlisting/runtime_test.go`

**Interfaces:**
- Consumes: existing `RoutingMode`, `RouteOutcome`, `RouteDecision`, `RouteRequest`, and `Router`.
- Produces: `ExecutionPlan`, `ExecutionPlanner`, `CacheStatus`, and `InvocationRecord.CacheStatus` for Tasks 2 and 3.

- [ ] **Step 1: Write failing contract tests**

Add tests that compile against the desired API and prove plan normalization and cache-status persistence:

```go
func TestExecutionPlanValidateAcceptsActiveAndLegacyPlans(t *testing.T) {
    active := ExecutionPlan{
        Mode: RoutingModeActive, RouteOutcome: RouteOutcomeActive,
        Decision: RouteDecision{
            Capability: CapabilityProductEnrichText,
            Operation: OperationProductEnrichTextExtract,
            ProviderID: "openai", ModelID: "gpt", RoutingKey: "fast", CredentialReference: "fast",
        },
    }
    require.NoError(t, active.Validate())

    legacy := ExecutionPlan{
        Mode: RoutingModeLegacy, RouteOutcome: RouteOutcomeLegacy,
        LegacyClients: []string{"fast", "default"},
    }
    require.NoError(t, legacy.Validate())
}

func TestExecutionPlanValidateRejectsDeniedOrUnboundExecutablePlan(t *testing.T) {
    require.Error(t, (ExecutionPlan{Mode: RoutingModeActive}).Validate())
    require.Error(t, (ExecutionPlan{Mode: RoutingModeLegacy}).Validate())
}
```

Extend the GORM recorder test to record `CacheStatusHit`, reload the row, and assert `cache_status == "hit"`. Extend the product-listing schema test to assert the invocation ledger contains the additive `cache_status` column.

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
go test ./internal/aicapability ./internal/aicapability/store ./internal/app/schema/productlisting
```

Expected: FAIL because `ExecutionPlan`, `CacheStatus`, and `cache_status` do not exist.

- [ ] **Step 3: Implement the minimal shared contracts**

Create:

```go
type CacheStatus string

const (
    CacheStatusNotApplicable CacheStatus = "not_applicable"
    CacheStatusHit           CacheStatus = "hit"
    CacheStatusMiss          CacheStatus = "miss"
)

type ExecutionPlan struct {
    Mode          RoutingMode
    RouteOutcome  RouteOutcome
    Decision      RouteDecision
    LegacyClients []string
}

type ExecutionPlanner interface {
    Plan(context.Context, RouteRequest) (ExecutionPlan, error)
}
```

`ExecutionPlan.Validate` accepts only a fully bound active decision or a legacy plan with at least one normalized client candidate. Add `CacheStatus CacheStatus` to `InvocationRecord`, map it to `invocationRow.CacheStatus` with `gorm:"column:cache_status;size:32"`, and default blank values to `CacheStatusNotApplicable` in row conversion.

- [ ] **Step 4: Run focused tests and verify GREEN**

```powershell
go test ./internal/aicapability ./internal/aicapability/store ./internal/app/schema/productlisting
```

Expected: PASS.

- [ ] **Step 5: Commit Task 1**

```powershell
git add -- internal/aicapability/execution_plan.go internal/aicapability/execution_plan_test.go internal/aicapability/invocation.go internal/aicapability/store/gorm_invocation_recorder.go internal/aicapability/store/gorm_invocation_recorder_test.go internal/app/schema/productlisting/runtime_test.go
git commit -m "feat: define governed execution plans"
```

---

### Task 2: Replace policy-denied fallback with one ProductEnrich execution lifecycle

**Files:**
- Create: `internal/productenrich/enrich/governed_execution.go`
- Create: `internal/productenrich/enrich/governed_execution_test.go`
- Modify: `internal/productenrich/enrich/text_governance.go`
- Modify: `internal/productenrich/enrich/text_governance_test.go`
- Modify: `internal/productenrich/enrich/image_governance.go`
- Modify: `internal/productenrich/enrich/image_governance_test.go`
- Modify: `internal/productenrich/httpapi/ai_capability_text_catalog.go`
- Modify: `internal/productenrich/httpapi/ai_capability_text_catalog_test.go`
- Modify: `internal/app/httpapi/runtime_productenrich.go`
- Modify: `internal/app/httpapi/runtime_ai_capability_test.go`

**Interfaces:**
- Consumes: `aicapability.ExecutionPlan`, `ExecutionPlanner`, `InvocationRecorder`, ProductEnrich `LLMManager`, and tenant-aware client-config resolver metadata.
- Produces: one package-private `preparedExecution` lifecycle; Task 3 exposes only a parent-package scoring interface, avoiding a parent-to-child import cycle.

- [ ] **Step 1: Write failing rollout and lifecycle tests**

Add table-driven tests for these observable behaviors:

```go
type staticExecutionPlanner struct {
    plan aicapability.ExecutionPlan
    err  error
}

func (p staticExecutionPlanner) Plan(context.Context, aicapability.RouteRequest) (aicapability.ExecutionPlan, error) {
    return p.plan, p.err
}

type staticLegacyRouteMetadataResolver struct{}

func (staticLegacyRouteMetadataResolver) ResolveLegacyRoute(_ context.Context, clientName string) (aicapability.RouteDecision, error) {
    return aicapability.RouteDecision{
        ProviderID: "openai", ModelID: "legacy-model", RoutingKey: clientName,
        CredentialReference: clientName, ConfigurationVersion: clientName + "-config-v1",
    }, nil
}

func TestGovernedTextLegacyUsesDefaultAndRecordsOneSuccess(t *testing.T) {
    manager := &routedTextManager{defaultResponse: "legacy response"}
    recorder := &textInvocationRecorder{}
    generator, err := productenrichenrich.NewGovernedTextGenerator(manager, productenrichenrich.GovernedTextGeneratorConfig{
        Planner: staticExecutionPlanner{plan: aicapability.ExecutionPlan{
            Mode: aicapability.RoutingModeLegacy, RouteOutcome: aicapability.RouteOutcomeLegacy,
            LegacyClients: []string{"fast", "default"},
        }},
        LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
    })
    require.NoError(t, err)

    ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-legacy", UserID: "user-a"})
    got, err := generator.Generate(ctx, "prompt")
    require.NoError(t, err)
    require.Equal(t, "legacy response", got)
    require.Len(t, recorder.records, 1)
    require.Equal(t, aicapability.RoutingModeLegacy, recorder.records[0].RouteMode)
    require.Equal(t, aicapability.RouteOutcomeLegacy, recorder.records[0].RouteOutcome)
    require.Equal(t, aicapability.InvocationSucceeded, recorder.records[0].Outcome)
}

func TestGovernedTextPolicyDeniedDoesNotCallLegacyProvider(t *testing.T) {
    planner := staticExecutionPlanner{err: aicapability.NewError(aicapability.ErrorPolicyDenied, "text_extract", nil)}
    manager := &routedTextManager{response: "active", legacyResponse: "named", defaultResponse: "default"}
    recorder := &textInvocationRecorder{}
    generator, constructErr := productenrichenrich.NewGovernedTextGenerator(manager, productenrichenrich.GovernedTextGeneratorConfig{
        Planner: planner, LegacyRouteMetadata: staticLegacyRouteMetadataResolver{}, Recorder: recorder,
    })
    require.NoError(t, constructErr)

    ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-active", UserID: "user-a"})
    _, err := generator.Generate(ctx, "prompt")
    require.Equal(t, aicapability.ErrorPolicyDenied, aicapability.CategoryOf(err))
    require.False(t, manager.called)
    require.False(t, manager.legacyCalled)
}
```

Extend the existing `routedTextManager`/`routedImageManager` fixtures with `defaultResponse`; make `GetDefaultClient` return the corresponding legacy client only when that field is non-empty. Change `textInvocationRecorder` and `imageInvocationRecorder` from one `record` field to `records []aicapability.InvocationRecord`, updating existing assertions to index the sole record. Mirror the legacy success/failure and real-denial cases for image analysis. Assert a failed legacy provider produces exactly one failed legacy record. Update catalog tests so allowlist exclusion is handled by an execution planner returning `Mode=legacy`; active router policy tests continue to reject genuine invalid capability/operation requests.

- [ ] **Step 2: Run focused tests and verify RED**

```powershell
go test ./internal/productenrich/enrich ./internal/productenrich/httpapi ./internal/app/httpapi
```

Expected: FAIL because current generators record `policy_denied` before an unrecorded fallback and have no planner/prepared-execution API.

- [ ] **Step 3: Implement tenant rollout planning**

In the ProductEnrich HTTP adapter, implement an `ExecutionPlanner` that owns the active tenant set and delegates only active tenants to the existing `Router`:

```go
func (p tenantRolloutPlanner) Plan(ctx context.Context, request aicapability.RouteRequest) (aicapability.ExecutionPlan, error) {
    if _, active := p.activeTenantIDs[strings.TrimSpace(request.TenantID)]; !active {
        plan := aicapability.ExecutionPlan{
            Mode: aicapability.RoutingModeLegacy,
            RouteOutcome: aicapability.RouteOutcomeLegacy,
            LegacyClients: append([]string(nil), p.legacyClients...),
        }
        return plan, plan.Validate()
    }
    decision, err := p.router.Decide(ctx, request)
    if err != nil {
        return aicapability.ExecutionPlan{}, err
    }
    plan := aicapability.ExecutionPlan{
        Mode: aicapability.RoutingModeActive,
        RouteOutcome: aicapability.RouteOutcomeActive,
        Decision: decision,
    }
    return plan, plan.Validate()
}
```

Expose it as:

```go
func BuildProductEnrichExecutionPlanner(
    router aicapability.Router,
    activeTenantIDs []string,
    legacyClients []string,
) aicapability.ExecutionPlanner
```

Remove `allowedTenantIDs` from all six `BuildProductEnrich*CapabilityRouter` signatures and from their policy resolvers. Those routers receive only active requests and continue validating tenant/capability/operation through `PolicyRouter`; rollout selection belongs exclusively to `BuildProductEnrichExecutionPlanner`.

- [ ] **Step 4: Implement one prepared execution and recorder path**

`governed_execution.go` should prepare identity, plan, and client binding before returning a private execution object:

```go
type LegacyRouteMetadataResolver interface {
    ResolveLegacyRoute(context.Context, string) (aicapability.RouteDecision, error)
}

type preparedExecution struct {
    identity aiidentity.Identity
    plan aicapability.ExecutionPlan
    decision aicapability.RouteDecision
    promptKey, promptVersion, promptScope string
    prompt, input string
    call func(context.Context) (string, error)
    recorder aicapability.InvocationRecorder
}

func (g *governedTextGenerator) prepare(context.Context, string) (*preparedExecution, error)
func (a *governedImageAnalyzer) prepare(context.Context, string, string) (*preparedExecution, error)
```

For active plans, bind through `RoutedLLMManager.GetClientWithRoute`. For legacy plans, try each named candidate in order and use `GetDefaultClient` for the `default` candidate when named lookup fails. Resolve provider/model/configuration metadata through `LegacyRouteMetadataResolver`; fail with `credential_unavailable` rather than performing an unattributed provider call. If planning or binding fails, record at most one rejected execution with no token/cost usage; when every legacy candidate is unavailable, the record must retain `Mode=legacy`/`RouteOutcome=legacy` and the terminal categorized error.

`preparedExecution.invoke(ctx, cacheStatus)` times the real call and writes exactly one record. `Generate` and `AnalyzeImage` become convenience methods that call `Prepare...` then `invoke(..., CacheStatusNotApplicable)`. Remove the old `policy_denied` fallback branches and the duplicated record construction from text/image files.

- [ ] **Step 5: Wire runtime planners and metadata resolvers**

Replace generator configuration fields `Router` and `FallbackClient` with `Planner aicapability.ExecutionPlanner` and `LegacyRouteMetadata LegacyRouteMetadataResolver`. The planner owns the ordered legacy candidates, while the metadata resolver maps the selected client name to provider/model/routing/configuration identity using the existing OpenAI client-config resolver. Add this adapter constructor in `internal/productenrich/httpapi`:

```go
func BuildProductEnrichLegacyRouteMetadataResolver(
    resolver openaiclient.ClientConfigResolver,
) productenrichenrich.LegacyRouteMetadataResolver
```

Runtime candidates are:

```go
text understanding:  []string{"fast", "default"}
vision understanding: []string{"vision", "default"}
listing/fusion:       []string{"default"}
quality text:         unique(scorerClientName(cfg, "fast"), "default")
quality image:        unique(scorerClientName(cfg, "vision"), "default")
```

Keep client/config resolution in `internal/productenrich/httpapi` and `internal/app/httpapi`; do not import OpenAI/Gemini types into `internal/aicapability`.

- [ ] **Step 6: Run focused tests and verify GREEN**

```powershell
go test ./internal/productenrich/enrich ./internal/productenrich/httpapi ./internal/app/httpapi
```

Expected: PASS with one record per active or legacy execution and zero provider calls on genuine denial.

- [ ] **Step 7: Commit Task 2**

```powershell
git add -- internal/productenrich/enrich internal/productenrich/httpapi/ai_capability_text_catalog.go internal/productenrich/httpapi/ai_capability_text_catalog_test.go internal/app/httpapi/runtime_productenrich.go internal/app/httpapi/runtime_ai_capability_test.go
git commit -m "refactor: unify ProductEnrich execution lifecycle"
```

---

### Task 3: Move governed scoring cache behind the prepared route

**Files:**
- Create: `internal/productenrich/governed_scoring.go`
- Create: `internal/productenrich/governed_scoring_test.go`
- Modify: `internal/productenrich/llm_score_cache.go`
- Create: `internal/productenrich/llm_score_cache_test.go`
- Modify: `internal/productenrich/llm_scorer.go`
- Modify: `internal/productenrich/llm_scorer_unit_test.go`
- Modify: `internal/productenrich/enrich/governed_execution.go`
- Modify: `internal/productenrich/enrich/governed_execution_test.go`

**Interfaces:**
- Consumes: Task 2 package-private `preparedExecution` through methods implemented by the governed text/image adapters.
- Produces: parent-package `GovernedScoreExecution`, `TextExecutionPreparer`, `ImageExecutionPreparer`, `ScoreCacheIdentity`, governed cache get/set methods, and cache-hit recording. Defining these in `internal/productenrich` lets the scorer consume them without importing its `enrich` child package.

- [ ] **Step 1: Write failing cache-partition tests**

Add a typed identity and tests proving every governance dimension changes the key:

```go
func TestScoreCacheIdentityPartitionsGovernedExecutions(t *testing.T) {
    base := ScoreCacheIdentity{
        Version: 1, TenantID: "tenant-a",
        Capability: aicapability.CapabilityProductEnrichText,
        Operation: aicapability.OperationProductEnrichTextQualityScore,
        RouteMode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
        ProviderID: "openai", ModelID: "gpt-4.1-mini", RoutingKey: "fast",
        PolicyVersion: "policy-v1", ConfigurationVersion: "config-v1",
        PromptKey: "productenrich.llm_scorer.text_scoring", PromptVersion: "v1", PromptScope: "product_enrich",
        BaseScore: "80", InputHash: "input-a",
    }
    variants := []struct {
        name   string
        mutate func(*ScoreCacheIdentity)
    }{
        {"schema version", func(v *ScoreCacheIdentity) { v.Version = 2 }},
        {"tenant", func(v *ScoreCacheIdentity) { v.TenantID = "tenant-b" }},
        {"capability", func(v *ScoreCacheIdentity) { v.Capability = aicapability.CapabilityProductEnrichVision }},
        {"operation", func(v *ScoreCacheIdentity) { v.Operation = aicapability.OperationProductEnrichVisionQualityScore }},
        {"route mode", func(v *ScoreCacheIdentity) { v.RouteMode = aicapability.RoutingModeLegacy }},
        {"route outcome", func(v *ScoreCacheIdentity) { v.RouteOutcome = aicapability.RouteOutcomeLegacy }},
        {"provider", func(v *ScoreCacheIdentity) { v.ProviderID = "gemini" }},
        {"model", func(v *ScoreCacheIdentity) { v.ModelID = "gemini-2.5-flash" }},
        {"routing key", func(v *ScoreCacheIdentity) { v.RoutingKey = "quality" }},
        {"policy version", func(v *ScoreCacheIdentity) { v.PolicyVersion = "policy-v2" }},
        {"configuration version", func(v *ScoreCacheIdentity) { v.ConfigurationVersion = "config-v2" }},
        {"prompt key", func(v *ScoreCacheIdentity) { v.PromptKey = "other.prompt" }},
        {"prompt version", func(v *ScoreCacheIdentity) { v.PromptVersion = "prompt-v2" }},
        {"prompt scope", func(v *ScoreCacheIdentity) { v.PromptScope = "other_scope" }},
        {"base score", func(v *ScoreCacheIdentity) { v.BaseScore = "81" }},
        {"input hash", func(v *ScoreCacheIdentity) { v.InputHash = "different-input" }},
    }
    for _, tc := range variants {
        t.Run(tc.name, func(t *testing.T) {
            variant := base
            tc.mutate(&variant)
            require.NotEqual(t, base.Key(), variant.Key())
        })
    }
}
```

Add scorer tests with the same raw input for tenant A and tenant B. Assert the second tenant invokes its prepared execution instead of receiving tenant A's cached score. Add a same-tenant cache-hit test asserting the provider is skipped but one record is emitted with `CacheStatusHit`, zero token/cost counters, and correct route/prompt metadata.

- [ ] **Step 2: Run focused tests and verify RED**

```powershell
go test ./internal/productenrich ./internal/productenrich/enrich
```

Expected: FAIL because cache methods accept only raw text/image keys and cache lookup occurs before prepared execution.

- [ ] **Step 3: Implement the versioned governed cache identity**

Add to `internal/productenrich/governed_scoring.go`:

```go
type GovernedScoreExecution interface {
    ScoreCacheIdentity(baseScore, inputHash string) ScoreCacheIdentity
    Invoke(context.Context, aicapability.CacheStatus) (string, error)
    RecordCacheHit(context.Context, string) error
}

type TextExecutionPreparer interface {
    PrepareText(context.Context, string) (GovernedScoreExecution, error)
}

type ImageExecutionPreparer interface {
    PrepareImage(context.Context, string, string) (GovernedScoreExecution, error)
}

type ScoreCacheIdentity struct {
    Version int
    TenantID string
    Capability aicapability.Capability
    Operation aicapability.Operation
    RouteMode aicapability.RoutingMode
    RouteOutcome aicapability.RouteOutcome
    ProviderID, ModelID, RoutingKey string
    PolicyVersion, ConfigurationVersion string
    PromptKey, PromptVersion, PromptScope string
    BaseScore string
    InputHash string
}
```

The unexported Task 2 `preparedExecution` implements `GovernedScoreExecution`; its text/image owners expose `PrepareText`/`PrepareImage` methods returning the parent-package interface. `Key()` requires a positive schema version, normalizes strings, serializes the fields in a fixed struct order, hashes with SHA-256, and returns `llm_score:governed:v<Version>:<hex>`. Add `GetGovernedScoreResult` and `SetGovernedScoreResult` to `LLMScoreCache`. Keep raw text/image cache methods only for paths without a governed preparer; governed paths never read their namespace.

- [ ] **Step 4: Prepare before cache lookup and record hits/misses**

In `scoreTextResult` and `scoreImageResult`:

1. resolve the scoring prompt;
2. type-assert the generator/analyzer to the Task 2 preparer interface;
3. prepare the bound execution;
4. derive `ScoreCacheIdentity` from the prepared route plus base score and SHA-256 input hash;
5. on hit, call `prepared.recordCacheHit(ctx, cachedScoreString)` and return the cached score;
6. on miss, call `prepared.invoke(ctx, CacheStatusMiss)`, parse the provider response, then store the parsed score under the governed identity.

The legacy ungoverned branch retains the existing raw cache behavior. Do not call `scoreWithCache` around a governed generator.

- [ ] **Step 5: Run focused tests and verify GREEN**

```powershell
go test ./internal/productenrich ./internal/productenrich/enrich
```

Expected: PASS; governed cache hits occur only after identity/route preparation and are tenant/config/prompt/base-score isolated.

- [ ] **Step 6: Commit Task 3**

```powershell
git add -- internal/productenrich/governed_scoring.go internal/productenrich/governed_scoring_test.go internal/productenrich/llm_score_cache.go internal/productenrich/llm_score_cache_test.go internal/productenrich/llm_scorer.go internal/productenrich/llm_scorer_unit_test.go internal/productenrich/enrich/governed_execution.go internal/productenrich/enrich/governed_execution_test.go
git commit -m "fix: partition governed scoring caches"
```

---

### Task 4: Make tenant scoping a repository conformance contract

**Files:**
- Create: `internal/shared/aiidentity/tenant_scope.go`
- Create: `internal/shared/aiidentity/tenant_scope_test.go`
- Modify: `internal/amazonlisting/store/mem_store.go`
- Modify: `internal/amazonlisting/store/task_repo.go`
- Create: `internal/amazonlisting/store/task_repo_contract_test.go`

**Interfaces:**
- Consumes: verified `aiidentity.Identity` from context and persisted `execution_tenant_id`.
- Produces: `TenantMatchesContext(context.Context, string) bool`, used by all Amazon repository implementations.

- [ ] **Step 1: Write failing shared and repository contract tests**

Test shared semantics:

```go
func TestTenantMatchesContext(t *testing.T) {
    scoped := WithIdentity(context.Background(), Identity{TenantID: "tenant-a"})
    require.True(t, TenantMatchesContext(scoped, "tenant-a"))
    require.False(t, TenantMatchesContext(scoped, "tenant-b"))
    require.True(t, TenantMatchesContext(context.Background(), "tenant-b"))
}
```

Build a repository contract helper and run it against both GORM and memory factories. Create tenant A/B tasks, then assert tenant A cannot get, list, mark processing, update, retry, or save results for tenant B; every cross-tenant mutation returns `ErrTaskNotFound` and leaves tenant B unchanged.

- [ ] **Step 2: Run tests and verify RED**

```powershell
go test ./internal/shared/aiidentity ./internal/amazonlisting/store
```

Expected: FAIL for the memory implementation, whose get/list/mutations ignore tenant context.

- [ ] **Step 3: Implement and apply the shared contract**

```go
func TenantMatchesContext(ctx context.Context, persistedTenantID string) bool {
    tenantID := strings.TrimSpace(FromContext(ctx).TenantID)
    return tenantID == "" || tenantID == strings.TrimSpace(persistedTenantID)
}
```

Use this predicate in memory `GetTask`, `ListTasks`, and one internal `taskForUpdate(ctx, taskID)` helper called by every mutation. Keep GORM's SQL filter but derive the tenant through the same normalized helper function so both stores follow identical semantics.

- [ ] **Step 4: Run tests and verify GREEN**

```powershell
go test ./internal/shared/aiidentity ./internal/amazonlisting/store ./internal/amazonlisting/...
```

Expected: PASS for both repository factories and existing Amazon workflows.

- [ ] **Step 5: Commit Task 4**

```powershell
git add -- internal/shared/aiidentity/tenant_scope.go internal/shared/aiidentity/tenant_scope_test.go internal/amazonlisting/store/mem_store.go internal/amazonlisting/store/task_repo.go internal/amazonlisting/store/task_repo_contract_test.go
git commit -m "fix: enforce Amazon tenant scope consistently"
```

---

### Task 5: Centralize persisted-envelope absent/partial/present state

**Files:**
- Modify: `internal/shared/aiidentity/envelope.go`
- Modify: `internal/shared/aiidentity/envelope_test.go`
- Modify: `internal/productenrich/service_process.go`
- Modify: `internal/productenrich/service_process_test.go`
- Modify: `internal/productimage/service_process.go`
- Modify: `internal/productimage/service_process_test.go`

**Interfaces:**
- Consumes: all six persisted execution-envelope fields.
- Produces: `PersistedEnvelopeState`, `PersistedExecutionEnvelope.State()`, and a single integrity decision for service guards.

- [ ] **Step 1: Write failing state-classification tests**

```go
func TestPersistedEnvelopeStateRejectsTraceOnlyRow(t *testing.T) {
    persisted := PersistedExecutionEnvelope{ExecutionTraceID: "trace-only"}
    require.Equal(t, PersistedEnvelopePartial, persisted.State())
    _, err := persisted.ExecutionEnvelope("task-a")
    require.ErrorIs(t, err, ErrIdentityIntegrity)
}

func TestPersistedEnvelopeStateDistinguishesAbsentAndPresent(t *testing.T) {
    require.Equal(t, PersistedEnvelopeAbsent, (PersistedExecutionEnvelope{}).State())
    require.Equal(t, PersistedEnvelopePresent, PersistedExecutionEnvelopeFrom(validEnvelope()).State())
}
```

Add ProductEnrich and ProductImage tests proving a trace-only task fails before `WithTaskIdentity`, pipeline execution, or provider invocation.

- [ ] **Step 2: Run tests and verify RED**

```powershell
go test ./internal/shared/aiidentity ./internal/productenrich ./internal/productimage
```

Expected: FAIL because trace-only rows are currently treated as absent.

- [ ] **Step 3: Implement the classifier and remove field enumeration**

```go
type PersistedEnvelopeState int

const (
    PersistedEnvelopeAbsent PersistedEnvelopeState = iota
    PersistedEnvelopePartial
    PersistedEnvelopePresent
)
```

`State()` returns absent only when version, tenant, user, trace, platform, and task type are all zero. It returns present only when conversion validates a supported version and required fields; all other rows are partial. `ExecutionEnvelope` returns a zero envelope only for absent, returns `ErrIdentityIntegrity` for partial, and validates present rows. ProductEnrich/ProductImage guards switch on `State()` instead of enumerating fields.

- [ ] **Step 4: Run tests and verify GREEN**

```powershell
go test ./internal/shared/aiidentity ./internal/productenrich ./internal/productimage
```

Expected: PASS with trace-only rows rejected before execution.

- [ ] **Step 5: Commit Task 5**

```powershell
git add -- internal/shared/aiidentity/envelope.go internal/shared/aiidentity/envelope_test.go internal/productenrich/service_process.go internal/productenrich/service_process_test.go internal/productimage/service_process.go internal/productimage/service_process_test.go
git commit -m "fix: classify partial AI execution envelopes"
```

---

### Task 6: Correct image prompt attribution and bound product schema migration

**Files:**
- Modify: `internal/productenrich/enrich/image_governance.go`
- Modify: `internal/productenrich/enrich/image_governance_test.go`
- Modify: `internal/app/httpapi/runtime_productenrich.go`
- Modify: `internal/app/httpapi/runtime_ai_capability_test.go`
- Modify: `deployments/kubernetes/listingkit-workbench/jobs/product-listing-api-schema-migrate-job.yaml`
- Modify: `tests/listingkit_deploy_workflow_test.go`
- Modify: `scripts/tests/listingkit-schema-migrate-job-test.sh`

**Interfaces:**
- Consumes: Task 2 unified prepared execution and existing prompt constants.
- Produces: configurable image prompt metadata and a 900-second Kubernetes Job deadline.

- [ ] **Step 1: Write failing prompt and deadline tests**

Extend image governance tests:

```go
func TestGovernedVisionQualityRecordsQualityPromptIdentity(t *testing.T) {
    recorder := &imageInvocationRecorder{}
    manager := &routedImageManager{response: `{"score":90}`}
    analyzer, constructErr := productenrichenrich.NewGovernedImageAnalyzer(manager, productenrichenrich.GovernedImageAnalyzerConfig{
        Planner: staticExecutionPlanner{plan: aicapability.ExecutionPlan{
            Mode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
            Decision: aicapability.RouteDecision{
                Capability: aicapability.CapabilityProductEnrichVision,
                Operation: aicapability.OperationProductEnrichVisionQualityScore,
                ProviderID: "openai", ModelID: "vision-model", RoutingKey: "vision",
                CredentialReference: "vision", PolicyVersion: "policy-v1", ConfigurationVersion: "config-v1",
            },
        }},
        Recorder: recorder,
        PromptKey: "productenrich.llm_scorer.image_scoring", PromptVersion: "v1", PromptScope: "product_enrich",
    })
    require.NoError(t, constructErr)
    ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{TenantID: "tenant-a", UserID: "user-a"})
    _, err := analyzer.AnalyzeImage(ctx, "https://image", "score prompt")
    require.NoError(t, err)
    require.Equal(t, "productenrich.llm_scorer.image_scoring", recorder.records[0].PromptKey)
    require.Equal(t, "v1", recorder.records[0].PromptVersion)
    require.Equal(t, "product_enrich", recorder.records[0].PromptScope)
}
```

Add `TestProductListingSchemaMigrationJobDeadlineMatchesDriverWait` beside the existing ListingKit deadline test and add a shell assertion:

```bash
grep -q 'activeDeadlineSeconds: 900' "$product_manifest"
```

- [ ] **Step 2: Run tests and verify RED**

```powershell
go test ./internal/productenrich/enrich ./internal/app/httpapi ./tests
bash scripts/tests/listingkit-schema-migrate-job-test.sh
```

Expected: FAIL because image metadata is hard-coded to understanding and the product migration Job has no deadline.

- [ ] **Step 3: Implement prompt configuration and deadline**

Add `PromptKey`, `PromptVersion`, and `PromptScope` to `GovernedImageAnalyzerConfig`, default them to the existing understanding constants, and store them on the prepared execution. The quality-image runtime configuration supplies:

```go
PromptKey:     prompt.KProductEnrichLlmScorerImageScoring,
PromptVersion: "v1",
PromptScope:   "product_enrich",
```

Add directly under the product migration Job `spec`:

```yaml
activeDeadlineSeconds: 900
```

- [ ] **Step 4: Run focused tests and verify GREEN**

```powershell
go test ./internal/productenrich/enrich ./internal/app/httpapi ./tests
bash scripts/tests/listingkit-schema-migrate-job-test.sh
```

Expected: PASS.

- [ ] **Step 5: Commit Task 6**

```powershell
git add -- internal/productenrich/enrich/image_governance.go internal/productenrich/enrich/image_governance_test.go internal/app/httpapi/runtime_productenrich.go internal/app/httpapi/runtime_ai_capability_test.go deployments/kubernetes/listingkit-workbench/jobs/product-listing-api-schema-migrate-job.yaml tests/listingkit_deploy_workflow_test.go scripts/tests/listingkit-schema-migrate-job-test.sh
git commit -m "fix: complete governed execution metadata"
```

---

### Task 7: Verify the integrated branch and close review threads

**Files:**
- Modify only if verification exposes a regression; every such fix requires a new failing test first.
- GitHub: PR #177 review threads `PRRT_kwDOQg5lB86bW9Ws` through `PRRT_kwDOQg5lB86bW9W2`.

**Interfaces:**
- Consumes: all Tasks 1-6.
- Produces: a clean PR with fresh local and CI evidence and all seven addressed threads resolved.

- [ ] **Step 1: Run formatting and static diff checks**

```powershell
$taskGoFiles = git diff --name-only origin/master...HEAD -- '*.go'
if ($taskGoFiles) { gofmt -w $taskGoFiles }
git diff --check
```

Expected: no output from `git diff --check`.

- [ ] **Step 2: Run focused race/static verification**

```powershell
go test -race ./internal/aicapability/... ./internal/shared/aiidentity ./internal/productenrich/... ./internal/productimage/... ./internal/amazonlisting/...
go vet ./internal/aicapability/... ./internal/shared/aiidentity ./internal/productenrich/... ./internal/productimage/... ./internal/amazonlisting/...
```

Expected: exit 0.

- [ ] **Step 3: Run complete repository verification**

```powershell
go test ./...
bash scripts/tests/listingkit-schema-migrate-job-test.sh
Invoke-Pester -Path scripts/build-push-deploy-listingkit-workbench.Tests.ps1
```

Expected: all commands exit 0 and Pester reports zero failed tests.

- [ ] **Step 4: Push the verified branch**

```powershell
git status --short --branch
git push
```

Expected: clean worktree and updated `codex/ai-rollup-master`.

- [ ] **Step 5: Reply to each inline thread with the root fix and evidence**

Reply in each GitHub review thread, mapping findings to commits/tests:

```text
Fixed by the unified execution lifecycle: rollout exclusion now plans an explicit legacy path, resolves named-then-default clients, and records the actual invocation exactly once. Regression coverage: <test names>.
```

Use specific wording for cache, tenant repository conformance, envelope state, prompt metadata, and migration deadline. Do not post a single generic top-level comment.

- [ ] **Step 6: Resolve only verified threads and re-check PR state**

Resolve each of the seven thread IDs only after its focused test and the full suite pass. Then run:

```powershell
gh pr checks 177 --repo qq550723504/task-processor
gh pr view 177 --repo qq550723504/task-processor --json state,mergeStateStatus,headRefOid,url
```

Expected: all CI checks pass and `mergeStateStatus` is not `DIRTY` or `BLOCKED` because of unresolved findings.
