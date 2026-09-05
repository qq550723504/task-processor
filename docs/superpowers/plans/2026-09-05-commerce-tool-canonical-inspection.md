# B0 Canonical Product Inspection Tool 实施计划

> **前置条件：** 绑定设计
> `docs/superpowers/specs/2026-09-05-commerce-tool-canonical-inspection-design.md`
> 必须先通过独立 Architecture Review 并标记 `IMPLEMENTATION_READY`。设计评审前不得
> 开始下列生产代码任务。

**目标：** 交付 `product.canonical.inspect@v1.0.0`，让同一 Commerce Tool Registry
通过 task-scoped tenant/owner 查询读取权威 `ProductSnapshot`，返回无副作用的严格
Schema 投影和确定性诊断，为 #134 提供首个真实领域纵向切片证据。

**架构：** `internal/listing/task` 定义 Actor、CanonicalSubject 和窄 Reader Port；现有
ListingKit repository 复用现有 task table 实现该 Port；
`internal/product/catalog/tools/canonicalinspect` 实现 Tool Definition、Schema、Executor
和投影；`internal/integration/commercetoolauth` 复用 authidentity 与 Casbin；真实 SQLite
conformance test 通过 Registry 调用完整链路。同一个配置化 `ListingKitAuthorizer` 注入
permission preflight、task scope 和 Executor；8 MiB 物化上限只由 B0 的 bounded
read-only Catalog adapter 执行。没有公网 endpoint、业务写入或 Agent Framework。

**技术栈：** Go、GORM/SQLite/PostgreSQL query behavior、Casbin、OpenTelemetry、
`santhosh-tekuri/jsonschema/v6`、现有 `internal/commercetool`。

---

## 全局约束

- 工作分支：`codex/commerce-tool-canonical-inspection`。
- 基线：`origin/main` commit `9915a0dee2103e2305674d233c4f45248b86dab8`。
- 使用 TDD：每个行为先写失败测试、确认失败原因正确，再写最小实现。
- 每个任务独立提交；不得把 #30、#34、#130 或 Product Agent Runtime 混入 B0。
- 不修改主工作区未提交的 `go.mod/go.sum`；所有工作只在本 worktree 完成。
- 不增加第三方依赖，不运行 `go get`。
- `internal/commercetool` 不导入任何领域或 Integration 包。
- Product Tool 不导入根 `internal/listingkit`、Store、GORM、HTTP、Workflow、Provider SDK、
  Marketplace Client 或 Agent Framework。
- 不增加数据库表、migration、cache、queue、Temporal workflow 或公网 route。
- 所有读失败安全关闭；不允许 missing identity、unsupported version reader 或 repository
  错误退化到 current/global/unscoped fallback。
- 设计/评审 finding 按 AGENTS.md 的五类规则处理；只有明确 Blocker 才重新打开设计。

## 预计文件结构

```text
internal/listing/task/
  canonical_subject.go
  canonical_subject_test.go

internal/listingkit/store/
  task_repo_listing.go                     # 在现有 repository 上实现目标 Port
  canonical_subject_reader_test.go

internal/integration/commercetoolauth/
  principal.go
  principal_test.go
  authorizer.go
  authorizer_test.go

internal/product/catalog/tools/canonicalinspect/
  definition.go
  definition_test.go
  schema.go
  schema_test.go
  projection.go
  projection_test.go
  executor.go
  executor_test.go

tests/
  commerce_tool_canonical_inspection_conformance_test.go

internal/product/catalog/
  publisher_test.go                         # 共享 publication 大快照兼容性

internal/integration/persistence/product/catalog/
  repository.go                             # 共享 repository + B0 bounded reader
  repository_contract_test.go

internal/commercetool/
  invocation_contracts.go                  # canonical Principal hardening
  invocation_test.go

tests/
  commerce_tool_canonical_inspection_boundary_test.go

docs/architecture/project-target-architecture.md
docs/development/repository-structure.md
docs/superpowers/specs/2026-09-05-commerce-tool-canonical-inspection-design.md
docs/superpowers/plans/2026-09-05-commerce-tool-canonical-inspection.md
```

若实现中需要新增文件，必须先证明它不属于上述现有 owner；不得为了缩短文件而新建
第二套 service、repository 或 auth package。

---

### Task 0：完成设计独立评审并冻结基线

