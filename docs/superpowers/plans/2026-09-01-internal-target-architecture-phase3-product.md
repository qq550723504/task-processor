# 阶段三产品域目标架构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在一个最终 PR 内建立唯一的产品事实、来源、丰富化、资产和图片能力边界，令 ImageAgent 成为唯一图片工作流，并删除旧 ProductEnrich/ProductImage 任务体系及五个旧产品根目录。

**Architecture:** 先建立可单独验证的目标契约，再按 Catalog/Sourcing、Asset、Enrichment、Image 的顺序迁移纯领域能力；具体 GORM、Crawler、OpenAI、GRSAI、HTTP 图片和 S3 实现留在 Integration，运行时只由 App 装配。ListingKit、SDS、AmazonListing 改成读取 `ProductSnapshot` 和 `ApprovedAssetInventory`，ImageAgent 负责图片计划、执行、恢复、审批以及批准资产提交。

**Tech Stack:** Go 1.25、Gin、GORM、Goose、Temporal、golangci-lint/depguard、Testify、OpenAPI、pnpm 11、TypeScript 6。

**Spec:** `docs/superpowers/specs/2026-09-01-internal-target-architecture-phase3-product-design.md`

## Global Constraints

- 最终 PR 必须删除 `internal/catalog`、`internal/asset`、`internal/imageasset`、`internal/productenrich`、`internal/productimage`，且不得保留转发包、Deprecated Facade、类型别名兼容层或双写。
- `internal/product/{catalog,sourcing,enrichment,asset,image}` 不得导入 `internal/app`、`internal/platform`、`internal/integration`、GORM、Temporal、Redis、RabbitMQ 或 Provider SDK。
- `internal/product/asset` 不得导入 `internal/product/image`；候选图片到批准资产的映射由 ImageAgent 完成。
- ImageAgent 是唯一产品图片任务、计划、预算、重试、恢复和审批所有者；不得新增 ProductImage Queue、Worker、Task 或 HTTP API。
- 本阶段不实现 ProductAgent；Enrichment 只产生无持久化副作用的 `Proposal`。
- ListingKit、SDS、AmazonListing 只能读取 `ProductSnapshot` 和 `ApprovedAssetInventory`；缺失时返回明确的未就绪错误，禁止选择来源图或第一张图兜底。
- 生产装配缺少数据库、ImageAgent 能力、Artifact Store 或 Asset Repository 时必须失败；内存实现只能由测试显式构造。
- 停止代码读写 `product_enrich_tasks`、`product_image_tasks`，但不得在本 PR 中物理删表。
- `internal/pipeline` 保持现状且禁止增长；它的 TEMU/Marketplace 拆分属于阶段四。
- 现有 `internal/product` 根包不是规范产品域：其中抓取/缓存运行时迁入 `internal/marketplace/sourceproduct`，跨平台筛选规则迁入 `internal/marketplace/productpolicy`；最终 `internal/product` 根目录只包含五个目标子包和说明文档。
- 每个任务遵循红—绿—重构顺序；每次提交前运行该任务列出的聚焦测试和 `git diff --check`。

---

## 文件与职责映射

| 目标 | 文件/目录 | 职责 |
|---|---|---|
| 产品事实 | `internal/product/catalog/{snapshot.go,trace.go,normalize.go,errors.go}` | 规范 `ProductSnapshot`、来源证据、确定性归一化 |
| 来源交接 | `internal/product/sourcing/{source_envelope.go,source_identity.go,source_request.go,normalize.go}` | Provider-neutral 来源身份、证据、lineage、warnings |
| 丰富化 | `internal/product/enrichment/{model.go,ports.go,proposer.go,validation.go,scoring.go,errors.go}` | 只读输入、Proposal、验证与评分 |
| 资产事实 | `internal/product/asset/{model.go,inventory.go,repository.go,approval.go,errors.go}` | 已批准资产、血缘、幂等审批提交 Port |
| 图片能力 | `internal/product/image/{model.go,ports.go,errors.go,heuristics.go,scene.go}` | Provider-neutral 图片输入、候选与窄能力 Port |
| 资产持久化 | `internal/integration/persistence/product/asset/{model.go,repository.go,repository_contract_test.go}` | GORM Adapter、tenant scope、审批幂等性 |
| 来源 Adapter | `internal/integration/crawler/{a1688,amazon}/product_source.go`、`internal/sds/adapter/product_source.go` | 将具体来源结构转换成 `SourceEnvelope` |
| 图片 Adapter | `internal/integration/{openai,grsai,httpimage}/*product_image*.go` | 实现 `product/image` 定义的能力接口 |
| ImageAgent 接线 | `internal/imageagent/tools/product_image_executor.go`、`internal/imageagent/assetpublication/publisher.go` | 能力执行、错误映射、批准资产原子提交 |
| App 装配 | `internal/app/worker/imageagent/{capabilities.go,dependencies.go}`、`internal/app/httpapi/*` | 生产依赖检查、ImageAgent Worker/API 装配、旧模块退役 |
| 消费方 | `internal/listingkit/*`、`internal/sds/*`、`internal/amazonlisting/*` | 只读 Snapshot/Approved Inventory，不触发工作流 |
| 架构护栏 | `tests/target_architecture_phase3_product_test.go`、`.golangci.yml`、`tests/depguard_config_test.go` | 旧目录消失、依赖方向、单一编排所有者 |

---

### Task 1: 建立阶段三迁移护栏和基线

**Files:**
- Create: `tests/target_architecture_phase3_product_test.go`
- Modify: `tests/depguard_config_test.go`
- Modify: `.golangci.yml`
- Modify: `docs/refactoring/phase2-runtime-inventory.md`

**Interfaces:**
- Consumes: `tests/import_scan_test.go` 中现有 AST/import 扫描帮助函数。
- Produces: `TestPhase3ProductTargetDependencies`、`TestPhase3LegacyProductRootsDoNotGrow`、`TestPhase3PipelineDoesNotGrow`；最终任务会把 legacy growth test 收紧为目录不存在。

- [ ] **Step 1: 写目标依赖和增长基线测试**

```go
func TestPhase3ProductTargetDependencies(t *testing.T) {
	for _, name := range []string{"catalog", "sourcing", "enrichment", "asset", "image"} {
		root := filepath.Join("..", "internal", "product", name)
		if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
			continue
		}
		assertNoBannedImportPrefixes(t, root, []string{
			"task-processor/internal/app", "task-processor/internal/platform",
			"task-processor/internal/integration", "gorm.io/gorm",
			"go.temporal.io", "github.com/redis", "github.com/rabbitmq",
		}, nil)
	}
}

func TestPhase3LegacyProductRootsDoNotGrow(t *testing.T) {
	want := map[string]int{"catalog": 6, "asset": 31, "imageasset": 1, "productenrich": 74, "productimage": 92}
	for root, max := range want {
		if got := productionGoFileCount(t, filepath.Join("..", "internal", root)); got > max {
			t.Errorf("internal/%s production files = %d, baseline max = %d", root, got, max)
		}
	}
}
```

基线只统计生产 `.go` 文件；先用 `rg --files` 重新确认当前数字并把实测值写入测试，不猜测测试文件数量。

- [ ] **Step 2: 运行测试确认当前依赖缺陷被准确捕获**

Run: `go test ./tests -run 'TestPhase3(ProductTargetDependencies|LegacyProductRootsDoNotGrow|PipelineDoesNotGrow)' -count=1 -v`

Expected: growth tests PASS；依赖测试仅报告目标子包当前存在的具体违规，不把 `internal/product` 根包误算为目标子包。

- [ ] **Step 3: 配置只覆盖五个目标子包的 depguard 规则**

```yaml
phase3_product_domain_boundaries:
  files:
    - "**/internal/product/catalog/**/*.go"
    - "**/internal/product/sourcing/**/*.go"
    - "**/internal/product/enrichment/**/*.go"
    - "**/internal/product/asset/**/*.go"
    - "**/internal/product/image/**/*.go"
  deny:
    - pkg: task-processor/internal/app
    - pkg: task-processor/internal/platform
    - pkg: task-processor/internal/integration
    - pkg: gorm.io/gorm
    - pkg: go.temporal.io
```

在 `tests/depguard_config_test.go` 逐项断言五个 glob 和禁用包存在。更新阶段二 inventory，记录 `internal/product` 根包将按职责拆到 Marketplace，而不是成为新 Catalog。

- [ ] **Step 4: 运行护栏测试**

Run: `go test ./tests -run 'TestPhase3|TestDepguard' -count=1`

Expected: PASS。

- [ ] **Step 5: 提交护栏**

```powershell
git add .golangci.yml tests/target_architecture_phase3_product_test.go tests/depguard_config_test.go docs/refactoring/phase2-runtime-inventory.md
git diff --cached --check
git commit -m "test(architecture): guard phase 3 product boundaries"
```

---

### Task 2: 清空旧 `internal/product` 根包的错误所有权

**Files:**
- Create: `internal/marketplace/sourceproduct/doc.go`
- Move: `internal/product/{cache_manager.go,cache_manager_test.go,data_parser.go,product_fetcher.go,product_fetcher_test.go,source_request.go,source_request_test.go,source_request_boundary_test.go,types.go,validator.go}` → `internal/marketplace/sourceproduct/`
- Create: `internal/marketplace/productpolicy/doc.go`
- Move: `internal/product/{filter_rule.go,fulfillment.go,price_helper.go,price_helper_test.go,rule_checker.go}` → `internal/marketplace/productpolicy/`
- Modify: all exact importers returned by `rg -l '"task-processor/internal/product"' internal tests --glob '*.go'`
- Modify: `tests/target_architecture_phase3_product_test.go`

**Interfaces:**
- Consumes: `sourcing.AmazonCrawlerSource` and legacy marketplace `model.Product` without changing behavior.
- Produces: `sourceproduct.FetchRequest`、`sourceproduct.ProductFetcher`、`productpolicy.FilterRule` and pricing/filter helpers; no root `package product` remains.

- [ ] **Step 1: 写根包消失和职责包依赖测试**

```go
func TestPhase3ProductRootContainsNoGoPackage(t *testing.T) {
	entries, err := filepath.Glob(filepath.Join("..", "internal", "product", "*.go"))
	require.NoError(t, err)
	require.Empty(t, entries)
}
```

为 `marketplace/sourceproduct` 保留现有 fetch/cache 测试，为 `marketplace/productpolicy` 保留价格、库存和规则测试；测试包名与生产包名同步修改。

- [ ] **Step 2: 运行测试确认根包仍存在**

Run: `go test ./tests -run TestPhase3ProductRootContainsNoGoPackage -count=1 -v`

Expected: FAIL，列出 `internal/product/*.go`。

- [ ] **Step 3: 按职责移动文件并一次性更新调用方**

```go
// internal/marketplace/sourceproduct/doc.go
// Package sourceproduct owns legacy marketplace source fetch/cache execution.
// It is not the canonical product domain and must not be imported by internal/product/*.
package sourceproduct

// internal/marketplace/productpolicy/doc.go
// Package productpolicy owns marketplace screening and price/inventory policy.
package productpolicy
```

所有调用点直接改成新包名；不得在 `internal/product` 留 alias。更新 `tests/target_architecture_phase2_test.go` 中旧 import 字符串和目标说明。

- [ ] **Step 4: 运行迁移包和调用方测试**

Run: `go test ./internal/marketplace/sourceproduct ./internal/marketplace/productpolicy ./internal/crawler/fetcher ./internal/processor ./internal/temu/... ./internal/shein/... ./internal/app/bootstrap/... ./internal/app/runner/... ./tests -run 'TestPhase3ProductRootContainsNoGoPackage|TestFetch|TestPrice|TestInventory|TestRule' -count=1`

Expected: PASS，且 `rg -n '"task-processor/internal/product"' internal tests --glob '*.go'` 返回零结果。

- [ ] **Step 5: 提交所有权修复**

```powershell
git add internal/product internal/marketplace internal/crawler internal/processor internal/temu internal/shein internal/app tests
git diff --cached --check
git commit -m "refactor(product): move marketplace runtime out of product root"
```

---

### Task 3: 迁移 Catalog 并建立 `ProductSnapshot`

**Files:**
- Move: `internal/catalog/` → `internal/product/catalog/`
- Rename: `internal/product/catalog/model.go` → `snapshot.go`
- Rename: `internal/product/catalog/from_canonical.go` → `normalize.go`
- Create: `internal/product/catalog/errors.go`
- Modify: every file returned by `rg -l 'task-processor/internal/catalog' internal tests --glob '*.go'`
- Modify: `tests/import_scan_test.go`
- Test: `internal/product/catalog/{model_test.go,from_canonical_test.go,boundary_guard_test.go}` renamed with production files

**Interfaces:**
- Consumes: `catalog/canonical.Product` only inside Catalog normalization.
- Produces: `catalog.ProductSnapshot` and `catalog.Normalize(*canonical.Product) (*ProductSnapshot, error)`.

- [ ] **Step 1: 把现有 Catalog 测试改成目标名称**

```go
func TestNormalizeProducesDeterministicSnapshot(t *testing.T) {
	input := &canonical.Product{Title: "Bottle", Attributes: map[string]canonical.Attribute{
		"material": {Value: "steel"}, "color": {Value: "black"},
	}}
	first, err := Normalize(input)
	require.NoError(t, err)
	second, err := Normalize(input)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, []string{"color", "material"}, []string{first.Attributes[0].Name, first.Attributes[1].Name})
}
```

- [ ] **Step 2: 运行目标包测试确认新路径尚不存在**

Run: `go test ./internal/product/catalog/... -count=1`

Expected: FAIL，目标包不存在。

- [ ] **Step 3: 移动包并公开 Snapshot 契约**

```go
var ErrInvalidSnapshot = errors.New("invalid product snapshot")

type ProductSnapshot struct {
	Title          string
	Brand          string
	CategoryPath   []string
	Description    string
	SellingPoints  []string
	SEOKeywords    []string
	Attributes     []Attribute
	Specifications *Specifications
	Variants       []Variant
	Images         []Image
	Review         *ReviewState
	Sources        []SourceRecord
}

func Normalize(product *canonical.Product) (*ProductSnapshot, error)
```

保留 JSON 字段契约，所有 map 转 slice 和 source 收集必须排序。直接更新调用方为 `product/catalog` 和 `ProductSnapshot`；不得留下 `type Product = ProductSnapshot`。

- [ ] **Step 4: 运行 Catalog 与全体直接调用方测试**

Run: `go test ./internal/product/catalog/... ./internal/product/sourcing/... ./internal/listingkit/... ./internal/amazonlisting/... ./internal/publishing/... ./internal/marketplace/... ./tests -count=1`

