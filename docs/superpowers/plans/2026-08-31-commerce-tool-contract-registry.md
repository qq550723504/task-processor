# Commerce Tool 合同与注册表实施计划

> **供智能体执行者使用：** 必须使用 `superpowers:subagent-driven-development`（推荐）或 `superpowers:executing-plans` 子技能逐任务实施本计划。所有步骤使用复选框（`- [ ]`）跟踪。

**目标：** 实现 Phase 2A 的框架无关 Commerce Tool 合同、不可变注册表、精确 Agent allowlist、Schema 校验、安全调用链和审计元数据，完成 GitHub Issue #133 的代码范围。

**架构：** 新建独立的 `internal/commercetool` 包；领域 Tool 以 `Definition + Executor` 形式注入，不进入 `kernel/module.Registry`。Registry 构造时编译 Schema，绑定时固定 Agent 可用的精确 Tool 版本，调用时依次执行风险、可信身份、权限、幂等、输入 Schema、超时、单次 Executor、输出 Schema、Trace 和审计。

**技术栈：** Go 1.26、`github.com/santhosh-tekuri/jsonschema/v6`、`golang.org/x/mod/semver`、OpenTelemetry Trace API、`testify/require`。

**规格：** `docs/superpowers/specs/2026-08-31-commerce-tool-foundation-design.md`

## 全局约束

- 实施前，当前主分支必须已经包含目标目录架构 Phase 2；执行 `git show HEAD:internal/platform/featureflag/runtime.go` 必须成功。
- 实施分支必须基于届时最新 `main`，不能直接在当前旧基线或正在开发的目录架构工作树中写代码。
- 本计划只实现规格中的切片 A（#133）；`product.canonical.inspect` 属于单独的切片 B 计划。
- 只新增 `internal/commercetool`、必要依赖声明和稳定架构文档；不得修改 `internal/kernel/module.Registry`，不得新增对根 `internal/listingkit` 的生产 import。
- `internal/commercetool` 不得导入 Agent Framework、Gin、Temporal、RabbitMQ、GORM、Provider SDK、Marketplace Client、Product DTO 或 Listing DTO。
- Registry 可以识别全部风险值，但 Phase 2A 只能绑定和执行 `read`、`compute`、`propose`。
- tenant、user、roles 不得出现在模型可见 Tool JSON 中，只能来自注入的可信 `PrincipalResolver`。
- Registry 不自动重试 Executor；一次 `Invoke` 最多调用一次 Executor。
- AI 模型路由、成本、Provider 重试和模型 Invocation Ledger 继续由 `internal/aicapability` 所有。
- Input/Output 使用 JSON Schema Draft 2020-12；Object Schema 必须声明 `additionalProperties: false`。
- 所有实现步骤测试先行，每个任务独立提交，不混入 #134 的领域 Adapter。

---

## 文件结构

| 文件 | 职责 |
| --- | --- |
| `internal/commercetool/doc.go` | 包级所有权与禁止依赖说明 |
| `internal/commercetool/definition.go` | Tool/Agent 标识与全部治理声明 |
| `internal/commercetool/errors.go` | Tool 边界错误和固定 retryability |
| `internal/commercetool/schema.go` | Draft 2020-12 Schema 编译与校验 |
| `internal/commercetool/invocation_contracts.go` | Executor、Principal、Call、Result 和依赖接口 |
| `internal/commercetool/audit.go` | 非敏感 Audit Record 和 Recorder Port |
| `internal/commercetool/registry.go` | 不可变 Registry 与精确 Agent 绑定 |
| `internal/commercetool/invocation.go` | 安全 Invoke 顺序、Trace、Hash 和 Audit |
| `internal/commercetool/*_test.go` | 各职责的 TDD、闭环和边界测试 |
| `docs/architecture/project-target-architecture.md` | Commerce Tool 永久所有权 |
| `docs/development/repository-structure.md` | 新稳定包职责 |
| `go.mod`、`go.sum` | 将实际生产 import 的既有依赖提升为 direct dependency |

本计划不创建 `adapter`、`store`、`httpapi` 或 `runtime` 子目录。

---

### 任务 1：定义 Tool 元数据与静态一致性规则

**文件：**

- 新建：`internal/commercetool/doc.go`
- 新建：`internal/commercetool/definition.go`
- 新建：`internal/commercetool/definition_test.go`
- 修改：`go.mod`
- 修改：`go.sum`

**接口：**

- 产出：`ToolRef.Validate() error`
- 产出：`AgentRef.Validate() error`
- 产出：`Definition.Validate() error`
- 后续消费：全部风险、权限、副作用、幂等、超时、重试和用量类型

- [ ] **步骤 1：编写失败测试**

在 `definition_test.go` 建立唯一合法 fixture：

```go
func validDefinition() Definition {
	return Definition{
		Ref:         ToolRef{ID: "product.canonical.inspect", Version: "v1.0.0"},
		Capability:  "product.canonical",
		Owner:       "product.catalog",
		Description: "Inspect the authorized canonical product projection.",
		InputSchema: json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["task_id"],"properties":{"task_id":{"type":"string","minLength":1}}}`),
		OutputSchema: json.RawMessage(`{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object","additionalProperties":false,"required":["task_id"],"properties":{"task_id":{"type":"string"}}}`),
		Risk:        RiskRead,
		Permission:  PermissionRequirement{Permission: "listingkit.admin.read"},
		SideEffects: SideEffectPolicy{Mode: SideEffectNone},
		Idempotency: IdempotencyPolicy{Mode: IdempotencyDeterministic},
		Timeout:     TimeoutPolicy{Duration: 2 * time.Second},
		Retry:       RetryPolicy{Owner: RetryOwnerCaller},
		Usage:       UsagePolicy{Owner: UsageOwnerUnmetered},
	}
}

