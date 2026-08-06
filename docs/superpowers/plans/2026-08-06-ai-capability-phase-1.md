# AI Capability Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在现有 Go 模块化单体内交付一个可回滚的 AI 模型能力治理基础切片，让 ListingKit Studio 图片能力先 shadow、后 active 地使用 provider-neutral 路由，并把每次逻辑调用脱敏写入统一账本。

**Architecture:** 新建 `internal/aicapability` 作为不依赖 ListingKit、具体 provider 或 HTTP runtime 的控制面合同与确定性路由模块；现有租户凭据通过 ListingKit HTTP 集成适配器解析为模型目录项。Studio 领域继续依赖自己的 `AIImageGenerator`，外层窄适配器只负责路由决定、shadow/active 切换和 best-effort 调用记录，现有 OpenAI/Gemini/GrsAI 客户端与请求语义不变。

**Tech Stack:** Go 1.26、GORM 1.31、Viper、Logrus、Testify、现有 `internal/infra/clients/openai|geminiimage|grsai` 适配器、SQLite 单元测试。

## Global Constraints

- 实现范围只覆盖已批准设计的 Phase 1；不引入 Agent 框架，不实现 Agent PoC，不迁移 ProductImage、ProductEnrich、SHEIN、TEMU 或 Amazon 的其他 AI 调用点。
- 保持模块化单体；不新增 Python、Node.js 或 Go 微服务，不新增网络 API。
- 业务包依赖 provider-neutral 合同；`internal/aicapability` 禁止导入 `internal/listingkit`、业务模块、`internal/app/httpapi` 或 `internal/infra/clients/*`。
- 只接入 `listingkit.studio.image` 一个业务能力，同时覆盖当前同步生成、同步编辑、异步提交和异步查询合同。
- 路由模式只有 `legacy`、`shadow`、`active`；默认必须是 `legacy`。只能在 shadow 决策与旧逻辑一致后把生产配置切到 `active`。
- `shadow` 只记录新路由决定，仍以原请求调用旧路径；`active` 才把 `RouteDecision.RoutingKey` 应用到旧路径。
- 不改变 Prompt 注册表、Prompt 内容或 provider 请求字段；Phase 1 只记录 prompt SHA-256，`prompt_key/version/scope` 在当前调用拿不到时保持空值。
- 调用账本存储健康时记录每个 Studio adapter 逻辑调用；底层 SDK 内部重试仍由现有 provider adapter 管理，不在本阶段拆分成多个账本 attempt。
- 账本写入失败必须告警，但不得把成功的模型调用改成失败，也不得触发重复模型调用。
- 不在自动化测试中访问真实模型或产生付费调用；provider 行为一律使用 fake resolver、stub generator 或 mock HTTP 验证。
- 保留旧 adapter 路径和 feature flag 回滚；不得删除现有 `listingKitRoutedImageClient` 或现有租户凭据表。
- 每个任务只暂存其 `Files` 列出的文件；不得使用 `git add -A`，不得修改工作区内无关文件。

---

## File Map

### New control-plane contracts

- `internal/aicapability/model.go`：能力、operation、模型特征、模型目录项和租户策略数据类型。
- `internal/aicapability/errors.go`：统一错误类别与 `CapabilityError`。
- `internal/aicapability/routing.go`：路由请求、路由决定、`Router`/`ModelCatalog`/`PolicyResolver` 接口和确定性 `PolicyRouter`。
- `internal/aicapability/routing_mode.go`：`legacy|shadow|active` 模式解析。
- `internal/aicapability/invocation.go`：调用记录、Prompt 元数据、usage、outcome 和 recorder 接口。

### New persistence adapter

- `internal/aicapability/store/gorm_invocation_recorder.go`：`ai_invocations` GORM row、迁移和 best-effort recorder 的持久化实现。

### New ListingKit adapters

- `internal/listingkit/httpapi/ai_capability_studio_catalog.go`：把现有 `ClientConfigResolver` 和 Studio selector 映射成 provider-neutral 模型目录与策略。
- `internal/listingkit/studio_ai_capability_adapter.go`：包裹 ListingKit 本地 `AIImageGenerator`，实现 shadow/active、调用计时、prompt hash 和账本记录。

### New runtime/config files

- `internal/core/config/type_ai_capability.go`：`AICapabilityConfig`。
- `internal/core/config/validator_ai_capability.go`：路由模式配置校验。
- `internal/app/httpapi/adapters_ai_capability.go`：构造 GORM invocation recorder。
- `internal/app/httpapi/runtime_ai_capability.go`：仅在 shadow/active 时装配账本资源。

### Existing files to modify

- `internal/core/config/config.go:26-48,646-649`：顶层配置与环境变量绑定。
- `internal/core/config/loader.go:102-240`：默认 `legacy`。
- `internal/core/config/defaults.go:9-42`：Viper 默认值。
- `internal/core/config/loader_builder.go:180-225`：组装 `AICapabilityConfig`。
- `internal/core/config/validator.go:15-28`、`validator_validator.go:10-56`、`manager.go:100-120`：接入配置校验。
- `internal/listingkit/httpapi/bootstrap_contracts.go:145-176`：router builder 和 invocation recorder 依赖。
- `internal/listingkit/httpapi/runtime_builder.go:18-65`：透传 recorder。
- `internal/listingkit/httpapi/bootstrap_submit_module.go:12-175`：构造 Studio 路由与窄适配器，并让无效 active/shadow 装配在启动时失败。
- `internal/listingkit/httpapi/bootstrap_runtime.go:90-115`：处理 `buildSubmitModule` 返回的错误。
- `internal/listingkit/httpapi/runtime_support_hooks.go:10-50`：注册 Studio capability router builder。
- `internal/app/httpapi/runtime.go:8-65`、`runtime_shared_deps.go:1-27`、`feature_builder_listingkit.go:85-110`：装配并传递 recorder。
- `internal/app/httpapi/adapters_schema_migration.go:20-40`、`internal/listingkit/httpapi/builders_repository_schema.go:97-195`、`internal/app/runtime/listingkitschemamigrate/runtime.go:100-215`：迁移 `ai_invocations`。
- `tests/import_boundaries_test.go`、`docs/architecture/project-boundaries.md`、`docs/architecture/external-client-boundary-inventory.md`：固化新边界和迁移状态。

---

### Task 1: Provider-neutral model, routing, error, and invocation contracts

**Files:**
- Create: `internal/aicapability/model.go`
- Create: `internal/aicapability/errors.go`
- Create: `internal/aicapability/routing.go`
- Create: `internal/aicapability/routing_mode.go`
- Create: `internal/aicapability/invocation.go`
- Test: `internal/aicapability/routing_test.go`
- Test: `internal/aicapability/routing_mode_test.go`

**Interfaces:**
- Consumes: only Go standard library.
- Produces: `Capability`, `Operation`, `ModelFeature`, `ModelDefinition`, `TenantModelPolicy`, `RouteRequest`, `RouteDecision`, `Router`, `ModelCatalog`, `PolicyResolver`, `NewPolicyRouter`, `RoutingMode`, `ParseRoutingMode`, `ErrorCategory`, `CategoryOf`, `InvocationRecord`, and `InvocationRecorder`.

- [ ] **Step 1: Write failing routing and mode tests**

Create table-driven tests with these exact externally visible expectations:

