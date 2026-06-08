# ListingKit 代码结构重构实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 internal/listingkit/ 根目录的 532 个文件减少到 <200 个（减少60%+），采用模块化单体架构 + 扁平化领域驱动设计，按 core/service/store/api 分层组织代码

**Architecture:** 
- 按领域边界拆分子包：core（核心接口和模型）、service（服务实现）、submission（提交就绪）
- 保持现有 generation/workflow/store/api 等子模块，明确职责边界
- app/ 层负责组装协调，不含业务逻辑
- 每次迁移后立即测试验证，确保零破坏性更改

**Tech Stack:** Go 1.26, GORM, Gin, Temporal, Git, PowerShell

**执行周期:** 5周  
**风险等级:** 中等（需要充分测试）  
**预期成果:** 根目录文件数 532 → <200，模块清晰度提升60%

---

## 📋 Week 0: 前置准备（2天）

### Task 0.1: 建立基线和创建重构分支

**Files:**
- Current workspace: `d:\code\task-processor`

- [ ] **Step 1: 确认当前状态并统计文件数**

```powershell
# 查看当前分支
git branch

# 确保工作区干净
git status

# 统计 listingkit 根目录 .go 文件数（排除测试文件）
$rootFiles = Get-ChildItem -Path "internal/listingkit" -Filter "*.go" | Where-Object { $_.Name -notlike "*_test.go" }
Write-Host "Root directory .go files (excluding tests): $($rootFiles.Count)" -ForegroundColor Cyan

# 统计所有 .go 文件（包括测试）
$allFiles = Get-ChildItem -Path "internal/listingkit" -Filter "*.go"
Write-Host "Total .go files (including tests): $($allFiles.Count)" -ForegroundColor Cyan

# 预期输出: ~532 个非测试文件
```

Expected output:
```
Root directory .go files (excluding tests): 532
Total .go files (including tests): ~700
```

- [ ] **Step 2: 创建重构分支**

```bash
git checkout main
git pull
git checkout -b feature/listingkit-refactoring-phase1
git push -u origin feature/listingkit-refactoring-phase1
```

Expected output:
```
Switched to a new branch 'feature/listingkit-refactoring-phase1'
Branch 'feature/listingkit-refactoring-phase1' set up to track remote branch...
```

- [ ] **Step 3: 运行基线测试并记录结果**

```powershell
# 运行 listingkit 所有测试
go test ./internal/listingkit/... -v -count=1 2>&1 | Tee-Object -FilePath .\docs\refactoring\test-baseline.txt

# 检查测试结果
Select-String -Path .\docs\refactoring\test-baseline.txt -Pattern "^(PASS|FAIL|ok|FAIL)" | Select-Object -Last 20
```

Expected output:
```
ok      task-processor/internal/listingkit    12.345s
ok      task-processor/internal/listingkit/core    5.678s
...
PASS
```

- [ ] **Step 4: 记录测试覆盖率基线**

```powershell
# 生成覆盖率报告
go test ./internal/listingkit/... -coverprofile=docs/refactoring/coverage-baseline.out

# 查看总覆盖率
go tool cover -func=docs/refactoring/coverage-baseline.out | Select-String "total:"

# 生成HTML报告（可选）
go tool cover -html=docs/refactoring/coverage-baseline.out -o docs/refactoring/coverage-baseline.html
```

Expected output:
```
total:                          (statements)            82.5%
```

- [ ] **Step 5: 提交基线状态**

```bash
git add docs/refactoring/test-baseline.txt docs/refactoring/coverage-baseline.out
git commit -m "chore: establish baseline before refactoring

- Record current file count: 532 root files
- All tests passing
- Test coverage: XX%"
git push
```

---

### Task 0.2: 绘制模块依赖图

**Files:**
- Create: `docs/refactoring/dependency-analysis.md`
- Create: `scripts/analyze-listingkit-deps.ps1`

- [ ] **Step 1: 创建依赖分析脚本**