func TestDefinitionValidateAcceptsCompleteReadTool(t *testing.T) {
	require.NoError(t, validDefinition().Validate())
}
```

增加表驱动测试，分别清空 `Ref.ID`、`Ref.Version`、`Capability`、`Owner`、
`Description`、两个 Schema、Risk、Permission、SideEffects、Idempotency、Timeout、
Retry Owner、Usage Owner，逐项断言 `Validate()` 失败。另用矩阵测试以下唯一合法组合：

| Risk | SideEffect | Idempotency |
| --- | --- | --- |
| `read` / `compute` / `propose` | `none` | `not_applicable` / `deterministic` / `required_key` |
| `write` | `business_mutation` | `required_key` |
| `publish` | `external_mutation` | `required_key` |

- [ ] **步骤 2：运行测试，确认失败**

运行：`go test ./internal/commercetool -run 'TestDefinition' -count=1 -v`

预期：FAIL，包含 `undefined: Definition` 或包尚不存在。

- [ ] **步骤 3：实现最小合同**

`definition.go` 定义以下精确枚举：

```go
type RiskLevel string
const (
	RiskRead RiskLevel = "read"
	RiskCompute RiskLevel = "compute"
	RiskPropose RiskLevel = "propose"
	RiskWrite RiskLevel = "write"
	RiskPublish RiskLevel = "publish"
)

type SideEffectMode string
const (
	SideEffectNone SideEffectMode = "none"
	SideEffectBusinessMutation SideEffectMode = "business_mutation"
	SideEffectExternalMutation SideEffectMode = "external_mutation"
)

type IdempotencyMode string
const (
	IdempotencyNotApplicable IdempotencyMode = "not_applicable"
	IdempotencyDeterministic IdempotencyMode = "deterministic"
	IdempotencyRequiredKey IdempotencyMode = "required_key"
)

type RetryOwner string
const (
	RetryOwnerNone RetryOwner = "none"
	RetryOwnerCaller RetryOwner = "caller"
	RetryOwnerAICapability RetryOwner = "ai_capability"
	RetryOwnerDomainWorkflow RetryOwner = "domain_workflow"
)

type UsageOwner string
const (
	UsageOwnerUnmetered UsageOwner = "unmetered"
	UsageOwnerAICapability UsageOwner = "ai_capability"
	UsageOwnerDomainLedger UsageOwner = "domain_ledger"
)

type ToolRef struct {
	ID      string
	Version string
}

type AgentRef struct {
	ID      string
	Version string
}

type PermissionRequirement struct{ Permission string }
type SideEffectPolicy struct{ Mode SideEffectMode }
type IdempotencyPolicy struct{ Mode IdempotencyMode }
type TimeoutPolicy struct{ Duration time.Duration }
type RetryPolicy struct{ Owner RetryOwner }
type UsagePolicy struct{ Owner UsageOwner }

type Definition struct {
	Ref          ToolRef
	Capability   string
	Owner        string
	Description  string
	InputSchema  json.RawMessage
	OutputSchema json.RawMessage
	Risk         RiskLevel
	Permission   PermissionRequirement
	SideEffects  SideEffectPolicy
	Idempotency  IdempotencyPolicy
	Timeout      TimeoutPolicy
	Retry        RetryPolicy
	Usage        UsagePolicy
}
```

`ToolRef` 与 `AgentRef` 的 ID，以及 Capability、Owner、Permission，必须匹配
`^[a-z][a-z0-9_-]*(\.[a-z][a-z0-9_-]*)*$`；Description 必须 trim 后非空。Version
必须同时满足
`semver.IsValid(version)` 和
`semver.Canonical(version) == version`，从而拒绝 `v1`、`1.0.0` 等非完整规范形式；
Timeout 必须大于 0；其余校验严格实现测试矩阵。此任务只检查 Schema 非空，不编译
Schema。

在表驱动测试加入大写、空格、斜杠和冒号 ID，以及 `v1`、无 `v` 前缀、build metadata
Version；全部必须构造失败，防止后续 Registry Key 或 Schema URI 出现歧义。

除枚举值域和上表外，静态一致性规则必须包括：`write/publish` 只能使用
`required_key`；`RetryOwnerAICapability` 必须同时配合 `RiskPropose` 与
`UsageOwnerAICapability`；`UsageOwnerAICapability` 只能用于 `RiskPropose`。这些规则用
独立表驱动 Case 验证，避免靠后续 Bind 才发现矛盾 Definition。

- [ ] **步骤 4：验证并整理依赖**

```powershell
go test ./internal/commercetool -run 'TestDefinition' -count=1 -v
go mod tidy
go test ./internal/commercetool -run 'TestDefinition' -count=1
```

预期：全部 PASS；`golang.org/x/mod` 使用架构分支已锁定的版本并成为 direct dependency。

- [ ] **步骤 5：提交**

```powershell
git add go.mod go.sum internal/commercetool/doc.go internal/commercetool/definition.go internal/commercetool/definition_test.go
git commit -m "feat(commercetool): define governed tool metadata"
```

---

### 任务 2：实现确定性 Tool Error 分类

**文件：**

- 新建：`internal/commercetool/errors.go`
- 新建：`internal/commercetool/errors_test.go`

**接口：**

- 产出：`NewError(code ErrorCode, safeMessage string, cause error) error`
- 产出：`CodeOf(error) ErrorCode`
- 产出：`IsRetryable(ErrorCode) bool`

- [ ] **步骤 1：编写失败测试**

```go
func TestToolErrorDoesNotExposeCauseInMessage(t *testing.T) {
	err := NewError(ErrorPermissionDenied, "tool permission denied", errors.New("secret database detail"))
	require.Equal(t, "permission_denied: tool permission denied", err.Error())
	require.NotContains(t, err.Error(), "secret database detail")
	require.Equal(t, ErrorPermissionDenied, CodeOf(err))
	require.ErrorContains(t, errors.Unwrap(err), "secret database detail")
}

func TestErrorRetryabilityIsFixedByCode(t *testing.T) {
	require.True(t, IsRetryable(ErrorDeadlineExceeded))
	require.True(t, IsRetryable(ErrorDependencyUnavailable))
	require.False(t, IsRetryable(ErrorPermissionDenied))
	require.False(t, IsRetryable(ErrorUnknownExecutionState))
}