```go
func TestPolicyRouterUsesRequestedRoutingKeyWhenAllowed(t *testing.T) {
	policy := TenantModelPolicy{
		TenantID:             "tenant-a",
		Capability:           CapabilityListingKitStudioImage,
		AllowedRoutingKeys:   []string{"gpt-image-2", "nanobanana"},
		PreferredRoutingKeys: []string{"gpt-image-2"},
		Version:              "legacy-studio-v1",
	}
	catalog := stubCatalog{models: map[string]ModelDefinition{
		"nanobanana": {
			ProviderID:          "grsai",
			ModelID:             "nano-banana-pro",
			RoutingKey:          "nanobanana",
			CredentialReference: "image_nanobanana",
			Features:            []ModelFeature{FeatureImageGenerate, FeatureImageEdit, FeatureAsyncImageJob},
			Enabled:             true,
			ConfigurationVersion: "credential-7",
		},
	}}
	router := NewPolicyRouter(catalog, staticPolicyResolver{policy: policy})

	decision, err := router.Decide(context.Background(), RouteRequest{
		TenantID:           "tenant-a",
		Capability:         CapabilityListingKitStudioImage,
		Operation:          OperationImageEdit,
		RequestedRoutingKey: "nanobanana",
		RequiredFeatures:   []ModelFeature{FeatureImageEdit},
	})
	require.NoError(t, err)
	assert.Equal(t, "grsai", decision.ProviderID)
	assert.Equal(t, "nano-banana-pro", decision.ModelID)
	assert.Equal(t, "nanobanana", decision.RoutingKey)
	assert.Equal(t, "legacy-studio-v1", decision.PolicyVersion)
	assert.Equal(t, "credential-7", decision.ConfigurationVersion)
	assert.Equal(t, 0, decision.FallbackIndex)
}

func TestPolicyRouterRejectsModelWithoutRequiredFeature(t *testing.T) {
	router := NewPolicyRouter(
		stubCatalog{models: map[string]ModelDefinition{
			"gpt-image-2": {
				RoutingKey: "gpt-image-2",
				Features:   []ModelFeature{FeatureImageGenerate},
				Enabled:    true,
			},
		}},
		staticPolicyResolver{policy: TenantModelPolicy{
			TenantID:             "tenant-a",
			Capability:           CapabilityListingKitStudioImage,
			PreferredRoutingKeys: []string{"gpt-image-2"},
			Version:              "legacy-studio-v1",
		}},
	)

	_, err := router.Decide(context.Background(), RouteRequest{
		TenantID:         "tenant-a",
		Capability:       CapabilityListingKitStudioImage,
		Operation:        OperationImageEdit,
		RequiredFeatures: []ModelFeature{FeatureImageEdit},
	})
	require.Error(t, err)
	assert.Equal(t, ErrorCapabilityUnavailable, CategoryOf(err))
}

func TestParseRoutingModeRejectsUnknownValue(t *testing.T) {
	_, err := ParseRoutingMode("automatic")
	require.Error(t, err)
	assert.Equal(t, ErrorInvalidInput, CategoryOf(err))
}
```

Use in-test `stubCatalog` and `staticPolicyResolver` implementations; do not import a provider client into the test package.

- [ ] **Step 2: Run the tests and verify the package does not exist yet**

Run:

```powershell
go test ./internal/aicapability -run 'TestPolicyRouter|TestParseRoutingMode' -count=1
```

Expected: FAIL because the package files and exported contracts are not defined.

- [ ] **Step 3: Implement the stable model and policy types**

Use these exact names and values in `model.go`:

```go
package aicapability

import "time"

type Capability string

const CapabilityListingKitStudioImage Capability = "listingkit.studio.image"

type Operation string

const (
	OperationImageGenerate       Operation = "image_generate"
	OperationImageEdit           Operation = "image_edit"
	OperationAsyncImageGenerate  Operation = "async_image_generate"
	OperationAsyncImageEdit      Operation = "async_image_edit"
	OperationAsyncImageQuery     Operation = "async_image_query"
)

type ModelFeature string

const (
	FeatureImageGenerate ModelFeature = "image_generate"
	FeatureImageEdit     ModelFeature = "image_edit"
	FeatureAsyncImageJob ModelFeature = "async_image_job"
)

type ModelPricing struct {
	Currency          string
	InputUnitMicros   int64
	OutputUnitMicros  int64
	ImageUnitMicros   int64
}

type ModelDefinition struct {
	ProviderID           string
	ModelID              string
	RoutingKey           string
	CredentialReference  string
	Features              []ModelFeature
	InputModalities       []string
	OutputModalities      []string
	SupportsJSONSchema    bool
	SupportsTools         bool
	SupportsAsync         bool
	Region                string
	DataPolicyTags        []string
	DefaultTimeout        time.Duration
	MaxConcurrency        int
	Pricing               ModelPricing
	Enabled               bool
	ConfigurationVersion  string
}

func (m ModelDefinition) Supports(feature ModelFeature) bool {
	for _, candidate := range m.Features {
		if candidate == feature {
			return true
		}
	}
	return false
}

type TenantModelPolicy struct {
	TenantID                  string
	Capability                Capability
	AllowedRoutingKeys        []string
	PreferredRoutingKeys      []string
	MaxEstimatedCostMicros    int64
	MaxRuntime                time.Duration
	RequiredDataPolicyTags    []string
	AllowCrossProviderFallback bool
	CredentialReference       string
	Version                   string
}
```

Keep slices immutable by convention: router code reads them but never appends into the caller-owned backing array.

- [ ] **Step 4: Implement errors, route algorithm, mode parsing, and invocation records**

`errors.go` must expose all approved design categories, including `ErrorUnknownRemoteState`, through one wrapper:

```go
type ErrorCategory string

const (
	ErrorInvalidInput           ErrorCategory = "invalid_input"
	ErrorPolicyDenied           ErrorCategory = "policy_denied"
	ErrorCapabilityUnavailable  ErrorCategory = "capability_unavailable"
	ErrorCredentialUnavailable  ErrorCategory = "credential_unavailable"
	ErrorRateLimited            ErrorCategory = "rate_limited"
	ErrorProviderTimeout        ErrorCategory = "provider_timeout"
	ErrorProviderUnavailable    ErrorCategory = "provider_unavailable"
	ErrorProviderRejected       ErrorCategory = "provider_rejected"
	ErrorInvalidProviderResponse ErrorCategory = "invalid_provider_response"
	ErrorStructuredOutputInvalid ErrorCategory = "structured_output_invalid"
	ErrorBudgetExceeded         ErrorCategory = "budget_exceeded"
	ErrorAgentStepLimitExceeded ErrorCategory = "agent_step_limit_exceeded"
	ErrorAgentToolDenied        ErrorCategory = "agent_tool_denied"
	ErrorUnknownRemoteState     ErrorCategory = "unknown_remote_state"
	ErrorUnknown                ErrorCategory = "unknown"
)

type CapabilityError struct {
	Category  ErrorCategory
	Operation string
	Err       error
}
```

Implement `Error()`, `Unwrap()`, `NewError(category, operation, err)` and `CategoryOf(err)`. `CategoryOf(nil)` returns an empty category; unknown non-nil errors return `ErrorUnknown`.

`routing.go` must implement these signatures:

```go
type ModelCatalog interface {
	ResolveModel(ctx context.Context, routingKey string) (ModelDefinition, error)
}

type PolicyResolver interface {
	ResolvePolicy(ctx context.Context, request RouteRequest) (TenantModelPolicy, error)
}

type Router interface {
	Decide(ctx context.Context, request RouteRequest) (RouteDecision, error)
}

type RouteRequest struct {
	TenantID            string
	UserID              string
	Capability          Capability
	Operation           Operation
	RequestedRoutingKey string
	RequiredFeatures    []ModelFeature
	IdempotencyKey      string
	TraceID             string
}

type RouteDecision struct {
	Capability           Capability
	Operation            Operation
	ProviderID           string
	ModelID              string
	RoutingKey           string
	CredentialReference  string
	PolicyVersion        string
	ConfigurationVersion string
	FallbackIndex        int
	Reason               string
}

func NewPolicyRouter(catalog ModelCatalog, policies PolicyResolver) *PolicyRouter
func (r *PolicyRouter) Decide(ctx context.Context, request RouteRequest) (RouteDecision, error)
```