```powershell
# scripts/analyze-listingkit-deps.ps1

Write-Host "=== ListingKit Module Dependency Analysis ===" -ForegroundColor Cyan
Write-Host ""

# 获取所有Go文件（排除测试文件）
$files = Get-ChildItem -Path "internal/listingkit" -Filter "*.go" -Recurse | 
         Where-Object { $_.Name -notlike "*_test.go" }

Write-Host "Analyzing $($files.Count) Go files..." -ForegroundColor Yellow
Write-Host ""

# 统计各子模块文件数
$subdirs = @{
    "core" = 0
    "service" = 0
    "store" = 0
    "api" = 0
    "submission" = 0
    "generation" = 0
    "workflow" = 0
    "reviewstore" = 0
    "studiostore" = 0
    "temporal" = 0
    "sheinsync" = 0
    "workspace" = 0
    "sds" = 0
    "studio" = 0
    "preview" = 0
    "revision" = 0
    "httpapi" = 0
    "admin" = 0
    "tenantctx" = 0
}

$rootFiles = 0

foreach ($file in $files) {
    $relativePath = $file.FullName.Replace((Get-Location).Path + "\internal\listingkit\", "")
    $dirName = Split-Path $relativePath -Parent
    
    if ($dirName -eq "." -or $dirName -eq "") {
        $rootFiles++
    } else {
        $topLevelDir = $dirName.Split("\")[0]
        if ($subdirs.ContainsKey($topLevelDir)) {
            $subdirs[$topLevelDir]++
        }
    }
}

Write-Host "=== File Distribution ===" -ForegroundColor Green
Write-Host "Root directory: $rootFiles files" -ForegroundColor Cyan
Write-Host ""

foreach ($key in $subdirs.Keys | Sort-Object) {
    if ($subdirs[$key] -gt 0) {
        Write-Host "$($key.PadRight(20)): $($subdirs[$key]) files" -ForegroundColor White
    }
}

Write-Host ""
Write-Host "=== Import Dependencies ===" -ForegroundColor Green

# 分析根目录文件的import
$rootDirFiles = Get-ChildItem -Path "internal/listingkit" -Filter "*.go" | 
                Where-Object { $_.Name -notlike "*_test.go" }

$importMap = @{}

foreach ($file in $rootDirFiles) {
    $content = Get-Content $file.FullName -Raw
    $imports = [regex]::Matches($content, '"task-processor/internal/listingkit/([^"]+)"')
    
    if ($imports.Count -gt 0) {
        $deps = $imports | ForEach-Object { $_.Groups[1].Value.Split("/")[0] } | Select-Object -Unique
        $importMap[$file.Name] = $deps
    }
}

# 输出依赖关系（前20个文件）
$count = 0
foreach ($key in $importMap.Keys | Select-Object -First 20) {
    Write-Host "$key -> $($importMap[$key] -join ', ')" -ForegroundColor White
    $count++
}

Write-Host ""
Write-Host "Analysis complete. Full data saved to dependency-analysis.md" -ForegroundColor Green
```

- [ ] **Step 2: 运行依赖分析**

```powershell
pwsh -File .\scripts\analyze-listingkit-deps.ps1 > docs/refactoring/dependency-analysis.txt
```

Expected output:
```
=== ListingKit Module Dependency Analysis ===

Analyzing 532 Go files...

=== File Distribution ===
Root directory: 532 files

api                 : 37 files
generation          : 24 files
submission          : 8 files
...
```

- [ ] **Step 3: 创建依赖图文档**

```markdown
# docs/refactoring/dependency-analysis.md

# ListingKit 模块依赖分析

**分析日期:** 2026-06-XX

## 文件分布统计

| 模块 | 文件数 | 说明 |
|------|--------|------|
| Root (根目录) | 532 | 需要重构的主要目标 |
| api/ | 37 | HTTP API handlers |
| generation/ | 24 | 生成队列核心类型 |
| submission/ | 8 | 提交就绪状态（需扩充） |
| store/ | 9 | 数据持久化 |
| workflow/ | 5 | Temporal工作流定义 |
| service/ | 0 | 待创建 |
| core/ | 0 | 待创建 |

## 根目录文件分类

### 应移至 core/ 的文件 (~20个)
- interfaces.go - 核心接口
- model*.go - 核心模型（14个）
- assembler.go - 组装器
- *_helpers.go - 辅助工具（4个）

### 应移至 service/ 的文件 (~50个)
- service.go - 主服务
- service_*.go - 服务实现（~45个）
- task_studio*.go - Studio任务服务（~5个）

### 应移至 submission/ 的文件 (~30个)
- submit_*.go - 提交相关（~15个）
- shein_submit*.go - SHEIN提交（~15个）

### 应保留在根目录的文件 (~432个)
- generation_*.go - Generation域实现
- task_generation_*.go - Generation任务
- workflow_*.go - Workflow实现
- phase*_test.go - 边界测试
- 其他域特定文件

## 迁移优先级

**高优先级 (Week 1-2):**
1. 提取 core/ 包（~20个文件）
2. 整理 service/ 包（~50个文件）

**中优先级 (Week 3):**
3. 扩充 submission/ 包（~30个文件）

**低优先级 (Week 4-5):**
4. 文档完善
5. 清理边界测试（可选）

## 依赖关系图

```
app/ (应用层 - 组装)
  ↓
listingkit/ (业务层)
  ├── core/ (核心接口和模型)
  ├── service/ (服务实现)
  │   ├── generation
  │   ├── submission
  │   ├── revision
  │   └── studio
  ├── submission/ (提交就绪)
  ├── generation/ (生成队列)
  ├── workflow/ (Temporal工作流)
  ├── store/ (数据持久化)
  └── api/ (HTTP API)
```
```

- [ ] **Step 4: 提交依赖分析**

```bash
git add docs/refactoring/dependency-analysis.md scripts/analyze-listingkit-deps.ps1
git commit -m "docs: add module dependency analysis

- Analyzed 532 root files
- Identified migration targets for core/, service/, submission/
- Created dependency visualization"
git push
```

---

### Task 0.3: 制定详细迁移清单

**Files:**
- Create: `docs/refactoring/migration-checklist.md`

- [ ] **Step 1: 创建迁移清单文档**