Expected: PASS；`rg -n 'task-processor/internal/catalog' internal tests --glob '*.go'` 返回零结果。

- [ ] **Step 5: 提交 Catalog 迁移**

```powershell
git add internal/catalog internal/product/catalog internal/listingkit internal/amazonlisting internal/publishing internal/marketplace internal/product/sourcing tests
git diff --cached --check
git commit -m "refactor(product): establish canonical product catalog"
```

---

### Task 4: 纯化 Sourcing 并把具体来源转换移到 Adapter

**Files:**
- Modify: `internal/product/sourcing/{source_envelope.go,source_identity.go,source_request.go,source_result.go,doc.go}`
- Create: `internal/product/sourcing/normalize.go`
- Move: `internal/product/sourcing/{a1688_scraped_data.go,a1688_snapshot.go,a1688_source_envelope.go,a1688_source_result.go}` → `internal/integration/crawler/a1688/product_source.go`
- Move: `internal/product/sourcing/{amazon_crawl_requests.go,amazon_source_envelope.go,amazon_source_fetcher.go,amazon_source_platform.go}` → `internal/integration/crawler/amazon/product_source.go`
- Move: `internal/product/sourcing/sdspod/` → `internal/sds/adapter/product_source/`
- Modify: `internal/product/sourcing/catalog_asset_handoff.go`
- Test: move corresponding source-specific tests with their adapters

**Interfaces:**
- Consumes: source adapters create `sourcing.SourceEnvelope`.
- Produces: `sourcing.Normalize(SourceEnvelope) (SourceEnvelope, error)` and `sourcing.ToSnapshot(SourceEnvelope) (catalog.ProductSnapshot, error)`; Sourcing no longer imports `internal/model`、ProductEnrich、Asset 或具体 Crawler。

- [ ] **Step 1: 写证据保留和禁止具体依赖测试**

```go
func TestNormalizePreservesEvidenceLineageAndWarnings(t *testing.T) {
	in := SourceEnvelope{
		Identity: SourceIdentity{SourceType: " AMAZON ", SourceID: " B001 "},
		RawReference: RawSourceReference{ReferenceID: "raw-1", Checksum: "sha256:abc"},
		Warnings: []SourceWarning{{Code: " Missing_Title ", Field: " title ", Message: " missing "}},
		Trace: SourceTrace{SourceRunID: "run-1", Notes: []string{"crawler evidence"}},
	}
	out, err := Normalize(in)
	require.NoError(t, err)
	require.Equal(t, "missing_title", out.Warnings[0].Code)
	require.Equal(t, in.RawReference.Checksum, out.RawReference.Checksum)
	require.Equal(t, in.Trace.SourceRunID, out.Trace.SourceRunID)
}
```

- [ ] **Step 2: 运行测试确认当前具体依赖违规**

Run: `go test ./internal/product/sourcing/... ./tests -run 'TestNormalizePreservesEvidenceLineageAndWarnings|TestPhase3ProductTargetDependencies' -count=1 -v`

Expected: FAIL，违规只来自现有 source-specific 文件和旧 Asset/ProductEnrich import。

- [ ] **Step 3: 移动 Adapter 并收紧归一化入口**

```go
func Normalize(in SourceEnvelope) (SourceEnvelope, error) {
	out := in.Normalize()
	if !out.Identity.Valid() {
		return SourceEnvelope{}, ErrSourceIdentityRequired
	}
	return out, nil
}

func ToSnapshot(in SourceEnvelope) (catalog.ProductSnapshot, error) {
	normalized, err := Normalize(in)
	if err != nil {
		return catalog.ProductSnapshot{}, err
	}
	return catalog.ProductSnapshot{
		Title: normalized.ProductCandidate.Title,
		Brand: normalized.ProductCandidate.Brand,
		Description: normalized.ProductCandidate.Description,
		Sources: []catalog.SourceRecord{{Type: normalized.Identity.SourceType, Detail: normalized.RawReference.ReferenceID}},
	}, nil
}
```

`catalog_asset_handoff.go` 只产生 Catalog Snapshot；资产候选保留在 Envelope，后续由 ImageAgent 授权目录 Adapter 转换，不再让 Sourcing import Asset。

- [ ] **Step 4: 运行领域和 Adapter 测试**

Run: `go test ./internal/product/sourcing ./internal/integration/crawler/a1688 ./internal/integration/crawler/amazon ./internal/sds/adapter/... ./tests -run 'TestPhase3ProductTargetDependencies|Test.*Source' -count=1`

Expected: PASS；`go list -f '{{join .Imports "\n"}}' ./internal/product/sourcing` 不含 `internal/model`、`internal/integration`、`internal/productenrich`、`internal/asset`。

- [ ] **Step 5: 提交 Sourcing 纯化**

```powershell
git add internal/product/sourcing internal/integration/crawler internal/sds/adapter tests
git diff --cached --check
git commit -m "refactor(product): isolate source envelopes from adapters"
```

---

### Task 5: 建立批准资产领域契约

**Files:**
- Create: `internal/product/asset/{model.go,inventory.go,approval.go,repository.go,errors.go}`
- Create/adapt from: `internal/asset/{facts.go,inventory.go,inventory_test.go,model.go}`；旧文件保留到 Task 13，避免尚未迁移的 ListingKit 在中间提交断编译
- Create/adapt from: `internal/asset/{policy,recipe}/` → `internal/product/asset/{policy,recipe}/`；不创建跨路径 alias
- Create: `internal/product/asset/assettest/memory_repository.go`
- Test: `internal/product/asset/{approval_test.go,repository_contract_test.go,boundary_guard_test.go}`

**Interfaces:**
- Consumes: ImageAgent 后续提交 `ApprovalCommit`；消费方读取 `ApprovedAssetInventory`。
- Produces: `asset.Repository.CommitApproval`、`asset.Repository.GetApprovedInventory`、`assettest.NewMemoryRepository()` 和 `assettest.ExerciseRepositoryContract(t, factory)`。

- [ ] **Step 1: 写 tenant、幂等性和未就绪契约测试**

```go
type RepositoryFactory func(t *testing.T) asset.Repository

func ExerciseRepositoryContract(t *testing.T, factory RepositoryFactory) {
	t.Helper()
	repo := factory(t)
	commit := asset.ApprovalCommit{
		TenantID: "tenant-a", ProductKey: "product-1", ActionID: "approve-1",
		Assets: []asset.ApprovedAsset{{
			ID: "asset-1", RunID: "run-1", PlanRevision: 2, SlotID: "main", Attempt: 1,
			Role: asset.RoleMain, URL: "https://cdn.example/asset-1.png",
		}},
	}
	first, err := repo.CommitApproval(context.Background(), commit)
	require.NoError(t, err)
	second, err := repo.CommitApproval(context.Background(), commit)
	require.NoError(t, err)
	require.Equal(t, first, second)
	inventory, err := repo.GetApprovedInventory(context.Background(), asset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
	require.NoError(t, err)
	require.Len(t, inventory.Assets, 1)
}
```

- [ ] **Step 2: 运行测试确认目标契约尚不存在**

Run: `go test ./internal/product/asset/... -count=1`

Expected: FAIL，目标包或类型不存在。

- [ ] **Step 3: 实现不可变批准资产模型和 Repository Port**

```go
var ErrApprovedAssetsNotReady = errors.New("approved product assets are not ready")

type InventoryScope struct { TenantID, ProductKey string }

type Role string

const (
	RoleDesign Role = "design"
	RoleMain Role = "main"
	RoleWhiteBackground Role = "white_background"
	RoleGallery Role = "gallery"
)

type ApprovedAsset struct {
	ID, RunID, SlotID, URL, SourceAssetID string
	Role Role
	PlanRevision int64
	Attempt int
	Width, Height int
	Operations []string
}

type ApprovalCommit struct {
	TenantID, ProductKey, ActionID string
	Assets []ApprovedAsset
}

type ApprovalReceipt struct { ActionID string; AssetIDs []string }

type ApprovedAssetInventory struct {
	Scope InventoryScope
	Assets []ApprovedAsset
}

type Repository interface {
	CommitApproval(context.Context, ApprovalCommit) (ApprovalReceipt, error)
	GetApprovedInventory(context.Context, InventoryScope) (ApprovedAssetInventory, error)
}
```

校验唯一身份至少覆盖 tenant、run、revision、slot、attempt、action；Inventory 只含批准资产。测试内存实现和导出的契约测试函数放在 `assettest`，生产代码不得 import 它；架构测试扫描所有非 `_test.go` 生产文件并拒绝 `internal/product/asset/assettest` import。

- [ ] **Step 4: 运行 Asset 领域测试**

Run: `go test ./internal/product/asset/... ./tests -run 'TestPhase3ProductTargetDependencies|Test.*Asset' -count=1`

Expected: PASS；`rg -n 'product/image|productimage|gorm.io' internal/product/asset --glob '*.go'` 返回零结果。

- [ ] **Step 5: 提交 Asset 契约**

```powershell
git add internal/product/asset tests
git diff --cached --check
git commit -m "feat(product): define approved asset inventory"
```

---

### Task 6: 把 Asset GORM 实现迁入 Integration

**Files:**
- Create: `internal/integration/persistence/product/asset/{model.go,repository.go,repository_contract_test.go}`
- Modify: `internal/listingkit/schema/runtime.go`
- Modify: `internal/app/schema/productlisting/runtime.go`
- Modify: `internal/app/schema/productlisting/runtime_test.go`

**Interfaces:**
- Consumes: `product/asset.Repository` from Task 5.
- Produces: `assetpersistence.NewRepository(*gorm.DB) (asset.Repository, error)`；唯一索引实现审批幂等性。

- [ ] **Step 1: 用同一契约测试 GORM Adapter**

```go
func TestRepositoryContract(t *testing.T) {
	assettest.ExerciseRepositoryContract(t, func(t *testing.T) productasset.Repository {
		db, err := gorm.Open(sqlite.Open("file:"+url.QueryEscape(t.Name())+"?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		require.NoError(t, db.AutoMigrate(&ApprovedAssetRecord{}, &ApprovalReceiptRecord{}))
		repo, err := NewRepository(db)
		require.NoError(t, err)
		return repo
	})
}
```

增加跨 tenant 查询返回 `ErrApprovedAssetsNotReady`、同一 action 不同 payload 返回 `ErrApprovalConflict`、事务失败不产生半批记录的测试。

- [ ] **Step 2: 运行测试确认 Adapter 不存在**

Run: `go test ./internal/integration/persistence/product/asset -count=1`

Expected: FAIL，包不存在。

- [ ] **Step 3: 实现记录模型和原子提交**

```go
type ApprovedAssetRecord struct {
	TenantID string `gorm:"primaryKey;size:128"`
	RunID string `gorm:"primaryKey;size:128"`
	PlanRevision int64 `gorm:"primaryKey"`
	SlotID string `gorm:"primaryKey;size:128"`
	Attempt int `gorm:"primaryKey"`
	ActionID string `gorm:"primaryKey;size:128"`
	AssetID string `gorm:"uniqueIndex:ux_product_approved_asset_id;size:128"`
	ProductKey string `gorm:"index:ix_product_approved_inventory,priority:2;size:128"`
	PayloadJSON []byte `gorm:"type:json"`
}

type ApprovalReceiptRecord struct {
	TenantID string `gorm:"primaryKey;size:128"`
	ActionID string `gorm:"primaryKey;size:128"`
	PayloadHash string `gorm:"size:64;not null"`
	AssetIDsJSON []byte `gorm:"type:json;not null"`
}
```

`CommitApproval` 在一个事务中锁定/读取 action receipt、比较 canonical payload hash、插入全部资产和 receipt。`GetApprovedInventory` 必须同时过滤 tenant 和 product key。

- [ ] **Step 4: 更新 schema 但不删除旧表**

`listingkit/schema.AutoMigrateRuntime` 和 `app/schema/productlisting.AutoMigrateRuntime` 在本任务只新增批准资产表；旧 Asset generation snapshot 仍服务尚未迁移的 ListingKit，Task 16 再移除其新建依赖。不得调用 `DropTable`。运行：

Run: `go test ./internal/integration/persistence/product/asset ./internal/listingkit/schema ./internal/app/schema/productlisting -count=1`

Expected: PASS；schema 测试证明新库创建批准资产表；旧 task 表依赖的最终移除由 Task 16 验证。

- [ ] **Step 5: 提交持久化迁移**

```powershell
git add internal/integration/persistence/product/asset internal/listingkit/schema internal/app/schema/productlisting
git diff --cached --check
git commit -m "refactor(persistence): move product assets behind domain port"
```

---

### Task 7: 定义无副作用的 Enrichment Proposal

**Files:**
- Create: `internal/product/enrichment/{model.go,ports.go,proposer.go,validation.go,scoring.go,errors.go}`
- Move/adapt tests from: `internal/productenrich/{validator_test.go,validator_extra_test.go,result_validator_test.go,scorer_test.go,strategy_test.go,suggester_test.go}`
- Test: `internal/product/enrichment/{proposer_test.go,immutability_test.go,boundary_guard_test.go}`

**Interfaces:**
- Consumes: `catalog.ProductSnapshot`、`sourcing.SourceEnvelope`、显式 `PolicySnapshot`。
- Produces: `enrichment.Proposer.Propose(context.Context, Request) (Proposal, error)`；不暴露 Provider、Task、Repository 或重试类型。

- [ ] **Step 1: 写输入不变和 Proposal 证据测试**

```go
func TestProposeDoesNotMutateSnapshot(t *testing.T) {
	snapshot := catalog.ProductSnapshot{Title: "Bottle"}
	before, err := json.Marshal(snapshot)
	require.NoError(t, err)
	generator := candidateGeneratorFunc(func(context.Context, GenerationRequest) (Candidate, error) {
		return Candidate{Changes: []FieldChange{{Field: "description", Value: "Steel bottle", EvidenceIDs: []string{"raw-1"}}}}, nil
	})
	proposer, err := NewProposer(Dependencies{Generator: generator})
	require.NoError(t, err)
	source := sourcing.SourceEnvelope{RawReference: sourcing.RawSourceReference{ReferenceID: "raw-1"}}
	proposal, err := proposer.Propose(context.Background(), Request{Snapshot: snapshot, Source: source, Policy: PolicySnapshot{Version: "v1"}})
	require.NoError(t, err)
	after, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
	require.Equal(t, "raw-1", proposal.Changes[0].EvidenceIDs[0])
}

type candidateGeneratorFunc func(context.Context, GenerationRequest) (Candidate, error)

func (f candidateGeneratorFunc) Generate(ctx context.Context, req GenerationRequest) (Candidate, error) {
	return f(ctx, req)
}
```