`Decide` must trim identity and routing keys; reject missing tenant/capability/operation as `invalid_input`; resolve policy once; treat an empty `AllowedRoutingKeys` slice as unrestricted compatibility mode; use the requested key when present and allowed, otherwise use ordered preferred keys; reject disabled models, missing features and missing required data tags; and only try later keys when `AllowCrossProviderFallback` is true. It returns a copied provider-neutral decision and never exposes API keys or provider config objects.

`routing_mode.go` must treat blank as `legacy` and accept only:

```go
const (
	RoutingModeLegacy RoutingMode = "legacy"
	RoutingModeShadow RoutingMode = "shadow"
	RoutingModeActive RoutingMode = "active"
)
```

`invocation.go` must define `InvocationRecorder.RecordInvocation(context.Context, InvocationRecord) error` and a record with these stable fields: invocation/parent/agent IDs; tenant/user/business task/trace; capability/operation; route mode/outcome; provider/model/routing key/credential reference; policy/config versions; prompt key/version/scope/hash; start/finish/latency; attempt/fallback index; prompt/completion/total tokens and image count; estimated cost/currency; outcome/error categories/error code; provider request/job IDs; input/output hashes. Do not add raw prompt, raw response, API key, image bytes or cookie fields.

Use these exact invocation types so later tasks share one vocabulary:

```go
type RouteOutcome string

const (
	RouteOutcomeLegacy          RouteOutcome = "legacy"
	RouteOutcomeShadowDecided   RouteOutcome = "shadow_decided"
	RouteOutcomeShadowRouteError RouteOutcome = "shadow_route_error"
	RouteOutcomeActive          RouteOutcome = "active"
)

type InvocationOutcome string

const (
	InvocationSucceeded InvocationOutcome = "succeeded"
	InvocationFailed    InvocationOutcome = "failed"
)

type InvocationRecord struct {
	InvocationID          string
	ParentInvocationID    string
	AgentRunID             string
	TenantID               string
	UserID                 string
	BusinessTaskID         string
	TraceID                string
	Capability             Capability
	Operation              Operation
	RouteMode              RoutingMode
	RouteOutcome           RouteOutcome
	ProviderID             string
	ModelID                string
	RequestedRoutingKey    string
	RoutingKey             string
	CredentialReference    string
	PolicyVersion          string
	ConfigurationVersion   string
	PromptKey              string
	PromptVersion          string
	PromptScope            string
	PromptHash             string
	StartedAt              time.Time
	FinishedAt             time.Time
	LatencyMilliseconds    int64
	Attempt                 int
	FallbackIndex           int
	PromptTokens            int
	CompletionTokens        int
	TotalTokens             int
	ImageCount              int
	EstimatedCostMicros     int64
	Currency                string
	Outcome                 InvocationOutcome
	ErrorCategory           ErrorCategory
	RouteErrorCategory      ErrorCategory
	ErrorCode               string
	ProviderRequestID       string
	UpstreamJobID           string
	InputHash               string
	OutputHash              string
}

type InvocationRecorder interface {
	RecordInvocation(ctx context.Context, record InvocationRecord) error
}
```

- [ ] **Step 5: Run the package tests**

Run:

```powershell
gofmt -w internal/aicapability/model.go internal/aicapability/errors.go internal/aicapability/routing.go internal/aicapability/routing_mode.go internal/aicapability/invocation.go internal/aicapability/routing_test.go internal/aicapability/routing_mode_test.go
go test ./internal/aicapability -count=1
```

Expected: PASS with requested routing, preferred routing, fallback gating, feature rejection, policy rejection, category extraction and routing-mode cases covered.

- [ ] **Step 6: Commit Task 1**

```powershell
git add -- internal/aicapability/model.go internal/aicapability/errors.go internal/aicapability/routing.go internal/aicapability/routing_mode.go internal/aicapability/invocation.go internal/aicapability/routing_test.go internal/aicapability/routing_mode_test.go
git diff --cached --check
git commit -m "feat(ai): add capability routing contracts"
```

---

### Task 2: Credential-backed ListingKit Studio model catalog

**Files:**
- Create: `internal/listingkit/httpapi/ai_capability_studio_catalog.go`
- Test: `internal/listingkit/httpapi/ai_capability_studio_catalog_test.go`
- Modify: `internal/listingkit/httpapi/ai_client_image_routing.go:113-145`

**Interfaces:**
- Consumes: `aicapability.ModelCatalog`, `aicapability.PolicyResolver`, `aicapability.NewPolicyRouter`, existing `openaiclient.ClientConfigResolver`, existing Studio selector/client-name constants and `normalizeImageAPIStyle`.
- Produces: `BuildStudioAICapabilityRouter(resolver openaiclient.ClientConfigResolver) aicapability.Router`.

- [ ] **Step 1: Write failing catalog compatibility tests**

Cover the current routing behavior, not an idealized replacement:

```go
func TestStudioCapabilityCatalogMapsCurrentSelectorsToCredentialBindings(t *testing.T) {
	resolver := &stubStudioConfigResolver{configs: map[string]*openaiclient.ResolvedClientConfig{
		listingKitImageClientNameGPTImage2: {
			CacheKey: "credential:gpt:v3",
			Config: &openaiclient.ClientConfig{APIKey: "secret", BaseURL: "https://example.test", Model: "gpt-image-2", APIStyle: "openai"},
		},
		listingKitImageClientNameNanobanana: {
			CacheKey: "credential:nano:v8",
			Config: &openaiclient.ClientConfig{APIKey: "secret", BaseURL: "https://grs.example.test", Model: "nano-banana-pro", APIStyle: "grsai_async"},
		},
	}}
	router := BuildStudioAICapabilityRouter(resolver)

	tests := []struct {
		name       string
		requested  string
		binding    string
		provider   string
		model      string
		routingKey string
	}{
		{name: "default", binding: listingKitImageClientNameGPTImage2, provider: "openai", model: "gpt-image-2", routingKey: "gpt-image-2"},
		{name: "banana alias", requested: "nano-banana-fast", binding: listingKitImageClientNameNanobanana, provider: "grsai_async", model: "nano-banana-pro", routingKey: "nanobanana"},
		{name: "custom model stays on legacy default binding", requested: "custom-image-model", binding: listingKitImageClientNameNanobanana, provider: "grsai_async", model: "custom-image-model", routingKey: "custom-image-model"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := router.Decide(openaiclient.WithIdentity(context.Background(), openaiclient.Identity{TenantID: "tenant-a", UserID: "user-a"}), aicapability.RouteRequest{
				TenantID:            "tenant-a",
				UserID:              "user-a",
				Capability:          aicapability.CapabilityListingKitStudioImage,
				Operation:           aicapability.OperationImageGenerate,
				RequestedRoutingKey: tt.requested,
				RequiredFeatures:    []aicapability.ModelFeature{aicapability.FeatureImageGenerate},
			})
			require.NoError(t, err)
			assert.Equal(t, tt.binding, decision.CredentialReference)
			assert.Equal(t, tt.provider, decision.ProviderID)
			assert.Equal(t, tt.model, decision.ModelID)
			assert.Equal(t, tt.routingKey, decision.RoutingKey)
		})
	}
}
```