**文件：**

- 修改：`docs/superpowers/specs/2026-09-05-commerce-tool-canonical-inspection-design.md`
- 修改：`docs/superpowers/plans/2026-09-05-commerce-tool-canonical-inspection.md`

**Step 1：提交设计与计划**

```powershell
git diff --check
git status --short
git add docs/superpowers/specs/2026-09-05-commerce-tool-canonical-inspection-design.md
git add docs/superpowers/plans/2026-09-05-commerce-tool-canonical-inspection.md
git commit -m "docs(commercetool): design canonical inspection slice"
```

**Step 2：创建 Draft PR 并触发独立 Architecture Review**

PR 正文必须列出：

- Product Decisions、Threat Model、Must/Should/Out of Scope/Accepted Risks；
- task、catalog、tool contract、auth 的所有权；
- tenant/owner/admin 行为；
- BusinessTaskID binding；
- pinned/legacy snapshot 语义；
- 无持久化事务的原因；
- 资源边界与验证证据。

**Step 3：分类评审 finding**

每条使用：

```text
Finding:
Product requirement affected:
Classification: BLOCKER | IMPLEMENTATION_TEST | BACKLOG | ACCEPTED_RISK | NOT_APPLICABLE
Reason:
Action:
```

最多两轮正常 Architecture Review。第二轮之后的非 Blocker P1/P2 转为
`IMPLEMENTATION_TEST` 或 `BACKLOG`。

**Step 4：冻结设计**

没有未处理 Blocker 后，把状态从 `ARCHITECTURE_REVIEW_PENDING` 修改为
`IMPLEMENTATION_READY`，记录 review URL/commit，并提交：

```powershell
git add docs/superpowers/specs/2026-09-05-commerce-tool-canonical-inspection-design.md
git commit -m "docs(commercetool): freeze canonical inspection architecture"
```

---

### Task 1：定义纯 Listing Task Canonical Subject Port

**文件：**

- 新建：`internal/listing/task/canonical_subject.go`
- 新建：`internal/listing/task/canonical_subject_test.go`
- 修改：`internal/listing/task/README.md`

**Step 1：先写合同失败测试**

覆盖：

- 合法 actor/task/subject；
- tenant、user、task ID 为空、空白、首尾空白和超长；
- roles 为空、空白、超过 32 项、单项超过 128 bytes；
- same-tenant owner 可见；
- same-tenant cross-owner 不可见；
- `listingkit_admin` 和 `platform_admin` 可见同 tenant 其他 owner；
- 任意角色不能跨 tenant；
- 返回 Roles/SourceLineage 必须 defensive copy；
- 固定错误 `ErrInvalidActor`、`ErrInvalidTaskID`、
  `ErrCanonicalSubjectNotFound`、`ErrCanonicalSubjectNotReady`。

运行：

```powershell
go test ./internal/listing/task -run 'Test(CanonicalSubject|Actor|CanRead)' -count=1 -v
```

预期：因目标类型/函数不存在而失败。

**Step 2：实现最小纯合同**

要求：

- 只依赖标准库；以 `TenantAdminChecker` 窄 Port 接收权限判定；
- `Actor`、`SourceLineage`、`CanonicalSubject`、`CanonicalSubjectReader`；
- `ValidateActor`、`ValidateTaskID`、`CanReadCanonicalSubject`；
- admin 判定只调用注入的 `TenantAdminChecker`，不依赖全局默认 authorizer；
- 校验拒绝隐式 trim，不在领域边界静默改写 identity。

**Step 3：运行测试和边界扫描**

```powershell
gofmt -w internal/listing/task
go test ./internal/listing/task -count=1
rg -n 'internal/(listingkit|commercetool|integration)|gorm.io|go.temporal.io|gin-gonic|openai' internal/listing/task -g '*.go' -g '!**/*_test.go'
```

最后一个命令预期无输出。

**Step 4：提交**

```powershell
git add internal/listing/task
git commit -m "feat(listing): define canonical task subject port"
```

---

### Task 2：让现有 Task Repository 实现显式 Actor Scope

**文件：**

- 修改：`internal/listingkit/store/task_repo_listing.go`
- 修改：`internal/listingkit/store/task_repo_scope.go`
- 新建：`internal/listingkit/store/canonical_subject_reader_test.go`

