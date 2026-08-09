# ProductImage 场景生成治理实施计划

> 状态：治理入口与启用前身份上下文接入已实现，待独立发布与启用决策。

## 目标

在现有 Go 模块化单体中新增一个默认关闭的 `productimage.scene_generation` 能力入口：直接使用 provider-neutral 路由、租户策略和 `ai_invocations` 调用账本，支持 route-aware 的图片模型调用。当前未使用的旧 ProductImage 场景路径保持不变，不做 shadow、双写或旧路径兼容迁移。

## 非目标

- 不引入 LangGraph、Eino 或其他 Agent 框架。
- 不改造旧 `SceneGenerator` 调用链，不迁移主体提取、白底渲染、审核模型或远端旧 scene endpoint。
- 不新增数据库表；复用已存在的 `ai_invocations`。
- 不在自动化测试中调用真实或付费模型。
- 不在本计划中启用生产配置或执行部署。

## 设计约束

- 新能力默认 `disabled`，未显式开启时启动和现有请求行为不变。
- 开启后只允许 `active` 路由；缺少租户/用户身份、路由决定或 route-aware provider 时 fail closed。
- 新入口不接受 API key、原始 prompt、原始响应或图片字节作为账本字段。
- 不做 provider fallback；路由决定与实际发送的 `ModelID/RoutingKey` 必须一致。
- `productimage` 领域包只依赖 provider-neutral 合同；provider resolver 和具体客户端只在 HTTP/bootstrap 适配层出现。

## 实施任务

### Task 1：扩展 provider-neutral 能力合同

**文件：**

- 修改 `internal/aicapability/model.go`
- 修改 `internal/aicapability/routing.go`（仅在需要时）
- 修改/新增 `internal/aicapability/*_test.go`

**内容：**

- 新增 `CapabilityProductImageScene`，值为 `productimage.scene_generation`。
- 新增专用场景生成 operation，避免复用 Studio 图片 operation 的策略语义。
- 复用现有 `ModelFeature`，场景生成至少要求图片编辑/生成能力。
- 保持 `RouteRequest`、`RouteDecision` 和 `InvocationRecord` provider-neutral。

**验收：**

```powershell
go test ./internal/aicapability -count=1
```

### Task 2：新增显式开关与配置校验

**文件：**

- 修改 `internal/core/config/type_ai_capability.go`
- 修改 `internal/core/config/config.go`
- 修改 `internal/core/config/loader.go`
- 修改 `internal/core/config/loader_builder.go`
- 修改 `internal/core/config/defaults.go`
- 修改 `internal/core/config/validator_ai_capability.go`
- 新增/修改配置测试

**内容：**

- 增加 `ProductImageSceneEnabled bool`，默认 `false`。
- 绑定环境变量 `TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ENABLED`。
- 开启时要求治理依赖能够装配；关闭时不得创建新 recorder/router/provider。
- 不增加 legacy/shadow 模式，避免为未使用能力引入迁移兼容状态机。

**验收：**

- 默认关闭测试通过。
- 环境变量只接受明确布尔值。
- `go test ./internal/core/config -run 'Test.*ProductImageScene|Test.*AICapability' -count=1`

### Task 3：建立 ProductImage 场景模型目录与策略适配器

**文件：**

- 新增 `internal/productimage/httpapi/ai_capability_scene_catalog.go`
- 新增对应测试
- 如需暴露现有 resolver，最小修改 `internal/infra/clients/openai/client_manager.go` 并增加测试

**内容：**

- 复用现有 `ClientConfigResolver`，只返回 provider-neutral `ModelDefinition`。
- 将现有 `image` client 的配置模型映射为一个明确 routing key、model ID、credential reference 和 configuration version。
- 缺少凭据、BaseURL 或 Model 时返回 `ErrorCredentialUnavailable`。
- 测试确保 API key 不出现在 `ModelDefinition`、路由决定或日志字段中。

**验收：**

```powershell
go test ./internal/productimage/httpapi -run 'Test.*Capability.*Scene|Test.*Scene.*Catalog' -count=1
```

### Task 4：给新场景 provider 增加 route-aware 入口

**文件：**

- 修改 `internal/productimage/openai_scene_generator.go`
- 修改 `internal/productimage/openai_image_edit_adapter.go`（如需传递 model ID）
- 修改 `internal/productimage/interfaces.go` 或新增 provider-neutral route-aware 接口
- 新增 fake/provider 合同测试

**内容：**