Also test: nil resolver returns `credential_unavailable`; disabled/missing credential returns `credential_unavailable`; async operations require `FeatureAsyncImageJob`; and the returned `RouteDecision` contains the resolver cache key as `ConfigurationVersion` but never contains the API key.

- [ ] **Step 2: Run the focused test and verify failure**

```powershell
go test ./internal/listingkit/httpapi -run 'TestStudioCapabilityCatalog' -count=1
```

Expected: FAIL because `BuildStudioAICapabilityRouter` is undefined.

- [ ] **Step 3: Extract the legacy selector decision into a reusable helper**

In `ai_client_image_routing.go`, add a provider-free result:

```go
type listingKitImageRoute struct {
	RoutingKey          string
	CredentialReference string
	UsesConfiguredModel bool
}

func resolveListingKitImageRoute(selector string, hasResolver bool) listingKitImageRoute
```

The exact mapping is:

- blank or `gpt-image-2` -> `gpt-image-2`, `image_gpt_image_2`, configured model;
- any selector containing `banana` -> `nanobanana`, `image_nanobanana`, configured model;
- any other selector with a resolver -> original trimmed selector, `image_nanobanana`, request-selected model;
- any other selector without a resolver -> original trimmed selector, `image`, request-selected model.

Refactor `listingKitRoutedImageClient.resolveBySelector` to consume this helper without changing its existing tests or provider request behavior.

- [ ] **Step 4: Implement the credential-backed catalog and legacy policy**

`ai_capability_studio_catalog.go` should keep concrete resolver knowledge inside the existing allowlisted HTTP integration seam:

```go
func BuildStudioAICapabilityRouter(resolver openaiclient.ClientConfigResolver) aicapability.Router {
	catalog := &studioCredentialModelCatalog{resolver: resolver}
	policies := studioLegacyPolicyResolver{}
	return aicapability.NewPolicyRouter(catalog, policies)
}
```

`studioLegacyPolicyResolver.ResolvePolicy` returns policy version `listingkit-studio-legacy-v1`, preferred key `gpt-image-2`, an empty allowed-key list meaning “preserve current custom selector support,” and `AllowCrossProviderFallback=false`.

`studioCredentialModelCatalog.ResolveModel` must:

1. call `resolveListingKitImageRoute`;
2. call `resolver.ResolveClientConfig(ctx, route.CredentialReference, nil)`;
3. reject nil, disabled-equivalent or incomplete config without logging secrets;
4. use the compatibility-only `normalizeImageAPIStyle` for `ProviderID`;
5. set `ModelID` to configured model when `UsesConfiguredModel=true`, otherwise the original routing key;
6. always advertise generate/edit; advertise `async_image_job` and set `SupportsAsync=true` only for the verified `grsai_async` compatibility style, because the current OpenAI and Gemini image clients explicitly return `SupportsAsyncImageGeneration=false`;
7. copy the resolver `CacheKey` into `ConfigurationVersion`.

- [ ] **Step 5: Run old and new routing tests together**

```powershell
gofmt -w internal/listingkit/httpapi/ai_client_image_routing.go internal/listingkit/httpapi/ai_capability_studio_catalog.go internal/listingkit/httpapi/ai_capability_studio_catalog_test.go
go test ./internal/listingkit/httpapi -run 'TestStudioCapabilityCatalog|TestListingKitRoutedImageClient|TestBuildStrictListingKit' -count=1
```

Expected: PASS; existing routing tests prove provider request selection did not change.

- [ ] **Step 6: Commit Task 2**

```powershell
git add -- internal/listingkit/httpapi/ai_client_image_routing.go internal/listingkit/httpapi/ai_capability_studio_catalog.go internal/listingkit/httpapi/ai_capability_studio_catalog_test.go
git diff --cached --check
git commit -m "feat(ai): map Studio credentials into model catalog"
```

---

### Task 3: Persistent, sensitive-data-safe invocation ledger

**Files:**
- Create: `internal/aicapability/store/gorm_invocation_recorder.go`
- Test: `internal/aicapability/store/gorm_invocation_recorder_test.go`

**Interfaces:**
- Consumes: `aicapability.InvocationRecord` and `aicapability.InvocationRecorder` from Task 1.
- Produces: `NewGormInvocationRecorder(db *gorm.DB) *GormInvocationRecorder` and `AutoMigrateInvocationLedger(db *gorm.DB) error`.

- [ ] **Step 1: Write failing schema, round-trip, and sensitive-column tests**

Use a unique in-memory SQLite DSN per test. Assert the persisted row through a test-only struct/Table query:

```go
func TestGormInvocationRecorderPersistsNormalizedMetadata(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ai-ledger-roundtrip?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, AutoMigrateInvocationLedger(db))
	recorder := NewGormInvocationRecorder(db)

	started := time.Date(2026, 8, 6, 1, 2, 3, 0, time.UTC)
	finished := started.Add(1250 * time.Millisecond)
	err = recorder.RecordInvocation(context.Background(), aicapability.InvocationRecord{
		InvocationID:        "inv-1",
		TenantID:            "tenant-a",
		UserID:              "user-a",
		Capability:          aicapability.CapabilityListingKitStudioImage,
		Operation:           aicapability.OperationImageGenerate,
		RouteMode:           aicapability.RoutingModeShadow,
		RouteOutcome:        aicapability.RouteOutcomeShadowDecided,
		ProviderID:          "openai",
		ModelID:             "gpt-image-2",
		RoutingKey:          "gpt-image-2",
		CredentialReference: "image_gpt_image_2",
		PolicyVersion:       "listingkit-studio-legacy-v1",
		PromptHash:          strings.Repeat("a", 64),
		StartedAt:           started,
		FinishedAt:          finished,
		LatencyMilliseconds: 1250,
		Attempt:             1,
		Outcome:             "succeeded",
		ProviderRequestID:   "req-1",
		ImageCount:          1,
	})
	require.NoError(t, err)

	var row map[string]any
	require.NoError(t, db.Table("ai_invocations").Where("invocation_id = ?", "inv-1").Take(&row).Error)
	assert.Equal(t, "tenant-a", row["tenant_id"])
	assert.Equal(t, "listingkit.studio.image", row["capability"])
	assert.Equal(t, strings.Repeat("a", 64), row["prompt_hash"])
}

func TestInvocationTableHasNoSensitivePayloadColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:ai-ledger-columns?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, AutoMigrateInvocationLedger(db))
	columns, err := db.Migrator().ColumnTypes("ai_invocations")
	require.NoError(t, err)

	names := make(map[string]struct{}, len(columns))
	for _, column := range columns {
		names[strings.ToLower(column.Name())] = struct{}{}
	}
	for _, banned := range []string{
		"api_key", "prompt", "raw_prompt", "response", "raw_response",
		"image_bytes", "cookie", "authorization",
	} {
		_, exists := names[banned]
		assert.False(t, exists, "sensitive column %q must not exist", banned)
	}
}
```

Also assert nil DB returns an explicit error and blank `InvocationID` is rejected before SQL execution.

- [ ] **Step 2: Run and verify failure**

```powershell
go test ./internal/aicapability/store -count=1
```

Expected: FAIL because the store package does not exist.

- [ ] **Step 3: Implement the GORM row, migration, and recorder**

Use table name `ai_invocations`. The row must use bounded varchar columns for IDs, categories and the sanitized error code; it must not need any raw-payload `text` column. Add indexes for:

- `(tenant_id, started_at)`;
- `(capability, started_at)`;
- `(provider_id, model_id, started_at)`;
- `business_task_id`;
- `provider_request_id`;
- `upstream_job_id`.