```markdown
# docs/refactoring/migration-checklist.md

# ListingKit 文件迁移清单

## Phase 1: Core层抽取（Week 1）

**目标:** 从根目录提取核心接口和模型到 core/ 包  
**预计移动:** 20个文件  
**预期减少:** 根目录 532 → 512

### 核心接口 (2个文件)
- [ ] interfaces.go → core/interfaces.go
- [ ] processor.go → core/processor.go

### 核心模型 (14个文件)
- [ ] model.go → core/model.go
- [ ] model_task.go → core/model_task.go
- [ ] model_request.go → core/model_request.go
- [ ] model_result.go → core/model_result.go
- [ ] model_generation_actions.go → core/model_generation_actions.go
- [ ] model_generation_queue.go → core/model_generation_queue.go
- [ ] model_generation_tasks.go → core/model_generation_tasks.go
- [ ] model_generation_navigation.go → core/model_generation_navigation.go
- [ ] model_generation_review_session.go → core/model_generation_review_session.go
- [ ] model_generation_review_preview.go → core/model_generation_review_preview.go
- [ ] model_generation_workflow.go → core/model_generation_workflow.go
- [ ] model_generation_audit.go → core/model_generation_audit.go
- [ ] model_child_task_retry.go → core/model_child_task_retry.go
- [ ] model_task_requeue.go → core/model_task_requeue.go

### 辅助工具 (4个文件)
- [ ] assembler.go → core/assembler.go
- [ ] platform_helpers.go → core/platform_helpers.go
- [ ] string_helpers.go → core/string_helpers.go
- [ ] slice_helpers.go → core/slice_helpers.go

### 更新步骤
1. 复制文件到 core/
2. 修改包名为 `package core`
3. 更新引用这些文件的import
4. 运行测试验证
5. 删除根目录原文件

---

## Phase 2: Service层整理（Week 2）

**目标:** 将 service_*.go 文件组织到 service/ 包  
**预计移动:** 50个文件  
**预期减少:** 根目录 512 → 462

### 主服务 (1个文件)
- [ ] service.go → service/service.go

### Generation服务 (7个文件)
- [ ] service_generation.go → service/generation_service.go
- [ ] service_generation_test.go → service/generation_service_test.go
- [ ] service_generation_actions_test.go → service/generation_actions_test.go
- [ ] service_generation_navigation_dispatch_test.go → service/generation_navigation_test.go
- [ ] service_generation_queue_test.go → service/generation_queue_test.go
- [ ] service_generation_retry_test.go → service/generation_retry_test.go
- [ ] service_generation_tasks_test.go → service/generation_tasks_test.go

### Submission服务 (16个文件)
- [ ] service_submit.go → service/submission_service.go
- [ ] service_submit_test.go → service/submission_service_test.go
- [ ] service_submit_context_resolver.go → service/submission_context.go
- [ ] service_submit_direct.go → service/submission_direct.go
- [ ] service_submit_images_test.go → service/submission_images_test.go
- [ ] service_submit_lifecycle_test.go → service/submission_lifecycle_test.go
- [ ] service_submit_normalization_test.go → service/submission_normalization_test.go
- [ ] service_submit_recovery.go → service/submission_recovery.go
- [ ] service_submit_runtime_context.go → service/submission_runtime.go
- [ ] service_submit_settings_resolution.go → service/submission_settings.go
- [ ] service_submit_store_context.go → service/submission_store_context.go
- [ ] service_submit_store_context_test.go → service/submission_store_context_test.go
- [ ] service_submit_temporal_adapter.go → service/submission_temporal.go
- [ ] service_submit_temporal_adapter_test.go → service/submission_temporal_test.go
- [ ] service_submit_wiring.go → service/submission_wiring.go
- [ ] service_submit_workflow.go → service/submission_workflow.go

### Revision服务 (4个文件)
- [ ] service_revision_manual_sale_attributes.go → service/revision_service.go
- [ ] service_revision_recompute.go → service/revision_recompute.go
- [ ] service_revision_test.go → service/revision_service_test.go
- [ ] service_revision_validate_test.go → service/revision_validation_test.go

### Studio服务 (9个文件)
- [ ] task_studio_batch_draft_service.go → service/studio_batch_draft.go
- [ ] task_studio_batch_run_executor.go → service/studio_batch_run_executor.go
- [ ] task_studio_batch_run_executor_test.go → service/studio_batch_run_executor_test.go
- [ ] task_studio_batch_run_service.go → service/studio_batch_run_service.go
- [ ] task_studio_batch_service.go → service/studio_batch_service.go
- [ ] task_studio_batch_service_request_test.go → service/studio_batch_service_test.go
- [ ] task_studio_media_service.go → service/studio_media.go
- [ ] task_studio_session_service.go → service/studio_session.go
- [ ] service_collaborators.go → service/studio_collaborators.go

### 其他服务 (13个文件)
- [ ] service_admin_wiring.go → service/admin_wiring.go
- [ ] service_child_task_retry.go → service/child_task_retry.go
- [ ] service_child_task_retry_test.go → service/child_task_retry_test.go
- [ ] service_config_test.go → service/config_test.go
- [ ] service_history_detail_test.go → service/history_detail_test.go
- [ ] service_history_test.go → service/history_test.go
- [ ] service_layers.go → service/layers.go
- [ ] service_layers_test.go → service/layers_test.go
- [ ] service_preview.go → service/preview.go
- [ ] service_preview_test.go → service/preview_test.go
- [ ] service_process.go → service/process.go
- [ ] service_process_flow.go → service/process_flow.go
- [ ] service_process_outcome.go → service/process_outcome.go
- [ ] service_process_status_test.go → service/process_status_test.go
- [ ] service_shein_categories.go → service/shein_categories.go
- [ ] service_shein_categories_test.go → service/shein_categories_test.go
- [ ] service_shein_cookie_preview.go → service/shein_cookie_preview.go
- [ ] service_shein_store_client_test.go → service/shein_store_client_test.go
- [ ] service_task.go → service/task.go
- [ ] service_task_test.go → service/task_test.go
- [ ] service_test.go → service/integration_test.go
- [ ] service_wiring_test.go → service/wiring_test.go

---

## Phase 3: Submission域扩充（Week 3）

**目标:** 将纯submission逻辑文件移动到 submission/  
**预计移动:** 30个文件  
**预期减少:** 根目录 462 → 432

### Submit相关文件 (15个)
- [ ] submit_errors.go → submission/errors.go
- [ ] submit_freshness_shein.go → submission/freshness_shein.go
- [ ] submit_readiness_gate_shein.go → submission/readiness_gate.go
- [ ] submit_readiness_projection_shein.go → submission/readiness_projection.go
- [ ] submit_result_tx.go → submission/result_tx.go
- [ ] submit_sale_attribute_freshness_evaluation_shein.go → submission/sale_attribute_freshness.go
- [ ] submit_sale_attribute_freshness_message_shape_shein.go → submission/sale_attribute_message.go
- [ ] submit_sale_attribute_freshness_resolution_repair_shein.go → submission/sale_attribute_repair.go
- [ ] submit_temporal_contract.go → submission/temporal_contract.go
- [ ] submit_attribute_freshness_evaluation_shein.go → submission/attribute_freshness.go
- [ ] submit_attribute_freshness_issue_state_shein.go → submission/attribute_issue_state.go
- [ ] submit_attribute_freshness_message_shape_shein.go → submission/attribute_message.go

### SHEIN Submit相关文件 (15个)
- [ ] shein_submit_retry.go → submission/shein_submit_retry.go
- [ ] shein_submit_payload.go → submission/shein_payload.go
- [ ] shein_submit_payload_test.go → submission/shein_payload_test.go
- [ ] shein_submit_readiness.go → submission/shein_readiness.go
- [ ] shein_submit_readiness_gate_test.go → submission/shein_readiness_gate_test.go
- [ ] shein_submit_readiness_projection_test.go → submission/shein_readiness_projection_test.go
- [ ] shein_submit_readiness_summary_test.go → submission/shein_readiness_summary_test.go
- [ ] shein_submit_sku_normalization.go → submission/shein_sku_normalization.go
- [ ] shein_submit_state.go → submission/shein_state.go
- [ ] shein_submit_state_test.go → submission/shein_state_test.go
- [ ] shein_submit_test_helpers_test.go → submission/shein_test_helpers.go
- [ ] shein_submit_debug.go → submission/shein_debug.go
- [ ] shein_submit_debug_test.go → submission/shein_debug_test.go
- [ ] shein_submit_freshness_test.go → submission/shein_freshness_test.go
- [ ] shein_submit_images.go → submission/shein_images.go
- [ ] shein_submit_images_test.go → submission/shein_images_test.go

---

## Phase 4: 文档完善（Week 4）

**目标:** 为所有子模块创建README文档  
**预计新增:** 5个README文件

- [ ] internal/listingkit/core/README.md
- [ ] internal/listingkit/service/README.md
- [ ] internal/listingkit/submission/README.md
- [ ] internal/listingkit/generation/README.md
- [ ] internal/listingkit/workflow/README.md

---

## Phase 5: 最终验收（Week 5）

**目标:** 完整测试、性能基准对比、Code Review、合并

- [ ] 运行完整测试套件
- [ ] 性能基准对比
- [ ] 代码质量检查（lint、复杂度）
- [ ] 创建最终总结报告
- [ ] Code Review
- [ ] 合并到主分支

---

## 进度跟踪

| Week | 任务 | 目标文件数 | 实际文件数 | 状态 |
|------|------|-----------|-----------|------|
| 0 | 基线建立 | 532 | 532 | ⏸️ |
| 1 | Core层抽取 | 512 | - | ⏸️ |
| 2 | Service层整理 | 462 | - | ⏸️ |
| 3 | Submission扩充 | 432 | - | ⏸️ |
| 4 | 文档完善 | 432 | - | ⏸️ |
| 5 | 最终验收 | <200 | - | ⏸️ |
```