- 保留现有 `GenerateScene` 供旧路径使用。
- 新增明确的 route-aware 方法，接收 `ModelID/RoutingKey` 后将实际请求模型设置为路由决定的 model ID。
- 不把 `aicapability.RouteDecision` 或 provider DTO 传入领域 provider；使用最小字符串/值对象参数。
- 测试断言实际 HTTP 请求中的 model 与路由决定一致。

**验收：**

```powershell
go test ./internal/productimage -run 'Test.*Scene.*Route|Test.*OpenAI.*Scene' -count=1
```

### Task 5：实现 clean-slate governed scene generator

**文件：**

- 新增 `internal/productimage/governed_scene_generator.go`
- 新增 `internal/productimage/governed_scene_generator_test.go`

**内容：**

- 新入口接收 router、invocation recorder、route-aware provider 和 identity resolver。
- 调用前校验 `tenant_id/user_id`；缺失时返回 `ErrorInvalidInput`，不调用模型。
- 调用 router 得到决定后，只调用一次 route-aware provider。
- 成功/失败均 best-effort 写入 `ai_invocations`；账本失败只告警，不重复模型调用。
- 记录 prompt hash、输入/输出 hash、provider/model/routing key、配置版本、耗时和错误分类，不记录原文/图片字节/API key。
- 不提供旧 provider fallback，不提供 shadow 双调用。

**验收：**

- 缺身份、路由失败、provider 失败、账本失败均有独立测试。
- 成功路径严格验证调用次数为 1。
- `go test ./internal/productimage -run 'Test.*Governed.*Scene' -count=1`

### Task 6：在 ProductImage HTTP/bootstrap 层按开关装配

**文件：**

- 修改 `internal/productimage/httpapi/bootstrap.go`
- 修改 `internal/productimage/httpapi/model_provider_builder.go` 或新增专用 builder
- 修改运行时依赖传递结构（仅涉及 ProductImage 能力）
- 新增 bootstrap 边界测试

**内容：**

- `ProductImageSceneEnabled=false` 时保持当前 model provider 和 scene renderer 装配结果不变。
- 开启时，要求 resolver、recorder、route-aware scene provider 和 identity resolver 全部存在，否则启动失败。
- 新 governed generator 只作为显式启用能力的入口，不覆盖旧未使用路径。
- 维持 `internal/aicapability` 不依赖业务包或 provider 包的边界测试。

**验收：**

```powershell
go test ./internal/productimage/httpapi -run 'Test.*ProductImageScene|Test.*Bootstrap.*Scene' -count=1
go test ./tests -run 'TestAICapabilityModuleDoesNotImportBusinessOrProviderPackages' -count=1
```

### Task 7：文档、配置示例与全量验证

**文件：**

- 修改 ProductImage 配置/README 示例
- 修改 `docs/architecture/project-boundaries.md`（如需）
- 修改 `docs/superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md` 的实施状态

**内容：**

- 明确新能力默认关闭、启用前置条件、回滚方式和“不使用 LangGraph”的当前决策。
- 不写入真实凭据和真实 provider URL。

**验证顺序：**

```powershell
go test ./internal/aicapability ./internal/core/config ./internal/productimage ./internal/productimage/httpapi -count=1
go test ./tests -count=1
git diff --check
```

完成上述验证后，才进入独立的发布/启用决策；本计划不自动切生产开关。

### Task 8：接通启用前身份上下文

**文件：**

- 新增 `internal/shared/aiidentity/context.go`
- 修改 `internal/infra/clients/openai/identity.go`、`internal/listingkit/authenticated_identity.go`
- 修改 ProductImage task/service/pipeline/httpapi 装配

**内容：**

- 将已验证的租户/用户身份写入 provider-neutral shared context；OpenAI identity API 保持兼容代理。
- ProductImage 创建任务时固化 `tenant_id/user_id`，启用治理时缺失身份直接拒绝。
- worker/inline 执行从任务恢复 AI identity context，治理 generator 不读取未验证请求头。
- 任务字段由现有 ProductImage AutoMigrate/schema wiring 管理；未新增表，不做破坏性迁移。
- 恢复独立 `product-listing-api-schema-migrate` 命令和 Kubernetes Job，启用前先迁移 Product Listing API/ProductImage schema，再迁移 ListingKit schema。

**验收：**

- HTTP authenticated identity → task creation context
- task persistence → worker context restoration
- missing identity rejection when governance is enabled
- `go test ./tests -count=1`

## 回滚

- 运行时关闭 `TASK_PROCESSOR_AI_CAPABILITY_PRODUCT_IMAGE_SCENE_ENABLED` 即可恢复默认行为。
- 不删除旧场景 provider 和旧配置。
- 不执行破坏性 schema 操作。