Required behavior:

```go
func AutoMigrateInvocationLedger(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("ai invocation ledger database is nil")
	}
	return db.AutoMigrate(&invocationRecord{})
}

func (r *GormInvocationRecorder) RecordInvocation(ctx context.Context, record aicapability.InvocationRecord) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("ai invocation recorder database is nil")
	}
	if strings.TrimSpace(record.InvocationID) == "" {
		return fmt.Errorf("invocation_id is required")
	}
	return r.db.WithContext(ctx).Create(newInvocationRecord(record)).Error
}
```

Normalize string whitespace, force timestamps to UTC, derive latency from start/finish only when the caller did not set it, and reject negative usage/cost counters. Do not expose the row type outside the store package.

- [ ] **Step 4: Run store and contract tests**

```powershell
gofmt -w internal/aicapability/store/gorm_invocation_recorder.go internal/aicapability/store/gorm_invocation_recorder_test.go
go test ./internal/aicapability/... -count=1
```

Expected: PASS, including schema and sensitive-column checks.

- [ ] **Step 5: Commit Task 3**

```powershell
git add -- internal/aicapability/store/gorm_invocation_recorder.go internal/aicapability/store/gorm_invocation_recorder_test.go
git diff --cached --check
git commit -m "feat(ai): persist invocation ledger records"
```

---

### Task 4: Bounded Studio shadow/active capability adapter

**Files:**
- Create: `internal/listingkit/studio_ai_capability_adapter.go`
- Test: `internal/listingkit/studio_ai_capability_adapter_test.go`

**Interfaces:**
- Consumes: existing `listingkit.AIImageGenerator`, `aicapability.Router`, `aicapability.InvocationRecorder`, `aicapability.RoutingMode`, `RequestIdentityFromContext`, `TenantIDFromContext`, and `RequestTraceFromContext`.
- Produces: `NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig) (AIImageGenerator, error)`.

- [ ] **Step 1: Write failing behavior tests before the adapter**

Use a stub router, a counting local generator and an in-memory recorder. Cover at least these cases:

```go
func TestStudioAIImageCapabilityAdapterShadowKeepsLegacyRequest(t *testing.T) {
	legacy := &recordingStudioImageGenerator{response: &AIImageResponse{
		Data:      []AIImageData{{URL: "https://example.test/image.png"}},
		RequestID: "req-1",
		Usage:     AIUsage{PromptTokens: 5, CompletionTokens: 7, TotalTokens: 12},
	}}
	recorder := &memoryInvocationRecorder{}
	adapter, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy:   legacy,
		Router:   staticRouter{decision: aicapability.RouteDecision{ProviderID: "grsai_async", ModelID: "nano-banana-pro", RoutingKey: "nanobanana", PolicyVersion: "v1"}},
		Recorder: recorder,
		Mode:     aicapability.RoutingModeShadow,
		Now:      monotonicTestClock(),
		NewID:    func() string { return "inv-1" },
	})
	require.NoError(t, err)

	_, err = adapter.GenerateImage(WithRequestIdentity(WithTenantID(context.Background(), "tenant-a"), RequestIdentity{TenantID: "tenant-a", UserID: "user-a"}), &AIImageGenerateRequest{
		Model:  "custom-image-model",
		Prompt: "sensitive prompt body",
		Size:   "1024x1024",
		N:      1,
	})
	require.NoError(t, err)
	require.Equal(t, 1, legacy.generateCalls)
	assert.Equal(t, "custom-image-model", legacy.generateRequest.Model)
	require.Len(t, recorder.records, 1)
	assert.Equal(t, aicapability.RoutingModeShadow, recorder.records[0].RouteMode)
	assert.Equal(t, aicapability.RouteOutcomeShadowDecided, recorder.records[0].RouteOutcome)
	assert.Len(t, recorder.records[0].PromptHash, 64)
	assert.NotContains(t, fmt.Sprintf("%+v", recorder.records[0]), "sensitive prompt body")
}

func TestStudioAIImageCapabilityAdapterActiveAppliesDecisionRoutingKey(t *testing.T) {
	legacy := &recordingStudioImageGenerator{response: &AIImageResponse{}}
	adapter, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy:   legacy,
		Router:   staticRouter{decision: aicapability.RouteDecision{RoutingKey: "nanobanana"}},
		Recorder: &memoryInvocationRecorder{},
		Mode:     aicapability.RoutingModeActive,
	})
	require.NoError(t, err)

	_, err = adapter.GenerateImage(WithTenantID(context.Background(), "tenant-a"), &AIImageGenerateRequest{Model: "gpt-image-2", Prompt: "p"})
	require.NoError(t, err)
	assert.Equal(t, "nanobanana", legacy.generateRequest.Model)
}

func TestStudioAIImageCapabilityAdapterLedgerFailureDoesNotRetryProvider(t *testing.T) {
	legacy := &recordingStudioImageGenerator{response: &AIImageResponse{}}
	recordFailures := 0
	adapter, err := NewStudioAIImageCapabilityAdapter(StudioAIImageCapabilityAdapterConfig{
		Legacy: legacy,
		Router: staticRouter{decision: aicapability.RouteDecision{RoutingKey: "gpt-image-2"}},
		Recorder: failingInvocationRecorder{err: errors.New("ledger unavailable")},
		Mode: aicapability.RoutingModeShadow,
		OnRecordError: func(aicapability.InvocationRecord, error) { recordFailures++ },
	})
	require.NoError(t, err)

	_, err = adapter.GenerateImage(WithTenantID(context.Background(), "tenant-a"), &AIImageGenerateRequest{Prompt: "p"})
	require.NoError(t, err)
	assert.Equal(t, 1, legacy.generateCalls)
	assert.Equal(t, 1, recordFailures)
}
```

Also cover: shadow router failure still executes legacy; active router failure does not invoke legacy; provider failure is classified and recorded; edit preserves bytes/URLs; submit generate/edit keeps async request fields; query delegates using job ID; `SupportsAsyncImageGeneration` and `GetDefaultModel` remain transparent; nil request handling matches the legacy implementation; recorder context survives request cancellation for at most two seconds.

- [ ] **Step 2: Run and verify failure**

```powershell
go test ./internal/listingkit -run 'TestStudioAIImageCapabilityAdapter' -count=1
```

Expected: FAIL because the adapter constructor and config do not exist.

- [ ] **Step 3: Implement constructor validation and route request mapping**

Use this stable constructor surface:

```go
type StudioAIImageCapabilityAdapterConfig struct {
	Legacy        AIImageGenerator
	Router        aicapability.Router
	Recorder      aicapability.InvocationRecorder
	Mode          aicapability.RoutingMode
	OnRecordError func(aicapability.InvocationRecord, error)
	Now           func() time.Time
	NewID         func() string
}

func NewStudioAIImageCapabilityAdapter(config StudioAIImageCapabilityAdapterConfig) (AIImageGenerator, error)
```

Constructor rules:

- `Legacy` is always required;
- `legacy` mode returns the existing generator unchanged;
- `shadow` and `active` require both router and recorder;
- default `Now` is `time.Now`, default `NewID` is `uuid.NewString`;
- invalid mode returns `invalid_input`.

Map operations to required features exactly:

| Method | Operation | Required features |
| --- | --- | --- |
| `GenerateImage` | `image_generate` | `image_generate` |
| `EditImage` | `image_edit` | `image_edit` |
| `SubmitImageGeneration` | `async_image_generate` | `image_generate`, `async_image_job` |
| `SubmitImageEdit` | `async_image_edit` | `image_edit`, `async_image_job` |
| `QueryImageGeneration` | `async_image_query` | 不重新路由；只透传与记录 |