- [ ] **Step 2: 运行测试确认新契约不存在**

Run: `go test ./internal/product/enrichment -count=1`

Expected: FAIL，目标包不存在。

- [ ] **Step 3: 实现 Proposal 和窄生成 Port**

```go
type Proposer interface { Propose(context.Context, Request) (Proposal, error) }

type CandidateGenerator interface {
	Generate(context.Context, GenerationRequest) (Candidate, error)
}

type Request struct {
	Snapshot catalog.ProductSnapshot
	Source sourcing.SourceEnvelope
	Policy PolicySnapshot
}

type Proposal struct {
	Changes []FieldChange
	Evidence []Evidence
	Quality QualityScore
	Validation ValidationResult
	Warnings []Warning
	Rejections []Rejection
}
```

`NewProposer` 必须拒绝 nil Generator；验证顺序固定为输入校验、生成、证据校验、评分、输出校验。任何失败都只返回稳定领域错误。

- [ ] **Step 4: 运行 Enrichment 测试**

Run: `go test ./internal/product/enrichment/... ./tests -run 'TestPhase3ProductTargetDependencies|TestPropose' -count=1`

Expected: PASS，且包中没有 GORM、Gin、Logrus、OpenAI 或 queue import。

- [ ] **Step 5: 提交 Enrichment 契约**

```powershell
git add internal/product/enrichment internal/productenrich tests
git diff --cached --check
git commit -m "feat(product): define enrichment proposals"
```

---

### Task 8: 提取 Enrichment 实现并删除任务语义

**Files:**
- Create/adapt into `internal/product/enrichment/` from `internal/productenrich/{failure.go,generator.go,governed_scoring.go,llm_score_cache.go,llm_scorer_prompt.go,parser.go,pipeline.go,quality_scoring_metadata.go,response.go,result_validator.go,scorer.go,strategy.go,suggester.go,understanding.go,url_validation.go,validation_cache.go,validator.go,variant.go}` and their unit tests
- Create/adapt into `internal/product/enrichment/` from `internal/productenrich/enrich/{category_path.go,generator_json.go,identity_errors.go,parser.go,source_backed_product_json.go,variant.go,variant_scraped.go}` and their unit tests
- Create/adapt into the OpenAI adapter from `internal/productenrich/{llm_adapter.go,llm_mock.go,llm_scorer.go}` and `internal/productenrich/enrich/{generator.go,governed_execution.go,image_governance.go,prompt_templates.go,text_governance.go,understanding.go}`
- Create/adapt `internal/integration/crawler/a1688/product_enrichment_input.go` from `internal/productenrich/enrich/scraper_adapter.go`
- Create: `internal/integration/openai/product_enrichment_adapter.go`
- Create: `internal/integration/openai/product_enrichment_adapter_test.go`
- Retain unchanged until Task 16: `internal/productenrich/{api,httpapi,pipeline,store}/` and root Task/Service runtime files, because ListingKit/AmazonListing/App still compile against them in this intermediate commit
- Do not modify: `internal/app/httpapi/runtime_productenrich.go`; production ProductAgent composition is intentionally not added

**Interfaces:**
- Consumes: Task 7 `CandidateGenerator`。
- Produces: OpenAI Adapter 实现 `enrichment.CandidateGenerator`；领域包保留验证、评分、Prompt-neutral 解析规则。

- [ ] **Step 1: 为 Adapter 写 Provider 错误隔离测试**

```go
func TestProductEnrichmentAdapterDoesNotExposeProviderTypes(t *testing.T) {
	adapter := NewProductEnrichmentAdapter(stubTextInvoker{output: `{"description":"Steel bottle"}`})
	got, err := adapter.Generate(context.Background(), enrichment.GenerationRequest{Prompt: "describe"})
	require.NoError(t, err)
	require.Equal(t, "Steel bottle", got.Changes[0].Value)
}
```

增加 Provider 失败映射为 `enrichment.ErrExternalCapabilityUnavailable`、非法 JSON 映射为 `enrichment.ErrOutputValidation` 的测试。

- [ ] **Step 2: 运行迁移测试确认旧实现仍耦合运行时**

Run: `go test ./internal/product/enrichment ./internal/integration/openai -run 'TestProductEnrichment|TestPropose' -count=1`

Expected: FAIL，新 Adapter 尚不存在。

- [ ] **Step 3: 复制/抽取纯逻辑并隔离 Provider Adapter**

保留的算法按职责拆入 `validation.go`、`scoring.go`、`parser.go`、`prompt.go`；OpenAI invocation、capability routing 和配置解析全部留在 Integration/App。新目标包不得定义 `GenerateRequest`、`Task`、`TaskResult`、`ProductService`、`TaskSubmitter`、Redis fallback 或 worker processor；旧包等消费方切换后由 Task 16 一次删除。

- [ ] **Step 4: 运行领域、Integration 和旧符号扫描**

Run: `go test ./internal/product/enrichment/... ./internal/integration/openai ./internal/app/httpapi -count=1`

Run: `rg -n 'CreateGenerateTask|product_enrich_tasks|internal/productenrich/(api|httpapi|pipeline|store)' internal/product internal/integration --glob '*.go'`

Expected: tests PASS；扫描在目标 Product 和 Integration 树中返回零结果；旧运行时只存在于尚待切换的旧包/App/消费方。

- [ ] **Step 5: 提交 Enrichment 提取**

```powershell
git add internal/product/enrichment internal/integration/openai internal/integration/crawler/a1688
git diff --cached --check
git commit -m "refactor(product): remove enrichment task runtime"
```

---

### Task 9A: 建立纯 Product Image 能力

**Files:**
- Create: `internal/product/image/{model.go,ports.go,errors.go,heuristics.go,scene.go}`
- Create/adapt pure tests from `internal/productimage/*_test.go`
- Create/adapt into `internal/product/image/` from `internal/productimage/{cleanup_heuristics.go,generation_metadata_maps.go,inspection_heuristics.go,ip_risk.go,readable_source.go,scene_generation_metadata.go,scene_layout.go,scene_options.go,scene_preset_resolver.go,scene_profile.go,scene_prompt_resolver.go,scene_request_context.go,selling_point_content.go,selling_point_draw_output.go,selling_point_draw_preview_executor.go,selling_point_fill_input.go,selling_point_layout.go,selling_point_metadata.go,selling_point_render_blocks.go,selling_point_render_output.go,selling_point_render_output_layout.go,selling_point_render_plan.go,selling_point_slots.go}` and their unit tests
- Create/adapt task-free types from `internal/productimage/domain/{model.go,scene_options.go}`；omit Task、TaskStatus、request persistence and SQL scanner methods
- Retain only until Task 16: `internal/productimage/{lifecycle.go,model_fallback_policy.go,pipeline.go,pipeline_degradation.go,subject_fallback.go}`；none of these files may be copied into the target package
- Copy/adapt: `internal/productimage/presets/scene_profiles.yaml` → `internal/product/image/presets/scene_profiles.yaml`；旧资源随 Task 16 删除
- Test: `internal/product/image/{ports_test.go,heuristics_test.go,scene_test.go,boundary_guard_test.go}`

**Interfaces:**
- Consumes: Provider-neutral `ProductContext` and source assets.
- Produces: `SubjectExtractor`、`WhiteBackgroundRenderer`、`SceneRenderer`、`Reviewer` and `UsageQuoter` ports; no task/result/publisher types.

- [ ] **Step 1: 写能力边界和禁止降级测试**

```go
func TestSceneRendererRejectsSourcePassThrough(t *testing.T) {
	renderer := NewSceneCapability(stubSceneBackend{assets: []Asset{{URL: "https://source.example/a.png", Operations: []string{"pass_through"}}}})
	_, err := renderer.RenderScene(context.Background(), SceneRequest{Source: Asset{URL: "https://source.example/a.png"}})
	require.ErrorIs(t, err, ErrOutputValidation)
}

type stubSceneBackend struct { assets []Asset }

func (s stubSceneBackend) Render(context.Context, SceneRequest) ([]Asset, error) {
	return append([]Asset(nil), s.assets...), nil
}
```

- [ ] **Step 2: 运行测试确认目标包不存在**

Run: `go test ./internal/product/image/... -count=1`

Expected: FAIL，目标包不存在。

- [ ] **Step 3: 实现无工作流状态的模型和 Port**

```go
type Asset struct {
	URL, SourceURL, SourceAssetID string
	Role Role
	Width, Height int
	Operations []string
}

type ProductContext struct {
	ProductKey, Title, ProductType string
	Attributes map[string]string
}

type SubjectExtractor interface { Extract(context.Context, ExtractRequest) (Candidate, error) }
type WhiteBackgroundRenderer interface { RenderWhiteBackground(context.Context, RenderRequest) (Candidate, error) }
type SceneRenderer interface { RenderScene(context.Context, SceneRequest) ([]Candidate, error) }
type Reviewer interface { Review(context.Context, ReviewRequest) (Review, error) }
```

删除 `TaskStatus`、`ImageProcessRequest`、`ImageProcessResult`、`ReviewTaskRequest`、Repository、Publisher 和生命周期状态。纯算法不得读取环境变量或配置单例。

- [ ] **Step 4: 运行 Product Image 领域测试**

Run: `go test ./internal/product/image/... ./tests -run 'TestPhase3ProductTargetDependencies|Test.*Image' -count=1`

Expected: PASS；`go list -f '{{join .Imports "\n"}}' ./internal/product/image` 不含 App/Platform/Integration/Provider SDK。

- [ ] **Step 5: 提交图片领域能力**

```powershell
git add internal/product/image tests
git diff --cached --check
git commit -m "feat(product): define provider-neutral image capabilities"
```

---

### Task 9B: 建立数据驱动的 Marketplace Image Policy Resolver

> **修订约束（2026-09-02）：** 本任务不迁移 `internal/productimage/marketplace_profile.go` 的关键词匹配、词形处理、category 推断或平台 `switch`。这些逻辑把分类职责藏在图片策略中，并要求在 Go 代码里持续穷举业务词汇，属于目标架构缺陷。历史实现提交 `0244ff8a8`、`b685f1c91` 及其后未提交修订只作为迁移证据，不作为本任务目标实现。

**Files:**
- Replace: `internal/marketplace/imagepolicy/product_image_profile.go` → `internal/marketplace/imagepolicy/{model.go,resolver.go,validation.go}`
- Replace: `internal/marketplace/imagepolicy/product_image_profile_test.go` → `internal/marketplace/imagepolicy/{resolver_test.go,boundary_guard_test.go}`
- Do not modify: `internal/product/image/`；Task 9A 已完成且保持 Marketplace-neutral

**Interfaces:**
- Consumes: 调用方已经确定的结构化 `Marketplace`、`Country`、`Family`、`SceneCategory` 精确键，以及 App 注入的不可变 `PolicySet`。
- Produces: 与精确键关联的 review thresholds 和 `product/image.SceneOptions`；非法输入返回稳定的 `ErrInvalidProfileInput`，合法但找不到策略时返回稳定的 `ErrPolicyNotFound`。
- Does not consume: `ProductType`、Title、自由文本、旧配置单例、环境变量、文件路径或 Provider SDK。

```go
type PolicyKey struct {
	Marketplace   string
	Country       string
	Family        string
	SceneCategory string
}

type Thresholds struct {
	MainReview            float64
	WhiteBackgroundReview float64
	WhiteCanvasPenalty    float64
}

type Policy struct {
	Key           PolicyKey
	Thresholds    Thresholds
	SceneDefaults productimage.SceneOptions
}

type PolicySet struct {
	Version  string
	Policies []Policy
}

type ProfileInput struct {
	Marketplace   string
	Country       string
	Family        string
	SceneCategory string
}

type Resolver struct { /* constructor-owned immutable exact-key index */ }

func NewResolver(PolicySet) (*Resolver, error)
func (r *Resolver) Resolve(ProfileInput) (ProductImageProfile, error)
```

`NewResolver` 必须先完整校验再复制输入：version 非空；PolicySet 非空且条目数、字符串和总字节数有明确上限；键为规范 ASCII identifier；键不得重复；所有阈值为有限数且在 `[0,1]`；`SceneDefaults` 通过 `product/image` 的公开校验边界。构造完成后修改原始 slice、字符串引用或 `StyleReferenceIDs` 不得改变 Resolver 行为。

Resolver 只做已经规范的精确键查询；输入含首尾空白、大小写不规范或非法字符时直接拒绝，不在 Resolver 中 trim 或 fold。它不实现 fallback precedence、通配符、默认平台、包含匹配、tokenization、stemming、单复数、Unicode 兼容折叠或 category 推断。若业务需要一个“默认”策略，调用方必须显式传入例如 `Family: "default"` 的完整键，并且该键必须真实存在于注入数据中；代码不得隐式降级。

- [ ] **Step 1: 写通用 Resolver 契约测试**

测试使用两个任意、互不相关的结构化策略 fixture，证明：

1. 精确键分别返回对应阈值和完整 `SceneOptions`；
2. 未知 family/category/marketplace/country 返回 `ErrPolicyNotFound`，不会落入另一条策略；
3. 重复键、非法键、空集合、非有限/越界阈值和非法 SceneOptions 在构造阶段失败；
4. 构造前后的调用方 mutation 不会修改 Resolver；并发 Resolve 无数据竞争；
5. 边界守卫固定 `ProfileInput` 的四个结构化字段、禁止生产 package 声明 builtin `PolicySet`、禁止词法/词形依赖、`os`/文件读取和旧配置包。代码评审的硬性拒绝项是任何平台/category/lexeme 业务表或推断分支。

测试不得复制生产策略全集，也不得按 marketplace/category 穷举断言；它只验证通用解析契约。

- [ ] **Step 2: 运行测试确认旧实现不满足新契约**

Run: `go test ./internal/marketplace/imagepolicy -count=1`

Expected: FAIL，直到旧的全局函数、关键词推断和内置平台/category 分支被替换。

- [ ] **Step 3: 实现不可变精确键 Resolver**

删除 `ResolveProductImageProfile` 全局入口、`ProductType` 输入、所有 lexeme/family/category 推断和 hard-coded platform defaults。生产包不得提供 builtin PolicySet，也不得自行加载 YAML/JSON、环境变量或项目配置。

- [ ] **Step 4: 运行 Policy 单元测试和依赖守卫**

Run: `go test -race ./internal/marketplace/imagepolicy -count=1`

Run: `go list -f '{{join .Imports "\n"}}' ./internal/marketplace/imagepolicy`