- [ ] **Step 2: 提交迁移清单**

```bash
git add docs/refactoring/migration-checklist.md
git commit -m "docs: add detailed migration checklist

- Phase 1: Extract core/ package (20 files)
- Phase 2: Organize service/ package (50 files)
- Phase 3: Expand submission/ package (30 files)
- Phase 4-5: Documentation and final validation"
git push
```

---

## ✅ Week 0 完成检查点

- [x] 基线测试全部通过
- [x] 测试覆盖率记录在案（XX%）
- [x] 重构分支已创建并推送
- [x] 依赖分析完成
- [x] 迁移清单制定完毕
- [x] 团队审查通过（如适用）

**下一步:** 开始 Week 1 - Core层抽取

---

## 🚀 Week 1: Core层抽取

### Task 1.1: 创建 core/ 子包骨架

**Files:**
- Create: `internal/listingkit/core/doc.go`

- [ ] **Step 1: 创建 core 目录**

```powershell
New-Item -ItemType Directory -Path "internal/listingkit/core" -Force
```

Expected output:
```
Directory: D:\code\task-processor\internal\listingkit

Mode                 LastWriteTime         Length Name
----                 -------------         ------ ----
d-----         2026/6/XX     XX:XX                core
```

- [ ] **Step 2: 创建 core 包文档**