**Step 1：先写真实 Repository 失败测试**

使用 SQLite task table，覆盖：

- owner 成功读取 TaskID、TenantID、OwnerUserID、ProductKey、SnapshotVersion、Source；
- cross-owner 返回 `ErrCanonicalSubjectNotFound`；
- cross-tenant 即使 platform admin 也返回 not found；
- listingkit admin/platform admin 只在同 tenant 绕过 owner；
- legacy row 的空 `user_id` 使用 request.user_id；
- task/request 缺 ProductKey 返回 `ErrCanonicalSubjectNotReady`；
- 无效 Actor 在执行 SQL 前失败；
- canceled context 不返回 subject；
- 返回 SourceLineage 不与 GORM-loaded request 共享可变指针；
- 构造器返回值可 type assert 为 `listingtask.CanonicalSubjectReader`。
- 配置化 platform-admin user/role 通过注入的同一 authorizer 获得同 tenant 读取能力；
- 数据库临时故障映射为 `ErrCanonicalSubjectUnavailable`，不吞掉 context 错误。

运行：

```powershell
go test ./internal/listingkit/store -run 'TestTaskRepositoryCanonicalSubject' -count=1 -v
```

预期：repository 尚未实现该 Port。

**Step 2：复用现有 scope helper 实现 Port**

要求：

- 在现有 repository 类型上增加方法，不新建第二个 repository；
- 查询显式包含 actor tenant；
- 非 admin 复用 PostgreSQL/SQLite owner JSON fallback helper；
- 查询后再次调用 `listingtask.CanReadCanonicalSubject`；
- query scope 与查询后校验使用构造时注入的同一个 `TenantAdminChecker`；
- 把 legacy `core.ErrTaskNotFound`/GORM not found 归一化为目标 Port 错误；
- 仅映射必要字段，不返回 `listingkit.Task`。

**Step 3：运行 sibling access tests**

```powershell
gofmt -w internal/listingkit/store
go test ./internal/listingkit/store -run 'TestTaskRepository(OwnerScope|PlatformAdmin|CanonicalSubject)|Test.*Tenant' -count=1
go test ./internal/listingkit/store -count=1
```

**Step 4：提交**

```powershell
git add internal/listingkit/store/task_repo_listing.go
git add internal/listingkit/store/task_repo_scope.go
git add internal/listingkit/store/canonical_subject_reader_test.go
git commit -m "feat(listing): expose scoped canonical subject reader"
```

---

### Task 3：复用可信身份与 Casbin Adapter

**文件：**

- 新建：`internal/integration/commercetoolauth/principal.go`
- 新建：`internal/integration/commercetoolauth/principal_test.go`
- 新建：`internal/integration/commercetoolauth/authorizer.go`
- 新建：`internal/integration/commercetoolauth/authorizer_test.go`
- 修改：`internal/commercetool/invocation_contracts.go`
- 修改：`internal/commercetool/invocation_test.go`

**Step 1：先写失败测试**

覆盖：

- Context Resolver 只接受 `authidentity.AuthenticatedIdentityFromContext`；
- 缺 tenant/user/roles 时返回 identity error；
- 映射值 defensive copy；
- Casbin Adapter 使用注入的 `ListingKitAuthorizer`；
- operator/admin/platform admin 对 `listingkit.admin.read` 的现有 policy；
- viewer/未知角色拒绝；
- nil authorizer 构造失败；
- Commerce Tool Principal 拒绝 tenant/user/role 首尾空白，而不是只拒绝全空白。
- 配置化 platform-admin user/role 同时获得 `listingkit.admin.read`，且其 tenant-admin
  判定来自同一个 `ListingKitAuthorizer` 实例；
- 历史 `admin` platform-admin alias 同样获得 `listingkit.admin.read` 并进入端到端矩阵；
- 原始 Call.Arguments 在任何 clone/decode/PrincipalResolver 前执行 64 KiB exact/over-limit
  检查；over-limit 返回 `invalid_input`，不保留或哈希完整载荷，只记录固定 audit marker
  hash，并断言 Resolver/Authorizer/Executor 均未调用。

运行：

```powershell
go test ./internal/integration/commercetoolauth ./internal/commercetool -run 'Test(ContextPrincipal|Casbin|Invoke.*Principal)' -count=1 -v
```