Expected: PASS；依赖只包含标准库和 `internal/product/image`，不包含 App、Platform、Integration、旧配置或旧 ProductImage。

- [ ] **Step 5: 提交通用 Resolver**

```powershell
git add internal/marketplace/imagepolicy
git diff --cached --check
git commit -m "refactor(marketplace): resolve image policy from injected data"
```

---

### Task 10: 把具体图片实现放入 Integration 并改接 ImageAgent Tool

**Files:**
- Create: `internal/integration/openai/product_image_adapter.go`
- Create: `internal/integration/grsai/product_image_adapter.go`
- Create: `internal/integration/httpimage/product_image_adapter.go`
- Create: `internal/integration/policy/productimage/{catalog.go,catalog_test.go,policies.yaml}`
- Create: `internal/app/worker/imageagent/capabilities.go`
- Create: `internal/app/worker/imageagent/image_policy.go`
- Modify: `internal/app/worker/imageagent/dependencies.go`
- Modify: `internal/app/worker/imageagent/dependencies_test.go`
- Modify: `internal/imageagent/{model.go,ports.go,service.go,slot_effect.go}` and paired tests
- Modify: `internal/imageagent/httpapi/{handler.go,handler_test.go,dto.go}`
- Rename/Modify: `internal/imageagent/tools/productimage_executor.go` → `internal/imageagent/tools/product_image_executor.go`
- Modify: `internal/imageagent/tools/productimage_executor_test.go`
- Modify: `internal/imageagent/temporal/{activities.go,manual_acceptance_test.go,slot_effect_v3_activity_test.go}`
- Create/adapt OpenAI-facing adapters from `internal/productimage/{default_model_provider.go,governed_scene_generator.go,llm_review_model.go,model_provider.go,model_review_assessor.go,model_scene_renderer.go,model_subject_extractor.go,model_white_background_renderer.go,openai_image_edit_adapter.go,openai_image_editor.go,openai_scene_generator.go,prompt_templates.go,remote_faithful_editor.go,remote_scene_generator.go,scene_client.go}`
- Create/adapt HTTP/segmentation adapters from `internal/productimage/{background_client.go,image_edit_client.go,segmenter_client.go}`
- Create/adapt App-only composition from `internal/productimage/{capability_usage.go,component_helpers.go,default_components.go,real_components.go,scene_renderer.go,tenant_model_gate.go,usage_quote.go}` and `internal/productimage/httpapi/{ai_capability_scene_catalog.go,governed_model_invocations.go,image_agent_capabilities.go,image_pipeline_component_builder.go,model_provider_builder.go,scene_governance_builder.go,tenant_gated_model.go}`
- Retain all old ProductImage files until ListingKit、SDS、AmazonListing complete Tasks 13–15；Task 16 performs the single deletion

**Interfaces:**
- Consumes: Task 9A image ports、Task 9B `Resolver`、ImageAgent `SlotExecutionInput`/budget contracts，以及 run-scoped 的结构化 `ImagePolicyContext`。
- Produces: `imageagenttools.NewProductImageSlotExecutor(Dependencies)`；图片能力依赖保持为 `product/image` ports，策略依赖为消费方定义的窄 `ProfileResolver` 接口。App 构造具体 adapters 和 Resolver，缺少生产依赖或策略时失败。
- Policy data source: `internal/integration/policy/productimage/policies.yaml` 是唯一版本化策略目录；专用 Integration loader 使用仓库已有的 `gopkg.in/yaml.v3` 严格解码为不依赖 Marketplace 的 typed `Catalog` DTO。App composition 是唯一同时依赖该 Catalog 与 `imagepolicy` 的边界，并把 Catalog 映射为 `imagepolicy.PolicySet` 后构造 Resolver。它不是通用配置框架，不读取旧 `config.Config`、环境变量或配置单例。

> **执行切片裁决（2026-09-02）：** 实施审查实测本任务列出的旧 Provider/App 生产文件约 4,569 行，尚未包含 ImageAgent、Temporal、HTTP 和 Worker 改造，不能作为一个超过 1,500 行准入阈值的单一开发单元执行。目标接口和最终验收不变，但按依赖顺序拆为独立、始终可编译并分别复审的切片：10A 专用策略 Catalog Adapter；10B Run/HTTP/Store/Temporal 的结构化 PolicyContext；10D1 OpenAI Adapter；10D2 HTTP Image/GRSAI Adapter；10D3 App-only provider composition；10C Executor 与 Worker 一次性切换到 `product/image` 和精确 Resolver；10E fail-closed 装配收尾及 Task 10 全量清理。10B 后的实施审查确认，若在 provider adapters 和 App composition 之前切 Executor，唯一能维持编译的方法是新增临时 legacy bridge/constructor；这与禁止兼容债务冲突，因此先完成可独立编译的 adapters，再原子切换消费方。旧 `productimage` 只可在尚未迁移的切片中暂存，不能增加兼容入口或 fallback；10E 完成时一次性满足本任务的 scoped import 验收。

- [x] **Step 1: 改写 Executor 契约测试**

```go
func TestExecutorUsesProductImagePortsAndRejectsFallbackCandidate(t *testing.T) {
	executor := NewProductImageSlotExecutor(Dependencies{SceneRenderer: stubSceneRenderer{
		candidates: []productimage.Candidate{{Asset: productimage.Asset{URL: "https://source.example/a.png", Operations: []string{"pass_through"}}}},
	}})
	input := imageagent.SlotExecutionInput{
		RunID: "run-1", TenantID: "tenant-a", UserID: "user-a", PlanRevision: 1, Attempt: 1, IdempotencyKey: "attempt-1",
		Slot: imageagent.Slot{ID: "scene-1", Role: imageagent.SlotRoleScene, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-1"},
		AssetCatalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/a.png", SourceURL: "https://source.example/a.png"}}},
	}
	_, err := executor.ExecuteSlot(context.Background(), input)
	require.ErrorIs(t, err, imageagent.ErrValidation)
}

type stubSceneRenderer struct { candidates []productimage.Candidate }

func (s stubSceneRenderer) RenderScene(context.Context, productimage.SceneRequest) ([]productimage.Candidate, error) {
	return append([]productimage.Candidate(nil), s.candidates...), nil
}
```

`productimage` alias in测试代码必须指向 `internal/product/image`，不是旧包。

另写契约测试证明 Executor 只把 immutable `Run.TargetPlatform` 作为 Marketplace，并与 `ImagePolicyContext` 的 Country/Family/SceneCategory 组成精确键传给 Resolver；它不读取 `ProductType`、Title 或 Attributes 推断策略。Resolver 返回 `ErrPolicyNotFound` 时 slot 明确失败。Catalog loader 测试使用独立小型 YAML fixture 验证 strict fields、单文档、schema version、资源上限和 PolicySet 交接，不在 Go 测试里镜像 `policies.yaml` 的业务全集。

- [x] **Step 2: 运行 Executor 测试确认仍引用旧 ProductImage**

Run: `go test ./internal/imageagent/tools ./internal/app/worker/imageagent -count=1`

Expected: FAIL，直到 Tool 和生产装配切换完成。

- [x] **Step 3: 迁移 Adapter 与 App 组合**

```go
type ImagePolicyContext struct {
	Country       string
	Family        string
	SceneCategory string
}

type ProfileResolver interface {
	Resolve(imagepolicy.ProfileInput) (imagepolicy.ProductImageProfile, error)
}

type ImageCapabilities struct {
	SubjectExtractor productimage.SubjectExtractor
	WhiteBackgroundRenderer productimage.WhiteBackgroundRenderer
	SceneRenderer productimage.SceneRenderer
	Reviewer productimage.Reviewer
	UsageQuoter productimage.UsageQuoter
	ProfileResolver ProfileResolver
}

func buildImageCapabilities(deps providerDependencies, resolver ProfileResolver) (ImageCapabilities, error)
```

专用策略 Catalog 的公开边界固定为基础设施 DTO，不能直接 import Marketplace 业务域：

```go
// LoadEmbedded 严格解析 package 内 go:embed 的 policies.yaml。
func LoadEmbedded() (Catalog, error)

// Decode 只用于同一 Adapter 的确定性 fixture 测试和 LoadEmbedded 复用；
// 调用方不能传运行时文件路径。
func Decode(io.Reader) (Catalog, error)
```

`internal/app/worker/imageagent` 负责 `Catalog -> imagepolicy.PolicySet -> imagepolicy.NewResolver`。这是依赖反转边界，不得通过给 Infrastructure→Marketplace 增加 import guard 例外来规避。

`policies.yaml` 使用单一、带版本的 typed schema，不允许 map-of-maps 或未声明字段：

```yaml
schema: product-image-policy/v1
policies:
  - marketplace: marketplace-a
    country: xx
    family: family-a
    scene_category: category-a
    thresholds:
      main_review: 0.60
      white_background_review: 0.70
      white_canvas_penalty: 0.10
    scene_defaults:
      scene_category: category-a
      scene_style: studio
      background_tone: neutral
      composition: centered
      props_level: none
      audience_hint: general
```

示例只说明 schema，不规定只支持该条目。业务条目只在 YAML 中维护；Go 代码和测试都不得复制该目录。

`ImagePolicyContext` 是显式 run input：由创建 ImageAgent Run 的上游用结构化业务事实填写，并与唯一的 `Run.TargetPlatform` 一起持久化、参与 Start 幂等比较和 `SlotExecutionFingerprint`。HTTP `createRunRequest` 必须显式接收 `target_platform` 和 `image_policy_context`，Service 在解析 AssetCatalog 或启动 workflow 前验证它们；不从 BusinessTask、ProductType、Title 或 Attributes 回填。`SlotExecutionInput` 携带 TargetPlatform 与 PolicyContext，Temporal activity 只透传。Task 10 不负责发明 family/category 分类器；缺失字段或无精确策略均 fail closed。

为保持已有 ImageAgent Temporal history 可重放，新增的 PolicyContext 字段使用可省略的新 wire 字段：新 Run ingress 必填，历史 payload 缺失时 replay 仍按旧记录完成且不会调度新的无策略 effect；不得为新执行增加旧 ProductImage fallback。增加 replay fixture 证明旧 history 的 command payload 不变，并证明新 history 的 policy key 已进入 effect fingerprint。

生产构造必须逐项验证非 nil。策略装配顺序固定为 `integration catalog.Load` → App 映射 typed Catalog 为 `imagepolicy.PolicySet` → `imagepolicy.NewResolver` → 作为 `ProfileResolver` 注入 Executor；任一步失败都阻止 worker 启动。Loader 只解析嵌入的版本化策略资源，不接受运行时路径，不调用历史配置加载器，也不提供代码内 fallback。Provider 运行参数由各 Integration Adapter 的 typed constructor 显式接收，再以 typed dependencies 传入；本函数不接收 `config.Config`。`ProductImageConfig` 中仍被 ImageAgent 使用的 workdir、model、publisher 配置在 Task 16 改名为 `ImageAgentConfig`；本任务不得把图片策略接入该历史配置对象。

- [x] **Step 4: 运行 ImageAgent Tool、Temporal replay 和 Worker 装配测试**

Run: `go test ./internal/integration/policy/productimage ./internal/imageagent/httpapi ./internal/imageagent/tools ./internal/imageagent/temporal ./internal/app/worker/imageagent -count=1`

Expected: PASS；已有 Temporal history 的序列化命令保持不变，新 Run 的显式 policy key 进入新 effect fingerprint，生产 Go 文件不再 import `internal/productimage`。

- [x] **Step 5: 提交 ImageAgent 能力接线**

```powershell
git add internal/integration/openai internal/integration/grsai internal/integration/httpimage internal/integration/policy/productimage internal/imageagent internal/app/worker/imageagent
git diff --cached --check
git commit -m "refactor(imageagent): execute product image capabilities directly"
```

---

### Task 11: 让 ImageAgent 原子提交批准资产

**Files:**
- Create: `internal/imageagent/assetpublication/publisher.go`
- Create: `internal/imageagent/assetpublication/publisher_test.go`
- Modify: `internal/imageagent/ports.go`
- Modify: `internal/imageagent/model.go`
- Modify: `internal/app/worker/imageagent/dependencies.go`
- Move/adapt behavior from: `internal/listingkit/httpapi/image_agent_approved_publisher.go`
- Delete: `internal/listingkit/httpapi/image_agent_approved_publisher.go` and its tests after migration

**Interfaces:**
- Consumes: ImageAgent projection、`product/asset.Repository`、durable public URL resolver.
- Produces: `assetpublication.NewPublisher(projections, assets, publicURLs)` implementing `imageagent.ApprovedAssetPublisherV3`；批准不再修改 ListingKit task JSON。

- [x] **Step 1: 写批准前不写、重复批准不重复写测试**

```go
func TestPublisherCommitsApprovedAssetsExactlyOnce(t *testing.T) {
	repo := assettest.NewMemoryRepository()
	projection := approvedV3Projection(t)
	publisher, err := NewPublisher(staticProjectionSource{projection: projection}, repo, staticPublicURLResolver{})
	require.NoError(t, err)
	input := approvedV3PublicationInput(projection)
	first, err := publisher.PublishApprovedV3(context.Background(), input)
	require.NoError(t, err)
	second, err := publisher.PublishApprovedV3(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, first, second)
	inventory, err := repo.GetApprovedInventory(context.Background(), productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
	require.NoError(t, err)
	require.Len(t, inventory.Assets, 2)
}
```

把现有 `approvedV3Projection`、`approvedV3PublicationInput`、`staticProjectionSource` 和 `staticPublicURLResolver` 测试帮助函数从被删除的 ListingKit publisher 测试原样迁入本测试文件，再把资产断言改成 `product/asset` 类型。

- [x] **Step 2: 运行测试确认新 Publisher 尚不存在**

Run: `go test ./internal/imageagent/assetpublication -count=1`

Expected: FAIL，包不存在。

- [x] **Step 3: 实现 Projection 校验和 ApprovalCommit 映射**

Publisher 必须验证 tenant/user/run/revision/result digest、候选批准状态、durable object identity、slot 和 attempt；以 ImageAgent `ActionID` 作为批准动作身份，使用 `ProductContextRef.ProductID` 作为 ProductKey。Repository 失败时返回错误，Temporal 不得把 Run 标记完成。

- [x] **Step 4: 运行 Publisher、Repository 和 Temporal 恢复测试**

Run: `go test ./internal/imageagent/assetpublication ./internal/integration/persistence/product/asset ./internal/imageagent/temporal ./internal/app/worker/imageagent -count=1`

Expected: PASS；ListingKit transaction repository 不再参与批准路径。

- [x] **Step 5: 提交批准资产所有权迁移**