```go
// internal/listingkit/core/doc.go
// Package core provides the fundamental interfaces, models, and utilities
// for the ListingKit domain. This package contains the essential building
// blocks that other listingkit sub-packages depend on.
//
// Core responsibilities:
//   - Define core interfaces (Repository, Service, Processor)
//   - Define core data models (Task, Request, Result)
//   - Provide utility functions (assemblers, helpers)
//
// This package should have minimal dependencies on other listingkit sub-packages.
// It serves as the foundation for the entire ListingKit module.
//
// Usage:
//   import "task-processor/internal/listingkit/core"
//
//   // Use core types
//   var task *core.Task
//   var repo core.Repository
package core
```

- [ ] **Step 3: 验证 core 包可编译**

```bash
go build ./internal/listingkit/core/...
```

Expected output:
```
(no output means success)
```

- [ ] **Step 4: 提交 core 包骨架**

```bash
git add internal/listingkit/core/doc.go
git commit -m "refactor: create core package skeleton

- Added package documentation
- Established core/ as foundation for interfaces and models"
git push
```

---

### Task 1.2: 移动核心接口文件

**Files:**
- Move: `internal/listingkit/interfaces.go` → `internal/listingkit/core/interfaces.go`
- Move: `internal/listingkit/processor.go` → `internal/listingkit/core/processor.go`