预期：Adapter 尚不存在，且 canonical Principal case 失败。

**Step 2：实现最小 Adapter 和根因修复**

- Resolver 不读取 header、query、Tool JSON 或未验证的 legacy identity context；
- Authorizer 不复制 policy，只委托现有 Casbin owner；
- Commerce Tool Principal validation 拒绝非 canonical whitespace；
- 在 `BoundToolSet.Invoke` 的最外层、`newInvocationState` 之前检查原始参数长度；不得先
  复制超限字节再返回错误；
- 超限审计 state 清空 Arguments 并使用固定 oversized marker hash，不能在 `finish`
  中重新线性扫描攻击载荷；
- 不新增 permission；B0 使用 `authz.PermissionListingKitAdminRead` 的字符串值。

**Step 3：验证**

```powershell
gofmt -w internal/integration/commercetoolauth internal/commercetool
go test ./internal/integration/commercetoolauth ./internal/commercetool -count=1
go test -race ./internal/commercetool -count=1
```

**Step 4：提交**

```powershell
git add internal/integration/commercetoolauth
git add internal/commercetool/invocation_contracts.go internal/commercetool/invocation_test.go
git commit -m "feat(commercetool): adapt trusted listing identity"
```

---

### Task 4：定义 Tool、严格 Schema、安全投影与 B0 Catalog 物化上限

**文件：**

- 新建：`internal/product/catalog/tools/canonicalinspect/definition.go`
- 新建：`internal/product/catalog/tools/canonicalinspect/definition_test.go`
- 新建：`internal/product/catalog/tools/canonicalinspect/schema.go`
- 新建：`internal/product/catalog/tools/canonicalinspect/schema_test.go`
- 新建：`internal/product/catalog/tools/canonicalinspect/projection.go`
- 新建：`internal/product/catalog/tools/canonicalinspect/projection_test.go`
- 修改：`internal/product/catalog/publisher_test.go`
- 修改：`internal/integration/persistence/product/catalog/repository.go`
- 修改：`internal/integration/persistence/product/catalog/repository_contract_test.go`

**Step 1：先写 Definition/Schema 失败测试**

断言完整元数据：

```text
product.canonical.inspect / v1.0.0 / product.canonical / product.catalog
read / none / deterministic / caller / unmetered
listingkit.admin.read / 3s
```

Schema tests 覆盖：

- input 只接受 required `task_id`；
- additional property、authority field、空值、超长值失败；
- output fixture 覆盖 snapshot 所有现有 JSON 字段；
- output additional property 和缺 required field 失败；
- Go JSON tag 与 Schema property parity；
- Catalog 新增字段但 Schema 未更新时测试失败。

运行：

```powershell
go test ./internal/product/catalog/tools/canonicalinspect -run 'Test(Definition|InputSchema|OutputSchema|SchemaParity)' -count=1 -v
```

预期：包尚不存在。

**Step 2：先写 Projection 失败测试**

覆盖：

- 深复制 snapshot slices/maps/pointers；
- task source lineage 单独返回；
- diagnostics 精确投影 `Review.NeedsReview`、`Review.Reasons` 和 `Warnings`；
- 不新增 SHEIN/TEMU/Amazon readiness；
- 所有嵌套 `SourceRecord.Metadata` 被清理；
- 输入 snapshot 不被修改；
- 1 MiB exact boundary 成功，超过 1 byte 失败且不截断；
- 输出 JSON 不包含 Commerce Tool reserved authority field。

**Step 3：先写 B0 encoded payload 边界与共享兼容失败测试**

覆盖：

- 共享 Catalog repository 可继续发布和读取超过 8 MiB 的合法 snapshot；
- B0 bounded reader 读取 exact-limit persisted JSON 可进入 decode path；
- B0 bounded reader 使用数据库侧 byte-length 条件投影；over-limit persisted JSON 不被
  扫描进 Go，并在 hash、unmarshal 和 `CloneProductSnapshot` 前失败；
- bounded guard 同时覆盖 current 和 versioned read；
- bounded reader 的动态类型不实现 `SnapshotWriter`；
- SQLite SQL capture 断言存在 `CASE WHEN LENGTH(snapshot_json) <= ?`；PostgreSQL 使用
  `OCTET_LENGTH(snapshot_json::text)`；