```powershell
git add internal/imageagent internal/integration/persistence/product/asset internal/app/worker/imageagent internal/listingkit/httpapi
git diff --cached --check
git commit -m "feat(imageagent): publish approvals to product assets"
```

---

### Task 12: 将 ListingKit 产品流程改为只读 Snapshot

**Files:**
- Modify: `internal/listingkit/interfaces_dependencies.go`
- Modify: `internal/listingkit/service_config_groups.go`
- Modify: `internal/listingkit/service_config_prepare.go`
- Modify: `internal/listingkit/service_workflow_dependencies.go`
- Modify: `internal/listingkit/workflow_standard_canonical_phase.go`
- Modify: `internal/listingkit/canonical_product_cache.go`
- Modify: `internal/listingkit/standard_snapshot.go`
- Modify: `internal/listingkit/assembler.go`
- Modify: `internal/listingkit/httpapi/{bootstrap_contracts.go,bootstrap_service_config.go,runtime_builder.go}`
- Delete/update tests that stub `productenrich.ProductService`; add `internal/listingkit/product_snapshot_reader_test.go`

**Interfaces:**
- Consumes: Catalog `ProductSnapshot` from Task 3.
- Produces: ListingKit-local `ProductSnapshotReader.GetProductSnapshot(context.Context, ProductSnapshotQuery) (catalog.ProductSnapshot, error)`。

- [x] **Step 1: 写只读 Snapshot 和未就绪测试**

```go
type ProductSnapshotQuery struct { TenantID, ProductKey string }

type ProductSnapshotReader interface {
	GetProductSnapshot(context.Context, ProductSnapshotQuery) (catalog.ProductSnapshot, error)
}

type recordingSnapshotReader struct { snapshot catalog.ProductSnapshot; calls int }

func (r *recordingSnapshotReader) GetProductSnapshot(context.Context, ProductSnapshotQuery) (catalog.ProductSnapshot, error) {
	r.calls++
	return r.snapshot, nil
}

func TestCanonicalPhaseReadsSnapshotOnce(t *testing.T) {
	reader := &recordingSnapshotReader{snapshot: catalog.ProductSnapshot{Title: "Bottle"}}
	phase := standardWorkflowCanonicalPhase{snapshots: reader}
	got, err := phase.run(context.Background(), ProductSnapshotQuery{TenantID: "tenant-a", ProductKey: "product-1"})
	require.NoError(t, err)
	require.Equal(t, "Bottle", got.Title)
	require.Equal(t, 1, reader.calls)
}
```

增加 Snapshot 不存在时返回 `ErrProductSnapshotNotReady` 且 ListingKit task 进入明确 blocked/review 状态的测试。

- [x] **Step 2: 运行测试确认现有 workflow 仍创建 ProductEnrich task**

Run: `go test ./internal/listingkit -run 'TestCanonicalPhaseReadsSnapshotOnce|Test.*SnapshotNotReady' -count=1 -v`

Expected: FAIL。

- [x] **Step 3: 替换 ProductService 为读取 Port**

删除 `ProductService`、`resolveWorkflowProductService`、child product task ID、Create/Get/Process 调用和相关恢复语义。现有 canonical cache repository 可实现 ListingKit-local reader，但返回值必须先经 Catalog Normalize；不得调用 Enrichment。

- [x] **Step 4: 运行 ListingKit 核心和 HTTP bootstrap 测试**

Run: `go test ./internal/listingkit/... -run 'Test.*(Snapshot|Canonical|Workflow|Bootstrap)' -count=1`

Expected: PASS；`rg -n 'ProductService|CreateGenerateTask\(ctx.*productenrich|ProcessProduct|internal/productenrich' internal/listingkit --glob '*.go'` 返回零结果。

- [x] **Step 5: 提交 ListingKit Snapshot 读取迁移**

```powershell
git add internal/listingkit
git diff --cached --check
git commit -m "refactor(listingkit): read canonical product snapshots"
```

---

### Task 13: 删除 ListingKit 图片编排和 Asset Generation

**Files:**
- Modify: `internal/listingkit/interfaces_dependencies.go`
- Modify: `internal/listingkit/workflow_standard_asset_phase.go`
- Modify: `internal/listingkit/workflow_standard.go`
- Modify: `internal/listingkit/model_result.go`
- Modify: `internal/listingkit/standard_snapshot.go`
- Create: `internal/listingkit/approved_asset_reader_test.go`
- Delete: `internal/listingkit/{workflow_asset_generation_dispatch.go,workflow_platform_asset_dispatch_apply.go,workflow_platform_asset_dispatch_bundle_apply.go,workflow_platform_asset_dispatch_bundle_reshape.go,workflow_platform_asset_dispatch_persist.go,workflow_platform_asset_dispatch_phase.go,workflow_platform_asset_dispatch_task_merge.go}`
- Delete: `internal/listingkit/{task_generation_current_state_snapshot.go,task_generation_retry_mutation.go,task_generation_retry_persist.go,task_generation_retry_projection.go,task_generation_service.go,task_generation_tasks_read_snapshot.go}` and paired tests
- Delete: `internal/listingkit/{asset_generation_projection.go,asset_workflow_platform_support.go,generation_queue_tasks.go,generation_review_state.go,generation_task_list.go,model_generation_tasks.go}` and paired tests
- Delete: `internal/listingkit/{service_generation_actions_test.go,service_generation_navigation_dispatch_test.go,service_generation_queue_test.go,service_generation_retry_test.go,service_generation_tasks_test.go,service_generation_test.go,task_generation_service_test.go,workflow_assets_test.go,workflow_model_generation_test.go}`
- Delete: `internal/listingkit/generation/{asset_targets.go,retry_selection.go,summary_stats.go}`
- Delete: `internal/asset/{bundle,generation}/` and all paired tests after their ListingKit callers are removed
- Delete: `internal/asset/{from_productimage.go,from_productimage_test.go}`; ImageAgent publication now maps candidates directly to `product/asset.ApprovalCommit`
- Delete: `internal/asset/{policy,recipe,repository}/` and `internal/asset/{facts.go,inventory.go,inventory_test.go,model.go,boundary_guard_test.go}` after all imports switch to `internal/product/asset`
- Delete: `internal/imageasset/`; its only production file is an unreferenced compatibility context over the retired `internal/asset` model, so retaining it would keep the old aggregate alive without a consumer
- Modify: `internal/listing/preview/{attachment.go,projection_test.go,read_model_test.go,task_read_model_test.go}`、`internal/compatibility/listingkit/preview_adapter_test.go`、`internal/publishing/common/{helpers.go,types.go,selection_image_test.go,variant_fallback.go,variant_fallback_test.go}` and `internal/publishing/shein/{assembler.go,derived_refresh.go}` to consume approved asset values or consumer-local projections
- Modify to remove generation fields/calls: `internal/listingkit/{export_model.go,preview_model_shell.go,service_defaults.go,service_task_generation_support_helpers.go,service_task_layer_processing_helpers.go,service_task_layers_logic.go,service_task_wiring_support.go,service_types.go,service_workflow_dependencies.go,task_export_service.go,task_preview_service_support.go,workflow_platform_adaptation.go,workflow_platform_finalize_phase.go}`
- Delete: `internal/listingkit/api/studio_product_images_handler.go`
- Delete: `internal/listingkit/api/studio_product_image_usage_admission.go` and product-image-specific tests
- Delete: `internal/listingkit/httpapi/{studio_product_image_usage.go,studio_product_image_usage_test.go}` and remove the product-image ledger wiring from bootstrap
- Modify: `internal/listingkit/api/{studio_async_jobs_handler_entrypoints.go,studio_async_jobs_handler_runner.go}` to remove `/studio/product-images` while preserving design jobs
- Modify: `internal/listingkit/{service_studio_batch_wiring_support.go,service_studio_media_generation_entrypoints.go,task_studio_media_service.go}` to remove only product-image generation methods while preserving Studio design/media reads
- Modify: `internal/listingkit/httpapi/routes_descriptor_task.go` and route interfaces/tests
- Modify: `docs/api/listingkit-asset.openapi.yaml`

**Interfaces:**
- Consumes: `product/asset.ApprovedAssetInventory`。
- Produces: ListingKit-local `ApprovedAssetInventoryReader.GetApprovedInventory(context.Context, asset.InventoryScope) (asset.ApprovedAssetInventory, error)`；无生成/重试/审批接口。

**实施切片（按依赖顺序完成，不设置兼容层）：**

1. 13A：建立批准资产只读 Port，并让标准工作流及平台装配只消费 `ApprovedAssetInventory`。
2. 13B：删除 ListingKit 内的生成、重试、队列和执行状态。
3. 13C：删除 Studio 商品图生成 API、路由和用量账本，仅保留 design job 与媒体读取。
4. 13D：迁移 preview/publishing 等剩余投影，删除 `internal/asset` 与无消费者的 `internal/imageasset`。

13C 同时删除 Studio batch 内部“批准设计 → 生成商品图 → 创建 ListingKit 任务”的隐藏物化链路及其 task-link/usage 状态；否则公开 API 虽被删除，ListingKit 仍会继续充当商品图生成器。批量设计生成、审核、背景处理和媒体读取保留，未来任务创建只能消费 Product Snapshot 与 ImageAgent 已批准资产。

- [x] **Step 1: 写只消费批准资产且不回退来源图的测试**

```go
func TestWorkflowRequiresApprovedAssets(t *testing.T) {
	reader := approvedAssetReaderFunc(func(context.Context, productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
		return productasset.ApprovedAssetInventory{}, productasset.ErrApprovedAssetsNotReady
	})
	phase := standardWorkflowAssetPhase{approvedAssets: reader}
	_, err := phase.run(context.Background(), productasset.InventoryScope{TenantID: "tenant-a", ProductKey: "product-1"})
	require.ErrorIs(t, err, productasset.ErrApprovedAssetsNotReady)
}

type approvedAssetReaderFunc func(context.Context, productasset.InventoryScope) (productasset.ApprovedAssetInventory, error)

func (f approvedAssetReaderFunc) GetApprovedInventory(ctx context.Context, scope productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	return f(ctx, scope)
}
```

增加 inventory 只含 gallery、缺少批准 main 时明确未就绪的测试；不得断言“取第一张”。

- [x] **Step 2: 运行测试确认现有 Asset Generation 仍被调用**

Run: `go test ./internal/listingkit -run 'TestWorkflowRequiresApprovedAssets|Test.*ApprovedAsset' -count=1 -v`

Expected: FAIL。

- [x] **Step 3: 用批准资产投影替代生成状态**

ListingKit result 只投影批准资产及 role，不保存 `AssetGenerationTasks`、`PendingGeneration`、execution mode 或 ProductImage result。删除 `/api/v1/listing-kits/studio/product-images` 及异步 `/studio/product-images` 分支；ImageAgent API 是唯一生成入口。

- [x] **Step 4: 运行 ListingKit、OpenAPI 和扫描测试**

Run: `go test ./internal/listingkit/... ./internal/app/httpapi -count=1`

Run: `rg -n 'asset/generation|CreateProcessTask|ProcessImages|GenerateStudioProductImages|studio/product-images|internal/productimage' internal/listingkit docs/api/listingkit-asset.openapi.yaml --glob '*.go' --glob '*.yaml'`

Expected: tests PASS；扫描返回零结果。

- [x] **Step 5: 提交 ListingKit 图片编排退役**

```powershell
git add internal/listingkit docs/api/listingkit-asset.openapi.yaml
git diff --cached --check
git commit -m "refactor(listingkit): consume approved assets only"
```

---

### Task 14: 将 SDS 改为只读取批准资产

**Files:**
- Delete: `internal/sds/workflow/productimage.go`
- Create: `internal/sds/workflow/approved_asset.go`
- Modify: `internal/sds/workflow/{service.go,types.go,service_test.go}`
- Modify: `internal/sds/usecase/{service.go,types.go,service_test.go}`
- Modify: `internal/sds/adapter/{service.go,types.go,service_test.go}`
- Modify: `internal/sds/httpbootstrap/{support.go,support_test.go}`
- Modify: `internal/listingkit/workflow_sds_sync.go` and SDS service test stubs
- Create: `internal/listingkit/workflow_sds_sync_approved_asset_support.go`
- Delete: `internal/listingkit/{workflow_sds_sync_remote_support.go,workflow_sds_sync_uploaded_support.go}`
- Modify: `internal/app/httpapi/{runtime_support_listingkit.go,feature_builder_listingkit.go,types.go}` and runtime composition tests

**Interfaces:**
- Consumes: SDS-local `ApprovedAssetReader` 返回 `product/asset.ApprovedAssetInventory`。
- Produces: `SelectApprovedDesignAsset(inventory) (asset.ApprovedAsset, error)`，只接受批准且具有明确 design/main/white-background role 的资产。

ListingKit 必须把 `InventoryScope` 交给 SDS 用例，由 SDS 自己读取批准资产；不得继续把批准资产降级成裸 URL/本地文件后调用通用入口。App 只构造一个批准资产 Reader，并将同一实例装配给 ListingKit 与 SDS，避免新增无生产调用的 Reader 或重复数据库连接。

- [x] **Step 1: 写禁止第一张图兜底测试**

```go
func TestSelectApprovedDesignAssetRejectsUnassignedGallery(t *testing.T) {
	inventory := productasset.ApprovedAssetInventory{Assets: []productasset.ApprovedAsset{{ID: "gallery-1", Role: productasset.RoleGallery}}}
	_, err := SelectApprovedDesignAsset(inventory)
	require.ErrorIs(t, err, productasset.ErrApprovedAssetsNotReady)
}
```

- [x] **Step 2: 运行测试确认旧选择器仍接收 ProductImage result**

Run: `go test ./internal/sds/... -run 'TestSelectApprovedDesignAsset|Test.*ProductImage' -count=1 -v`

Expected: FAIL，新选择器不存在或旧测试仍依赖 ProductImage。

- [x] **Step 3: 替换输入契约并删除旧选择器**

优先级只能基于显式已批准 role：`design` → `main` → `white_background`；无匹配返回未就绪。删除 SDS 的 raw URL、本地文件和 ProductImage 入口，只保留批准资产 URL 的下载细节。SDS 不导入 ImageAgent store/Temporal 或 ProductImage。

- [x] **Step 4: 运行 SDS 测试和 import 扫描**

Run: `go test ./internal/sds/... -count=1`

Run: `rg -n 'internal/productimage|internal/imageagent/(store|temporal)|ImageProcessResult' internal/sds --glob '*.go'`

Expected: tests PASS；扫描返回零结果。

- [x] **Step 5: 提交 SDS 读取迁移**