Identity is read from `RequestIdentityFromContext`; if its tenant is blank, use `TenantIDFromContext`. Use `RequestTrace.BatchRunID`, then `SessionID`, as `BusinessTaskID`; serialize only non-sensitive trace IDs, never full request data.

- [ ] **Step 4: Implement execution and best-effort recording**

For generate/edit/submit:

1. create `RouteRequest` from identity, operation and request model;
2. call router once;
3. in shadow mode keep the original request object and values;
4. in active mode shallow-copy the request, deep-copy `ImageURLs`, set only the copy's `Model=decision.RoutingKey`;
5. call legacy exactly once;
6. create `InvocationRecord` with SHA-256 prompt/input/output hashes, normalized usage, route metadata, latency and provider request/job IDs;
7. write with `context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)`;
8. call `OnRecordError` on ledger failure without changing the provider result.

For `QueryImageGeneration`, do not call the router. Keep the old job ID and delegate once. Because the old interface does not carry a routing key with the job ID, record the operation/job/outcome but leave unknown route fields empty; do not guess provider or model from the job ID.

Classify `context.DeadlineExceeded` as `provider_timeout`, existing `CapabilityError` through `CategoryOf`, and other provider errors as `unknown`. Store an error code/category, not the full provider response body.

- [ ] **Step 5: Run Studio unit and regression tests**

```powershell
gofmt -w internal/listingkit/studio_ai_capability_adapter.go internal/listingkit/studio_ai_capability_adapter_test.go
go test ./internal/listingkit -run 'TestStudioAIImageCapabilityAdapter|TestResolveStudioDesignImageModel|TestGenerateStudioDesignImage|TestTaskStudioMedia' -count=1
```

Expected: PASS; the new tests prove shadow does not mutate requests and ledger failure does not duplicate provider calls.

- [ ] **Step 6: Commit Task 4**

```powershell
git add -- internal/listingkit/studio_ai_capability_adapter.go internal/listingkit/studio_ai_capability_adapter_test.go
git diff --cached --check
git commit -m "feat(ai): add Studio capability shadow adapter"
```

---

### Task 5: Validated `legacy|shadow|active` configuration

**Files:**
- Create: `internal/core/config/type_ai_capability.go`
- Create: `internal/core/config/validator_ai_capability.go`
- Test: `internal/core/config/ai_capability_test.go`
- Modify: `internal/core/config/config.go:26-48,646-649`
- Modify: `internal/core/config/loader.go:102-240`
- Modify: `internal/core/config/defaults.go:9-42`
- Modify: `internal/core/config/loader_builder.go:180-225`
- Modify: `internal/core/config/validator.go:15-28`
- Modify: `internal/core/config/validator_validator.go:10-56`
- Modify: `internal/core/config/manager.go:100-120`
- Modify: `config/config-dev.yaml`
- Modify: `config/config-test.yaml`
- Modify: `config/config-prod.yaml`

**Interfaces:**
- Consumes: no Task 1 package dependency; configuration remains a plain string at the config boundary.
- Produces: `Config.AICapability.StudioImageRoutingMode` with validated values and default `legacy`.

- [ ] **Step 1: Write failing default, YAML, env, and invalid-mode tests**

```go
func TestAICapabilityRoutingDefaultsToLegacy(t *testing.T) {
	cfg, err := LoadFromBytes([]byte(`
openai:
  apiKey: test-key
  model: test-model
  baseURL: https://example.test/v1
  timeout: 30
`))
	require.NoError(t, err)
	assert.Equal(t, "legacy", cfg.AICapability.StudioImageRoutingMode)
}

func validMinimalConfigYAML() []byte {
	return []byte(`
openai:
  apiKey: test-key
  model: test-model
  baseURL: https://example.test/v1
  timeout: 30
`)
}

func TestAICapabilityRoutingModeUsesEnvironmentOverride(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_AI_CAPABILITY_STUDIO_IMAGE_ROUTING_MODE", "shadow")
	cfg, err := LoadFromBytes(validMinimalConfigYAML())
	require.NoError(t, err)
	assert.Equal(t, "shadow", cfg.AICapability.StudioImageRoutingMode)
}

func TestAICapabilityRoutingRejectsUnknownMode(t *testing.T) {
	_, err := LoadFromBytes(append(validMinimalConfigYAML(), []byte("\naiCapability:\n  studioImageRoutingMode: automatic\n")...))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "aiCapability.studioImageRoutingMode")
}
```

Use an existing valid config helper or define the complete minimal YAML in this test file; do not depend on developer machine environment.

- [ ] **Step 2: Run and verify failure**

```powershell
go test ./internal/core/config -run 'TestAICapabilityRouting' -count=1
```

Expected: FAIL because `Config.AICapability` is undefined.

- [ ] **Step 3: Add config type, default, binding, builder, and validation**

Use this config shape:

```go
type AICapabilityConfig struct {
	StudioImageRoutingMode string `mapstructure:"studioImageRoutingMode" yaml:"studioImageRoutingMode"`
}
```

Add top-level YAML key `aiCapability`, env binding `TASK_PROCESSOR_AI_CAPABILITY_STUDIO_IMAGE_ROUTING_MODE`, and default `legacy`. `ValidateAICapabilityConfig` trims/lowercases a copy for comparison but rejects values other than `legacy`, `shadow`, and `active`; it does not silently rewrite invalid input.

Extend the central `Validator` with `aiCapability *AICapabilityConfig`, update both constructor call sites, include its errors in `Validate()`, and map fields prefixed `aiCapability.` to module label `AI Capability`.

Add this explicit block to all three checked-in configs:

```yaml
aiCapability:
  studioImageRoutingMode: legacy
```

Production remains `legacy`; deployment promotion to shadow/active is an operations step after code rollout and is not performed by this implementation.

- [ ] **Step 4: Run all config tests**

```powershell
gofmt -w internal/core/config/type_ai_capability.go internal/core/config/validator_ai_capability.go internal/core/config/ai_capability_test.go internal/core/config/config.go internal/core/config/loader.go internal/core/config/defaults.go internal/core/config/loader_builder.go internal/core/config/validator.go internal/core/config/validator_validator.go internal/core/config/manager.go
go test ./internal/core/config -count=1
```

Expected: PASS, including existing OpenAI, Browser, Amazon, RabbitMQ and platform validators.

- [ ] **Step 5: Commit Task 5**

```powershell
git add -- internal/core/config/type_ai_capability.go internal/core/config/validator_ai_capability.go internal/core/config/ai_capability_test.go internal/core/config/config.go internal/core/config/loader.go internal/core/config/defaults.go internal/core/config/loader_builder.go internal/core/config/validator.go internal/core/config/validator_validator.go internal/core/config/manager.go config/config-dev.yaml config/config-test.yaml config/config-prod.yaml
git diff --cached --check
git commit -m "feat(config): add Studio AI routing mode"
```

---

### Task 6: Runtime assembly, schema migration, and one-capability cutover