func TestCodeOfFindsWrappedToolError(t *testing.T) {
	err := fmt.Errorf("adapter boundary: %w", NewError(ErrorNotFound, "resource not found", nil))
	require.Equal(t, ErrorNotFound, CodeOf(err))
}
```

增加表驱动测试覆盖规格中的 13 个 Error Code；空或未知 Code 在 `NewError` 中必须
归一化为 `internal`，普通 Go Error 的 `CodeOf` 也必须返回 `internal`。

- [ ] **步骤 2：运行测试，确认失败**

运行：`go test ./internal/commercetool -run 'Test(ToolError|CodeOf|ErrorRetryability)' -count=1 -v`

预期：FAIL，包含 `undefined: NewError` 或 `undefined: ErrorCode`。

- [ ] **步骤 3：实现安全错误类型**

定义以下精确常量：

```go
type ErrorCode string

const (
	ErrorInvalidInput          ErrorCode = "invalid_input"
	ErrorIdentityIntegrity     ErrorCode = "identity_integrity"
	ErrorPermissionDenied      ErrorCode = "permission_denied"
	ErrorToolNotAllowed        ErrorCode = "tool_not_allowed"
	ErrorNotFound              ErrorCode = "not_found"
	ErrorFailedPrecondition    ErrorCode = "failed_precondition"
	ErrorConflict              ErrorCode = "conflict"
	ErrorDeadlineExceeded      ErrorCode = "deadline_exceeded"
	ErrorDependencyUnavailable ErrorCode = "dependency_unavailable"
	ErrorOutputInvalid         ErrorCode = "output_invalid"
	ErrorBudgetExceeded        ErrorCode = "budget_exceeded"
	ErrorUnknownExecutionState ErrorCode = "unknown_execution_state"
	ErrorInternal              ErrorCode = "internal"
)
```

`ToolError` 只保存导出的 `Code`、`SafeMessage` 和未导出的 `cause`。`Error()` 不能拼接
cause；`Unwrap()` 返回 cause；`CodeOf` 使用 `errors.As` 支持被 `%w` 包装的 ToolError。
只有 `deadline_exceeded`、`dependency_unavailable`
可重试，`unknown_execution_state` 需要对账而不是自动重试。

- [ ] **步骤 4：验证并提交**

```powershell
go test ./internal/commercetool -run 'Test(ToolError|CodeOf|ErrorRetryability)' -count=1 -v
git add internal/commercetool/errors.go internal/commercetool/errors_test.go
git commit -m "feat(commercetool): normalize tool boundary errors"
```

---

### 任务 3：编译并执行严格 JSON Schema

**文件：**

- 新建：`internal/commercetool/schema.go`
- 新建：`internal/commercetool/schema_test.go`
- 修改：`go.mod`
- 修改：`go.sum`

**接口：**

- 产出：`compileSchemas(Definition) (compiledSchemas, error)`
- 产出：`compiledSchemas.validateInput(json.RawMessage) error`
- 产出：`compiledSchemas.validateOutput(json.RawMessage) error`

- [ ] **步骤 1：编写失败测试**

```go
func TestCompileSchemasRejectsInvalidSchemaAtConstruction(t *testing.T) {
	definition := validDefinition()
	definition.InputSchema = json.RawMessage(`{"type":"not-a-json-schema-type"}`)
	_, err := compileSchemas(definition)
	require.Error(t, err)
}

func TestCompiledSchemasRejectsUndeclaredInputAuthority(t *testing.T) {
	schemas, err := compileSchemas(validDefinition())
	require.NoError(t, err)
	err = schemas.validateInput(json.RawMessage(`{"task_id":"task-1","tenant_id":"attacker"}`))
	require.Equal(t, ErrorInvalidInput, CodeOf(err))
}

func TestCompiledSchemasRejectsInvalidOutput(t *testing.T) {
	schemas, err := compileSchemas(validDefinition())
	require.NoError(t, err)
	err = schemas.validateOutput(json.RawMessage(`{"unexpected":true}`))
	require.Equal(t, ErrorOutputInvalid, CodeOf(err))
}

func TestCompileSchemasRequiresClosedRootObjects(t *testing.T) {
	definition := validDefinition()
	definition.InputSchema = json.RawMessage(`{
		"$schema":"https://json-schema.org/draft/2020-12/schema",
		"type":"object",
		"properties":{"task_id":{"type":"string"}}
	}`)
	_, err := compileSchemas(definition)
	require.Error(t, err)
}

func TestCompileSchemasRejectsTrailingJSON(t *testing.T) {
	definition := validDefinition()
	definition.OutputSchema = append(definition.OutputSchema, []byte(` {}`)...)
	_, err := compileSchemas(definition)
	require.Error(t, err)
}
```

增加合法 Input/Output 通过测试，以及无效 JSON 返回相应边界 Error Code 的测试。

- [ ] **步骤 2：运行测试，确认失败**

运行：`go test ./internal/commercetool -run 'TestCompile|TestCompiled' -count=1 -v`

预期：FAIL，包含 `undefined: compileSchemas`。

- [ ] **步骤 3：实现 Draft 2020-12 编译器**

```go
func compileSchema(location string, raw json.RawMessage) (*jsonschema.Schema, error) {
	var document any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("schema must contain exactly one JSON document")
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft2020)
	if err := compiler.AddResource(location, document); err != nil {
		return nil, err
	}
	return compiler.Compile(location)
}
```

Input/Output 分别使用以下内存 URI：

```text
urn:task-processor:commerce-tool:<tool-id>:<version>:input
urn:task-processor:commerce-tool:<tool-id>:<version>:output
```

编译前把根文档解码为 `map[string]any`，强制 Input/Output 根节点的 `type` 为
`object` 且 `additionalProperties` 精确为 `false`；不能依赖 Draft 默认值，也不能在
运行时偷偷补写调用方 Schema。校验时使用 `json.Decoder.UseNumber` 解码，再调用
`Schema.Validate`，并用第二次 Decode 必须返回 `io.EOF` 的规则拒绝尾随 JSON。Input
失败统一包装为
`invalid_input: tool input does not match schema`，Output 失败统一包装为
`output_invalid: tool output does not match schema`；底层 Schema Error 只作为 cause。

- [ ] **步骤 4：验证、整理依赖并提交**

```powershell
go test ./internal/commercetool -run 'TestCompile|TestCompiled' -count=1 -v
go mod tidy
go test ./internal/commercetool -count=1
git add go.mod go.sum internal/commercetool/schema.go internal/commercetool/schema_test.go
git commit -m "feat(commercetool): enforce tool JSON schemas"
```

预期：全部 PASS；`github.com/santhosh-tekuri/jsonschema/v6 v6.0.2` 成为 direct
dependency，不增加其他 Schema Library。

---

### 任务 4：构建不可变 Registry 与精确 Agent Allowlist

**文件：**

- 新建：`internal/commercetool/invocation_contracts.go`
- 新建：`internal/commercetool/audit.go`
- 新建：`internal/commercetool/registry.go`
- 新建：`internal/commercetool/registry_test.go`
- 修改：`go.mod`
- 修改：`go.sum`

**接口：**

- 产出：`NewRegistry(tools ...Tool) (*Registry, error)`
- 产出：`(*Registry).Bind(AgentDefinition, InvocationDependencies) (*BoundToolSet, error)`
- 产出：`(*BoundToolSet).Definitions() []Definition`
- 后续消费：任务 5 在 `BoundToolSet` 上实现 `Invoke`

- [ ] **步骤 1：编写 Registry 失败测试**

```go
func TestNewRegistryRejectsDuplicateExactToolRef(t *testing.T) {
	tool := validTool()
	_, err := NewRegistry(tool, tool)
	require.ErrorContains(t, err, "duplicate tool")
}