```powershell
git add internal/sds
git diff --cached --check
git commit -m "refactor(sds): select approved product assets"
```

---

### Task 15: 删除 AmazonListing 未投产的旧编排依赖

**Files:**
- Modify: `internal/amazonlisting/interfaces.go`
- Modify: `internal/amazonlisting/workflow_listing.go`
- Modify: `internal/amazonlisting/publishing_assembler.go`
- Modify: `internal/amazonlisting/model_types.go`
- Modify: `internal/amazonlisting/httpapi/{bootstrap.go,runtime_builder.go}`
- Modify/delete tests: `internal/amazonlisting/{workflow_test.go,service_process_recovery_test.go,service_submit_test.go,service_task_test.go,assembler_test.go,review_items_test.go}`
- Modify: `internal/marketplace/amazon/workspace/review_items.go`
- Modify actual consumers/guards: `internal/listingkit/{assembler.go,interfaces_dependencies.go,service_defaults.go}`、`internal/app/httpapi/{feature_builder_amazonlisting.go,runtime_support_listingkit.go,e2e_test.go,phase2_boundary_test.go}`、`cmd/product-listing-api/main_test.go`
- Modify lifecycle contract exposed by fail-closed readers: `internal/amazonlisting/store/{mem_store.go,task_repo_contract_test.go}`
- Modify review-remediation paths: `internal/amazonlisting/{workflow_process_service.go,workflow_processor.go,workflow_processor_test.go}`、`internal/listingkit/{interfaces_dependencies.go,layer_temporal_contract.go,workflow.go,workflow_platform_adaptation.go,service_task_layers_logic.go,service_child_task_retry_helpers.go}`、`internal/listingkit/temporal/{activities_layers.go,workflow_layers.go,workflow_layers_test.go}`、`internal/listingkit/store/{mem_store.go,task_repo_status.go,task_repo_processing_failure_test.go}`、`internal/product/asset/assettest/memory_repository.go`、`internal/integration/persistence/product/asset/{model.go,repository.go,repository_contract_test.go}`、`internal/{listingkit/schema,app/schema/productlisting}/runtime_test.go`

> **实施补充（2026-09-02）：** 原文件清单遗漏了 Amazon assembler 的真实调用方和 App 装配。若只改 AmazonListing 包，ListingKit 会继续把 Snapshot 降级成 canonical 后调用旧签名，App 也会继续注入 ProductService/ImageService，因此本任务同步切换这些直接消费方，但不提前执行 Task 16 的旧模块全局删除。只读 Reader 未装配时还暴露了内存仓储 `MarkFailed` 不进入终态的问题；本任务以跨 GORM/内存实现的契约测试修复该状态机缺陷。复审又发现三项根因：自动重试会把失败任务重新打开且吞掉提交错误，ListingKit Amazon adapter 会吞掉 assembler error，批准资产的追加列表无法表达“当前批准版本”。本任务因此移除 Amazon 自动重试、逐层传播 ListingKit assembler error，并以不可变批准历史加原子 head 建模当前资产清单。终态持久化也纳入同一错误契约：Amazon 返回原始处理错误与 `MarkFailed` 错误的 error chain；ListingKit 的 Temporal layer activity 在业务重试耗尽后执行独立、可重试的失败持久化 activity，若落库仍失败则把处理错误与持久化错误同时返回，任务不会静默遗留在 `processing`。失败落库使用 repository CAS，只允许 `processing → failed`；若 Activity 已提交 `completed`/`needs_review` 但响应丢失，后续失败 Activity 按幂等成功处理，不会覆盖真实成功终态。
>
> Task 12 删除旧 canonical cache 后，仓库内没有 Product Snapshot 的生产 owner、writer 或 repository；仅构造一个读取器既无数据可读，也会形成新的临时兼容债务。因此本任务在生产组合中 fail closed：缺少真实 `ProductSnapshotReader` 或批准资产 Reader 时不注册 AmazonListing module，而不是暴露一个必然进入失败态的路由。直接注入 Reader 的模块级 E2E 继续覆盖完整成功路径。生产启用由 Task 15A 完成。

**Interfaces:**
- Consumes: AmazonListing-local `ProductSnapshotReader` 和 `ApprovedAssetInventoryReader`。
- Produces: assembler 直接接受 `catalog.ProductSnapshot` 和 `asset.ApprovedAssetInventory`；无 ProductEnrich/ProductImage task。

- [x] **Step 1: 写未就绪和只读装配测试**

```go
func TestBuildDraftRequiresSnapshotAndApprovedMainAsset(t *testing.T) {
	assembler := NewAssembler()
	_, err := assembler.Build(DraftInput{Snapshot: catalog.ProductSnapshot{Title: "Bottle"}})
	require.ErrorIs(t, err, productasset.ErrApprovedAssetsNotReady)
}
```

- [x] **Step 2: 运行测试确认旧服务依赖仍存在**

Run: `go test ./internal/amazonlisting/... -run 'TestBuildDraftRequiresSnapshotAndApprovedMainAsset|Test.*Workflow' -count=1 -v`

Expected: FAIL。

- [x] **Step 3: 删除 ProductService/ImageService 和 child task 流程**

`Assembler` 改成 `Build(DraftInput) (*AmazonListingDraft, error)`；未投产模块不保留旧 request/result JSON 兼容字段。HTTP bootstrap 仅装配读取 Port、Repository、Validator、Exporter 和 Amazon submitter。

- [x] **Step 4: 运行 AmazonListing 和 Marketplace 测试**

Run: `go test ./internal/amazonlisting/... ./internal/marketplace/amazon/... -count=1`

Run: `rg -n 'internal/(productenrich|productimage)|CreateProcessTask|ProcessImages|ProductService|ImageService' internal/amazonlisting internal/marketplace/amazon --glob '*.go'`

Expected: tests PASS；扫描返回零结果。原命令中的 `CreateGenerateTask` 会匹配 AmazonListing 自身的父任务入口，并不能证明 ProductEnrich 子任务已删除，因此改为扫描旧包 import、旧图片任务调用和旧服务依赖。

- [x] **Step 5: 提交 AmazonListing 依赖退役**

```powershell
git add internal/amazonlisting internal/marketplace/amazon internal/listingkit internal/app/httpapi cmd/product-listing-api docs/superpowers/plans/2026-09-01-internal-target-architecture-phase3-product.md
git diff --cached --check
git commit -m "refactor(amazonlisting): read product facts and approved assets"
```

---

### Task 15A: 建立 Product Snapshot 所有权和生产读写链路

**Files:**
- Modify: `internal/product/catalog/`，为 canonical Product Snapshot 定义 repository port、版本/身份与写入契约
- Create: `internal/integration/persistence/product/catalog/`，实现 snapshot repository 和 schema
- Modify: `internal/product/sourcing/` 与 Catalog 的显式发布用例边界，使结构化来源输入在确定性归一化后原子发布 Product Snapshot；Enrichment 保持只读且不写 Snapshot
- Modify: `internal/app/httpapi/`，通过 typed dependency 注入生产 `ProductSnapshotReader`
- Modify: `internal/app/schema/productlisting/` and tests
- Modify: `.golangci.yml`、`tests/import_boundaries_test.go`、`tests/depguard_config_test.go`、`docs/architecture/architecture-review-checklist.md`，移除阻止生产 persistence adapter 实现 Catalog-owned Port 的旧单条禁令，保留其他业务域禁令，并同步架构守卫清单

**Interfaces:**
- Product Catalog 是 Product Snapshot 的唯一 owner；写入方发布规范化、带 tenant/product 精确身份的不可变版本，读取方只能按精确身份读取当前版本。
- Persistence adapter 同时实现写 Port 与只读 Port；App 只负责 typed composition，不接收或复用历史 `config.Config` 加载器，不通过 ProductEnrich task/result、canonical cache 或 JSON 兼容字段中转。
- AmazonListing 和 ListingKit 只依赖窄 Reader；生产 module 只在 Snapshot reader 与批准资产 reader 都存在时注册。
- 本阶段不虚构新的 intake HTTP/Queue/Worker/ProductAgent。Catalog 暴露显式、结构化、可由未来 intake 调用的 Publisher；App 用现有数据库 typed dependency 组装其生产 Repository，并只把窄 Reader 交给消费方。现有入口若缺 tenant/product/发布幂等身份则失败关闭。

> **实施补充（2026-09-02）：** 既有 `infrastructure_business_boundaries` 把 `internal/product/catalog` 视为基础设施永远不得导入的业务实现，这与本任务批准的“Persistence adapter 实现 Catalog-owned Port”相冲突。Task 15A 只移除这一条旧 Catalog 禁令，并增加正负守卫覆盖；其他业务域禁令以及 Integration 间的依赖规则保持不变。

- [x] **Step 1: 写 repository contract 与生产组合失败/成功测试**

覆盖 tenant/product 隔离、原子发布、幂等重放、当前版本读取、无 snapshot 的稳定未就绪错误，以及 App 缺依赖不注册、有真实 reader 才注册。

- [x] **Step 2: 运行测试确认缺少生产所有者和 Reader**

Run: `go test ./internal/product/catalog/... ./internal/integration/persistence/product/catalog/... ./internal/app/httpapi -count=1`

Expected: FAIL，直到 owner、persistence、writer 与 typed composition 全部存在。

- [x] **Step 3: 实现 Product Snapshot owner、repository 和发布边界**

不得恢复已删除的 canonical cache，不得让 AmazonListing/ListingKit 或 Enrichment 写 snapshot，不得调用历史配置加载器或新增兼容 fallback。Catalog Publisher 接受精确 tenant/product/发布幂等身份和已经确定性归一化的 Snapshot；Sourcing 只负责把结构化 SourceEnvelope 转成 Snapshot 后调用该 Publisher。若现有 intake 尚无足够结构化身份，保留显式发布用例供未来入口调用并让现有入口失败关闭，不从自由文本或旧 task result 猜测字段，也不为本阶段新建临时入口或编排器。

- [x] **Step 4: 启用生产 Reader 并运行端到端验证**

Run: `go test ./internal/product/catalog/... ./internal/integration/persistence/product/catalog/... ./internal/app/httpapi ./cmd/product-listing-api -count=1`

Expected: PASS；测试先发布 snapshot 与批准资产，再通过生产组合生成 AmazonListing；不存在旧 ProductEnrich/ProductImage 编排或配置加载依赖。

- [x] **Step 5: 提交 Product Snapshot 生产链路**

```powershell
git add internal/product/catalog internal/product/sourcing internal/integration/persistence/product/catalog internal/app/httpapi internal/app/schema/productlisting cmd/product-listing-api docs/superpowers/specs/2026-09-01-internal-target-architecture-phase3-product-design.md docs/superpowers/plans/2026-09-01-internal-target-architecture-phase3-product.md
git diff --cached --check
git commit -m "feat(product): publish production product snapshots"
```

---

### Task 16A: 退役旧 Product HTTP、运行时装配和 task schema

> **实施拆分（2026-09-02）：** `internal/productenrich` 与 `internal/productimage` 合计约 18,906 行生产代码和 16,757 行测试代码，原 Task 16 同时要求删除两棵目录、重写 App、schema、配置及所有调用方，无法形成可审查的单元。Task 16 因此拆成三个连续、每步可编译的硬切：16A 先移除 App 入口和持久化接线，16B 再删除旧根目录，16C 最后把仍由 ImageAgent 使用的配置改到正确所有权；不增加任何中间兼容入口。

**Files:**
- Modify/delete: `internal/app/httpapi/` 中 ProductEnrich/ProductImage module、adapter、task repository、runtime、worker-pool 和 route 组合及对应测试
- Modify: `internal/app/schema/productlisting/runtime.go` and tests
- Modify: `cmd/product-listing-api/{main_test.go,wire_test.go,wrappers_test.go,adapters_test.go,README.md}`
- Modify: `internal/app/worker/imageagent/` dependency tests only when an acceptance gap exists

**Interfaces:**
- Consumes: ImageAgent HTTP module、Catalog/Approved Asset readers 和现有 production worker dependency resolver。
- Produces: App 启动不再构造、注册或持有 ProductEnrich/ProductImage module、queue、worker pool、task repository 或 task-table schema；五条旧 API 固定为 404。
- 本切片不改名配置，也不修改旧根目录内部实现；它只切断生产 App 所有权，确保下一切片可以直接删除旧包。

- [x] **Step 1: 写旧路由、module registry 和 schema 退役测试**

覆盖五条旧 API 返回 404、runtime registry 不含旧 module/worker pool、`AutoMigrateRuntime` 新数据库不创建 `product_enrich_tasks`/`product_image_tasks`。审计 `ResolveImageAgentTemporalDependencies` 已有数据库、Artifact Store、Image capabilities、Asset Repository 缺失测试；只为真实缺口补 RED，不写重复 mock 断言。

- [x] **Step 2: 运行测试确认旧 App 接线仍存在**

Run: `go test ./internal/app/httpapi ./internal/app/schema/productlisting ./internal/app/worker/imageagent ./cmd/product-listing-api -run 'TestLegacyProductRoutesAreNotRegistered|Test.*Legacy.*Module|Test.*Legacy.*Table|Test.*Dependencies' -count=1 -v`

Expected: FAIL，失败必须来自仍注册的旧路由/module/schema，而不是缺少测试 fixture。

- [x] **Step 3: 删除旧 App runtime 和 task persistence 接线**

删除 ProductEnrich/ProductImage HTTP module、adapter、runtime deps、task repository、queue/worker pool 组合和 `runtime_productenrich*`。从 `AutoMigrateRuntime` 移除两种旧 Task；不得加入 drop migration，既有生产表保持原状。ImageAgent、AmazonListing、ListingKit、SDS 和 Catalog 组合保持可用，禁止临时 facade 或 nil fallback。

- [x] **Step 4: 运行 App/schema/cmd 验证与零引用扫描**

Run: `go test ./internal/app/httpapi ./internal/app/schema/productlisting ./internal/app/worker/imageagent ./cmd/product-listing-api -count=1`

Run: `rg -n 'task-processor/internal/(productenrich|productimage)|product_enrich_tasks|product_image_tasks|product_enrich|product_image' internal/app/httpapi internal/app/schema/productlisting cmd/product-listing-api --glob '*.go'`

Expected: tests PASS；生产文件扫描零匹配，测试只能保留明确的退役断言文本。

- [x] **Step 5: 提交 App 运行时退役**

```powershell
git add internal/app/httpapi internal/app/schema/productlisting internal/app/worker/imageagent cmd/product-listing-api docs/superpowers/plans/2026-09-01-internal-target-architecture-phase3-product.md
git diff --cached --check
git commit -m "refactor(app): retire legacy product http runtimes"
```