- 不把 B0 reader 的 8 MiB 上限误用为共享 Catalog 上限或 Tool 1 MiB 输出上限。

运行：

```powershell
go test ./internal/product/catalog ./internal/integration/persistence/product/catalog -run 'Test.*Snapshot.*Size|Test.*Encoded.*Limit' -count=1 -v
```

预期：当前 publication/read 尚无 encoded payload 上限。

**Step 4：实现 Definition、Schema、Projection 与 B0 bounded reader**

- 复用 `catalog.CloneProductSnapshot`；
- 类型只用于 wire projection，不提供 persistence/write conversion；
- Output 中不包含 TenantID/OwnerUserID；
- Schema 使用现有 Registry compiler，不引入 schema generator 依赖。
- `canonicalinspect.MaxCatalogSnapshotBytes` 固定为 8 MiB；只注入
  `catalogpersistence.NewBoundedSnapshotReader`，不得修改共享 Catalog publication/read
  的兼容范围；
- Tool output 仍固定 1 MiB，不能因 Catalog 上限较大而放宽。

**Step 5：验证并提交**

```powershell
gofmt -w internal/product/catalog/tools/canonicalinspect
go test ./internal/product/catalog/tools/canonicalinspect -run 'Test(Definition|InputSchema|OutputSchema|SchemaParity|Projection)' -count=1
go test ./internal/product/catalog ./internal/integration/persistence/product/catalog -run 'Test.*Snapshot.*Size|Test.*Encoded.*Limit' -count=1
git add internal/product/catalog/tools/canonicalinspect
git add internal/product/catalog/publisher_test.go internal/integration/persistence/product/catalog/repository.go
git add internal/integration/persistence/product/catalog/repository_contract_test.go
git commit -m "feat(product): define canonical inspection tool contract"
```

---

### Task 5：实现单次、无 fallback 的 Executor

**文件：**

- 新建：`internal/product/catalog/tools/canonicalinspect/executor.go`
- 新建：`internal/product/catalog/tools/canonicalinspect/executor_test.go`

**Step 1：先写执行矩阵失败测试**

覆盖：

- 正常 owner 调用；
- `input.task_id != BusinessTaskID` 返回 `invalid_input` 且两个 Reader 均未调用；
- Principal 映射为 Actor，不从 input 读 authority；
- subject task/tenant/owner mismatch 安全映射 `not_found`；
- subject 缺 ProductKey 映射 `failed_precondition`；
- version > 0 只调用 `GetSnapshot`，绝不 fallback current；
- version == 0 只调用 `GetCurrentSnapshot`；
- Snapshot not ready 映射 `failed_precondition`；
- repository unavailable 映射 `dependency_unavailable`；
- state invalid/unknown error 不泄露 cause，映射 `internal`；
- context canceled 后不调用下一个 Reader；
- 3 秒 Definition timeout 只描述 Executor；input/output schema 分别由 64 KiB/1 MiB
  上限约束，audit 使用独立 AuditTimeout；
- 每个 Reader 最多一次调用；
- projection over limit 返回 `failed_precondition`；
- 成功结果没有 AIInvocationID。

运行：

```powershell
go test ./internal/product/catalog/tools/canonicalinspect -run 'TestExecutor' -count=1 -v
```

预期：Executor 尚不存在。

**Step 2：实现最小 Executor**

构造器必须要求：

- 非 nil `listingtask.CanonicalSubjectReader`；
- 同时实现 current/versioned 的 Catalog Reader；
- 与 task repository 相同的非 nil `listingtask.TenantAdminChecker`；
- 不接受 broad `catalog.Repository` 作为写能力依赖。

执行严格按设计顺序，不自行重试、不调用模型、不修改状态。

**Step 3：验证并提交**

```powershell
gofmt -w internal/product/catalog/tools/canonicalinspect
go test ./internal/product/catalog/tools/canonicalinspect -count=1
go test -race ./internal/product/catalog/tools/canonicalinspect -count=1
git add internal/product/catalog/tools/canonicalinspect/executor.go
git add internal/product/catalog/tools/canonicalinspect/executor_test.go
git commit -m "feat(product): execute canonical inspection reads"
```

---

### Task 6：通过真实 Registry 和 Repository 完成纵向闭环

**文件：**

- 新建：`tests/commerce_tool_canonical_inspection_conformance_test.go`