func TestNewRegistryRejectsTypedNilExecutor(t *testing.T) {
	var executor *stubExecutor
	_, err := NewRegistry(Tool{Definition: validDefinition(), Executor: executor})
	require.ErrorContains(t, err, "executor is nil")
}

func TestRegistryBindRequiresExactToolVersion(t *testing.T) {
	tool := validTool()
	registry, err := NewRegistry(tool)
	require.NoError(t, err)
	agent := agentAllowing(ToolRef{ID: tool.Definition.Ref.ID, Version: "v2.0.0"})
	_, err = registry.Bind(agent, validInvocationDependencies())
	require.Equal(t, ErrorToolNotAllowed, CodeOf(err))
}
```

增加 `write`、`publish` Definition 可以通过 `NewRegistry`，但在 `Bind` 时返回
`tool_not_allowed` 的测试。增加防御性拷贝测试：Registry 构造后修改原始 Schema，或
修改 `Definitions()` 返回值，都不能改变 Registry 内保存的 Definition 和编译结果。

- [ ] **步骤 2：运行测试，确认失败**

运行：`go test ./internal/commercetool -run 'Test(NewRegistry|RegistryBind|BoundToolSet)' -count=1 -v`

预期：FAIL，包含 `undefined: NewRegistry` 或 `undefined: Tool`。

- [ ] **步骤 3：定义调用与审计合同**

`invocation_contracts.go` 定义：

```go
type Executor interface {
	Execute(context.Context, json.RawMessage) (json.RawMessage, error)
}

type ExecutorFunc func(context.Context, json.RawMessage) (json.RawMessage, error)

func (f ExecutorFunc) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	return f(ctx, input)
}

type Principal struct {
	TenantID string
	UserID   string
	Roles    []string
}

type PrincipalResolver interface {
	ResolvePrincipal(context.Context) (Principal, error)
}

type Authorizer interface {
	Authorize(context.Context, Principal, PermissionRequirement) error
}

type InvocationDependencies struct {
	PrincipalResolver PrincipalResolver
	Authorizer        Authorizer
	Recorder          AuditRecorder
	Tracer            trace.Tracer
	Now               func() time.Time
	AuditTimeout      time.Duration
}

type Tool struct {
	Definition Definition
	Executor   Executor
}

type AgentDefinition struct {
	ID           string
	Version      string
	AllowedTools []ToolRef
}
```

`audit.go` 定义以下精确类型：

```go
type AuditOutcome string

const (
	AuditOutcomeSucceeded AuditOutcome = "succeeded"
	AuditOutcomeFailed    AuditOutcome = "failed"
)

type AuditRecord struct {
	CallID         string
	AgentID        string
	AgentVersion   string
	AgentRunID     string
	ToolID         string
	ToolVersion    string
	Capability     string
	Owner          string
	TenantID       string
	UserID         string
	BusinessTaskID string
	TraceID        string
	Risk           RiskLevel
	Permission     string
	RetryOwner     RetryOwner
	UsageOwner     UsageOwner
	StartedAt      time.Time
	FinishedAt     time.Time
	LatencyMillis  int64
	InputHash      string
	OutputHash     string
	Outcome        AuditOutcome
	ErrorCode      ErrorCode
	AIInvocationID string
}

type AuditRecorder interface {
	RecordToolCall(context.Context, AuditRecord) error
}
```

`AIInvocationID` 在确定性切片 A 中保持空值，供后续 AI-backed Adapter 关联真实
`aicapability` Ledger；Core 不创建假 ID。结构体禁止新增 Raw Input、Raw Output、Prompt、
Credential 或 Provider Response 字段。

`InvocationDependencies.Validate` 拒绝 typed nil Resolver、Authorizer、Recorder、Tracer，
以及 nil `Now` 和非正 `AuditTimeout`。解析出的 Principal 必须同时有非空 TenantID、
UserID 和至少一个 Role，否则返回 `identity_integrity`，不得调用 Authorizer。

`registry_test.go` 同时定义后续测试复用的唯一合同 fixture，不在生产代码增加 mock API：

```go
type stubExecutor struct {
	output json.RawMessage
	err    error
	calls  *int
}

func (s *stubExecutor) Execute(context.Context, json.RawMessage) (json.RawMessage, error) {
	if s.calls != nil {
		*s.calls = *s.calls + 1
	}
	return cloneRaw(s.output), s.err
}

type resolverStub struct {
	principal Principal
	err       error
}

func (s resolverStub) ResolvePrincipal(context.Context) (Principal, error) {
	return s.principal, s.err
}

type authorizerStub struct{ err error }

func (s authorizerStub) Authorize(context.Context, Principal, PermissionRequirement) error {
	return s.err
}

type recordingAuditStub struct {
	records []AuditRecord
	err     error
}