---

### Task 16B: 删除 ProductEnrich/ProductImage 旧根目录

**Files:**
- Delete: `internal/productenrich/`
- Delete: `internal/productimage/`
- Delete: `hack/debug/test-analyzeimage/`
- Delete: `hack/debug/test-productenrich/`
- Modify: remaining tests/guards that import or enumerate the retired roots, including `internal/imageagent/temporal/slot_effect_v3_activity_test.go`、`internal/{marketplace/imagepolicy,product/image}/*boundary_guard_test.go` and `tests/*`

**Interfaces:**
- Consumes: Task 16A 已切断的生产 App 边界，以及 Tasks 7–10 已迁出的 `product/enrichment`、`product/image` 能力。
- Produces: 两个旧根目录、两个仅服务旧 runtime 的 debug 目录和全仓真实 Go import declaration 均不存在；absence/depguard 测试可以把旧路径作为禁用数据声明，但不得通过字符串拆分来规避扫描。测试不得复制旧 production 类型形成隐性兼容层。
- 历史 Temporal 测试若只需身份值，使用当前 ImageAgent/product-image contract 或测试本地值对象；不得保留 `internal/productimage` alias/package。

- [x] **Step 1: 把旧根与专用 legacy debug 入口的 absence/import 护栏写成 RED**

护栏检查目录不存在、全仓生产与测试 Go 文件没有旧 import，且 `internal/product/{enrichment,image}` 与 ImageAgent 的行为测试仍覆盖已迁移能力。

- [x] **Step 2: 运行护栏确认旧根和真实旧 import 仍存在**

Run: `go test ./tests -run 'TestPhase3.*Legacy|TestPhase3.*Import|Test.*Product.*Boundary' -count=1 -v`

Expected: FAIL，并精确指出两个旧根目录或 import。

- [x] **Step 3: 删除旧根、专用 legacy debug 入口并清理外部测试依赖**

直接删除两个旧根及 `hack/debug/test-{analyzeimage,productenrich}` 的全部文件；不得保留转发包、Deprecated facade、type alias、JSON 兼容结构、旧 task/result fixture 或为已退休 runtime 改接的新 debug 编排。只迁移仍验证当前架构的测试；纯旧运行时测试随目录删除。

- [x] **Step 4: 运行目标域、ImageAgent、架构和全仓编译验证**

Run: `go test ./internal/product/... ./internal/imageagent/... ./tests -run 'Test(Product|ImageAgent|Phase3|.*Import.*|.*Depguard.*)' -count=1`

Run: `go test ./... -run '^$' -count=1`

Run: `go test ./tests -run 'TestPhase3.*Legacy|TestPhase3.*Import|Test.*Product.*Boundary' -count=1 -v`

Run: 使用 `go/parser`/现有 `loadGoFileIndex` 扫描 `internal`、`cmd`、`tests`、`hack` 的 import declarations，拒绝 `task-processor/internal/productenrich`、`task-processor/internal/productimage` 及其子包；不要用原始文本 `rg` 误伤护栏中的禁用路径字面量。

Expected: tests/compile PASS；旧目录不存在；真实 import declaration 零匹配。护栏 fixture 中作为数据出现的旧路径仍可被测试识别并拒绝。

- [x] **Step 5: 提交旧根删除**

```powershell
git add -A internal/productenrich internal/productimage hack/debug/test-analyzeimage hack/debug/test-productenrich internal/imageagent internal/marketplace/imagepolicy internal/product/image internal/app/httpapi tests docs/refactoring/phase2-runtime-inventory.md docs/superpowers/plans/2026-09-01-internal-target-architecture-phase3-product.md
git diff --cached --check
git commit -m "refactor(product): remove legacy product task roots"
```

---

### Task 16C: 将图片运行配置收归 ImageAgent

**Files:**
- Rename/modify: `internal/core/config/type_productimage.go` → `type_imageagent.go`
- Modify: `internal/core/config/{config.go,defaults.go,loader.go,loader_builder.go,type_ai_capability.go,validator_ai_capability.go}` and tests
- Modify: `config/{config-test.yaml,config-dev.yaml,config-prod.yaml,worker.yaml}`
- Modify: `internal/app/worker/imageagent/`、`internal/app/httpapi/`、`cmd/image-agent-temporal-worker/` and tests
- Modify: `internal/listingkit/httpapi/` and ListingKit config only if retained manual-upload storage needs its own typed ownership

**Interfaces:**
- `Config.ImageAgent` 是唯一产品图片运行配置，只保留 ImageAgent 当前真实消费的 durable artifact publication/storage 字段。预检确认 WorkDir 和 Lifecycle 已无生产消费者，因此与旧 Segmenter/WhiteBackground/Scene 一并删除，不以“可能将来使用”为由保留死配置。
- 不接受 `productimage` YAML、`TASK_PROCESSOR_PRODUCTIMAGE_*`、`TASK_PROCESSOR_PRODUCTENRICH_*` 或旧字段 fallback/alias；保留通用 OpenAI capability routing 配置。
- ListingKit 不得读取 `Config.ImageAgent`。Studio 手工上传使用 `Config.ListingKit` 自己的显式 typed image-upload 配置/依赖；不得从旧 ProductImage 或新 ImageAgent 配置隐式继承、复制默认值或 fallback。选择 S3 时构造失败必须失败关闭，不得自动切到 local；local 只能由 ListingKit 配置显式选择。
- `aiCapability.studioImageRoutingMode` 暂不在本切片做孤立改名或强制 active；预检确认它控制的是 ListingKit 自有的直接 AI 生成/编辑链，而不是 ImageAgent。该整条工作流及配置在 Task 16E 原子删除，避免先维护一个马上废弃的过渡路由。

- [x] **Step 1: 写新配置所有权与旧键拒绝测试**

覆盖 YAML/env 只接受 `imageagent` 新键，旧 ProductImage/ProductEnrich 根、debug/capability 键及环境变量被明确拒绝而不是静默忽略；ImageAgent worker 使用新 typed 配置；ListingKit 不读取 ImageAgent 配置。测试使用独立字面 fixture，不镜像 loader 的字段表。

- [x] **Step 2: 运行测试确认仍依赖旧配置名**

Run: `go test ./internal/core/config ./internal/app/worker/imageagent ./internal/app/httpapi ./internal/listingkit/httpapi ./cmd/image-agent-temporal-worker -count=1`

Expected: FAIL，直到旧配置字段和调用方全部硬切。

- [x] **Step 3: 硬改名并删除旧配置语义**

更新 Go 类型、YAML、env binding 和所有真实调用方；不得双读、迁移 fallback 或保留 Deprecated 字段。旧 YAML/env 必须在 load preflight 明确报错，不能因为 Viper 未绑定而悄悄忽略。ListingKit 手工上传由 ListingKit-owned config/typed builder 组装，不能继续借用图片发布配置；S3 构造失败不得降级 local。

- [x] **Step 4: 运行配置/worker/App/ListingKit 验证和扫描**

Run: `go test ./internal/core/config ./internal/app/worker/imageagent ./internal/app/httpapi ./internal/listingkit/httpapi ./cmd/image-agent-temporal-worker ./cmd/product-listing-api -count=1`

Run: `rg -n 'ProductImageConfig|ProductImagePublisher|ProductEnrich(Text|Vision|Listing|Mock)|productimage:|productEnrich|productImageScene|TASK_PROCESSOR_PRODUCTIMAGE|TASK_PROCESSOR_PRODUCTENRICH' internal/core/config internal/app internal/listingkit/httpapi cmd config --glob '*.go' --glob '*.yaml' --glob '*.yml'`

Expected: tests PASS；扫描零匹配。

- [x] **Step 5: 提交配置所有权硬切**

```powershell
git add internal/core/config internal/app internal/listingkit/httpapi cmd config docs/superpowers/plans/2026-09-01-internal-target-architecture-phase3-product.md
git diff --cached --check
git commit -m "refactor(config): make imageagent own image runtime settings"
```

---

### Task 16D1: 删除 AmazonListing/ListingKit 生产仓储静默降级

> **实施补充（2026-09-02）：** Task 16A 的独立审查证明，AmazonListing 与 ListingKit 的 production builder 在缺少数据库时会自动选择内存 Task/支持仓储。该行为会让进程以非持久化状态继续运行，并使生产 E2E 假绿；它不是需要保留的兼容能力。测试内存实现可以继续作为显式 fixture，但 production composition 不得自动选择它们。
>
> **组合根因裁定（2026-09-02）：** ListingKit 当前把完整 `Config + Logger` 注入约 28 个 repository builder，再由每个 builder 决定 DB、memory 或 nil；这不是窄依赖注入，而是把降级决策分散到运行时。硬切后由 production composition 基于一个已验证的数据库配置/连接一次性组装 typed persistent repository set，并把实际 repository 接口值及其 ownership/closer 交给模块。`BuildService`/`BuildModule` 不再接收 repository factory bundle，也不在业务 bootstrap 内读取数据库配置。测试可显式注入 memory repository set，但 production runtime support 不得构造它。SourceAccount 不再借道 ListingKit 的通用 fallback helper，改由其自身 bootstrap/显式依赖负责。

**Files:**
- Modify: `internal/amazonlisting/httpapi/{bootstrap.go,runtime_builder.go}` and tests
- Modify: `internal/listingkit/httpapi/builders_repositories*.go`、bootstrap contracts/validation and tests
- Modify: `internal/app/httpapi/` production composition and tests
- Modify: `cmd/product-listing-api/README.md`

**Interfaces:**
- Production module construction requires explicitly assembled persistent repository dependencies or a valid production database configuration; missing persistence fails closed before route/pool registration.
- Existing in-memory repositories remain test utilities only. Production builders do not call `NewMem*` and do not use a generic `buildRepositoryWithFallback` path.
- New dependency seams accept narrow repository interfaces or typed groups. They do not accept the historical whole-project config loader, logger-backed factories, environment aliases, or nil/default fallback.
- 一个 production repository set 共享一个明确拥有者的数据库连接/生命周期；不得继续按 repository 重复打开连接并堆叠 closers。

- [x] **Step 1: 写 production fail-closed 与持久化身份测试**

覆盖 AmazonListing/ListingKit 缺 production persistence 时不注册 module/pool；显式注入 temporary SQLite production repositories 时真实 HTTP 流程可完成，并由 fresh connection/repository 复读。增加生产源码守卫，拒绝 production builder 选择 `NewMem*` 或 `buildRepositoryWithFallback`。

- [x] **Step 2: 运行测试确认当前静默内存降级**

Run: `go test ./internal/amazonlisting/httpapi ./internal/listingkit/httpapi ./internal/app/httpapi -run 'Test.*(Persistence|Repository|Fallback|FailClosed)' -count=1 -v`

Expected: FAIL，并精确证明缺数据库时仍注册模块或选择内存仓储。

- [x] **Step 3: 硬切 production repository composition**

删除 production 自动内存降级；把持久化依赖变成实际 repository 接口组成的窄 typed dependency。production composition 只打开一次持久化连接并构造完整 repository set，模块不再逐仓储执行 `func(*config.Config, *logrus.Logger)` 工厂。不得删除单元测试显式构造的 memory repositories，不得用 `config.Config` factory、Deprecated builder、alias 或环境默认值替代旧 fallback。

- [x] **Step 4: 运行消费者、App 与全仓验证**

Run: `go test ./internal/amazonlisting/... ./internal/listingkit/... ./internal/app/httpapi ./cmd/product-listing-api -count=1`

Run: `go test ./... -run '^$' -count=1`

- [x] **Step 5: 提交仓储 fail-closed 硬切**

```powershell
git add internal/amazonlisting/httpapi internal/listingkit/httpapi internal/app/httpapi cmd/product-listing-api docs/superpowers/plans/2026-09-01-internal-target-architecture-phase3-product.md
git diff --cached --check
git commit -m "refactor(product): fail closed without persistent consumer stores"
```

---

### Task 16D2: 删除 ListingKit 解析器的隐式 fallback

> **实施补充（2026-09-02）：** Task 16A 的 current-only E2E 还证明，缺少 SHEIN category/attribute/sale-attribute resolver、cookie 或 API 能力时，ListingKit 会构造默认 resolver，并以 `source=fallback`、`status=partial` 或 `needs_review` 继续。目标架构不在 Go 代码中维护类别、关键词或词形穷举，也不把缺依赖伪装成可审核业务结果；缺失生产能力必须在组合边界失败关闭。
>
> **运行时根因裁定（2026-09-03）：** 隐式降级还存在于 `internal/publishing/shein/runtime_{category,attribute,sale_attribute}_resolver.go`：这些 adapter 持有离线 resolver，store/cookie/API client 不可用时把缺能力改写成 partial。Task 16D2 必须同时删除这三个 runtime fallback。production composition 缺 Cookie Store 或任一 resolver/API builder 时直接失败；请求期 store ID、cookie 或 API client 不可用时不得调用 nil API resolver 或生成 fallback/partial，而要让 ListingKit generation 返回错误并进入失败持久化路径。resolver 对真实远端数据作出的 partial/review 仍保留。

**Files:**
- Modify: `internal/listingkit/{service_defaults.go,service_config_groups.go,service_shein_runtime_dependencies.go}` and tests
- Modify: `internal/listingkit/httpapi/{bootstrap_submit_module.go,bootstrap_validation.go,runtime_support_hooks.go,runtime_support_shein.go}` and tests
- Modify: `internal/app/httpapi/` ListingKit production composition and E2E
- Modify: `internal/publishing/shein/runtime_{category,attribute,sale_attribute}_resolver.go` and focused tests; only capability-unavailable fallback is in scope

**Interfaces:**
- Category、attribute、sale-attribute resolver 及所需 cookie/API capability 是显式 typed production dependencies；缺失时 module/pool 不注册或请求明确失败，不生成 fallback/partial 伪结果。
- 不迁移或新建 marketplace/category/lexeme 表、关键词匹配、tokenization、stemming、单复数或代码内默认分类。策略/分类数据来自外部结构化事实或专用 Integration adapter。
- 业务 resolver 显式返回的真实 partial/review 结果仍按其契约处理；本任务只删除“依赖缺失时自动构造默认实现”的隐式降级。

- [x] **Step 1: 写缺依赖 fail-closed 与非穷举守卫**

覆盖三类 resolver、cookie/API capability 任一缺失时 fail closed；完整 typed dependencies 时 current E2E 严格 `completed`，三类 resolution 的 source/status 均来自注入能力且不是 fallback。源码守卫拒绝 production `NewCategoryResolver(nil)`、内置类别/词形表和缺依赖默认构造。