真实 integration adapters 的装配测试归仓库级 `tests` owner；现有 Phase 2 架构护栏禁止
领域目录（包括 `_test.go`）导入 concrete infrastructure。Tool 包内只保留纯合同与 fake
port 测试。

**Step 1：先搭建真实依赖 fixture**

使用：

- SQLite in-memory GORM；
- `listingkit.Task` 现有 schema 与注入配置 authorizer 的 task repository；
- `catalogpersistence.AutoMigrate/NewRepository/NewBoundedSnapshotReader`；
- `catalog.Publisher` 发布不可变 snapshot；
- `commercetoolauth.ContextPrincipalResolver`；
- `commercetoolauth.CasbinAuthorizer`；
- OpenTelemetry no-op tracer；
- test-only AuditRecorder；
- `commercetool.NewRegistry(...).Bind(...)`。

**Step 2：先写完整失败测试**

覆盖：

- Fake Agent 只通过 Registry 调用 B0 并获得真实 snapshot；
- owner 成功；
- same-tenant cross-owner `not_found`；
- tenant admin/platform admin same-tenant 成功；
- 配置化 platform-admin user/role same-tenant 成功并可通过 permission preflight；
- 历史 `admin` alias same-tenant 成功；
- platform admin cross-tenant `not_found`；
- viewer 在 repository 前被 `permission_denied`；
- task 没有 snapshot 时 `failed_precondition`；
- pinned task 在发布新版本后仍读取旧版本；
- legacy task 在发布新版本后读取 current 并返回实际 version；
- input/output schema、trace 和 audit 正常；
- 调用前后 task count、snapshot head/version count、task status/result 完全不变。

运行：

```powershell
go test ./tests -run 'TestRegistryConformanceCanonicalInspectionVerticalSlice' -count=1 -v
```

预期：在完整装配或行为未完成时失败。

**Step 3：完成最小测试装配并验证**

不新增生产 endpoint 或全局 registry。修复只能落在 Task 1-5 的 owner 中。

```powershell
go test ./tests -run 'TestRegistryConformanceCanonicalInspectionVerticalSlice|TestTargetDomainsDoNotImportConcreteInfrastructure' -count=1
go test ./internal/product/catalog/tools/canonicalinspect ./internal/listing/task ./internal/listingkit/store ./internal/integration/commercetoolauth ./internal/commercetool -count=1
```

**Step 4：提交**

```powershell
git add tests/commerce_tool_canonical_inspection_conformance_test.go
git commit -m "test(commercetool): prove canonical inspection vertical slice"
```

---

### Task 7：增加架构护栏并同步权威文档

**文件：**

- 新建：`tests/commerce_tool_canonical_inspection_boundary_test.go`
- 修改：`.golangci.yml`
- 修改：`docs/architecture/project-target-architecture.md`
- 修改：`docs/development/repository-structure.md`

**Step 1：先写失败的 Import Guard**

Guard 必须检查：

- canonicalinspect 禁止根 ListingKit、Store、GORM、HTTP、Workflow、Provider、Marketplace、
  Agent Framework；
- listing/task 禁止 commercetool、根 ListingKit、GORM、Provider、Agent Framework；
- commercetool 继续保持领域中立；
- Tool package 不存在 write/publish/queue/task-dispatch 语义；
- B0 生产代码不新增 HTTP route 或 migration；
- 不允许第二个 canonical Product type 或 task repository。

运行：

```powershell
go test ./tests -run 'TestCommerceToolCanonicalInspection' -count=1 -v
```

预期：缺 guard/config/docs 时失败。

**Step 2：增加窄 depguard 和文档**

`.golangci.yml` 使用严格 allowlist，只覆盖 B0 目标包；不得放宽现有 Phase 3 Product 或
Commerce Tool guard。

文档写明：

- canonical facts 属于 product/catalog；
- task-scoped resource resolution 属于 listing/task；
- Tool Adapter 仅做只读映射和错误归一化；
- Phase 2B 前没有用户可访问 runtime。

**Step 3：验证并提交**

```powershell
gofmt -w tests
go test ./tests -run 'TestCommerceToolCanonicalInspection|TestPhase3Product|Test.*TargetArchitecture' -count=1
git diff --check
git add .golangci.yml tests/commerce_tool_canonical_inspection_boundary_test.go
git add docs/architecture/project-target-architecture.md docs/development/repository-structure.md
git commit -m "test(architecture): guard canonical inspection boundaries"
```