func (s *recordingAuditStub) RecordToolCall(_ context.Context, record AuditRecord) error {
	s.records = append(s.records, record)
	return s.err
}

func verifiedResolver() PrincipalResolver {
	return resolverStub{principal: Principal{
		TenantID: "tenant-1",
		UserID:   "user-1",
		Roles:    []string{"listingkit_admin"},
	}}
}

func validTool() Tool {
	return Tool{
		Definition: validDefinition(),
		Executor: &stubExecutor{
			output: json.RawMessage(`{"task_id":"task-1"}`),
		},
	}
}

func agentAllowing(ref ToolRef) AgentDefinition {
	return AgentDefinition{
		ID:           "fake.product-agent",
		Version:      "v1.0.0",
		AllowedTools: []ToolRef{ref},
	}
}

func validInvocationDependencies() InvocationDependencies {
	return InvocationDependencies{
		PrincipalResolver: verifiedResolver(),
		Authorizer:        authorizerStub{},
		Recorder:          &recordingAuditStub{},
		Tracer:            otel.Tracer("commercetool-test"),
		Now:               time.Now,
		AuditTimeout:      time.Second,
	}
}
```

- [ ] **步骤 4：实现 Registry 与绑定**

`NewRegistry` 固定执行：`Definition.Validate` -> typed nil Executor 检查 ->
`compileSchemas` -> 重复 Ref 检查 -> 深拷贝两个 `json.RawMessage` 后写入私有 map。

`Registry.Bind` 固定执行：用 `AgentRef{ID: agent.ID, Version: agent.Version}` 校验 Agent
Identity/Allowlist/Dependencies -> 拒绝重复 Allowlist Ref
-> 查找精确 ToolRef -> 拒绝 write/publish -> 创建只含 Allowlist Tool 的新 map。

`Definitions()` 按 Tool ID、Version 排序并返回深拷贝。不得公开 Global Registry map 或
未保护 Executor。

- [ ] **步骤 5：验证、整理依赖并提交**

```powershell
go test ./internal/commercetool -run 'Test(NewRegistry|RegistryBind|BoundToolSet)' -count=1 -v
go mod tidy
go test ./internal/commercetool -count=1
git add go.mod go.sum internal/commercetool/invocation_contracts.go internal/commercetool/audit.go internal/commercetool/registry.go internal/commercetool/registry_test.go
git commit -m "feat(commercetool): bind immutable agent tool sets"
```

预期：全部 PASS；OpenTelemetry Trace API 使用仓库锁定的 `v1.44.0`。

---

### 任务 5：实现安全、单次执行的 Tool Invocation Pipeline

**文件：**

- 修改：`internal/commercetool/invocation_contracts.go`
- 新建：`internal/commercetool/invocation.go`
- 新建：`internal/commercetool/invocation_test.go`
- 修改：`go.mod`
- 修改：`go.sum`

**接口：**

- 消费：任务 4 的 `BoundToolSet`、`InvocationDependencies`
- 产出：`(*BoundToolSet).Invoke(context.Context, Call) (Result, error)`
- 产出：`Call`、`CallMetadata`、`Result`、`AuditStatus`

`invocation_test.go` 先建立精确调用和绑定 helper：

```go
func validCall() Call {
	return Call{
		Tool: validDefinition().Ref,
		Metadata: CallMetadata{
			CallID:         "call-1",
			AgentID:        "fake.product-agent",
			AgentVersion:   "v1.0.0",
			AgentRunID:     "run-1",
			BusinessTaskID: "task-1",
			TraceID:        "trace-1",
		},
		Arguments: json.RawMessage(`{"task_id":"task-1"}`),
	}
}

func bindToolForTest(t *testing.T, executor Executor, deps InvocationDependencies) *BoundToolSet {
	t.Helper()
	definition := validDefinition()
	definition.Timeout.Duration = 25 * time.Millisecond
	registry, err := NewRegistry(Tool{Definition: definition, Executor: executor})
	require.NoError(t, err)
	bound, err := registry.Bind(agentAllowing(definition.Ref), deps)
	require.NoError(t, err)
	return bound
}

func boundToolSetForTest(t *testing.T, calls *int, resolver PrincipalResolver) *BoundToolSet {
	executor := ExecutorFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
		*calls = *calls + 1
		return json.RawMessage(`{"task_id":"task-1"}`), nil
	})
	deps := validInvocationDependencies()
	deps.PrincipalResolver = resolver
	return bindToolForTest(t, executor, deps)
}

func boundToolSetWithAuthorizer(t *testing.T, calls *int, authorizer Authorizer) *BoundToolSet {
	executor := ExecutorFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
		*calls = *calls + 1
		return json.RawMessage(`{"task_id":"task-1"}`), nil
	})
	deps := validInvocationDependencies()
	deps.Authorizer = authorizer
	return bindToolForTest(t, executor, deps)
}

func boundToolSetWithExecutor(t *testing.T, executor Executor) *BoundToolSet {
	return bindToolForTest(t, executor, validInvocationDependencies())
}
```

- [ ] **步骤 1：编写 Preflight 失败测试**

```go
func TestInvokeRejectsMissingVerifiedPrincipalBeforeExecution(t *testing.T) {
	calls := 0
	bound := boundToolSetForTest(t, &calls, resolverStub{})
	result, err := bound.Invoke(context.Background(), validCall())
	require.Equal(t, ErrorIdentityIntegrity, CodeOf(err))
	require.Equal(t, 0, calls)
	require.Equal(t, AuditStatusRecorded, result.AuditStatus)
}

func TestInvokeRejectsPermissionDeniedBeforeExecution(t *testing.T) {
	calls := 0
	bound := boundToolSetWithAuthorizer(t, &calls, authorizerStub{
		err: NewError(ErrorPermissionDenied, "tool permission denied", nil),
	})
	_, err := bound.Invoke(context.Background(), validCall())
	require.Equal(t, ErrorPermissionDenied, CodeOf(err))
	require.Equal(t, 0, calls)
}