**Files:**
- Create: `internal/app/httpapi/adapters_ai_capability.go`
- Create: `internal/app/httpapi/runtime_ai_capability.go`
- Test: `internal/app/httpapi/runtime_ai_capability_test.go`
- Modify: `internal/app/httpapi/runtime.go:8-65`
- Modify: `internal/app/httpapi/runtime_shared_deps.go:1-27`
- Modify: `internal/app/httpapi/feature_builder_listingkit.go:85-110`
- Modify: `internal/app/httpapi/adapters_schema_migration.go:20-40`
- Modify: `internal/listingkit/httpapi/bootstrap_contracts.go:145-176`
- Modify: `internal/listingkit/httpapi/runtime_builder.go:18-65`
- Modify: `internal/listingkit/httpapi/bootstrap_submit_module.go:12-175`
- Modify: `internal/listingkit/httpapi/bootstrap_runtime.go:90-115`
- Modify: `internal/listingkit/httpapi/runtime_support_hooks.go:10-50`
- Modify: `internal/listingkit/httpapi/bootstrap_test.go`
- Modify: `internal/listingkit/httpapi/builders_repository_schema.go:97-195`
- Modify: `internal/listingkit/httpapi/builders_test.go:34-260`
- Modify: `internal/app/runtime/listingkitschemamigrate/runtime.go:100-215`
- Modify: `internal/app/runtime/listingkitschemamigrate/runtime_test.go:1-110`

**Interfaces:**
- Consumes: `BuildStudioAICapabilityRouter`, `NewStudioAIImageCapabilityAdapter`, `aicapability.InvocationRecorder`, GORM recorder and `Config.AICapability.StudioImageRoutingMode`.
- Produces: a complete runtime path from existing credential resolver and DB to the single Studio `AIImageGenerator` dependency.

- [ ] **Step 1: Write failing assembly and migration tests**

Add tests with these exact outcomes:

```go
func TestBuildSubmitModuleKeepsLegacyModeDependencyFree(t *testing.T) {
	module, err := buildSubmitModule(submitModuleInput{
		Config: &config.Config{AICapability: config.AICapabilityConfig{StudioImageRoutingMode: "legacy"}},
		Hooks: submitModuleHooks{
			StudioImageGeneratorBuilder: func(*config.Config, openaiclient.ClientConfigResolver) openaiclient.ImageGenerator {
				return &httpapiStubImageGenerator{}
			},
		},
	})
	require.NoError(t, err)
	require.NotNil(t, module.studio.imageGenerator)
}

func TestBuildSubmitModuleRejectsShadowWithoutGovernanceDependencies(t *testing.T) {
	_, err := buildSubmitModule(submitModuleInput{
		Config: &config.Config{AICapability: config.AICapabilityConfig{StudioImageRoutingMode: "shadow"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Studio AI capability routing requires")
}

func TestAutoMigrateListingKitRuntimeSchemaCreatesAIInvocationsTable(t *testing.T) {
	db := openListingKitBuilderTestDB(t)
	require.NoError(t, AutoMigrateListingKitRuntimeSchema(db))
	assert.True(t, db.Migrator().HasTable("ai_invocations"))
}
```

In `runtime_ai_capability_test.go`, verify legacy mode returns empty deps without opening DB, while shadow/active with nil database returns a startup error. Successful recorder persistence is already covered with SQLite in Task 3, so these runtime tests do not open PostgreSQL.

- [ ] **Step 2: Run focused tests and verify failure**

```powershell
go test ./internal/listingkit/httpapi ./internal/app/httpapi ./internal/app/runtime/listingkitschemamigrate -run 'TestBuildSubmitModule|Test.*AIInvocation|Test.*AICapability' -count=1
```

Expected: FAIL because recorder dependencies, error-returning submit assembly and migrations are not wired.

- [ ] **Step 3: Add dedicated AI capability persistence assembly**

`runtime_ai_capability.go` must expose an internal dependency group:

```go
type aiCapabilityRuntimeDeps struct {
	invocationRecorder aicapability.InvocationRecorder
	closers             []func() error
}

func buildAICapabilityRuntimeDeps(cfg *config.Config, logger *logrus.Logger) (*aiCapabilityRuntimeDeps, error)
```

`adapters_ai_capability.go` owns the concrete database construction behind this exact helper:

```go
func newDBAIInvocationRecorder(
	cfg *config.DatabaseConfig,
	logger *logrus.Logger,
) (aicapability.InvocationRecorder, func() error, error)
```

Behavior:

- `legacy` -> return empty deps and no DB reference;
- `shadow|active` -> require `cfg.Database`, open it through `database.NewSharedDatabaseFromConfig`, create `store.NewGormInvocationRecorder`, and return a matching `CloseSharedDatabase` closer;
- when product-listing-api auto migration is enabled, call `AutoMigrateInvocationLedger` before returning;
- startup errors identify the AI capability resource without logging DSN passwords.

Keep this separate from `runtime_openai.go`: the credential resolver remains an existing integration, while the invocation ledger is provider-neutral.

- [ ] **Step 4: Pass the recorder through runtime contracts**

Add `AIInvocationRecorder aicapability.InvocationRecorder` to:

- `sharedRuntimeDeps`;
- ListingKit `RuntimeDependencies`;
- `BuildServiceInput`;
- `submitModuleInput`.

Call `buildAICapabilityRuntimeDeps` immediately after `buildOpenAIRuntimeDeps`, append its closers, and pass the recorder from `newListingKitRuntimeBuildInput`. Do not place routing or policy decisions in `internal/app/*`.

- [ ] **Step 5: Build the Studio router and narrow adapter in ListingKit HTTP assembly**

Add this hook contract:

```go
StudioAICapabilityRouterBuilder func(openaiclient.ClientConfigResolver) aicapability.Router
```

Register `BuildStudioAICapabilityRouter` in `buildRuntimeSupportHooks`.

Change `buildSubmitModule` to return `(submitModule, error)`. After constructing the legacy local image generator:

1. parse the validated mode with `aicapability.ParseRoutingMode`;
2. return legacy unchanged for `legacy`;
3. for shadow/active, require credential resolver, router builder, invocation recorder and non-nil legacy generator;
4. construct the router through the hook;
5. call `listingkit.NewStudioAIImageCapabilityAdapter`;
6. pass an `OnRecordError(record, err)` callback that logs only `record.InvocationID`, tenant, capability, operation and the recorder error, never prompt/provider payload;
7. assign the returned local generator to `submit.studio.imageGenerator`.

Update `bootstrap_runtime.go` to return a wrapped startup error from the new `(submitModule, error)` result. Update existing direct unit-test calls accordingly.

- [ ] **Step 6: Register the ledger schema in every supported schema path**

Call `aicapabilitystore.AutoMigrateInvocationLedger(db)` from:

- `AutoMigrateProductListingAPIRuntimeSchema`;
- `AutoMigrateListingKitRuntimeSchema`;
- `listingkitschemamigrate.runMigrations`.

Wrap errors consistently as `ai invocation ledger auto-migrate failed: %w`. Add SQLite assertions to both existing schema test suites. Do not add a second credential table or model-policy table in Phase 1.

- [ ] **Step 7: Run assembly, schema, and Studio regression tests**

```powershell
gofmt -w internal/app/httpapi/adapters_ai_capability.go internal/app/httpapi/runtime_ai_capability.go internal/app/httpapi/runtime_ai_capability_test.go internal/app/httpapi/runtime.go internal/app/httpapi/runtime_shared_deps.go internal/app/httpapi/feature_builder_listingkit.go internal/app/httpapi/adapters_schema_migration.go internal/listingkit/httpapi/bootstrap_contracts.go internal/listingkit/httpapi/runtime_builder.go internal/listingkit/httpapi/bootstrap_submit_module.go internal/listingkit/httpapi/bootstrap_runtime.go internal/listingkit/httpapi/runtime_support_hooks.go internal/listingkit/httpapi/bootstrap_test.go internal/listingkit/httpapi/builders_repository_schema.go internal/listingkit/httpapi/builders_test.go internal/app/runtime/listingkitschemamigrate/runtime.go internal/app/runtime/listingkitschemamigrate/runtime_test.go
go test ./internal/app/httpapi ./internal/listingkit/httpapi ./internal/app/runtime/listingkitschemamigrate -count=1
go test ./internal/listingkit -run 'TestStudioAIImageCapabilityAdapter|TestGenerateStudioDesign|TestTaskStudio' -count=1
```