---

### Task 8：最终验证、#134 证据与 PR 评审

**Step 1：运行聚焦验证**

```powershell
go test ./internal/commercetool -count=1
go test -race ./internal/commercetool -count=1
go test ./internal/listing/task -count=1
go test ./internal/listingkit/store -count=1
go test ./internal/integration/commercetoolauth -count=1
go test -race ./internal/product/catalog/tools/canonicalinspect -count=1
go test ./internal/product/catalog ./internal/integration/persistence/product/catalog -count=1
go test ./tests -run 'TestCommerceToolCanonicalInspection|TestPhase3Product' -count=1
go vet ./internal/commercetool ./internal/listing/task ./internal/integration/commercetoolauth ./internal/product/catalog/tools/canonicalinspect
git diff --check origin/main...HEAD
```

**Step 2：运行仓库级验证**

```powershell
go test -run '^$' -count=0 ./...
go test ./... -count=1
golangci-lint run
```

如果全仓命令因环境或已知无关问题失败，记录第一条真实错误、exact commit 和聚焦测试
结果；不得把未运行或超时写成通过。

**Step 3：检查范围**

```powershell
git status --short
git diff --stat origin/main...HEAD
git log --oneline origin/main..HEAD
```

确认：

- 无 go.mod/go.sum 变化；
- 无公网 endpoint、Agent Runtime、write/publish Tool；
- 无新增 schema/migration；
- 无 #30/#34/#130 的伪实现；
- #134 只记录部分证据，不关闭。

**Step 4：更新 #134 跟踪说明**

在 PR 准备完成后给 #134 添加证据评论，并明确：

- canonical inspection 已完成的 exact commit/测试；
- #30、#34、#130 仍是剩余 gate；
- 把 #130 补入依赖说明；
- #134 保持 open。

**Step 5：请求 Code Review 并分类处理**

对每条 finding 先检查 sibling paths，再按 AGENTS.md 分类。非 Blocker 不重新打开已冻结
设计；通过实现测试或 backlog 处理。

**Step 6：最终提交状态**

```powershell
git status --short
git log -1 --format='%H %s'
```

PR 交付说明必须包含：

- exact commit；
- 所有通过、失败、超时和未运行命令；
- Architecture Review 结论；
- Code Review finding 分类；
- #134 仍未完成的 gates。

---

## 验收清单

- [ ] 设计已达到 `IMPLEMENTATION_READY`，无未处理 Blocker。
- [ ] `product.canonical.inspect@v1.0.0` 使用现有 Registry Contract。
- [ ] input 只有 `task_id`，并与 BusinessTaskID 强绑定。
- [ ] task query 明确执行 tenant/owner scope，管理员不能跨 tenant。
- [ ] Tool 不导入根 ListingKit、GORM、Provider、Marketplace 或 Agent Framework。
- [ ] current/versioned snapshot 行为明确且无隐式 fallback。
- [ ] 输出是深复制只读投影，严格 Schema、无保留 authority 字段、最多 1 MiB。
- [ ] diagnostics 只投影 Catalog Review/Warnings，不猜 marketplace readiness。
- [ ] timeout/cancellation 传播，Executor 不重试。
- [ ] raw Arguments 在 clone/decode 前受 64 KiB 上限保护，超限审计不保留/哈希完整载荷。
- [ ] B0 bounded Catalog reader 在 hash/unmarshal/clone 前受 8 MiB 上限保护，且共享
  Catalog publication/read 仍兼容大快照。
- [ ] permission preflight、task scope 与 Executor 共享配置化 admin 语义。
- [ ] 内置、配置化和历史 alias 平台管理员均通过 permission matrix，且都不能跨 tenant。
- [ ] task repository 不可用稳定映射为 `dependency_unavailable`。
- [ ] 3 秒合同准确描述 Executor，不误称整个 Invoke wall-clock timeout。
- [ ] 真实 SQLite + Casbin + Registry conformance tests 通过。
- [ ] 调用前后业务数据不变。
- [ ] 聚焦、race、vet、depguard/code-health 与可运行的全仓验证有 exact evidence。
- [ ] #134 只记录部分完成并继续保持 open；#30/#34/#130 未被越权实现。
