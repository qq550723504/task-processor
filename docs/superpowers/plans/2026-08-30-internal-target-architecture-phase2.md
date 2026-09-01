# Internal 目标架构迁移阶段 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 建立可强制执行的 shared、platform、integration 和 app 运行时边界，迁移明确的基础设施能力，并用 Goose、OpenTelemetry、OpenFeature 三条真实纵向链路证明目标架构可运行。

**Architecture:** 领域继续声明局部端口，app 负责把 platform 与 integration 实现装配进去。阶段 2 不把混合的 `core/config` 整包改名，而是先抽出无业务语义的配置加载机制；数据库、Redis、RabbitMQ、Temporal 连接、日志和 worker pool 进入 platform，已由稳定端口隔离的服务商客户端与 S3 进入 integration。仍与业务模型和编排耦合的 crawler/持久化实现先冻结增长，待所属领域阶段以纵向切片迁移。旧实现只有在生产和测试引用都归零后才删除；为避免业务包反向依赖 platform 而保留的纯转发兼容面必须有不可增长基线和明确退出阶段。

**Tech Stack:** Go 1.26、GORM 1.31.1、Goose 3.27.3、OpenFeature Go SDK 1.18.0、OpenTelemetry Go 1.44.0、OpenTelemetry contrib 0.66.0、Redis 9.18.0、RabbitMQ amqp091-go 1.10.0、Temporal SDK 1.43.0、AWS SDK v2、golangci-lint depguard、Go `testing`、PowerShell、Git

**Spec:** `docs/superpowers/specs/2026-08-30-internal-target-architecture-migration-design.md`

## Global Constraints

- 开始阶段 2 前，阶段 1 的 `go test ./tests -count=1`、`go test ./... -count=1` 和源码目录产物护栏必须通过，且阶段 1 文档提交后工作区必须干净。
- 最终规则是领域包不得导入 `internal/platform`、`internal/integration` 或服务商 SDK；platform 不得导入领域包；integration 只能导入领域的聚焦 `port` 或 `contract` 包。阶段 2 立即对九个目标领域根强制执行；历史业务根中随实现迁移而暴露的旧依赖必须进入 inventory，禁止新增调用方，并在各自领域阶段通过局部端口消除，不能被误报为最终架构已达成。
- `internal/core/config` 当前混合加载机制与业务配置；本阶段只抽取通用机制，禁止把整个包原样搬进 platform。
- 只迁移已经由稳定局部端口隔离的 adapter。现有 `internal/crawler` 和混合在领域包中的持久化实现不得为了目录整齐而平移；本阶段记录基线并禁止增长，分别在 product/marketplace 和 organization 等所属领域阶段抽取端口后迁移。
- app 是连接构造、provider 注册、迁移执行、HTTP instrumentation 和资源关闭的唯一所有者。
- Goose 固定为 `github.com/pressly/goose/v3 v3.27.3`；OpenFeature 固定为 `github.com/open-feature/go-sdk v1.18.0`。
- OpenTelemetry 稳定模块固定为 `v1.44.0`，contrib instrumentation 固定为 `v0.66.0`，不得混用不协调的 stable/contrib 版本。
- Goose 的首个 Go migration 是现有数据库的幂等基线迁移；它必须消除 API 启动过程中分散的 `AutoMigrate`，此后的 schema 变化必须新增版本，不能修改已发布版本。
- OpenFeature 使用官方 isolated API 与官方 in-memory provider；领域和 app 代码只消费本地 `BoolEvaluator`，不得暴露 OpenFeature SDK 类型。
- OpenTelemetry 禁用时使用 no-op provider，不发起网络连接；启用时由 app 创建并在最后关闭。
- 本阶段不引入 MCP、pgvector 或 TigerBeetle；Promptfoo 和前端 P0 不进入本计划。
- 不改变 HTTP、RabbitMQ、Temporal、数据库表和对象 key 的外部契约；只改变内部所有权和构造位置。
- 每个任务遵循 RED → GREEN → 聚焦回归 → `git diff --check` → 独立提交。

## Target File Structure

| Target | Responsibility |
| --- | --- |
| `internal/shared/{hashx,mathx,ptr,strx,timex}` | 无业务语义的稳定原语 |
| `internal/platform/config` | 文件/内存配置源、编解码、路径和通用 loader 原语 |
| `internal/app/lifecycle` | app 组件启动、依赖排序和逆序关闭 |
| `internal/platform/logging` | logrus 管理、格式、轮转和全局日志兼容入口 |
| `internal/platform/database` | PostgreSQL/GORM 连接、共享连接引用计数和连接关闭 |
| `internal/platform/database/migration` | 通用 Goose provider 与版本执行 |
| `internal/platform/featureflag` | OpenFeature isolated runtime 与 bool evaluation adapter |
| `internal/platform/redis` | Redis client 与 Redis lock runtime |
| `internal/platform/queue/rabbitmq` | RabbitMQ 连接、队列、consumer、重试和监控 |
| `internal/platform/workerpool` | 通用 worker pool runtime |
| `internal/shared/resilience` | 无业务状态的 retry、rate limit 和 circuit breaker 原语 |
| `internal/platform/temporal` | Temporal SDK client 构造与关闭 |
| `internal/platform/observability` | OTel trace runtime 与 HTTP handler instrumentation |
| `internal/app/monitoring` | 实现 app lifecycle 的进程指标与健康检查组件 |
| `internal/app/consumer/metrics` | 依赖任务、SHEIN 和 RabbitMQ 快照的 consumer 专用 Prometheus registry |
| `internal/integration/{openai,geminiimage,grsai,s3,httpimage}` | 服务商、对象存储和远端图片适配器 |
| `internal/app/configadapter` | 从迁移期 `core/config` 聚合模型翻译到 platform config |

---

## Track A: Baseline and ownership guards

### Task 1: Record the convergence baseline and enforce target import direction

**Files:**
- Create: `docs/refactoring/phase2-runtime-inventory.md`
- Create: `tests/target_architecture_phase2_test.go`
- Modify: `.golangci.yml`
- Modify: `tests/depguard_config_test.go`
- Modify: `internal/platform/README.md`
- Modify: `internal/integration/README.md`
- Modify: `internal/product/product_fetcher.go`, `product_fetcher_test.go`
- Modify: `internal/listing/submission/enqueue_retry.go`, `enqueue_retry_test.go`
- Modify: `internal/infra/worker/pool.go`, `worker_test.go`

**Interfaces:**
- Consumes: `loadGoFileIndex(root, skipRoot)` and `importMatchesPrefix` from `tests/import_scan.go`.
- Produces: `TestPhase2LegacyRootsDoNotGrow` with baselines `core production files <= 58`, `infra production files <= 68`, `crawler production files <= 134`, `core internal importer packages <= 145`, `infra internal importer packages <= 75`, and `core/logger direct importer packages <= 92`.
- Produces: target-direction tests covering `listing`, `product`, `marketplace`, `agent`, `knowledge`, `resourcecatalog`, `commercial`, `ledger`, and `organization`.

- [ ] **Step 1: Write the failing ownership-document test**

Add to `tests/target_architecture_phase2_test.go`:

```go
package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPhase2TargetReadmesMatchApprovedOwnership(t *testing.T) {
	platform := readRepositoryText(t, filepath.Join("..", "internal", "platform", "README.md"))
	for _, forbidden := range []string{"authz", "objectstore"} {
		if strings.Contains(platform, forbidden) {
			t.Errorf("platform README still claims %s ownership", forbidden)
		}
	}
	integration := readRepositoryText(t, filepath.Join("..", "internal", "integration", "README.md"))
	for _, required := range []string{"S3", "ZITADEL", "Casbin", "persistence adapters"} {
		if !strings.Contains(integration, required) {
			t.Errorf("integration README must name %s", required)
		}
	}
}

func productionGoFileCount(t *testing.T, root string) int {
	t.Helper()
	count := 0
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil { return err }
		if !entry.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") { count++ }
		return nil
	})
	if err != nil { t.Fatal(err) }
	return count
}

func internalImporterPackageCount(t *testing.T, target string) int {
	t.Helper()
	cmd := exec.Command("go", "list", "-f", `{{.ImportPath}}|{{join .Imports ","}}`, "./internal/...")
	cmd.Dir = ".."
	out, err := cmd.Output()
	if err != nil { t.Fatal(err) }
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		_, imports, ok := strings.Cut(strings.TrimSpace(line), "|")
		if !ok { continue }
		for _, imp := range strings.Split(imports, ",") {
			if importMatchesPrefix(imp, target) { count++; break }
		}
	}
	return count
}
```

- [ ] **Step 2: Run the document test and verify RED**

```powershell
go test ./tests -run TestPhase2TargetReadmesMatchApprovedOwnership -count=1 -v
```

Expected: FAIL because platform still claims `authz` and `objectstore`, and integration omits ZITADEL, Casbin, and persistence adapters.

- [ ] **Step 3: Add monotonic baseline and import-direction tests**

In the same file, add:

```go
func TestPhase2LegacyRootsDoNotGrow(t *testing.T) {
	root := filepath.Join("..", "internal")
	for _, tc := range []struct{ name string; max int }{{"core", 58}, {"infra", 68}, {"crawler", 134}} {
		if got := productionGoFileCount(t, filepath.Join(root, tc.name)); got > tc.max {
			t.Errorf("internal/%s production files = %d, baseline max = %d", tc.name, got, tc.max)
		}
	}
	for _, tc := range []struct{ path string; max int }{
		{"task-processor/internal/core", 145},
		{"task-processor/internal/infra", 75},
		{"task-processor/internal/core/logger", 92},
	} {
		if got := internalImporterPackageCount(t, tc.path); got > tc.max {
			t.Errorf("%s importer packages = %d, baseline max = %d", tc.path, got, tc.max)
		}
	}
}

func TestTargetDomainsDoNotImportConcreteInfrastructure(t *testing.T) {
	index, err := loadGoFileIndex(filepath.Join("..", "internal"), "")
	if err != nil { t.Fatal(err) }
	domains := map[string]struct{}{
		"listing": {}, "product": {}, "marketplace": {}, "agent": {}, "knowledge": {},
		"resourcecatalog": {}, "commercial": {}, "ledger": {}, "organization": {},
	}
	for path, facts := range index.files {
		rel, err := filepath.Rel(filepath.Join("..", "internal"), path)
		if err != nil { t.Fatal(err) }
		parts := strings.Split(filepath.ToSlash(rel), "/")
		if len(parts) == 0 { continue }
		if _, ok := domains[parts[0]]; !ok { continue }
		for imp := range facts.imports {
			clean := strings.Trim(imp, `"`)
			if importMatchesPrefix(clean, "task-processor/internal/platform") ||
				importMatchesPrefix(clean, "task-processor/internal/integration") ||
				importMatchesPrefix(clean, "task-processor/internal/infra") ||
				importMatchesPrefix(clean, "task-processor/internal/app") {
				t.Errorf("%s imports concrete infrastructure %s", filepath.ToSlash(rel), imp)
			}
		}
	}
}
```

Add matching depguard rules named `target_domain_concrete_infrastructure` and `platform_domain_dependencies`. Enumerate the nine target domain roots in `files`; deny exact and subtree imports for `internal/platform`, `internal/integration`, legacy `internal/infra`, app packages, `gorm.io`, `go.temporal.io`, `go.opentelemetry.io`, `github.com/open-feature`, `github.com/aws`, `github.com/redis`, and `github.com/rabbitmq`; and deny all nine domain roots from platform files. Use the existing `.golangci.yml` depguard schema:

```yaml
target_domain_concrete_infrastructure:
  files:
    - "**/internal/listing/*.go"
    - "**/internal/listing/**/*.go"
    - "**/internal/product/*.go"
    - "**/internal/product/**/*.go"
    - "**/internal/marketplace/*.go"
    - "**/internal/marketplace/**/*.go"
    - "**/internal/agent/*.go"
    - "**/internal/agent/**/*.go"
    - "**/internal/knowledge/*.go"
    - "**/internal/knowledge/**/*.go"
    - "**/internal/resourcecatalog/*.go"
    - "**/internal/resourcecatalog/**/*.go"
    - "**/internal/commercial/*.go"
    - "**/internal/commercial/**/*.go"
    - "**/internal/ledger/*.go"
    - "**/internal/ledger/**/*.go"
    - "**/internal/organization/*.go"
    - "**/internal/organization/**/*.go"
  deny:
    - pkg: task-processor/internal/platform
      desc: domains depend on local ports, never platform implementations
    - pkg: task-processor/internal/platform/**
      desc: domains depend on local ports, never platform implementations
    - pkg: task-processor/internal/integration
      desc: app owns adapter wiring
    - pkg: task-processor/internal/integration/**
      desc: app owns adapter wiring
    - pkg: task-processor/internal/infra
      desc: retired infrastructure roots are not domain contracts
    - pkg: task-processor/internal/infra/**
      desc: retired infrastructure roots are not domain contracts
platform_domain_dependencies:
  files:
    - "**/internal/platform/**/*.go"
  deny:
    - pkg: task-processor/internal/listing
      desc: platform is domain neutral
    - pkg: task-processor/internal/listing/**
      desc: platform is domain neutral
```

Expand the first rule with the exact/subtree patterns for app and every listed SDK prefix, and expand the second rule with both direct-file and descendant `files` globs plus the same exact/subtree deny pair for all nine domain roots. Add a config test that checks the rule names, both file-glob forms for all nine roots, and all exact/subtree deny patterns; this abbreviated YAML shows the exact rule shape without duplicating the mechanical entries.

- [ ] **Step 4: Correct the target READMEs and write the inventory**

`internal/platform/README.md` must list config loading, logging, observability, database runtime/migration, Redis, queue, Temporal, and worker pool. `internal/integration/README.md` must list OpenAI, image providers, S3, Playwright, marketplace APIs, crawlers, ZITADEL, Casbin, and domain persistence adapters.

The new domain guard exposes two current violations and both must be corrected in this task:

- `internal/product/product_fetcher.go` imports the app-owned `ports.CrawlSource` even though it only calls `ProcessWithContext`. Change both product constructors to accept the already local `sourcing.AmazonCrawlerSource`, remove the app import, and keep the same concrete crawler values and tests.
- `internal/listing/submission/enqueue_retry.go` compares `worker.ErrQueueFull`. Replace that dependency with a package-local `interface{ QueueFull() bool }` classified through `errors.As`. Change the existing worker sentinel in `internal/infra/worker/pool.go` to a comparable typed error implementing `QueueFull() bool`, while preserving `errors.Is(err, worker.ErrQueueFull)`. Submission tests return a local fake queue-full error and worker tests prove both contracts.

These are boundary corrections, not new product or listing behavior.

Use these exact behavioral seams:

```go
// in worker/pool.go; move unchanged to platform/workerpool in Task 10
type queueFullError struct{}

func (queueFullError) Error() string   { return "工作队列已满" }
func (queueFullError) QueueFull() bool { return true }

var ErrQueueFull error = queueFullError{}

// in listing/submission/enqueue_retry.go
type queueFullError interface{ QueueFull() bool }

func isQueueFull(err error) bool {
	var target queueFullError
	return errors.As(err, &target) && target.QueueFull()
}
```

`docs/refactoring/phase2-runtime-inventory.md` must record this exact initial baseline:

```markdown
| Root | Production Go files | Test files | Go packages | Internal importers |
| --- | ---: | ---: | ---: | ---: |
| `internal/core` | 58 | 22 | 5 | 145 |
| `internal/infra` | 68 | 35 | 14 | 75 |
| `internal/platform` | 0 | 0 | 0 | 0 |
| `internal/integration` | 10 | 13 | 4 | measured per slice |
| `internal/crawler` | 134 | 51 | 4 | frozen pending product/marketplace ports |
```

It must also classify every current `core`, `infra`, and technical `pkg` subdirectory using the target file structure above, while explicitly retaining `core/metrics` business metrics for later domain phases and retaining `pkg/{downloader,imagex,jsonx,skugen,types,watermark}` for their owning domain reviews. Add a crawler table with the exact baselines `alibaba1688 36 production/16 tests`, `amazon 77/27`, `fetcher 3/2`, and `shared 18/6`; record that they currently import business config/models, app ports, queue/Redis/worker runtime, and product/sourcing behavior, so moving them before those local ports exist would preserve the dependency defect under a new path. Record mixed domain persistence adapters the same way and point them to their owning later phases.

Also record the exact pre-migration direct-importer debt that must not gain new consumers: `core/logger 92 packages`, `infra/database 19`, `infra/redisclient 6`, `infra/rabbitmq 17`, `infra/worker 23`, `infra/clients/openai 28`, `infra/clients/geminiimage 1`, `infra/clients/grsai 2`, `infra/storage 6`, and `pkg/safeimagehttp 8`. For every slice, distinguish app importers that this phase rewires from legacy business importers that will temporarily follow the relocated package; list the latter by package path and assign their removal to the relevant domain phase. These entries are migration debt, not approved final dependencies.

- [ ] **Step 5: Verify GREEN and commit**

```powershell
go test ./tests -run 'Test(Phase2TargetReadmesMatchApprovedOwnership|Phase2LegacyRootsDoNotGrow|TargetDomainsDoNotImportConcreteInfrastructure|.*Depguard.*)$' -count=1
git diff --check
git add .golangci.yml tests/target_architecture_phase2_test.go tests/depguard_config_test.go docs/refactoring/phase2-runtime-inventory.md internal/platform/README.md internal/integration/README.md internal/product/product_fetcher.go internal/product/product_fetcher_test.go internal/listing/submission/enqueue_retry.go internal/listing/submission/enqueue_retry_test.go internal/infra/worker/pool.go internal/infra/worker/worker_test.go
git commit -m "test(architecture): guard phase 2 runtime boundaries"
```

---

### Task 2: Move stable runtime-neutral primitives into shared

**Files:**
- Move: `internal/pkg/hashx/*` → `internal/shared/hashx/*`
- Move: `internal/pkg/mathx/*` → `internal/shared/mathx/*`
- Move: `internal/pkg/ptr/*` → `internal/shared/ptr/*`
- Move: `internal/pkg/strx/*` → `internal/shared/strx/*`
- Move: `internal/pkg/timex/*` → `internal/shared/timex/*`
- Modify: all Go files returned by `rg -l 'task-processor/internal/pkg/(hashx|mathx|ptr|strx|timex)' --glob '*.go'`

**Interfaces:**
- Consumes: existing exported APIs without signature changes.
- Produces: identical package APIs under `task-processor/internal/shared/{hashx,mathx,ptr,strx,timex}`.
- Retains: all of `internal/core/errors` because its task/platform/provider error codes and retry configuration are business/application semantics, not stable shared primitives.

- [ ] **Step 1: Write a failing shared-boundary test**

Add `TestSharedPackagesDoNotImportAppDomainPlatformOrIntegration` to `tests/target_architecture_phase2_test.go`. Scan `internal/shared` and reject imports with prefixes `task-processor/internal/app`, the nine domain roots, `task-processor/internal/platform`, and `task-processor/internal/integration`.

- [ ] **Step 2: Run RED against the planned package paths**

Add the exact target existence test, then run:

```go
func TestSharedPackageTargetsExist(t *testing.T) {
	for _, name := range []string{"hashx", "mathx", "ptr", "strx", "timex"} {
		info, err := os.Stat(filepath.Join("..", "internal", "shared", name))
		if err != nil || !info.IsDir() {
			t.Errorf("internal/shared/%s must exist as a directory: %v", name, err)
		}
	}
}
```

```powershell
go test ./tests -run 'TestShared(PackageTargetsExist|PackagesDoNotImportAppDomainPlatformOrIntegration)$' -count=1 -v
```

Expected: FAIL because the new target directories do not all exist.

- [ ] **Step 3: Move code and mechanically rewrite exact imports**

Use `git mv` for the five whole utility directories. Replace exact old imports with the corresponding shared paths; do not introduce forwarding aliases. Do not move `core/errors`, `pkg/types`, `pkg/jsonx`, or image/download helpers into shared.

- [ ] **Step 4: Prove old utility imports are zero and run GREEN**

```powershell
rg -n 'task-processor/internal/pkg/(hashx|mathx|ptr|strx|timex)' --glob '*.go'
go test ./internal/shared/... ./tests -run 'TestShared|TestPhase2LegacyRootsDoNotGrow' -count=1
```

Expected: `rg` returns no matches; tests PASS.

- [ ] **Step 5: Commit**

```powershell
git diff --check
git add internal/shared internal/pkg tests/target_architecture_phase2_test.go
git commit -m "refactor(shared): move stable runtime-neutral primitives"
```

---

## Track B: Platform runtime foundations

### Task 3: Extract the generic configuration engine

**Files:**
- Create: `internal/platform/config/source.go`
- Create: `internal/platform/config/source_test.go`
- Create: `internal/platform/config/codec.go`
- Create: `internal/platform/config/path.go`
- Create: `internal/platform/config/loader.go`
- Modify: `internal/core/config/manager.go`
- Modify: `internal/app/bootstrap/app.go`
- Delete after zero references: `internal/core/config/source.go`, `file.go`, `path.go`, `loader_interface.go`

**Interfaces:**
- Produces: `type Source interface { Read() ([]byte, error); Watch(context.Context, func([]byte)) error; Name() string }`.
- Produces: `NewFileSource(path string) Source`, `NewMemorySource(name string, data []byte) Source`.
- Produces: `LoadJSON(path string, dst any) error`, `LoadYAML(path string, dst any) error`, `SaveJSON(path string, value any) error`, and `SaveYAML(path string, value any) error`.
- Produces: `ResolvePath(basePath, configPath string) string` and `ExecutableBasePath() (string, error)`.
- Consumes: `core/config` application schema and validation remain in place; its manager accepts `platform/config.Source`.

- [ ] **Step 1: Write failing platform config tests**

```go
func TestFileSourceReadsAndNamesConfiguredFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("enabled: true\n"), 0o600); err != nil { t.Fatal(err) }
	source := NewFileSource(path)
	data, err := source.Read()
	if err != nil { t.Fatal(err) }
	if string(data) != "enabled: true\n" { t.Fatalf("data = %q", data) }
	if source.Name() != "file:"+path { t.Fatalf("name = %q", source.Name()) }
}

func TestMemorySourceReturnsInputWithoutFilesystemAccess(t *testing.T) {
	source := NewMemorySource("unit", []byte("answer: 42"))
	data, err := source.Read()
	if err != nil { t.Fatal(err) }
	if string(data) != "answer: 42" { t.Fatalf("data = %q", data) }
}
```

- [ ] **Step 2: Run RED**

```powershell
go test ./internal/platform/config -count=1 -v
```

Expected: FAIL because the package and constructors do not exist.

- [ ] **Step 3: Move generic behavior without moving business schemas**

Implement the interfaces above from the existing source, codec, path, and loader files. `internal/platform/config` may import only standard library, fsnotify, and yaml/json codecs. Update `core/config.Manager` to accept `platformconfig.Source`, and update `app/bootstrap` to call `platformconfig.NewFileSource(configPath)`.

- [ ] **Step 4: Verify no target platform package imports business code**

```powershell
go test ./internal/platform/config ./internal/core/config ./internal/app/bootstrap -count=1
go test ./tests -run 'Test(TargetDomainsDoNotImportConcreteInfrastructure|SharedPackagesDoNotImportAppDomainPlatformOrIntegration)' -count=1
rg -n 'NewFileConfigSource|NewMemoryConfigSource|LoadJSONConfig|LoadYAMLConfig|SaveJSONConfig|SaveYAMLConfig' --glob '*.go'
```

Expected: tests PASS; the final `rg` returns no production references to deleted names.

- [ ] **Step 5: Commit**

```powershell
git diff --check
git add internal/platform/config internal/core/config internal/app/bootstrap
git commit -m "refactor(config): extract platform configuration engine"
```

---

### Task 4: Move lifecycle ownership into app

**Files:**
- Move: `internal/core/lifecycle/component.go` → `internal/app/lifecycle/component.go`
- Move: `internal/core/lifecycle/interfaces.go` → `internal/app/lifecycle/interfaces.go`
- Move: `internal/core/lifecycle/manager_impl.go` → `internal/app/lifecycle/manager.go`
- Modify: all files returned by `rg -l 'task-processor/internal/core/lifecycle' --glob '*.go'`

**Interfaces:**
- Produces unchanged: `Component`, `LifecycleManager`, `ComponentStatus`, `HealthChecker`, `NewBaseComponent`, and `NewLifecycleManager`.
- Guarantees: dependency order on start and reverse dependency order on stop remain unchanged.

- [ ] **Step 1: Write target lifecycle tests before moving implementation**

Create `internal/app/lifecycle/manager_test.go` from the existing lifecycle test cases and add a cycle assertion:

```go
func TestLifecycleManagerRejectsDependencyCycle(t *testing.T) {
	m := NewLifecycleManager(logrus.New())
	if err := m.Register(newTestComponent("a", []string{"b"}, 10)); err != nil { t.Fatal(err) }
	if err := m.Register(newTestComponent("b", []string{"a"}, 20)); err != nil { t.Fatal(err) }
	if err := m.StartAll(context.Background()); err == nil || !strings.Contains(err.Error(), "循环依赖") {
		t.Fatalf("StartAll() error = %v", err)
	}
}

type testComponent struct {
	*BaseComponent
}

func newTestComponent(name string, dependencies []string, priority int) *testComponent {
	return &testComponent{BaseComponent: NewBaseComponent(name, dependencies, priority)}
}

func (c *testComponent) Start(context.Context) error { c.SetRunning(true); return nil }
func (c *testComponent) Stop(context.Context) error  { c.SetRunning(false); return nil }
```

- [ ] **Step 2: Run RED**

```powershell
go test ./internal/app/lifecycle -count=1 -v
```

Expected: FAIL before the moved implementation exists.

- [ ] **Step 3: Move implementation and rewrite all five importer packages**

Change the package declaration to `package lifecycle`, replace the exact old import path, then delete `internal/core/lifecycle` when `rg` proves zero references.

- [ ] **Step 4: Run GREEN and commit**

```powershell
rg -n 'task-processor/internal/core/lifecycle' --glob '*.go'
go test ./internal/app/lifecycle ./internal/app/bootstrap ./internal/app/consumer -count=1
git diff --check
git add internal/app/lifecycle internal/core/lifecycle internal/app/bootstrap internal/app/consumer
git commit -m "refactor(app): own component lifecycle assembly"
```

Expected: `rg` returns no matches; tests PASS.

---

### Task 5: Move logging runtime into platform without pushing infrastructure into domains

**Files:**
- Move: `internal/core/logger/{manager.go,level_split_hook.go,rotating_writer.go,helpers.go,context.go}` → `internal/platform/logging/`
- Move: `internal/core/logger/{manager_test.go,helpers_test.go}` → `internal/platform/logging/`
- Create: `internal/core/logger/compat.go`
- Create: `internal/core/logger/compat_test.go`
- Modify: `internal/core/config/config.go`
- Modify: `internal/core/config/common_types.go`
- Modify: every app Go file returned by `rg -l 'task-processor/internal/core/logger' internal/app --glob '*.go'`
- Modify: `internal/product/product_fetcher.go`, `product_fetcher_test.go`, `price_helper.go`, `price_helper_test.go`
- Modify: `tests/target_architecture_phase2_test.go` and docs/tests that name the old logger path

**Interfaces:**
- Produces unchanged under the new import path: `LogConfig`, `LevelFileConfig`, `LogManager`, `DefaultLogConfig`, `NewLogManager`, `InitGlobalLogger`, `GetGlobalLogger`, `GetGlobalLogManager`, and `SetGlobalLogLevel`.
- Produces: a deprecated `internal/core/logger` compatibility facade made only of type aliases, constant aliases, and function-variable forwarding to `platform/logging`; it owns no logger state, file writer, hook, formatting, or lifecycle.
- Guarantees: default remains stdout-only and explicit file output remains under app-selected paths.
- Tightens the compatibility importer baseline from 92 packages to 84 by moving all six app packages and `core/config` to the platform path and removing the target `product` domain's global logger dependency. Other legacy business packages remain on the facade until their owning domain phases can add local injected logger ports; they must not be mechanically pointed at platform.

- [ ] **Step 1: Add a failing target-package regression test**

Move the existing logger tests to `internal/platform/logging` and retain:

```go
func TestDefaultLogManagerDoesNotCreateRuntimeFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	manager := NewLogManager(nil)
	t.Cleanup(func() { _ = manager.Close() })
	manager.GetLogger("default-no-file").Info("stdout only")
	if _, err := os.Stat("tmp"); !os.IsNotExist(err) {
		t.Fatalf("default logger created a runtime directory: %v", err)
	}
}
```

- [ ] **Step 2: Run RED, move code, and rewrite exact imports**

```powershell
go test ./internal/platform/logging -count=1 -v
```

Expected before the move: FAIL. Move all implementation and tests and change core config to reference `platform/logging.LevelFileConfig`. Replace the old import only in `internal/app` composition packages and `core/config`. Build `core/logger/compat.go` from aliases/forwarders for every exported identifier reported by `rg -n '^(type|func) [A-Z]|^const \(' internal/platform/logging -g '*.go'`; add a compile-time compatibility test that exercises `DefaultLogConfig`, `NewLogManager`, `GetGlobalLogger`, `WithLogger`, `FromContext`, `NewLoggerHelper`, and field constants against the same platform singleton.

Remove both target-domain imports of the compatibility facade: when `NewProductFetcher` receives no logger, use a package-local logrus logger with `io.Discard`; keep explicit logger injection unchanged. `price_helper.go` must return the same zero values for nil products without logging through a global service locator. Add/retain nil-input tests. After these eight importer packages move or disappear, lower the `core/logger` importer ceiling in `TestPhase2LegacyRootsDoNotGrow` from 92 to 84 so the compatibility surface cannot grow back.

- [ ] **Step 3: Prove the old path is gone and run GREEN**

```powershell
rg -n 'task-processor/internal/core/logger' internal/app internal/core/config internal/product --glob '*.go'
go test ./internal/platform/logging ./internal/core/config ./internal/app/... -count=1
go test ./tests -run 'Test(InternalPackagesContainNoLocalArtifacts|MaintainedLoggingFilesStayUnderLocalRuntimeRoot|Phase2LegacyRootsDoNotGrow)' -count=1
```

Expected: `rg` returns no matches; compatibility importer count is at most 84; tests PASS.

- [ ] **Step 4: Commit**

```powershell
git diff --check
git add internal/platform/logging internal/core/logger internal/core/config internal/product internal/app tests docs
git commit -m "refactor(logging): move runtime logging into platform"
```

---

### Task 6: Move database connection ownership into platform

**Files:**
- Create: `internal/platform/database/config.go`
- Move/refactor: `internal/infra/database/postgres_db.go` → `internal/platform/database/postgres.go`
- Move/refactor tests: `internal/infra/database/*_test.go` → `internal/platform/database/*_test.go`
- Create: `internal/app/configadapter/database.go`
- Create: `internal/app/configadapter/database_test.go`
- Modify: every Go file returned by `rg -l 'task-processor/internal/infra/database' --glob '*.go'`
- Delete after zero references: `internal/infra/database`

**Interfaces:**
- Produces: `platform/database.Config{Host string, Port int, User string, Password string, Database string, MaxConnections int, MaxIdleConnections int, ConnectionMaxLifetime time.Duration}`.
- Produces: `Open(*Config) (*gorm.DB, error)`, `OpenExistingReadOnly(*Config) (*gorm.DB, error)`, `OpenExistingWritable(*Config) (*gorm.DB, error)`, `OpenShared(*Config) (*gorm.DB, error)`, `Close(*gorm.DB) error`, and `CloseShared(*Config, *gorm.DB) error`.
- Produces: `configadapter.Database(*coreconfig.DatabaseConfig) *platformdatabase.Config`.

- [ ] **Step 1: Write the failing config translation test**

```go
func TestDatabaseConfigPreservesRuntimeFields(t *testing.T) {
	in := &coreconfig.DatabaseConfig{
		Host: "db", Port: 5432, User: "worker", Password: "secret", Database: "tasks",
		MaxConnections: 12, MaxIdleConnections: 4, ConnectionMaxLifetime: 3 * time.Minute,
	}
	got := Database(in)
	want := &platformdatabase.Config{
		Host: "db", Port: 5432, User: "worker", Password: "secret", Database: "tasks",
		MaxConnections: 12, MaxIdleConnections: 4, ConnectionMaxLifetime: 3 * time.Minute,
	}
	if !reflect.DeepEqual(want, got) { t.Fatalf("config = %#v, want %#v", got, want) }
}
```

- [ ] **Step 2: Run RED**

```powershell
go test ./internal/app/configadapter -run TestDatabaseConfigPreservesRuntimeFields -count=1 -v
```

Expected: FAIL because target types and adapter do not exist.

- [ ] **Step 3: Move connection code and remove the core/config dependency**

Implement the target Config and rename constructors exactly as listed. `internal/platform/database` may import GORM and PostgreSQL drivers but must not import `internal/core/config` or any domain. All app callers translate through `configadapter.Database` at the composition boundary.

- [ ] **Step 4: Verify behavioral parity and zero references**

```powershell
rg -n 'task-processor/internal/infra/database' --glob '*.go'
go test ./internal/platform/database ./internal/app/configadapter ./internal/app/runtime/... ./internal/app/httpapi -count=1
go test ./tests -run 'Test(TargetDomainsDoNotImportConcreteInfrastructure|Phase2LegacyRootsDoNotGrow)' -count=1
```

Expected: `rg` returns no matches; connection, read-only, create-if-missing, and shared-reference tests PASS.

- [ ] **Step 5: Commit**

```powershell
git diff --check
git add internal/platform/database internal/app/configadapter internal/infra/database internal/app tests
git commit -m "refactor(database): move connection runtime into platform"
```

---

### Task 7: Introduce the OpenFeature runtime and migrate one real switch

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/platform/featureflag/runtime.go`
- Create: `internal/platform/featureflag/runtime_test.go`
- Create: `internal/app/httpapi/feature_flags.go`
- Create: `internal/app/httpapi/feature_flags_test.go`
- Modify: `internal/core/config/config.go`
- Modify: `internal/core/config/defaults.go`
- Modify: `internal/core/config/config_env_test.go`
- Modify: `config/config-dev.yaml`, `config/config-test.yaml`, `config/config-prod.yaml`
- Modify: `internal/app/httpapi/runtime_shared_deps.go`
- Retain temporarily: `internal/core/config/runtime_flags.go`, `runtime_flags_test.go` for the legacy `productimage/httpapi` and `productenrich/httpapi` entrypoints; add a source comment naming those two remaining consumers.

**Interfaces:**
- Produces: `type BoolEvaluator interface { Bool(context.Context, string, bool, map[string]any) bool }` in app HTTP composition.
- Produces: `featureflag.Config{Flags map[string]bool}` and `featureflag.New(context.Context, Config) (*Runtime, error)`.
- Produces: `(*Runtime).Bool(context.Context, key string, defaultValue bool, attributes map[string]any) bool` and `(*Runtime).Shutdown(context.Context) error`.
- Migrates key: `product-listing-runtime-auto-migrate`, default `true`, environment override `TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE`.

- [ ] **Step 1: Add the pinned dependency and write failing runtime tests**

```powershell
go get github.com/open-feature/go-sdk@v1.18.0
```

Test the official isolated API and in-memory provider through the local adapter:

```go
func TestRuntimeEvaluatesConfiguredBooleanWithoutGlobalState(t *testing.T) {
	runtime, err := New(context.Background(), Config{Flags: map[string]bool{"product-listing-runtime-auto-migrate": false}})
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })
	if runtime.Bool(context.Background(), "product-listing-runtime-auto-migrate", true, nil) {
		t.Fatal("configured false flag evaluated true")
	}
}
```

- [ ] **Step 2: Run RED**

```powershell
go test ./internal/platform/featureflag -count=1 -v
```

Expected: FAIL because the runtime does not exist.

- [ ] **Step 3: Implement the adapter with official OpenFeature components**

Use `isolated.NewAPI()`, `memprovider.NewInMemoryProvider`, `api.SetProviderAndWait`, and `api.NewClient()`. Convert each bool to an enabled `memprovider.InMemoryFlag` with variants `map[string]any{"configured": value}` and default variant `configured`. `Bool` calls the client with `openfeature.NewEvaluationContext("task-processor", attributes)`. `Shutdown` delegates to the isolated API.

- [ ] **Step 4: Replace the environment helper at the app boundary**

Add YAML:

```yaml
featureFlags:
  flags:
    product-listing-runtime-auto-migrate: true
```

Bind `TASK_PROCESSOR_API_RUNTIME_AUTOMIGRATE` to `featureFlags.flags.product-listing-runtime-auto-migrate`. Build the runtime once in `buildRuntimeDeps`, append its shutdown function to app closers, and make `shouldAutoMigrateProductListingAPIRuntime(ctx, evaluator)` evaluate the named key. Stop the migrated `internal/app/httpapi` path from calling `core/config.ProductListingAPIRuntimeAutoMigrateEnabled`; retain that compatibility helper only for the two explicitly listed legacy entrypoints until their app composition moves in their owning domain phases.

- [ ] **Step 5: Run GREEN and commit**

```powershell
go test ./internal/platform/featureflag ./internal/core/config ./internal/app/httpapi -run 'Test.*(Feature|Flag|AutoMigrate).*' -count=1
rg -n 'ProductListingAPIRuntimeAutoMigrateEnabled' --glob '*.go'
git diff --check
git add go.mod go.sum internal/platform/featureflag internal/core/config internal/app/httpapi config
git commit -m "feat(featureflag): route runtime migration switch through OpenFeature"
```

Expected: production matches remain only in `internal/productimage/httpapi/task_repository_builder.go` and `internal/productenrich/httpapi/bootstrap.go`; `internal/app/httpapi` has zero matches and tests PASS.

---

### Task 8: Replace scattered API AutoMigrate calls with a Goose vertical slice

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/platform/database/migration/runner.go`
- Create: `internal/platform/database/migration/runner_test.go`
- Create: `internal/app/schema/productlisting/migrations.go`
- Modify: `internal/app/schema/productlisting/runtime.go`, `runtime_test.go`
- Modify: `internal/app/runtime/productlistingschemamigrate/runtime.go`, `runtime_test.go`
- Modify: `internal/app/httpapi/adapters_schema_migration.go`
- Modify: `internal/app/httpapi/adapters_ai_capability.go`
- Modify: `internal/app/httpapi/adapters_openai.go`
- Modify: `internal/app/httpapi/adapters_prompt.go`
- Modify: `internal/app/httpapi/adapters_task_repositories.go`
- Modify: `internal/app/httpapi/runtime.go`
- Modify: `internal/productimage/httpapi/task_repository_builder.go` and tests
- Modify: `internal/productenrich/httpapi/bootstrap.go` and tests

**Interfaces:**
- Produces: `migration.New(dialect goose.Dialect, db *sql.DB, migrations ...*goose.Migration) (*Runner, error)`.
- Produces: `(*Runner).Up(context.Context) ([]*goose.MigrationResult, error)` and `(*Runner).Status(context.Context) ([]*goose.MigrationStatus, error)`.
- Produces: `productlisting.Migrations(db *gorm.DB) []*goose.Migration` with immutable version `2026083001`.
- Produces: `productlisting.Migrate(ctx context.Context, db *gorm.DB) error`.

- [ ] **Step 1: Add Goose and write the idempotency test**

```powershell
go get github.com/pressly/goose/v3@v3.27.3
```

```go
func TestMigrateRecordsBaselineAndDoesNotRunTwice(t *testing.T) {
	db := openProductListingSchemaTestDB(t)
	if err := Migrate(context.Background(), db); err != nil { t.Fatal(err) }
	if err := Migrate(context.Background(), db); err != nil { t.Fatal(err) }
	sqlDB, err := db.DB()
	if err != nil { t.Fatal(err) }
	var count int
	if err := sqlDB.QueryRow(`SELECT count(*) FROM goose_db_version WHERE version_id = 2026083001 AND is_applied = 1`).Scan(&count); err != nil { t.Fatal(err) }
	if count != 1 { t.Fatalf("applied baseline rows = %d", count) }
}
```

- [ ] **Step 2: Run RED**

```powershell
go test ./internal/app/schema/productlisting -run TestMigrateRecordsBaselineAndDoesNotRunTwice -count=1 -v
```

Expected: FAIL because `Migrate` and the Goose version table do not exist.

- [ ] **Step 3: Implement the generic runner and versioned app migration**

The runner constructs `goose.NewProvider(dialect, db, nil, goose.WithGoMigrations(migrations...))` so it cannot accidentally scan repository SQL files. `productlisting.Migrations` returns `goose.NewGoMigration(2026083001, &goose.GoFunc{RunDB: func(context.Context, *sql.DB) error { return AutoMigrateRuntime(db) }}, nil)`. Select `goose.DialectSQLite3` when `db.Dialector.Name() == "sqlite"`; otherwise require `postgres` and use `goose.DialectPostgres`.

The baseline migration deliberately has no down migration because dropping existing production tables is unsafe. Add a source comment that new schema work receives a new immutable Goose version.

- [ ] **Step 4: Centralize app execution and delete scattered migrations**

Run `productlisting.Migrate` once per process after the feature flag evaluation and before repository construction. Replace the schema-migrate command dependency default with `productlisting.Migrate`. Remove direct `AutoMigrateRuntime` calls from the OpenAI, AI capability, prompt, task repository, product-image, and product-enrichment builders; the two legacy entrypoints may still use the compatibility flag helper, but when enabled they call the same idempotent Goose entrypoint. Their tests must assert repository construction does not mutate schema outside that explicit migration call.

- [ ] **Step 5: Run GREEN, prove one entrypoint, and commit**

```powershell
go test ./internal/platform/database/migration ./internal/app/schema/productlisting ./internal/app/runtime/productlistingschemamigrate ./internal/app/httpapi ./internal/productimage/httpapi ./internal/productenrich/httpapi -count=1
rg -n 'AutoMigrate(Runtime)?\(' internal/app/httpapi internal/productimage/httpapi internal/productenrich/httpapi -g '*.go'
go test ./tests -run 'Test(TargetDomainsDoNotImportConcreteInfrastructure|Phase2LegacyRootsDoNotGrow)' -count=1
git diff --check
git add go.mod go.sum internal/platform/database/migration internal/app/schema/productlisting internal/app/runtime/productlistingschemamigrate internal/app/httpapi internal/productimage/httpapi internal/productenrich/httpapi
git commit -m "feat(database): govern product listing schema with Goose"
```

Expected: no production direct `AutoMigrate(` or `AutoMigrateRuntime(` remains in the three migrated HTTP entrypoint packages; the only schema call is `productlisting.Migrate`; tests PASS.

---

### Task 9: Move Redis and distributed lock runtime into platform

**Files:**
- Create: `internal/platform/redis/config.go`
- Move/refactor: `internal/infra/redisclient/redis.go` → `internal/platform/redis/client.go`
- Move/refactor: `internal/infra/lock/redis_lock.go` → `internal/platform/redis/lock.go`
- Move/refactor: `internal/infra/lock/memory_lock.go` → `internal/platform/redis/memory_lock.go`
- Move/refactor: `internal/infra/lock/redis_lock_test.go` → `internal/platform/redis/lock_test.go`
- Move/refactor: `internal/infra/lock/memory_lock_test.go` → `internal/platform/redis/memory_lock_test.go`
- Create: `internal/app/scheduler/lock_port.go`
- Modify: `internal/app/scheduler/manager.go`, `locked_task_executor.go`, `manager_with_lock.go`, `task_executor.go`, `task_executor_test.go`
- Create: `internal/app/configadapter/redis.go`, `internal/app/configadapter/redis_test.go`
- Modify: all files returned by `rg -l 'task-processor/internal/infra/(redisclient|lock)' --glob '*.go'`
- Delete after zero references: `internal/infra/redisclient`, `internal/infra/lock`

**Interfaces:**
- Produces: `redis.Config{Host string, Port int, Password string, DB int, PoolSize int}`.
- Produces unchanged client operations: `Push`, `Get`, `Set`, `SetNX`, `Delete`, `Scan`, `SMembers`, `SAdd`, `ReplaceSet`, and `Close`.
- Produces: local `scheduler.DistributedLock` with `TryLock`, `Unlock`, `Extend`, and `IsLocked`; scheduler orchestration depends on this local port and never imports platform Redis.
- Produces: `platform/redis.RedisLock` and `MemoryLock` that satisfy the local scheduler port structurally. `LockOptions` and `DefaultLockOptions` remain implementation-side platform types because no app caller consumes them.
- Produces: `configadapter.Redis(*coreconfig.RedisConfig) *platformredis.Config`.

- [ ] **Step 1: Write failing config and miniredis behavior tests**

Add a config round-trip test matching all five fields, and move the existing lock/client behavior tests. Include:

```go
func TestReplaceSetRemovesStaleMembers(t *testing.T) {
	client := newTestClient(t)
	ctx := context.Background()
	if err := client.ReplaceSet(ctx, "owners", "a", "b"); err != nil { t.Fatal(err) }
	if err := client.ReplaceSet(ctx, "owners", "b", "c"); err != nil { t.Fatal(err) }
	got, err := client.SMembers(ctx, "owners")
	if err != nil { t.Fatal(err) }
	slices.Sort(got)
	if !slices.Equal(got, []string{"b", "c"}) { t.Fatalf("members = %v", got) }
}

func newTestClient(t *testing.T) *Client {
	t.Helper()
	server := miniredis.RunT(t)
	raw := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = raw.Close() })
	return &Client{rdb: raw}
}
```

- [ ] **Step 2: Run RED, move, and rewire**

```powershell
go test ./internal/platform/redis ./internal/app/configadapter -run 'Test(ReplaceSet|RedisConfig)' -count=1 -v
```

Expected before implementation: FAIL. Move behavior, remove the `core/config` import from platform, and translate config only in app. Copy the four-method lock interface into `internal/app/scheduler/lock_port.go`, switch scheduler fields and methods to that local interface, and delete `internal/infra/lock/distributed_lock.go`; do not make scheduler import the concrete platform package.

- [ ] **Step 3: Verify zero references and commit**

```powershell
rg -n 'task-processor/internal/infra/(redisclient|lock)' --glob '*.go'
go test ./internal/platform/redis ./internal/app/configadapter ./internal/app/scheduler ./internal/app/runner ./internal/app/consumer ./internal/app/httpapi ./internal/app/runtime/listing ./internal/app/runtime/listingcontrol -count=1
git diff --check
git add internal/platform/redis internal/app/configadapter internal/infra/redisclient internal/infra/lock internal/app
git commit -m "refactor(redis): move client and locks into platform"
```

Expected: `rg` returns no matches; tests PASS.

---

### Task 10: Move RabbitMQ and worker pool runtime into platform

**Files:**
- Move: `internal/infra/rabbitmq/*` → `internal/platform/queue/rabbitmq/*`
- Move: `internal/infra/worker/*` → `internal/platform/workerpool/*`
- Move: `internal/infra/metrics/*` → `internal/app/consumer/metrics/*`
- Create: `internal/platform/queue/rabbitmq/load_monitor_config.go`
- Create: `internal/platform/workerpool/config.go`
- Create: `internal/app/configadapter/queue.go`, `workerpool.go` and tests
- Modify: every Go file returned by `rg -l 'task-processor/internal/infra/(rabbitmq|worker|metrics)' --glob '*.go'`
- Delete after zero references: `internal/infra/rabbitmq`, `internal/infra/worker`, `internal/infra/metrics`

**Interfaces:**
- Produces existing RabbitMQ public API under `platform/queue/rabbitmq` without changing exchange, queue, routing-key, retry, reconnect, consumer, or message semantics.
- Produces local `rabbitmq.LoadMonitorConfig` and existing `workerpool.PoolConfig`; `configadapter.LoadMonitor` and `configadapter.WorkerPool` translate from current `core/config` types.
- Produces unchanged `workerpool.WorkerPool` methods: `Start(context.Context)`, `Stop(context.Context)`, `Submit(Task) error`, and stats accessors already used by app.
- Produces a typed queue-full sentinel that preserves `errors.Is(err, workerpool.ErrQueueFull)` and implements `interface{ QueueFull() bool }`; listing submission classifies this local behavior interface and never imports workerpool.

- [ ] **Step 1: Move tests first and add config translation assertions**

Add these translation tests and preserve `TestNamingService`, reconnect, queue initializer, retry strategy, message codec, and worker stop/drain tests under target paths:

```go
func TestLoadMonitorConfigPreservesAllFields(t *testing.T) {
	in := coreconfig.LoadMonitorConfig{UpdateInterval: 7 * time.Second, EnableCPU: true, EnableMemory: true, EnableTasks: false}
	want := rabbitmq.LoadMonitorConfig{UpdateInterval: 7 * time.Second, EnableCPU: true, EnableMemory: true, EnableTasks: false}
	if got := LoadMonitor(in); !reflect.DeepEqual(got, want) { t.Fatalf("config = %#v, want %#v", got, want) }
}

func TestWorkerPoolConfigUsesLegacyConcurrencyAndBuffer(t *testing.T) {
	got := WorkerPool(coreconfig.WorkerConfig{Concurrency: 9, BufferSize: 40})
	if got.Concurrency != 9 || got.BufferSize != 40 || got.TaskTimeout != 15*time.Minute || !got.EnableMetrics || got.ShutdownTimeout != 30*time.Second {
		t.Fatalf("pool config = %#v", got)
	}
}
```

- [ ] **Step 2: Run RED**

```powershell
go test ./internal/platform/queue/rabbitmq ./internal/platform/workerpool ./internal/app/configadapter -count=1 -v
```

Expected before implementation: FAIL because target packages do not exist.

- [ ] **Step 3: Move implementations and rewrite imports mechanically**

Keep `amqp091-go` types inside platform queue and app adapters. Remove `core/config` imports from both target packages. Do not move app-owned consumer orchestration, scheduler policy, HTTP health endpoints, or queue-handler construction into platform.

Preserve the typed queue-full behavior established by Task 1 when moving worker pool. Run the listing submission tests after the move to prove it still classifies a full queue without importing platform.

Move `internal/infra/metrics` to `internal/app/consumer/metrics` in the same slice because it consumes task, SHEIN, and RabbitMQ snapshots and is therefore consumer composition, not generic platform observability. Update its queue import to `internal/platform/queue/rabbitmq`; do not move `core/metrics` in this phase.

- [ ] **Step 4: Verify behavior and zero references**

```powershell
rg -n 'task-processor/internal/infra/(rabbitmq|worker|metrics)' --glob '*.go'
go test ./internal/platform/queue/rabbitmq ./internal/platform/workerpool ./internal/listing/submission ./internal/app/consumer/... ./internal/app/httpapi ./internal/app/bootstrap ./internal/app/crawler/distributed -count=1
go test ./tests -run 'Test(TargetDomainsDoNotImportConcreteInfrastructure|Phase2LegacyRootsDoNotGrow)' -count=1
```

Expected: `rg` returns no matches; tests PASS.

- [ ] **Step 5: Commit**

```powershell
git diff --check
git add internal/platform/queue internal/platform/workerpool internal/app/configadapter internal/app/consumer/metrics internal/infra/rabbitmq internal/infra/worker internal/infra/metrics internal/app tests
git commit -m "refactor(platform): own queue and worker pool runtime"
```

---

### Task 11: Move runtime-neutral resilience primitives into shared

**Files:**
- Move: `internal/infra/resilience/*` → `internal/shared/resilience/*`
- Modify: every Go file returned by `rg -l 'task-processor/internal/infra/resilience' --glob '*.go'`
- Delete after zero references: `internal/infra/resilience`

**Interfaces:**
- Produces unchanged: breaker, retry, and rate-limit public constructors and behavior under `internal/shared/resilience`.
- Guarantees: the target package imports only standard library, `github.com/cenkalti/backoff/v5`, `github.com/sony/gobreaker`, and `golang.org/x/time/rate`; it has no app, domain, platform, or integration dependency.
- Rationale: current consumers include Amazon, image-agent, and product-enrichment business policies. Putting this package in platform would force domains to depend on concrete runtime infrastructure; these stateless algorithms satisfy the shared boundary instead.

- [ ] **Step 1: Move the existing tests to the target path and run RED**

```powershell
go test ./internal/shared/resilience -count=1 -v
```

Expected before the implementation move: FAIL because the target package does not exist.

- [ ] **Step 2: Move implementation and rewrite exact imports**

Use `git mv` for all files, preserve package name `resilience`, replace the exact old import path, and add a boundary assertion that the target package has no internal imports.

- [ ] **Step 3: Verify and commit**

```powershell
rg -n 'task-processor/internal/infra/resilience' --glob '*.go'
go test ./internal/shared/resilience ./tests -run 'Test.*(Shared|Phase2).*' -count=1
git diff --check
git add internal/shared/resilience internal/infra/resilience tests internal/amazon internal/imageagent internal/productenrich internal/app
git commit -m "refactor(shared): move runtime-neutral resilience primitives"
```

Expected: `rg` returns no matches; tests PASS.

---

### Task 12: Centralize Temporal SDK client construction

**Files:**
- Create: `internal/platform/temporal/client.go`
- Create: `internal/platform/temporal/client_test.go`
- Modify: `internal/app/runtime/temporal_runtime.go`, `temporal_runtime_test.go`
- Modify: `internal/app/runtime/image_agent_temporal_runtime.go`, `image_agent_temporal_runtime_test.go`

**Interfaces:**
- Produces: `temporal.Config{Address string, Namespace string}`.
- Produces: `temporal.Options(Config) client.Options`.
- Produces: `temporal.Dial(context.Context, Config) (client.Client, func() error, error)` using the SDK's context-aware dial path.
- Defaults: address `localhost:7233`, namespace `default`.
- Retains: domain-specific task queues, workflow/activity registration, and worker policy outside platform.

- [ ] **Step 1: Write the failing option-resolution test**

```go
func TestOptionsAppliesDefaults(t *testing.T) {
	got := Options(Config{})
	if got.HostPort != "localhost:7233" || got.Namespace != "default" {
		t.Fatalf("options = %#v", got)
	}
}
```

- [ ] **Step 2: Run RED, implement, and rewire both app runtimes**

```powershell
go test ./internal/platform/temporal -run TestOptionsAppliesDefaults -count=1 -v
```

Expected before implementation: FAIL. Implement `Options` and `Dial`; `Dial` calls `sdkclient.DialContext(ctx, Options(cfg))` and returns an idempotent close function. Replace both direct SDK dial calls in app runtime. App retains environment parsing and logging; platform receives only resolved values.

- [ ] **Step 3: Verify direct SDK dial ownership and commit**

```powershell
rg -n 'sdkclient\.Dial(Context)?' internal/app internal/platform -g '*.go'
go test ./internal/platform/temporal ./internal/app/runtime -count=1
git diff --check
git add internal/platform/temporal internal/app/runtime
git commit -m "refactor(temporal): centralize SDK client construction"
```

Expected: production `sdkclient.DialContext` appears only in `internal/platform/temporal`; tests PASS.

---

## Track C: Integration adapters

### Task 13: Move AI provider clients into integration

**Files:**
- Move: `internal/infra/clients/openai/*` → `internal/integration/openai/*`
- Move: `internal/infra/clients/geminiimage/*` → `internal/integration/geminiimage/*`
- Move: `internal/infra/clients/grsai/*` → `internal/integration/grsai/*`
- Move/refactor: `internal/pkg/safeimagehttp/*` → `internal/integration/httpimage/*`
- Modify: every Go file returned by `rg -l 'task-processor/internal/(infra/clients/(openai|geminiimage|grsai)|pkg/safeimagehttp)' --glob '*.go'`
- Delete after zero references: the four old package paths

**Interfaces:**
- Produces the existing OpenAI manager, client, pool, credential resolver, image API, Gemini image client, and GRSAI client behavior under integration paths.
- Consumes only legacy provider-neutral `internal/ai` contracts, shared primitives, locally declared logging ports, and external SDKs.
- Produces identical local `Logger` method sets in provider packages, `AdaptLogrus(*logrus.Entry) Logger`, and explicit logger fields on `ClientConfig`, `PoolConfig`, `CachedClientConfig`, `ManagerConfig`, and `ResilientClientConfig`; app and legacy composition callers supply their existing component entries through the adapter.
- Does not move prompt, capability routing, product enrichment, or image-agent workflow behavior into integration.

```go
type Logger interface {
	Debug(message string, fields map[string]any)
	Info(message string, fields map[string]any)
	Warn(message string, fields map[string]any)
	Error(message string, fields map[string]any)
}
```

- [ ] **Step 1: Add a failing integration dependency-boundary test**

Extend `tests/target_architecture_phase2_test.go` so production files under `integration/{openai,geminiimage,grsai,httpimage}` reject imports of app, service/workflow/handler packages, and all domain roots except exact focused contract packages. For this slice, allow only `task-processor/internal/ai` and `task-processor/internal/shared/...` until the agent phase introduces final contracts.

- [ ] **Step 2: Run RED against absent target packages**

```powershell
go test ./tests -run TestIntegrationProviderAdaptersUseOnlyContracts -count=1 -v
```

Expected: FAIL because the required target packages do not exist.

- [ ] **Step 3: Move packages, preserve tests, and rewrite imports**

Rename `safeimagehttp` package to `httpimage` and update its callers. Remove every integration import of platform logging: add the local `Logger` interface and propagate an explicitly injected logger through OpenAI configs. Replace `WithFields`, `WithError`, and formatted logrus calls with the four structured methods, using `fmt.Sprintf` for formatted messages and a map for fields. Implement `AdaptLogrus` inside each provider package as a stateless wrapper around the supplied entry; this is a log transport adapter, not a global logger lookup. A package-local no-op logger is allowed only when a unit test omits logging. Update every production constructor caller—including legacy business composition packages—to pass its existing component entry through `AdaptLogrus`; do not silently downgrade production logging to a discard or standard global logger.

- [ ] **Step 4: Prove old paths are gone and run provider tests**

```powershell
rg -n 'task-processor/internal/(infra/clients/(openai|geminiimage|grsai)|pkg/safeimagehttp)' --glob '*.go'
go test ./internal/integration/openai ./internal/integration/geminiimage ./internal/integration/grsai ./internal/integration/httpimage ./internal/app/httpapi ./internal/app/worker/imageagent -count=1
go test ./tests -run 'Test(IntegrationProviderAdaptersUseOnlyContracts|Phase2LegacyRootsDoNotGrow)' -count=1
```

Expected: `rg` returns no matches; tests PASS.

- [ ] **Step 5: Commit**

```powershell
git diff --check
git add internal/integration internal/infra/clients internal/pkg/safeimagehttp internal/app tests
git commit -m "refactor(integration): move AI provider adapters"
```

---

### Task 14: Move S3 implementation into integration

**Files:**
- Move/refactor: `internal/infra/storage/s3_client.go` → `internal/integration/s3/client.go`
- Move/refactor: `internal/infra/storage/s3_uploader.go` → `internal/integration/s3/uploader.go`
- Move/refactor: `internal/infra/storage/object_url.go` → `internal/integration/s3/object_url.go`
- Move: corresponding tests
- Modify: `internal/app/worker/imageagent/dependencies.go`, `dependencies_test.go`
- Modify: all remaining imports returned by `rg -l 'task-processor/internal/infra/storage' --glob '*.go'`
- Delete after zero references: `internal/infra/storage`

**Interfaces:**
- Produces: `s3.ClientConfig`, `s3.NewClient`, `s3.UploaderOptions`, `s3.NewUploaderWithOptions`, and current object URL/capability behavior.
- Produces: local `s3.Logger` and `UploaderOptions.Logger`; app supplies the logger, and integration/s3 never imports platform logging.
- Produces: `s3.AdaptLogrus(*logrus.Entry) Logger`; every production caller passes its existing component entry, while only tests may rely on the no-op default.
- Implements: the existing `imageagenttemporal.DurableArtifactStore` through the app-owned `workerArtifactStore` bridge; the domain never imports S3 or AWS SDK types.

```go
type Logger interface {
	Debug(message string, fields map[string]any)
	Info(message string, fields map[string]any)
	Warn(message string, fields map[string]any)
	Error(message string, fields map[string]any)
}
```

- [ ] **Step 1: Move the uploader contract tests to the target package**

Retain endpoint, path-style, public-base, immutable-key, and capability validation assertions. Add:

```go
func TestUploaderRejectsEmptyBucket(t *testing.T) {
	_, err := NewUploaderWithOptions(fakeS3Client{}, UploaderOptions{})
	if err == nil || !strings.Contains(err.Error(), "bucket") { t.Fatalf("error = %v", err) }
}
```

- [ ] **Step 2: Run RED, move implementation, and update app wiring**

```powershell
go test ./internal/integration/s3 -count=1 -v
```

Expected before implementation: FAIL. Move code, replace global logging and every returned logrus entry with the local structured logger, and update every production constructor caller to pass `s3.AdaptLogrus(existingEntry)`. Keep AWS configuration and client lifetime in app/integration, not imageagent domain packages; legacy non-app constructors are recorded migration debt and must not fall back to a silent logger.

- [ ] **Step 3: Verify zero references and commit**

```powershell
rg -n 'task-processor/internal/infra/storage' --glob '*.go'
rg -n 'aws-sdk-go|service/s3|internal/integration/s3' internal/imageagent internal/listing internal/product internal/marketplace -g '*.go'
go test ./internal/integration/s3 ./internal/app/worker/imageagent ./internal/app/runtime -count=1
git diff --check
git add internal/integration/s3 internal/infra/storage internal/app/worker/imageagent internal/app/runtime
git commit -m "refactor(storage): move S3 adapter into integration"
```

Expected: both `rg` checks return no forbidden domain imports; tests PASS.

---

## Track D: Observability and closure

### Task 15: Trace the product-listing HTTP path with OpenTelemetry

**Files:**
- Modify: `go.mod`, `go.sum`
- Create: `internal/platform/observability/config.go`
- Create: `internal/platform/observability/tracing.go`
- Create: `internal/platform/observability/tracing_test.go`
- Modify: `internal/core/config/config.go`, `defaults.go`, `config_env_test.go`
- Modify: `config/config-dev.yaml`, `config/config-test.yaml`, `config/config-prod.yaml`
- Modify: `internal/app/httpapi/runtime.go`
- Modify: `internal/app/httpapi/runtime_shared_deps.go`
- Modify: `internal/app/httpapi/bootstrap.go`
- Modify: `internal/app/httpapi/app.go`
- Modify: `internal/app/httpapi/e2e_test.go`
- Move/refactor: `internal/infra/monitoring/*` → `internal/app/monitoring/*`
- Modify: importers of `internal/infra/monitoring`

**Interfaces:**
- Produces: `observability.Config{Enabled bool, ServiceName string, Endpoint string, Insecure bool}`.
- Produces: `observability.NewTraceRuntime(context.Context, Config) (*TraceRuntime, error)`.
- Produces: `(*TraceRuntime).WrapHTTPHandler(http.Handler, string) http.Handler` and `(*TraceRuntime).Shutdown(context.Context) error`.
- Defaults: disabled, service name `task-processor`, no exporter connection when disabled.

```go
type TraceRuntime struct {
	provider trace.TracerProvider
	shutdown func(context.Context) error
}
```

- [ ] **Step 1: Add aligned dependencies**

```powershell
go get go.opentelemetry.io/otel@v1.44.0 go.opentelemetry.io/otel/sdk@v1.44.0 go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc@v1.44.0 go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@v0.66.0
```

- [ ] **Step 2: Write failing disabled and span-recording tests**

```go
func TestDisabledTraceRuntimeDoesNotDialExporter(t *testing.T) {
	runtime, err := NewTraceRuntime(context.Background(), Config{Enabled: false, Endpoint: "127.0.0.1:1"})
	if err != nil { t.Fatal(err) }
	if err := runtime.Shutdown(context.Background()); err != nil { t.Fatal(err) }
}

func TestWrappedHandlerRecordsServerSpan(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	runtime := &TraceRuntime{provider: provider, shutdown: provider.Shutdown}
	handler := runtime.WrapHTTPHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), "product-listing-api")
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
	if got := recorder.Ended(); len(got) != 1 { t.Fatalf("ended spans = %d", len(got)) }
}
```

- [ ] **Step 3: Run RED and implement the trace runtime**

```powershell
go test ./internal/platform/observability -count=1 -v
```

Expected: FAIL because runtime functions do not exist. When enabled, construct the OTLP gRPC exporter, resource with `service.name`, batch span processor, and SDK provider. When disabled, use `trace.NewNoopTracerProvider`. `WrapHTTPHandler` must call `otelhttp.NewHandler` with the runtime provider rather than global state.

- [ ] **Step 4: Wire one real HTTP vertical slice**

Add YAML `observability.tracing` config with `enabled: false` by default. Build the trace runtime in `buildRuntimeDeps`, append shutdown last, and wrap `server.Handler` in `buildBootstrap` before returning. Add an HTTP e2e test using an injected span recorder and assert a request produces a server span without changing status/body.

Move the existing process metrics and health-check lifecycle components from `internal/infra/monitoring` to `internal/app/monitoring`. Update them to use `internal/app/lifecycle`; they stay in app because they implement app component lifecycle. Preserve their public behavior and tests, and prove the old monitoring import path has zero references.

- [ ] **Step 5: Run GREEN and commit**

```powershell
go test ./internal/platform/observability ./internal/app/monitoring ./internal/core/config ./internal/app/httpapi -run 'Test.*(Trace|Span|HTTP|Health|Metric).*' -count=1
go test ./internal/app/httpapi -count=1
git diff --check
git add go.mod go.sum internal/platform/observability internal/app/monitoring internal/infra/monitoring internal/core/config internal/app/httpapi config
git commit -m "feat(observability): trace product listing HTTP requests"
```

---

### Task 16: Close phase 2 and make the new direction permanent

**Files:**
- Modify: `.golangci.yml`
- Modify: `tests/target_architecture_phase2_test.go`
- Modify: `tests/depguard_config_test.go`
- Modify: `tests/architecture_docs_test.go`
- Modify: `docs/development/repository-structure.md`
- Modify: `docs/architecture/project-boundaries.md`
- Modify: `docs/refactoring/module-target-mapping.md`
- Modify: `docs/refactoring/phase2-runtime-inventory.md`
- Modify: `internal/platform/README.md`
- Modify: `internal/integration/README.md`

**Interfaces:**
- Consumes: all prior target packages and zero-reference proofs.
- Produces: permanent deny rules for resurrecting migrated `core`, `infra`, and `pkg` paths.
- Produces: a phase-2 completion table with remaining intentionally deferred mixed capabilities and their later owning phases.

- [ ] **Step 1: Add failing retirement assertions**

Add:

```go
func TestPhase2MigratedLegacyPackagesStayRetired(t *testing.T) {
	for _, path := range []string{
		"internal/core/lifecycle", "internal/infra/database",
		"internal/infra/redisclient", "internal/infra/lock", "internal/infra/rabbitmq",
		"internal/infra/worker", "internal/infra/clients/openai", "internal/infra/clients/geminiimage",
		"internal/infra/clients/grsai", "internal/infra/storage", "internal/infra/resilience",
		"internal/infra/metrics", "internal/infra/monitoring", "internal/pkg/safeimagehttp",
		"internal/pkg/hashx", "internal/pkg/mathx", "internal/pkg/ptr", "internal/pkg/strx", "internal/pkg/timex",
	} {
		if _, err := os.Stat(filepath.Join("..", filepath.FromSlash(path))); !os.IsNotExist(err) {
			t.Errorf("retired path still exists: %s", path)
		}
	}
}
```

- [ ] **Step 2: Run retirement RED if any old path remains**

```powershell
go test ./tests -run TestPhase2MigratedLegacyPackagesStayRetired -count=1 -v
```

Expected: PASS only when all packages migrated by this plan are absent; otherwise remove remaining callers or files before continuing.

- [ ] **Step 3: Convert migration guards to permanent deny rules**

Add depguard rules preventing any production package from importing the retired paths. Update `tests/depguard_config_test.go` to require each exact and subtree path. Do not deny the whole `internal/core` or `internal/infra` root while deferred business metrics, auth, HTTP transport, and other classified legacy items still exist; keep their monotonic baseline instead.

Add a second monotonic table to `TestPhase2LegacyRootsDoNotGrow` so temporary legacy consumers of relocated concrete packages cannot grow while later phases remove them:

```go
for _, tc := range []struct{ path string; max int }{
	{"task-processor/internal/core/logger", 84},
	{"task-processor/internal/platform/logging", 8},
	{"task-processor/internal/platform/database", 20},
	{"task-processor/internal/platform/redis", 8},
	{"task-processor/internal/platform/queue/rabbitmq", 18},
	{"task-processor/internal/platform/workerpool", 23},
	{"task-processor/internal/integration/openai", 28},
	{"task-processor/internal/integration/geminiimage", 1},
	{"task-processor/internal/integration/grsai", 2},
	{"task-processor/internal/integration/s3", 6},
	{"task-processor/internal/integration/httpimage", 8},
} {
	if got := internalImporterPackageCount(t, tc.path); got > tc.max {
		t.Errorf("%s importer packages = %d, phase-2 ceiling = %d", tc.path, got, tc.max)
	}
}
```

If the implementation naturally produces fewer importers, lower the checked ceiling to the observed value in the same closure commit; never raise one of these reviewed ceilings to make the test pass.

- [ ] **Step 4: Update stable documentation and final inventory**

Document:

- app-owned lifecycle, provider registration, migration execution, instrumentation, and shutdown;
- platform-owned config mechanism, logging, database/Goose, Redis, RabbitMQ, worker pool, Temporal dial, feature flags, and tracing;
- integration-owned OpenAI, Gemini, GRSAI, S3, and remote image HTTP;
- retained `core/config` application schema split for later domain phases;
- retained `core/logger` forwarding-only compatibility facade with an 84-package non-growth ceiling; its implementation/state live in platform and owning domain phases replace facade calls with local logger ports;
- retained `core/metrics`, `infra/auth`, and `infra/httpx` with explicit later owners from the inventory;
- retained, growth-frozen `internal/crawler` and mixed domain persistence implementations, with their current coupling documented and their product/marketplace/organization owning phases named;
- remaining legacy business packages that directly consume relocated platform/integration APIs, listed package-by-package with a non-growth rule and owning phase; documentation must state that only the nine target domain roots satisfy the final dependency rule at phase-2 close;
- MCP/pgvector/TigerBeetle still not admitted.

- [ ] **Step 5: Run focused architecture and dependency verification**

```powershell
go test ./tests -run 'Test(Phase2|TargetDomains|Platform|Integration|Shared|.*Depguard.*)' -count=1
golangci-lint run ./internal/platform/... ./internal/integration/... ./internal/app/...
git diff --check
```

Expected: PASS.

- [ ] **Step 6: Run full repository verification**

```powershell
go test ./tests -count=1
go test ./... -count=1
```

Expected: PASS. If an environment-dependent integration test cannot run, record the exact package, test, command, error, and environment prerequisite; do not report the repository suite as passing.

- [ ] **Step 7: Run the post-suite filesystem guard**

```powershell
go test ./tests -run 'Test(InternalPackagesContainNoLocalArtifacts|ProductionEntrypointsContainNoLocalArtifacts|HackSupportAreasContainNoLocalArtifacts|ToolsContainNoLocalArtifacts)$' -count=1 -v
git status --short
```

Expected: guards PASS and no runtime artifacts appear. Only the planned documentation changes remain before commit.

- [ ] **Step 8: Commit phase closure**

```powershell
git diff --check
git add .golangci.yml tests docs internal/platform/README.md internal/integration/README.md
git commit -m "docs(architecture): close runtime foundation phase"
git status --short
git log --oneline --decorate -15
```

Expected: clean working tree and a reviewable sequence of one commit per task.