func TestInvokeRejectsAuthorityFieldsOutsideInputSchema(t *testing.T) {
	calls := 0
	bound := boundToolSetForTest(t, &calls, verifiedResolver())
	call := validCall()
	call.Arguments = json.RawMessage(`{"task_id":"task-1","tenant_id":"attacker"}`)
	_, err := bound.Invoke(context.Background(), call)
	require.Equal(t, ErrorInvalidInput, CodeOf(err))
	require.Equal(t, 0, calls)
}
```

另测：Call AgentRef 与 Bound Agent 不一致、ToolRef 不在 Bound Set、缺失 required
idempotency key。所有用例都要求 Executor Calls=0。

- [ ] **步骤 2：编写执行、超时和 Output 失败测试**

```go
func TestInvokeExecutesToolExactlyOnce(t *testing.T) {
	calls := 0
	bound := boundToolSetForTest(t, &calls, verifiedResolver())
	result, err := bound.Invoke(context.Background(), validCall())
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
}

func TestInvokeMapsDeadlineWithoutRetrying(t *testing.T) {
	calls := 0
	bound := boundToolSetWithExecutor(t, ExecutorFunc(func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		calls++
		<-ctx.Done()
		return nil, ctx.Err()
	}))
	_, err := bound.Invoke(context.Background(), validCall())
	require.Equal(t, ErrorDeadlineExceeded, CodeOf(err))
	require.Equal(t, 1, calls)
}

func TestInvokeRejectsInvalidExecutorOutput(t *testing.T) {
	bound := boundToolSetWithExecutor(t, ExecutorFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return json.RawMessage(`{"unexpected":true}`), nil
	}))
	_, err := bound.Invoke(context.Background(), validCall())
	require.Equal(t, ErrorOutputInvalid, CodeOf(err))
}
```

另测：普通 Executor Error -> `internal`；已分类 `ToolError` 保留原 Code；Executor 返回
后 Context 已超时则丢弃 Output 并返回 `deadline_exceeded`。

- [ ] **步骤 3：编写 Trace、Hash 和 Audit Failure 测试**

使用 Recording Audit Stub 断言：

- Audit 使用可信 Principal tenant/user，而不是 JSON Input；
- InputHash/OutputHash 是小写 SHA-256 Hex；
- Success/Failure 各记录一次，Audit Record 不含 Raw Payload 字段；
- Recorder Error 不重放 Executor，成功 Output 仍返回，状态为 `record_failed`；
- Recorder Context 由 `context.WithoutCancel` 派生，并受 `AuditTimeout` 限制。

使用 OpenTelemetry In-memory Span Recorder 断言 Span 名为 `commerce.tool.invoke`，并包含
`commerce.tool.id`、`commerce.tool.version`、`commerce.agent.id`、
`commerce.agent.run_id`、`commerce.tool.risk`；不得包含 Raw Arguments。

- [ ] **步骤 4：运行测试，确认失败**

运行：`go test ./internal/commercetool -run 'TestInvoke' -count=1 -v`

预期：FAIL，包含 `bound.Invoke undefined` 或 `undefined: Call`。

- [ ] **步骤 5：定义 Call 与 Result**

在 `invocation_contracts.go` 增加：

```go
type CallMetadata struct {
	CallID         string
	AgentID        string
	AgentVersion   string
	AgentRunID     string
	BusinessTaskID string
	TraceID        string
	IdempotencyKey string
}

type Call struct {
	Tool      ToolRef
	Metadata  CallMetadata
	Arguments json.RawMessage
}

type AuditStatus string
const (
	AuditStatusRecorded AuditStatus = "recorded"
	AuditStatusRecordFailed AuditStatus = "record_failed"
)

type Result struct {
	Output      json.RawMessage
	AuditStatus AuditStatus
}
```

Call Preflight 要求 `CallID`、`AgentID`、`AgentVersion`、`AgentRunID`、
`BusinessTaskID` 非空；`AgentID + AgentVersion` 与 Bound Agent 完全一致；TraceID 可为空，
与现有 `aiidentity` 语义一致。

- [ ] **步骤 6：实现固定顺序 Invoke**

```go
func (b *BoundToolSet) Invoke(ctx context.Context, call Call) (Result, error) {
	startedAt := b.deps.Now().UTC()
	state := newInvocationState(startedAt, call)
	registered, err := b.preflight(ctx, call, &state)
	if err != nil {
		return b.finish(ctx, state, nil, err)
	}

	callCtx, cancel := context.WithTimeout(ctx, registered.definition.Timeout.Duration)
	defer cancel()
	callCtx, span := b.startSpan(callCtx, registered.definition, call.Metadata)
	defer span.End()

	output, callErr := registered.executor.Execute(callCtx, cloneRaw(call.Arguments))
	if callCtx.Err() != nil {
		callErr = NewError(ErrorDeadlineExceeded, "tool deadline exceeded", callCtx.Err())
	} else if callErr != nil {
		callErr = normalizeExecutorError(callErr)
	} else if err := registered.schemas.validateOutput(output); err != nil {
		callErr = err
	}
	return b.finish(callCtx, state, output, callErr)
}
```

`preflight` 固定顺序为：Call Metadata -> Bound Tool -> Risk Ceiling -> Principal ->
Permission -> Idempotency -> Input Schema。不得创建 Goroutine 包裹 Executor；Executor
Contract 必须遵守 Context，避免 Deadline 后后台副作用继续运行。

Preflight 映射固定为：缺失 Metadata 返回 `invalid_input`；Agent ID/version 不匹配、Tool
不在 Bound Set、风险超过 `propose` 返回 `tool_not_allowed`；Resolver Error 或缺失
TenantID/UserID/Role 返回 `identity_integrity`；Authorizer 的任意拒绝或内部错误在边界统一
返回安全的 `permission_denied`；缺失 `required_key` 返回 `invalid_input`；Schema 失败由
任务 3 返回 `invalid_input`。除 Schema Error 的安全固定文案外，不得把依赖错误正文写入
返回给模型的 Message。

`finish` 用 SHA-256 计算 Input/Output Hash，用注入的 `Now` 计算终态，通过
`context.WithTimeout(context.WithoutCancel(ctx), AuditTimeout)` 调用 Recorder。Recorder
Error 只改变 AuditStatus，不得覆盖 Tool Result/Error，也不得重试 Executor。

- [ ] **步骤 7：验证并提交**

```powershell
go test ./internal/commercetool -run 'TestInvoke' -count=1 -v
go test -race ./internal/commercetool -run 'TestInvoke' -count=1
go mod tidy
git add go.mod go.sum internal/commercetool/invocation_contracts.go internal/commercetool/invocation.go internal/commercetool/invocation_test.go
git commit -m "feat(commercetool): enforce governed tool invocation"
```

Trace 测试使用 `go.opentelemetry.io/otel/sdk/trace` 与 `sdk/trace/tracetest v1.44.0`，不
自建 Tracer。预期：全部 PASS，Race Detector 无报告。

---

### 任务 6：增加同合同闭环与导入边界护栏

**文件：**

- 新建：`internal/commercetool/conformance_test.go`
- 新建：`internal/commercetool/boundary_guard_test.go`
- 修改：`docs/architecture/project-target-architecture.md`
- 修改：`docs/development/repository-structure.md`

**接口：**

- 验证：Fake Agent 只经 `NewRegistry -> Bind -> Invoke` 完成调用
- 验证：同 ID 不同 Version 不可互换
- 验证：`internal/commercetool` 生产代码保持框架和领域实现中立

- [ ] **步骤 1：编写 Fake Agent 合同闭环测试**

在 `conformance_test.go` 复用前面已经落地的 `validDefinition`、
`validInvocationDependencies` 和 `validCall` fixture：

```go
func TestFakeAgentUsesRegistryBindInvokeContract(t *testing.T) {
	calls := 0
	tool := Tool{
		Definition: validDefinition(),
		Executor: ExecutorFunc(func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
			calls++
			var request struct {
				TaskID string `json:"task_id"`
			}
			require.NoError(t, json.Unmarshal(input, &request))
			return json.Marshal(map[string]string{"task_id": request.TaskID})
		}),
	}

	registry, err := NewRegistry(tool)
	require.NoError(t, err)
	bound, err := registry.Bind(AgentDefinition{
		ID:           "fake.product-agent",
		Version:      "v1.0.0",
		AllowedTools: []ToolRef{tool.Definition.Ref},
	}, validInvocationDependencies())
	require.NoError(t, err)

	result, err := bound.Invoke(context.Background(), validCall())
	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
}