- [x] **Step 2: 运行测试确认当前 fallback/partial 行为**

Run: `go test ./internal/listingkit ./internal/listingkit/httpapi ./internal/app/httpapi -run 'Test.*(Resolver|Capability|Fallback|FailClosed|CurrentListingKit)' -count=1 -v`

Expected: FAIL，并精确指出当前默认 resolver 或 fallback/partial 路径。

- [x] **Step 3: 删除隐式 resolver/capability fallback**

以窄 typed dependencies 替代默认构造；删除三个 SHEIN runtime resolver 对 nil request/store/cookie/API client 的离线 fallback，ListingKit 对缺失 resolution 返回生成错误。不得把测试 fixture、静态 category、关键词、历史配置或远端调用 fallback 搬进生产代码。

- [x] **Step 4: 运行 ListingKit、App、架构和全仓验证**

Run: `go test ./internal/listingkit/... ./internal/app/httpapi ./tests -run 'Test(ListingKit|Phase3|.*Boundary|.*Fallback)' -count=1`

Run: `go test ./... -run '^$' -count=1`

- [x] **Step 5: 提交解析能力 fail-closed 硬切**

```powershell
git add internal/listingkit internal/app/httpapi tests docs/superpowers/plans/2026-09-01-internal-target-architecture-phase3-product.md
git diff --cached --check
git commit -m "refactor(listingkit): fail closed without resolution capabilities"
```

---

### Task 16E: 删除 ListingKit Studio 的直接 AI 图片工作流

> **实施补充（2026-09-02）：** Task 16C 预检发现 `aiCapability.studioImageRoutingMode` 的 legacy/shadow/active 分支仍包裹 ListingKit 自己的 `AIImageGenerator`，并由 ListingKit 直接执行图片生成、编辑和异步查询。把模式固定为 active 仍会留下第二条图片工作流，违背“ImageAgent 是唯一图片工作流”和 ListingKit 只读 Product Snapshot/ApprovedAsset 的目标。ListingKit 尚未投入使用，不为这些入口保留兼容代理。

> **根因裁决（2026-09-02）：** Studio session/batch/batch-run、reference analysis、background removal、async job 及其持久化模型共同构成 ListingKit 自有的图片生产聚合，不是可独立保留的“手工上传”。其中 `manual-background-removal` 仍依赖 Studio batch/design 状态，保留它会迫使已退役聚合继续存在。因此 Task 16E 的保留边界只有通用 `/uploads/images` 与上传文件读删，以及标准 ListingKit 流程对 Product Snapshot/ApprovedAsset 的只读消费；其余 Studio 图片生产路由、服务、请求字段、仓储、表迁移和 capability/config 分支全部删除。不会把这些入口代理到 ImageAgent，也不会保留空壳类型、弃用别名或兼容表。

> **实施拆分：** 为让每次变更都能独立编译和审查，Task 16E 分成三个连续子任务，并以三者全部完成作为原子架构结果：16E1 先切断所有 production 路由、bootstrap、AI client/capability/config 与 Service 暴露；16E2 删除已不可达的 Studio application/domain/persistence 聚合及 schema/request 残留；16E3 增加全局负向架构守卫和端到端保留能力验证。中间提交不是兼容版本，不得发布，也不得增加桥接代码。

**Files:**
- Delete/modify: `internal/listingkit/{ai_contracts.go,studio_ai_capability_adapter.go,task_studio_media_service*.go,service_studio*_dependencies.go}` and paired tests that only cover direct AI image generation/editing
- Delete/modify: `internal/listingkit/httpapi/{ai_image_generator_adapter.go,ai_client_image_routing.go,ai_client_strict_image.go,bootstrap_submit_module.go}` and paired tests/hooks
- Modify/delete: ListingKit Studio image generation/edit/query handlers and route descriptors; retain only explicit manual-upload and read-only approved-asset flows
- Modify: `internal/app/httpapi/runtime_ai_capability.go` and App composition
- Modify: `internal/core/config/{type_ai_capability.go,defaults.go,loader_builder.go,config.go,validator_ai_capability.go}` and YAML/env tests
- Delete/modify: `internal/aicapability/` image-routing compatibility types only if they have no remaining non-ListingKit production consumer

**Interfaces:**
- ImageAgent HTTP/Temporal is the sole AI image generation/edit workflow.
- ListingKit may upload operator-provided files through its own store and may read ApprovedAsset inventory; it cannot own or invoke `AIImageGenerator`, image provider clients, async image jobs, capability shadow routing, or a legacy generator.
- Delete `aiCapability.studioImageRoutingMode` YAML/env/config and legacy/shadow/active compatibility branches atomically with their consumers. Do not replace them with an always-active adapter or proxy facade.
- If a caller needs generated assets, it starts an ImageAgent Run through the ImageAgent-owned API and later consumes approved assets; Task 16E does not add a hidden ListingKit-to-ImageAgent orchestration bridge.

- [ ] **Step 1: 写唯一图片工作流与路由退役测试**

覆盖 ListingKit production types/routes/imports 中不存在 image generator/edit/async contracts；旧 Studio AI image routes 固定 404；manual upload 和 ApprovedAsset read paths 保持可用；配置 loader 明确拒绝 `studioImageRoutingMode` 旧键/env。

- [ ] **Step 2: 运行测试确认 ListingKit 仍直接生成图片**

Run: `go test ./internal/listingkit/... ./internal/app/httpapi ./internal/core/config -run 'Test.*(Studio.*Image|Image.*Workflow|Legacy.*Route|RoutingMode)' -count=1 -v`

Expected: FAIL，并精确指出当前 generator/contracts/routes/config 分支。

- [ ] **Step 3: 原子删除直接 AI 图片工作流**

删除 contracts、adapters、handlers/routes、async binding 与配置开关；不保留 Deprecated API、proxy、alias、双写或 always-active 包装器。只保留手工上传和 ApprovedAsset 读取。

具体执行顺序：

1. **Task 16E1 — production 断路：** 删除 Studio 路由注册、API handler 暴露、bootstrap AI 图片客户端/路由模式、Service Studio 依赖装配和 `studioImageRoutingMode` 配置；确认所有旧 Studio 路由 404，而通用上传路由仍工作。
2. **Task 16E2 — 聚合清除：** 删除不可达的 Studio session/batch/batch-run/media/reference/background-removal/async 领域与仓储、`SheinStudio` 请求选项、专属 schema migration/owner inventory；不得保留只供测试引用的生产类型。
3. **Task 16E3 — 架构封口：** 增加全仓负向守卫，禁止 ListingKit production 重新出现 image generator/edit/async、Studio 路由/表/配置键或 ImageAgent 编排桥；验证手工上传与 ApprovedAsset 只读流程。

> **16E3 封口补充（2026-09-03）：** 16E2 复审后预检发现，订阅权威模块仍以 `ModuleStudio = "studio"` 承载 ListingKit generation、ImageAgent 与 SHEIN 发布额度，前端通用代理仍为已退役 `/studio/*` 保留超时分支，运维脚本仍示例查询旧 Studio 表。这些是同一兼容命名和死路由残留，不能被负向测试永久豁免。16E3 先将订阅模块原子硬切为 `ModuleListingKit = "listingkit"`（后端、默认 plan、usage settlement、管理端 UI/测试同步，不保留旧 module alias/双读），删除 proxy/script 残留，再建立精确架构守卫。`scene_style = "studio"` 等 ImageAgent 摄影场景值不是 ListingKit Studio 聚合，不在禁用范围。

- [ ] **Step 4: 运行 ListingKit、ImageAgent、App、配置和架构验证**

Run: `go test ./internal/listingkit/... ./internal/imageagent/... ./internal/app/httpapi ./internal/core/config ./tests -run 'Test(ListingKit|ImageAgent|Phase3|.*Boundary|.*Route)' -count=1`

Run: `go test ./... -run '^$' -count=1`

- [ ] **Step 5: 提交唯一图片工作流硬切**

```powershell
git add -A internal/listingkit internal/imageagent internal/app/httpapi internal/core/config internal/aicapability config tests docs/superpowers/plans/2026-09-01-internal-target-architecture-phase3-product.md
git diff --cached --check
git commit -m "refactor(listingkit): retire direct ai image workflow"
```

---

### Task 17: 删除旧目录、同步契约并启用最终护栏

**Files:**
- Verify absent: `internal/catalog/` (Task 3)
- Verify absent: `internal/asset/` (Task 13)
- Delete: `internal/imageasset/`
- Verify absent: `internal/productenrich/` and `internal/productimage/` (Task 16)
- Modify: `docs/api/listingkit-asset.openapi.yaml`
- Modify: `web/listingkit-ui/src/lib/api/generated/{types.gen.ts,index.ts}`
- Modify: `cmd/product-listing-api/README.md`
- Modify: `tests/target_architecture_phase3_product_test.go`
- Modify: `tests/{depguard_config_test.go,import_boundaries_test.go,import_scan_test.go}`
- Modify: `.golangci.yml`
- Modify: `docs/refactoring/phase2-runtime-inventory.md`

**Interfaces:**
- Consumes: Tasks 2–16 的全部目标包和只读 Port。
- Produces: 最终不可回退护栏；旧目录、旧 import、旧路由、旧 queue/table runtime 引用全部为零。

- [ ] **Step 1: 把增长测试收紧为旧目录不存在**

```go
func TestPhase3LegacyProductRootsAreAbsent(t *testing.T) {
	for _, name := range []string{"catalog", "asset", "imageasset", "productenrich", "productimage"} {
		_, err := os.Stat(filepath.Join("..", "internal", name))
		require.ErrorIs(t, err, os.ErrNotExist, "legacy root %s must be deleted", name)
	}
}

func TestPhase3ConsumersCannotOrchestrateProductImage(t *testing.T) {
	for _, root := range []string{"listingkit", "sds", "amazonlisting"} {
		assertNoBannedImportPrefixes(t, filepath.Join("..", "internal", root), []string{
			"task-processor/internal/product/image",
			"task-processor/internal/imageagent/store",
			"task-processor/internal/imageagent/temporal",
		}, nil)
	}
}
```

增加扫描断言：生产文件不得 import `internal/product/asset/assettest`；旧 import 前缀、五条旧 API、`product_enrich_tasks`、`product_image_tasks`、两个 worker pool/queue 名称在生产 Go/config/OpenAPI 中为零；历史设计文档不参与运行时扫描。

- [ ] **Step 2: 运行最终护栏确认剩余引用**

Run: `go test ./tests -run 'TestPhase3|TestDepguard|Test.*Import' -count=1 -v`

Expected: FAIL，并精确列出剩余旧文件或引用。

- [ ] **Step 3: 清理剩余目录、文档和生成客户端**

删除空目录和旧测试。更新 OpenAPI 后使用仓库已固定的生成器版本：

```powershell
Push-Location web/listingkit-ui
$generated = Join-Path ([System.IO.Path]::GetTempPath()) ("task-processor-openapi-" + [guid]::NewGuid().ToString("N"))
New-Item -ItemType Directory -Path $generated | Out-Null
pnpm exec openapi-ts -i ../../docs/api/listingkit-asset.openapi.yaml -o $generated -p @hey-api/typescript
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$names = @(Get-ChildItem -LiteralPath $generated -File | Sort-Object Name | Select-Object -ExpandProperty Name)
if (($names -join ',') -ne 'index.ts,types.gen.ts') { throw "unexpected OpenAPI output: $($names -join ',')" }
Copy-Item -LiteralPath (Join-Path $generated 'index.ts') -Destination src/lib/api/generated/index.ts -Force
Copy-Item -LiteralPath (Join-Path $generated 'types.gen.ts') -Destination src/lib/api/generated/types.gen.ts -Force
Pop-Location
```

`@hey-api/typescript` 是仓库锁定版本提供的 type-only 插件；输出不是精确两个文件时立即失败，不得手写保留旧 endpoint 类型。

- [ ] **Step 4: 运行聚焦验证**

Run: `go test ./internal/product/... ./internal/imageagent/... ./internal/listingkit/... ./internal/sds/... ./internal/amazonlisting/... ./internal/integration/persistence/product/asset -count=1`

Run: `go test ./tests -run 'Test(Product|ImageAgent|LegacyProductRoots|TargetDomains|Phase3|.*Depguard.*)' -count=1`

Run: `golangci-lint run ./internal/product/... ./internal/imageagent/... ./internal/app/...`

Expected: 全部 PASS。

- [ ] **Step 5: 运行 UI 契约验证**

```powershell
Push-Location web/listingkit-ui
pnpm typecheck
pnpm test
Pop-Location
```

Expected: PASS；生成客户端不再包含旧 ProductEnrich/ProductImage 和 ListingKit Studio product-images endpoint。

- [ ] **Step 6: 运行全仓最终验证**

```powershell
go test ./tests -count=1
go test ./... -count=1 -timeout 20m
git diff --check
rg -n 'task-processor/internal/(catalog|asset|imageasset|productenrich|productimage)' internal tests --glob '*.go'
rg -n '/api/v1/(products/generate|products/tasks|images/process|images/tasks)|product_enrich_tasks|product_image_tasks' internal cmd config docs/api web/listingkit-ui/src/lib/api/generated
```

Expected: 两组 `go test` 和 `git diff --check` PASS；两个 `rg` 命令返回退出码 1（零匹配）。

- [ ] **Step 7: 提交最终硬切**

```powershell
git add .golangci.yml internal tests docs config cmd web/listingkit-ui
git diff --cached --check
git commit -m "refactor(architecture): complete phase 3 product hard cut"
```

---

## 最终验收清单

- [ ] `internal/product` 根目录没有生产 `.go` 文件，只有 `catalog`、`sourcing`、`enrichment`、`asset`、`image` 子包和说明文档。
- [ ] 五个旧产品根目录不存在，全仓旧 import 为零。
- [ ] ProductEnrich/ProductImage Task、Queue、Worker、HTTP API 和 GORM task repository 不存在。
- [ ] ImageAgent 是唯一图片工作流，并把人工批准结果幂等提交到产品资产 Repository。
- [ ] ListingKit、SDS、AmazonListing 只读 Snapshot/Approved Inventory，未就绪时不回退来源图。
- [ ] 产品目标包没有具体运行时、持久化或 Provider 依赖。
- [ ] `product_enrich_tasks`、`product_image_tasks` 未被物理删除，但应用不再创建、查询或写入它们。
- [ ] `internal/pipeline` 未迁入产品域且生产文件数没有增长。
- [ ] Go 聚焦测试、架构护栏、lint、全仓测试以及 UI typecheck/test 全部通过。