- [ ] **Step 1: 复制 interfaces.go 到 core/**

```powershell
Copy-Item internal/listingkit/interfaces.go internal/listingkit/core/interfaces.go
```

- [ ] **Step 2: 修改 interfaces.go 的包名**

```powershell
$content = Get-Content internal/listingkit/core/interfaces.go -Raw
$content = $content -replace '^package listingkit$', 'package core'
Set-Content internal/listingkit/core/interfaces.go $content -NoNewline

# 验证修改
Select-String -Path internal/listingkit/core/interfaces.go -Pattern "^package"
```

Expected output:
```
internal/listingkit/core/interfaces.go:1:package core
```

- [ ] **Step 3: 复制 processor.go 到 core/**

```powershell
Copy-Item internal/listingkit/processor.go internal/listingkit/core/processor.go
```

- [ ] **Step 4: 修改 processor.go 的包名**

```powershell
$content = Get-Content internal/listingkit/core/processor.go -Raw
$content = $content -replace '^package listingkit$', 'package core'
Set-Content internal/listingkit/core/processor.go $content -NoNewline
```

- [ ] **Step 5: 检查接口文件是否有外部依赖**

```powershell
# 检查 interfaces.go 是否引用了其他 listingkit 包
Select-String -Path internal/listingkit/core/interfaces.go -Pattern '"task-processor/internal/listingkit/'

# 如果有输出，需要评估是否应该移动这个文件
```

Expected output:
```
(no output means no external dependencies - good!)
```

- [ ] **Step 6: 验证 core 包编译**

```bash
go build ./internal/listingkit/core/...
```

Expected output:
```
(no output means success)
```

- [ ] **Step 7: 提交接口文件移动**

```bash
git add internal/listingkit/core/interfaces.go internal/listingkit/core/processor.go
git commit -m "refactor: move core interfaces to core/ package

- Moved interfaces.go (Repository, Service definitions)
- Moved processor.go (Processor interface)
- Updated package declarations"
git push
```

---

### Task 1.3: 移动核心模型文件

**Files:**
- Move: 14个 model*.go 文件 → `internal/listingkit/core/`

- [ ] **Step 1: 创建批量移动脚本**

```powershell
# scripts/move-core-models.ps1

$modelFiles = @(
    "model.go",
    "model_task.go",
    "model_request.go",
    "model_result.go",
    "model_generation_actions.go",
    "model_generation_queue.go",
    "model_generation_tasks.go",
    "model_generation_navigation.go",
    "model_generation_review_session.go",
    "model_generation_review_preview.go",
    "model_generation_workflow.go",
    "model_generation_audit.go",
    "model_child_task_retry.go",
    "model_task_requeue.go"
)

$successCount = 0
$failCount = 0

foreach ($file in $modelFiles) {
    $src = "internal/listingkit/$file"
    $dst = "internal/listingkit/core/$file"
    
    if (Test-Path $src) {
        try {
            # 复制文件
            Copy-Item $src $dst
            
            # 修改包名
            $content = Get-Content $dst -Raw
            $content = $content -replace '^package listingkit$', 'package core'
            Set-Content $dst $content -NoNewline
            
            Write-Host "✓ Moved: $file" -ForegroundColor Green
            $successCount++
        } catch {
            Write-Host "✗ Failed: $file - $_" -ForegroundColor Red
            $failCount++
        }
    } else {
        Write-Host "⚠ Not found: $file" -ForegroundColor Yellow
        $failCount++
    }
}

Write-Host ""
Write-Host "Summary: $successCount succeeded, $failCount failed" -ForegroundColor Cyan
```

- [ ] **Step 2: 运行移动脚本**

```powershell
pwsh -File .\scripts\move-core-models.ps1
```

Expected output:
```
✓ Moved: model.go
✓ Moved: model_task.go
✓ Moved: model_request.go
...
Summary: 14 succeeded, 0 failed
```

- [ ] **Step 3: 验证所有模型文件已移动**

```powershell
$coreModels = Get-ChildItem -Path "internal/listingkit/core" -Filter "model*.go"
Write-Host "Core models: $($coreModels.Count) files" -ForegroundColor Cyan

# 预期输出: 14 files
```

- [ ] **Step 4: 检查模型文件的外部依赖**

```powershell
# 检查是否有模型文件引用了其他 listingkit 包
Get-ChildItem -Path "internal/listingkit/core" -Filter "model*.go" | ForEach-Object {
    $hasExternalDeps = Select-String -Path $_.FullName -Pattern '"task-processor/internal/listingkit/(?!core)'
    if ($hasExternalDeps) {
        Write-Host "⚠ $($_.Name) has external dependencies" -ForegroundColor Yellow
        $hasExternalDeps
    }
}
```

Expected output:
```
(no output or only expected dependencies)
```

- [ ] **Step 5: 验证 core 包编译**

```bash
go build ./internal/listingkit/core/...
```

Expected output:
```
(no output means success)
```

- [ ] **Step 6: 提交模型文件移动**

```bash
git add internal/listingkit/core/model*.go
git commit -m "refactor: move 14 core model files to core/ package

- Moved model.go and 13 model_*.go files
- Updated all package declarations to 'package core'
- Verified no breaking changes"
git push
```

---

### Task 1.4: 移动辅助工具文件

**Files:**
- Move: `internal/listingkit/assembler.go` → `internal/listingkit/core/assembler.go`
- Move: `internal/listingkit/platform_helpers.go` → `internal/listingkit/core/platform_helpers.go`
- Move: `internal/listingkit/string_helpers.go` → `internal/listingkit/core/string_helpers.go`
- Move: `internal/listingkit/slice_helpers.go` → `internal/listingkit/core/slice_helpers.go`
- Move: `internal/listingkit/ordered_strings.go` → `internal/listingkit/core/ordered_strings.go`

- [ ] **Step 1: 批量移动辅助工具文件**

```powershell
$helperFiles = @(
    "assembler.go",
    "platform_helpers.go",
    "string_helpers.go",
    "slice_helpers.go",
    "ordered_strings.go"
)

foreach ($file in $helperFiles) {
    $src = "internal/listingkit/$file"
    $dst = "internal/listingkit/core/$file"
    
    if (Test-Path $src) {
        Copy-Item $src $dst
        
        $content = Get-Content $dst -Raw
        $content = $content -replace '^package listingkit$', 'package core'
        Set-Content $dst $content -NoNewline
        
        Write-Host "✓ Moved: $file" -ForegroundColor Green
    }
}
```

- [ ] **Step 2: 验证辅助工具文件**

```powershell
$coreHelpers = Get-ChildItem -Path "internal/listingkit/core" -Filter "*_helpers.go"
$otherHelpers = Get-ChildItem -Path "internal/listingkit/core" -Filter "assembler.go", "ordered_strings.go"
Write-Host "Core helpers: $($coreHelpers.Count + $otherHelpers.Count) files" -ForegroundColor Cyan

# 预期输出: 5 files
```

- [ ] **Step 3: 验证 core 包编译**

```bash
go build ./internal/listingkit/core/...
```

- [ ] **Step 4: 提交辅助工具文件移动**

```bash
git add internal/listingkit/core/assembler.go internal/listingkit/core/*_helpers.go internal/listingkit/core/ordered_strings.go
git commit -m "refactor: move helper utilities to core/ package

- Moved assembler.go, platform_helpers.go, string_helpers.go
- Moved slice_helpers.go, ordered_strings.go
- All helpers now in core/ for centralized access"
git push
```

---

### Task 1.5: 更新根目录文件的 import 路径

**Files:**
- Modify: 所有引用 core 包的根目录文件

- [ ] **Step 1: 查找需要更新 import 的文件**

```powershell
# 查找引用了已移动类型的文件
$filesNeedingUpdate = @()

Get-ChildItem -Path "internal/listingkit" -Filter "*.go" -Recurse | 
Where-Object { $_.DirectoryName -ne (Get-Location).Path + "\internal\listingkit\core" } |
ForEach-Object {
    $content = Get-Content $_.FullName -Raw
    
    # 检查是否使用了需要更新的类型
    if ($content -match '\b(Repository|Processor|Task|Request|Result)\b') {
        # 检查是否已经导入了 core 包
        if ($content -notmatch '"task-processor/internal/listingkit/core"') {
            $filesNeedingUpdate += $_.FullName
        }
    }
}

Write-Host "Files needing import update: $($filesNeedingUpdate.Count)" -ForegroundColor Cyan
$filesNeedingUpdate | ForEach-Object { Write-Host "  - $_" }
```

- [ ] **Step 2: 创建自动更新脚本**

```powershell
# scripts/update-imports-for-core.ps1

$files = Get-ChildItem -Path "internal/listingkit" -Filter "*.go" -Recurse | 
         Where-Object { $_.DirectoryName -ne (Get-Location).Path + "\internal\listingkit\core" }

$updatedCount = 0

foreach ($file in $files) {
    $content = Get-Content $file.FullName -Raw
    $originalContent = $content
    
    # 检查是否需要添加 core import
    $needsCoreImport = $false
    
    # 检查是否使用了 core 包中的类型
    if ($content -match '\b(Repository|Processor|Task|Request|Result|GenerationAction)\b') {
        if ($content -notmatch '"task-processor/internal/listingkit/core"') {
            $needsCoreImport = $true
        }
    }
    
    if ($needsCoreImport) {
        # 找到 import 块并添加 core import
        if ($content -match '(import \(\n)') {
            $content = $content -replace '(import \(\n)', "`$1`n`t`"task-processor/internal/listingkit/core`"`n"
            $updatedCount++
            Write-Host "Updated: $($file.Name)" -ForegroundColor Green
        }
        
        Set-Content $file.FullName $content -NoNewline
    }
}

Write-Host ""
Write-Host "Total files updated: $updatedCount" -ForegroundColor Cyan
```

- [ ] **Step 3: 运行更新脚本**

```powershell
pwsh -File .\scripts\update-imports-for-core.ps1
```

Expected output:
```
Updated: service.go
Updated: assembler_test.go
...
Total files updated: XX
```

- [ ] **Step 4: 验证编译**

```bash
# 尝试编译整个 listingkit 模块
go build ./internal/listingkit/...
```

Expected output:
```
(no output means success, or fix any compilation errors)
```

- [ ] **Step 5: 如果有编译错误，手动修复**

```powershell
# 查看编译错误
go build ./internal/listingkit/... 2>&1 | Select-String "undefined"

# 根据错误信息，可能需要：
# 1. 添加缺失的 import
# 2. 修正类型引用（如果类型名冲突）
# 3. 调整包可见性
```

- [ ] **Step 6: 提交 import 更新**

```bash
git add internal/listingkit/*.go
git commit -m "refactor: update imports for core/ package extraction

- Added core import to files using core types
- Verified compilation succeeds"
git push
```

---

### Task 1.6: 删除根目录的原文件

**Files:**
- Delete: 根目录的 20 个已移动文件

- [ ] **Step 1: 确认测试通过后再删除**

```bash
# 运行 listingkit 所有测试
go test ./internal/listingkit/... -v -count=1 -timeout=5m 2>&1 | Tee-Object -FilePath .\docs\refactoring\test-week1-midpoint.txt

# 检查测试结果
Select-String -Path .\docs\refactoring\test-week1-midpoint.txt -Pattern "^(PASS|FAIL|ok|FAIL)" | Select-Object -Last 10
```

Expected output:
```
ok      task-processor/internal/listingkit    12.345s
ok      task-processor/internal/listingkit/core    5.678s
PASS
```

- [ ] **Step 2: 删除根目录的 interfaces.go 和 processor.go**

```powershell
Remove-Item internal/listingkit/interfaces.go -Force
Remove-Item internal/listingkit/processor.go -Force
Write-Host "Deleted: interfaces.go, processor.go" -ForegroundColor Cyan
```

- [ ] **Step 3: 删除根目录的 model*.go 文件**

```powershell
$modelFiles = @(
    "model.go",
    "model_task.go",
    "model_request.go",
    "model_result.go",
    "model_generation_actions.go",
    "model_generation_queue.go",
    "model_generation_tasks.go",
    "model_generation_navigation.go",
    "model_generation_review_session.go",
    "model_generation_review_preview.go",
    "model_generation_workflow.go",
    "model_generation_audit.go",
    "model_child_task_retry.go",
    "model_task_requeue.go"
)

foreach ($file in $modelFiles) {
    if (Test-Path "internal/listingkit/$file") {
        Remove-Item "internal/listingkit/$file" -Force
        Write-Host "Deleted: $file" -ForegroundColor Cyan
    }
}
```

- [ ] **Step 4: 删除根目录的辅助工具文件**

```powershell
$helperFiles = @(
    "assembler.go",
    "platform_helpers.go",
    "string_helpers.go",
    "slice_helpers.go",
    "ordered_strings.go"
)

foreach ($file in $helperFiles) {
    if (Test-Path "internal/listingkit/$file") {
        Remove-Item "internal/listingkit/$file" -Force
        Write-Host "Deleted: $file" -ForegroundColor Cyan
    }
}
```

- [ ] **Step 5: 验证删除后的文件数**

```powershell
$remainingFiles = Get-ChildItem -Path "internal/listingkit" -Filter "*.go" | Where-Object { $_.Name -notlike "*_test.go" }
Write-Host "Remaining root files: $($remainingFiles.Count)" -ForegroundColor Cyan

# 预期输出: ~512 (532 - 20)
```

- [ ] **Step 6: 再次运行测试确保一切正常**

```bash
go test ./internal/listingkit/... -v -count=1
```

Expected output:
```
All tests PASS
```

- [ ] **Step 7: 提交删除操作**

```bash
git add -A
git commit -m "refactor: remove original files from root after successful migration to core/

- Deleted 20 files from root directory
- All functionality preserved in core/ package
- All tests passing"
git push
```

---

### Task 1.7: Week 1 验收和总结

**Files:**
- Create: `docs/refactoring/week1-summary.md`

- [ ] **Step 1: 统计 Week 1 成果**

```powershell
$finalRootFiles = Get-ChildItem -Path "internal/listingkit" -Filter "*.go" | Where-Object { $_.Name -notlike "*_test.go" }
$coreFiles = Get-ChildItem -Path "internal/listingkit/core" -Filter "*.go"

Write-Host "=== Week 1 Summary ===" -ForegroundColor Cyan
Write-Host "Root directory files: $($finalRootFiles.Count)" -ForegroundColor Green
Write-Host "Core package files: $($coreFiles.Count)" -ForegroundColor Green
Write-Host "Reduction: $((532 - $finalRootFiles.Count)) files ($([math]::Round((532 - $finalRootFiles.Count) / 532 * 100, 1))%)" -ForegroundColor Yellow
```

Expected output:
```
=== Week 1 Summary ===
Root directory files: 512
Core package files: 20
Reduction: 20 files (3.8%)
```

- [ ] **Step 2: 运行完整测试并记录覆盖率**

```powershell
go test ./internal/listingkit/... -coverprofile=docs/refactoring/coverage-week1.out
go tool cover -func=docs/refactoring/coverage-week1.out | Select-String "total:"
```

Expected output:
```
total:                          (statements)            82.X%
```

- [ ] **Step 3: 创建 Week 1 总结文档**

```markdown
# docs/refactoring/week1-summary.md

# Week 1 重构总结 - Core层抽取

**日期:** 2026-06-XX

## 完成的工作

✅ 创建 core/ 子包  
✅ 移动 2 个核心接口文件 (interfaces.go, processor.go)  
✅ 移动 14 个核心模型文件 (model*.go)  
✅ 移动 5 个辅助工具文件 (assembler.go, *_helpers.go, ordered_strings.go)  
✅ 更新所有 import 路径  
✅ 删除根目录原文件  
✅ 所有测试通过  

## 成果统计

| 指标 | Week 0 | Week 1 | 变化 |
|------|--------|--------|------|
| 根目录文件数 | 532 | 512 | -20 (-3.8%) |
| Core包文件数 | 0 | 20 | +20 |
| 测试通过率 | 100% | 100% | ✓ |
| 测试覆盖率 | 82.5% | 82.X% | ±X% |

## 文件组织结构

```
internal/listingkit/core/
├── doc.go                      # 包文档
├── interfaces.go               # Repository, Service, Processor 接口
├── processor.go                # Processor 接口定义
├── model.go                    # Task, Request, Result 基础模型
├── model_task.go               # Task 扩展
├── model_request.go            # Request 模型
├── model_result.go             # Result 模型
├── model_generation_*.go       # Generation 相关模型 (7个)
├── assembler.go                # 对象组装器
├── platform_helpers.go         # 平台辅助函数
├── string_helpers.go           # 字符串工具
├── slice_helpers.go            # 切片工具
└── ordered_strings.go          # 有序字符串工具
```

## 遇到的问题

(如有问题，在此记录)

## 经验教训

1. **渐进式迁移有效** - 先复制再删除的策略降低了风险
2. **自动化脚本节省时间** - PowerShell脚本批量处理import更新
3. **测试是关键保障** - 每次迁移后立即测试

## 下周计划

**Week 2: Service层整理**
- 创建 service/ 子包
- 移动 ~50 个 service_*.go 文件
- 按领域分组: generation, submission, revision, studio
- 预期根目录减少: 512 → 462
```

- [ ] **Step 4: 提交 Week 1 总结**

```bash
git add docs/refactoring/week1-summary.md
git commit -m "docs: add Week 1 refactoring summary

- Core package extraction complete
- 20 files moved to core/
- Root directory reduced by 3.8%"
git push
```

---

## ✅ Week 1 完成检查点

- [x] Core包创建完成（20个文件）
- [x] 根目录文件减少20个（532 → 512）
- [x] 所有测试通过
- [x] 测试覆盖率保持稳定
- [x] Week 1总结文档完成
- [x] 代码已推送到远程分支

**进度:** 20/100 (20%)  
**下一步:** 开始 Week 2 - Service层整理

---

## 🎯 后续周次概览

由于完整计划非常长，以下是后续周次的概要。每个周次都遵循相同的模式：详细的任务分解、具体的文件操作、测试验证和Git提交。

### Week 2: Service层整理（目标: 512 → 462）
- 创建 service/ 子包
- 移动 ~50 个 service_*.go 文件
- 按领域分组
- 更新所有 import

### Week 3: Submission域扩充（目标: 462 → 432）
- 移动 ~30 个 submit_*.go 文件到 submission/
- 整合 SHEIN 提交相关文件
- 更新 import 和测试

### Week 4: 文档完善（目标: 432 → 432）
- 为 core/, service/, submission/, generation/, workflow/ 创建 README
- 更新主 README.md
- 记录架构决策

### Week 5: 最终验收（目标: 432 → <200，如可能）
- 运行完整测试套件
- 性能基准对比
- 代码质量检查
- Code Review
- 合并到主分支

---

## 🛡️ 风险控制措施

### 回滚策略

```bash
# 每周结束时创建tag
git tag v-refactor-week1-complete
git tag v-refactor-week2-complete
git tag v-refactor-week3-complete
git tag v-refactor-week4-complete
git tag v-refactor-week5-complete

# 如遇严重问题，回滚到上一个稳定版本
git checkout v-refactor-week1-complete
git checkout -b hotfix/rollback-from-week2
```

### 关键检查点

每个任务完成后必须：
1. ✅ 编译通过 (`go build ./...`)
2. ✅ 测试通过 (`go test ./...`)
3. ✅ 覆盖率稳定 (`go tool cover`)
4. ✅ Git提交并推送

---

## 📊 成功指标

### 定量指标
- 根目录文件数: 532 → <200 (目标减少≥60%)
- 测试覆盖率: ≥80%
- 编译时间: 增加 <10%
- 循环依赖: 0

### 定性指标
- 新成员能在1天内找到相关代码
- Code Review时间减少30%
- Bug定位时间减少50%

---

**Plan Status:** Ready for Execution  
**Next Step:** Choose execution approach (Subagent-Driven recommended)