func TestFakeAgentCannotSubstituteAnotherToolVersion(t *testing.T) {
	definition := validDefinition()
	executor := ExecutorFunc(func(_ context.Context, input json.RawMessage) (json.RawMessage, error) {
		return cloneRaw(input), nil
	})
	registry, err := NewRegistry(Tool{Definition: definition, Executor: executor})
	require.NoError(t, err)
	bound, err := registry.Bind(AgentDefinition{
		ID:           "fake.product-agent",
		Version:      "v1.0.0",
		AllowedTools: []ToolRef{definition.Ref},
	}, validInvocationDependencies())
	require.NoError(t, err)

	call := validCall()
	call.Tool.Version = "v1.0.1"
	_, err = bound.Invoke(context.Background(), call)
	require.Equal(t, ErrorToolNotAllowed, CodeOf(err))
}
```

该测试不能建立第二套 Agent/Tool 接口，也不能直接调用 Executor；它证明测试 Agent 与
后续真实 Product Agent 使用的是同一 Registry Contract。

- [ ] **步骤 2：编写可自证的 AST 导入边界测试**

在 `boundary_guard_test.go` 定义生产代码禁止前缀，并让同一个检查器既扫描真实包，也
扫描临时违规 fixture，避免护栏自身失效：

```go
var forbiddenProductionImportPrefixes = []string{
	"task-processor/internal/app",
	"task-processor/internal/infra",
	"task-processor/internal/integration",
	"task-processor/internal/kernel",
	"task-processor/internal/listing",
	"task-processor/internal/listingkit",
	"task-processor/internal/marketplace",
	"task-processor/internal/platform",
	"task-processor/internal/product",
	"task-processor/internal/productenrich",
	"task-processor/internal/productimage",
	"task-processor/internal/publishing",
	"github.com/cloudwego/eino",
	"github.com/gin-gonic/gin",
	"github.com/rabbitmq/amqp091-go",
	"go.temporal.io/sdk",
	"gorm.io/gorm",
}

func TestProductionImportsRemainFrameworkNeutral(t *testing.T) {
	violations, err := findForbiddenProductionImports(".")
	require.NoError(t, err)
	require.Empty(t, violations)
}

func TestImportGuardDetectsDomainLeak(t *testing.T) {
	dir := t.TempDir()
	source := []byte("package commercetool\nimport _ \"task-processor/internal/product\"\n")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "leak.go"), source, 0o600))

	violations, err := findForbiddenProductionImports(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"task-processor/internal/product"}, violations)
}