Expected: PASS. No test may instantiate a real provider client with live credentials.

- [ ] **Step 8: Commit Task 6**

```powershell
git add -- internal/app/httpapi/adapters_ai_capability.go internal/app/httpapi/runtime_ai_capability.go internal/app/httpapi/runtime_ai_capability_test.go internal/app/httpapi/runtime.go internal/app/httpapi/runtime_shared_deps.go internal/app/httpapi/feature_builder_listingkit.go internal/app/httpapi/adapters_schema_migration.go internal/listingkit/httpapi/bootstrap_contracts.go internal/listingkit/httpapi/runtime_builder.go internal/listingkit/httpapi/bootstrap_submit_module.go internal/listingkit/httpapi/bootstrap_runtime.go internal/listingkit/httpapi/runtime_support_hooks.go internal/listingkit/httpapi/bootstrap_test.go internal/listingkit/httpapi/builders_repository_schema.go internal/listingkit/httpapi/builders_test.go internal/app/runtime/listingkitschemamigrate/runtime.go internal/app/runtime/listingkitschemamigrate/runtime_test.go
git diff --cached --check
git commit -m "feat(ai): wire Studio capability governance runtime"
```

---

### Task 7: Enforce boundaries and complete release-grade verification

**Files:**
- Modify: `tests/import_boundaries_test.go`
- Modify: `docs/architecture/project-boundaries.md`
- Modify: `docs/architecture/external-client-boundary-inventory.md`
- Test: `tests/import_boundaries_test.go`

**Interfaces:**
- Consumes: final package layout from Tasks 1-6.
- Produces: executable dependency guards and documentation of the Phase 1 boundary.

- [ ] **Step 1: Write the failing import-boundary test**

Add this test using the existing helper:

```go
func TestAICapabilityModuleDoesNotImportBusinessOrProviderPackages(t *testing.T) {
	assertNoBannedImportPrefixes(t, filepath.Join("..", "internal", "aicapability"), []string{
		"task-processor/internal/app",
		"task-processor/internal/asset",
		"task-processor/internal/catalog",
		"task-processor/internal/infra/clients",
		"task-processor/internal/listingkit",
		"task-processor/internal/marketplace",
		"task-processor/internal/productenrich",
		"task-processor/internal/productimage",
		"task-processor/internal/publishing",
		"task-processor/internal/shein",
		"task-processor/internal/temu",
		"task-processor/internal/workspace",
	}, nil)
}
```

The GORM store remains valid because it imports `aicapability`, GORM and standard library only. Do not add the new Studio domain adapter to any concrete-provider allowlist.

- [ ] **Step 2: Run the boundary test**

```powershell
go test ./tests -run 'TestAICapabilityModuleDoesNotImportBusinessOrProviderPackages' -count=1
```

Expected before final cleanup: FAIL if any new generic contract or store imported a business/provider package; PASS only after those imports are removed.

- [ ] **Step 3: Update architecture inventories**

In `project-boundaries.md`:

- list `internal/aicapability` under platform/integration-style neutral modules;
- state that it owns model catalog/policy/routing/invocation contracts but not product facts, marketplace rules, Prompt meaning or provider SDKs;
- add `TestAICapabilityModuleDoesNotImportBusinessOrProviderPackages` to Current Enforcement.

In `external-client-boundary-inventory.md`:

- record the new direction `ListingKit local AI port -> Studio capability adapter -> aicapability router -> existing provider seam`;
- state that `listingkit/httpapi` still contains the compatibility credential/provider adapter in Phase 1;
- state that other direct OpenAI imports are unchanged and are not silently considered migrated.

- [ ] **Step 4: Run the complete targeted verification**

Run every command fresh and stop on the first failure:

```powershell
$goFiles = @(
  Get-ChildItem -LiteralPath internal/aicapability -Recurse -Filter '*.go'
  Get-Item -LiteralPath internal/listingkit/studio_ai_capability_adapter.go
  Get-Item -LiteralPath internal/listingkit/studio_ai_capability_adapter_test.go
  Get-Item -LiteralPath internal/listingkit/httpapi/ai_capability_studio_catalog.go
  Get-Item -LiteralPath internal/listingkit/httpapi/ai_capability_studio_catalog_test.go
  Get-Item -LiteralPath internal/listingkit/httpapi/ai_client_image_routing.go
  Get-ChildItem -LiteralPath internal/core/config -Filter '*.go'
  Get-ChildItem -LiteralPath internal/app/httpapi -Filter '*.go'
  Get-ChildItem -LiteralPath internal/app/runtime/listingkitschemamigrate -Filter '*.go'
  Get-Item -LiteralPath tests/import_boundaries_test.go
)
$unformatted = @(gofmt -l $goFiles.FullName)
if ($unformatted.Count -gt 0) { $unformatted; exit 1 }
go test ./internal/aicapability/... -count=1
go test ./internal/core/config -count=1
go test ./internal/listingkit ./internal/listingkit/httpapi -count=1
go test ./internal/app/httpapi ./internal/app/runtime/listingkitschemamigrate -count=1
go test ./tests/... -count=1
go test ./... -run '^$' -count=1
./scripts/analyze-project-deps.ps1 -Root . -FailOnViolation
git diff --check
```

Expected: all exit 0. The compile-only command validates all packages without invoking real providers; it is not a substitute for the focused behavioral tests above.

- [ ] **Step 5: Inspect the final diff against the approved Phase 1 scope**

```powershell
git status --short
git diff --stat
git diff -- internal/aicapability internal/listingkit/studio_ai_capability_adapter.go internal/listingkit/httpapi/ai_capability_studio_catalog.go internal/core/config internal/app/httpapi internal/app/runtime/listingkitschemamigrate tests/import_boundaries_test.go docs/architecture/project-boundaries.md docs/architecture/external-client-boundary-inventory.md
```

Confirm all of the following from the actual diff:

- no Agent framework or Agent runtime dependency;
- no new service entrypoint, API route or deployment manifest;
- no provider DTO in `internal/aicapability` or the Studio domain adapter;
- no Prompt content/registry behavior change;
- no raw prompt, response, image bytes or API key ledger columns/logs;
- `legacy` remains the checked-in default;
- shadow leaves provider requests unchanged;
- active is reversible through one config value;
- unrelated worktree files are not staged.

- [ ] **Step 6: Commit Task 7**

```powershell
git add -- tests/import_boundaries_test.go docs/architecture/project-boundaries.md docs/architecture/external-client-boundary-inventory.md
git diff --cached --check
git commit -m "docs(ai): enforce capability governance boundary"
```

---

## Operational rollout after merge

These are deployment gates, not implementation steps, and require current environment verification:

1. Deploy with `aiCapability.studioImageRoutingMode=legacy`; confirm schema migration and no runtime regression.
2. Enable `shadow` for one controlled deployment/tenant environment; compare ledger `RouteDecision` fields with the actual legacy provider/model for the fixed Studio test corpus.
3. Require zero unexplained selector/provider mismatches, acceptable ledger write failure rate, and unchanged Studio request/result contract before active promotion.
4. Enable `active` for the controlled scope; monitor provider errors, latency, usage, cost and async job completion.
5. Roll back immediately by restoring `legacy` if routing mismatch, data-policy violation, ledger instability correlated with runtime load, or Studio quality regression appears.
6. Keep Agent PoC and other capability migrations as separate approved plans.