func findForbiddenProductionImports(root string) ([]string, error) {
	seen := make(map[string]struct{})
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return fmt.Errorf("unquote import in %s: %w", path, err)
			}
			for _, prefix := range forbiddenProductionImportPrefixes {
				if importPath == prefix || strings.HasPrefix(importPath, prefix+"/") {
					seen[importPath] = struct{}{}
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	violations := make([]string, 0, len(seen))
	for importPath := range seen {
		violations = append(violations, importPath)
	}
	sort.Strings(violations)
	return violations, nil
}
```

该检查器只读取非测试 `.go` 文件；解析失败必须返回 Error，不能把坏文件当成无违规。

- [ ] **步骤 3：运行测试，确认失败**

运行：
`go test ./internal/commercetool -run 'Test(FakeAgent|ProductionImports|ImportGuard)' -count=1 -v`

预期：在测试文件尚未完成或生产边界泄漏时 FAIL；不能通过跳过测试或缩小禁止列表修复。

- [ ] **步骤 4：完成边界检查器与架构文档**

实现步骤 2 指定的 `findForbiddenProductionImports`；该 helper 只能位于 `_test.go`，不能
形成第二套生产 API。

在 `docs/architecture/project-target-architecture.md` 的目标边界中加入以下稳定规则：

- `internal/commercetool` 拥有 Tool Definition、Schema、Registry、Agent Allowlist、
  Invocation Policy 和 Tool Audit Port；
- `kernel/module.Registry` 仍只负责启动期贡献收集，不承担 Tool Runtime；
- 领域 Adapter 只通过窄 Service/Query Port 注入 Executor；Composition Root 负责组装；
- `read/compute/propose` 是 Phase 2A 上限，`write/publish` 必须留到后续治理阶段。

在 `docs/development/repository-structure.md` 加入 `internal/commercetool` 条目，明确禁止
Framework、Transport、Workflow、Persistence、Provider SDK、Marketplace Client 和
领域实现包依赖。文档不得把 `product.canonical.inspect` 描述为本切片已交付能力。

- [ ] **步骤 5：验证并提交**

```powershell
gofmt -w internal/commercetool
go test ./internal/commercetool -count=1
go test -race ./internal/commercetool -count=1
go vet ./internal/commercetool
git diff --check
git add internal/commercetool/conformance_test.go internal/commercetool/boundary_guard_test.go docs/architecture/project-target-architecture.md docs/development/repository-structure.md
git commit -m "test(commercetool): guard framework-neutral registry"
```

预期：Focused Test、Race、Vet、Whitespace Check 全部通过；架构文档只陈述切片 A 已有
能力和切片 B 的接入边界。

---

### 任务 7：执行仓库级验证并封存 #133 证据

**文件：**

- 验证：`internal/commercetool/**`
- 验证：`go.mod`
- 验证：`go.sum`
- 验证：`docs/architecture/project-target-architecture.md`
- 验证：`docs/development/repository-structure.md`
- 条件修改：仅当 `go mod tidy` 产生必要差异时修改 `go.mod`、`go.sum`

- [ ] **步骤 1：确认变更范围没有越过切片 A**

```powershell
$commerceBase = git merge-base main HEAD
$commerceRange = "$commerceBase...HEAD"
git rev-parse $commerceBase
git diff $commerceRange --name-only
git diff $commerceRange -- internal/kernel/module
$commerceForbidden = rg -n 'internal/(listingkit|product|productenrich|productimage|marketplace)|cloudwego/eino|gin-gonic/gin|go.temporal.io/sdk|gorm.io/gorm|rabbitmq/amqp091-go' internal/commercetool -g '*.go' -g '!**/*_test.go'
if ($LASTEXITCODE -eq 0) { $commerceForbidden; throw "forbidden commercetool production import" }
if ($LASTEXITCODE -ne 1) { throw "rg import scan failed with exit code $LASTEXITCODE" }
```

预期：文件列表只包含本计划声明的 `internal/commercetool`、两个架构文档和 Go Module
文件；Kernel Diff 为空；`rg` 无匹配。记录 `merge-base` Commit SHA，不能人为缩小 Diff
范围来隐藏本切片改动。

- [ ] **步骤 2：整理并审计依赖**

```powershell
go mod tidy
git diff -- go.mod go.sum
go list -m github.com/santhosh-tekuri/jsonschema/v6 golang.org/x/mod go.opentelemetry.io/otel go.opentelemetry.io/otel/sdk
```

预期：JSON Schema、SemVer 和 OpenTelemetry 复用既有成熟依赖；不得新增 Agent Framework、
RBAC、Workflow、ORM、Retry 或 Feature Flag Library。若 `go mod tidy` 产生依赖差异，只
能来自计划中的生产/测试 Import；先审阅差异再提交。

- [ ] **步骤 3：执行与 CI 对齐的完整验证**

```powershell
go test ./internal/commercetool -count=1
go test -race ./internal/commercetool -count=1
go test ./... -count=1
go vet ./...
golangci-lint run --config .golangci.yml --enable-only depguard ./...
pwsh -File scripts/code-health-audit.ps1 -Mode Verify
git diff --check
```

预期：全部命令 Exit Code 0。若本机缺少 `golangci-lint` 或外部服务使仓库级测试失败，
必须记录精确失败命令和输出并在 CI 补跑；不能把未执行或环境失败写成 PASS。Focused
Package Test、Race、边界测试和 `git diff --check` 仍是本切片不可跳过的门槛。

- [ ] **步骤 4：处理 tidy 差异并记录最终证据**

只有步骤 2 确实产生必要 Module 差异时执行：

```powershell
git add go.mod go.sum
git commit -m "build(commercetool): tidy tool contract dependencies"
```

随后记录：

```powershell
git status --short
git log --oneline --decorate -7
git rev-parse HEAD
```

预期：工作树为空，并保留任务 1 至任务 6 的独立提交证据；没有必要的 Module 差异时
不得创建空提交。

- [ ] **步骤 5：按验收映射报告完成范围**

最终报告必须明确：

- #133 的 Definition 必填校验、精确 Version Allowlist、Schema/Permission/Side-effect
  Declaration、`write/publish` 拒绝、Framework-neutral Import Guard 均有测试证据；
- #134 和 `product.canonical.inspect` 未在本计划交付，不能把 Fake Agent 当作真实领域能力；
- 没有新增 Runtime/Framework、第二套 Compatibility Interface、写工具或发布工具；
- 列出最终 Commit SHA，以及所有通过、未运行或因环境失败的验证命令。

---

## 后续切片 B 计划的启动门槛

只有下列条件全部满足，才为 `product.canonical.inspect` 编写并执行单独计划：

1. 目标目录架构 Phase 2 已合入 `main`，`internal/product` 的最终 Owner 和窄只读 Query
   Port 已稳定，不需要导入根 `internal/listingkit` 或 Legacy DTO；
2. 本计划的 Registry Contract、Fake Agent Conformance 和 Import Guard 已通过；
3. Canonical Product Service 已有 tenant、owner、Tenant Admin、Platform Admin 的访问语义测试；
4. Adapter 只做 Input/Output 映射和错误归一化，不直接访问 Store、GORM 或 Provider SDK；
5. source/fact Tool 等待 #30；AI proposal Tool 等待 #130；readiness/marketplace Tool
   等待 #34 与 Marketplace Ownership 稳定。

切片 B 完成后只形成 #134 的部分证据；必须等最小 Product Agent Tool Set 的全部 Owner
Gate 真实满足，才能关闭 #134 和退出 Phase 2A。
