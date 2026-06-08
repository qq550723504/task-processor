# Studio 服务迁移详细计划

**创建日期:** 2026-06-08  
**状态:** 📋 计划阶段，待执行  
**预计工作量:** 2-3 天专注工作

## 概述

本计划描述了如何将 33 个 Studio 相关文件从 `internal/listingkit/` 根目录迁移到 `internal/listingkit/service/studio/` 子包。

## 文件清单

### Repository 接口（4 个主要接口）
1. `studio_async_job_repository.go` - StudioAsyncJobRepository
2. `studio_batch_repository.go` - StudioBatchRepository (1141 行)
3. `studio_batch_run_repository.go` - StudioBatchRunRepository
4. `studio_session_repository.go` - StudioSessionRepository + 3 个内部接口

### Model 类型（2 个主要文件）
1. `studio_batch_model.go` - Batch/Item/Attempt/Design 记录类型 (140 行)
2. `studio_session_model.go` - Session/Draft/Seed 记录类型 (~500 行)

### Service 实现（6 个独立服务）
1. `task_studio_batch_service.go` - taskStudioBatchService (1015 行)
2. `task_studio_batch_draft_service.go` - taskStudioBatchDraftService
3. `task_studio_batch_run_service.go` - taskStudioBatchRunService
4. `task_studio_batch_run_executor.go` - taskStudioBatchRunExecutor
5. `task_studio_media_service.go` - taskStudioMediaService
6. `task_studio_session_service.go` - taskStudioSessionService

### 辅助函数和工具（21 个文件）
- `shein_studio_*.go` (6 个) - SHEIN Studio 图片处理
- `studio_*.go` (13 个) - Studio 通用工具
- `workflow_studio_*.go` (2 个) - Temporal workflow 集成

## 依赖关系图

```
┌─────────────────────────────────────┐
│   task_studio_*.go (Services)      │
│   - 6 independent service structs   │
└──────────────┬──────────────────────┘
               │ depends on
               ▼
┌─────────────────────────────────────┐
│   *_repository.go (Interfaces)     │
│   - 4 main repository interfaces    │
└──────────────┬──────────────────────┘
               │ uses types from
               ▼
┌─────────────────────────────────────┐
│   *_model.go (Data Types)          │
│   - studio_batch_model.go           │
│   - studio_session_model.go         │
└──────────────┬──────────────────────┘
               │ references
               ▼
┌─────────────────────────────────────┐
│   shein_studio_*.go (Helpers)      │
│   - Image processing utilities      │
│   - Selection snapshots             │
└─────────────────────────────────────┘
```

## 迁移策略：渐进式提取（3 个 Phase）

### Phase 1: 迁移 Model 类型（最底层依赖）

**目标:** 将 Model 类型移动到 `service/studio/models.go`

**步骤:**
1. 创建 `service/studio/models.go` 文件
2. 复制 `studio_batch_model.go` 的内容
3. 复制 `studio_session_model.go` 的内容
4. 更新包名为 `package studio`
5. 导出所有公开类型（首字母大写）
6. 验证编译成功

**风险:** ⚠️ 中等
- `studio_session_model.go` 定义了 `SheinStudioSelectionSnapshot` 等类型
- 这些类型被其他非-studio 文件引用
- 需要检查跨包引用

**验证:**
```bash
go build ./internal/listingkit/service/studio/...
go test ./internal/listingkit/... -run Studio
```

### Phase 2: 迁移 Repository 接口

**目标:** 将 Repository 接口移动到 `service/studio/interfaces.go`

**步骤:**
1. 创建 `service/studio/interfaces.go` 文件
2. 复制 4 个 Repository 接口定义
3. 复制 Mem* 实现（内存版本用于测试）
4. 更新包名为 `package studio`
5. 导出所有公开接口
6. 在根目录添加类型别名（向后兼容）：
   ```go
   // internal/listingkit/studio_aliases.go
   package listingkit
   
   import "task-processor/internal/listingkit/service/studio"
   
   type StudioBatchRepository = studio.StudioBatchRepository
   type StudioSessionRepository = studio.StudioSessionRepository
   // ... 其他别名
   ```
7. 验证编译成功

**风险:** ⚠️ 高
- Repository 方法签名引用了 Model 类型
- 如果 Phase 1 未完成，会导致编译错误
- 必须先完成 Phase 1

**验证:**
```bash
go build ./internal/listingkit/...
go test ./internal/listingkit/... -count=1
```

### Phase 3: 迁移 Service 实现

**目标:** 将 6 个 task_studio*.go 文件移动到 `service/studio/`

**步骤:**
1. 逐个移动文件（每次一个）：
   - `task_studio_batch_service.go`
   - `task_studio_batch_draft_service.go`
   - `task_studio_batch_run_service.go`
   - `task_studio_batch_run_executor.go`
   - `task_studio_media_service.go`
   - `task_studio_session_service.go`
2. 更新每个文件的包名为 `package studio`
3. 更新 import 路径（如果需要引用根目录类型）
4. 导出服务结构体和构造函数
5. 每移动一个文件后立即验证编译
6. 全部移动后运行测试

**风险:** ⚠️⚠️ 非常高
- 服务文件可能引用根目录的其他类型
- 可能需要大量 import 路径更新
- 建议最后执行此阶段

**验证:**
```bash
# 每移动一个文件后
go build ./internal/listingkit/service/studio/...

# 全部移动后
go test ./internal/listingkit/... -v -count=1
```

## 详细执行步骤

### Step 1: 准备工作（30 分钟）

1. **创建备份分支**
   ```bash
   git checkout -b feature/studio-migration-backup
   git push origin feature/studio-migration-backup
   git checkout feature/listingkit-refactoring-phase1
   ```

2. **记录基线状态**
   ```bash
   # 统计当前文件数
   Get-ChildItem internal/listingkit -Filter "*studio*.go" | Measure-Object
   
   # 运行基线测试
   go test ./internal/listingkit/... -run Studio -v
   
   # 记录测试结果
   ```

3. **创建迁移脚本目录**
   ```bash
   mkdir scripts/studio-migration
   ```

### Step 2: Phase 1 - 迁移 Models（2-3 小时）

1. **分析 Model 依赖**
   ```bash
   # 查找哪些文件引用了 studio_batch_model.go 中的类型
   grep -r "StudioBatchRecord" internal/listingkit/*.go
   
   # 查找哪些文件引用了 studio_session_model.go 中的类型
   grep -r "SheinStudioSelectionSnapshot" internal/listingkit/*.go
   ```

2. **创建 models.go**
   ```powershell
   # scripts/studio-migration/create-models.ps1
   $batchModel = Get-Content internal/listingkit/studio_batch_model.go -Raw
   $sessionModel = Get-Content internal/listingkit/studio_session_model.go -Raw
   
   $content = @"
   // Package studio provides Studio-related business logic services.
   package studio
   
   import "time"
   
   // === From studio_batch_model.go ===
   $batchModel
   
   // === From studio_session_model.go ===
   $sessionModel
   "@
   
   Set-Content internal/listingkit/service/studio/models.go $content -NoNewline
   ```

3. **更新包名和导出**
   - 将所有 `type studioBatchStatus` 改为 `type StudioBatchStatus`（如果未导出）
   - 确保所有 const 都导出（首字母大写）
   - 移除重复的 import 语句

4. **验证编译**
   ```bash
   go build ./internal/listingkit/service/studio/...
   ```

5. **提交**
   ```bash
   git add internal/listingkit/service/studio/models.go
   git commit -m "refactor: extract studio models to service/studio/
   
   - Moved studio_batch_model.go types
   - Moved studio_session_model.go types
   - All types exported for cross-package use"
   ```

### Step 3: Phase 2 - 迁移 Repositories（3-4 小时）

1. **创建 interfaces.go**
   ```powershell
   # 复制 4 个 Repository 接口
   $files = @(
       "studio_async_job_repository.go",
       "studio_batch_repository.go",
       "studio_batch_run_repository.go",
       "studio_session_repository.go"
   )
   
   $content = "package studio`n`n"
   foreach ($f in $files) {
       $fileContent = Get-Content "internal/listingkit/$f" -Raw
       # 提取 interface 定义和 Mem* 实现
       $content += $fileContent + "`n`n"
   }
   
   Set-Content internal/listingkit/service/studio/interfaces.go $content -NoNewline
   ```

2. **更新包名和引用**
   - 将所有 `package listingkit` 改为 `package studio`
   - 更新对 Model 类型的引用（现在在同一个包中）

3. **创建向后兼容别名**
   ```go
   // internal/listingkit/studio_repository_aliases.go
   package listingkit
   
   import "task-processor/internal/listingkit/service/studio"
   
   // Repository aliases for backward compatibility
   type StudioBatchRepository = studio.StudioBatchRepository
   type StudioBatchRunRepository = studio.StudioBatchRunRepository
   type StudioSessionRepository = studio.StudioSessionRepository
   type StudioAsyncJobRepository = studio.StudioAsyncJobRepository
   ```

4. **验证编译**
   ```bash
   go build ./internal/listingkit/...
   ```

5. **提交**
   ```bash
   git add internal/listingkit/service/studio/interfaces.go
   git add internal/listingkit/studio_repository_aliases.go
   git commit -m "refactor: extract studio repositories to service/studio/
   
   - Moved 4 repository interfaces
   - Added backward compatibility aliases
   - Verified compilation succeeds"
   ```

### Step 4: Phase 3 - 迁移 Services（4-6 小时）

**重要:** 这是最复杂的阶段，建议分多次会话完成。

1. **移动第一个服务文件**
   ```powershell
   Copy-Item internal/listingkit/task_studio_batch_service.go internal/listingkit/service/studio/batch_service.go
   $content = Get-Content internal/listingkit/service/studio/batch_service.go -Raw
   $content = $content -replace '^package listingkit$', 'package studio'
   Set-Content internal/listingkit/service/studio/batch_service.go $content -NoNewline
   ```

2. **更新 import 路径**
   - 如果服务引用了根目录的类型，添加导入：
     ```go
     import "task-processor/internal/listingkit"
     ```

3. **验证编译**
   ```bash
   go build ./internal/listingkit/service/studio/...
   ```

4. **重复步骤 1-3** 对于其他 5 个服务文件

5. **删除根目录的旧文件**（确认编译成功后）
   ```powershell
   Remove-Item internal/listingkit/task_studio_*.go
   ```

6. **运行完整测试**
   ```bash
   go test ./internal/listingkit/... -v -count=1 -timeout=10m
   ```

7. **提交**
   ```bash
   git add internal/listingkit/service/studio/*.go
   git rm internal/listingkit/task_studio_*.go
   git commit -m "refactor: migrate studio services to service/studio/
   
   - Moved 6 task_studio*.go files
   - Updated package to studio
   - All tests passing"
   ```

### Step 5: 清理和优化（2-3 小时）

1. **移动辅助函数**（可选）
   - `shein_studio_*.go` → `service/studio/helpers/`
   - `studio_*.go`（非 model/repository）→ `service/studio/utils/`

2. **更新文档**
   - 更新 `service/studio/doc.go`
   - 添加迁移说明

3. **性能测试**
   ```bash
   go test ./internal/listingkit/... -bench=. -benchmem
   ```

4. **最终提交**
   ```bash
   git add .
   git commit -m "refactor: complete studio migration cleanup
   
   - Moved helper functions
   - Updated documentation
   - Performance verified"
   ```

## 风险控制

### 回滚策略

如果任何阶段出现严重问题：

```bash
# 立即回滚到备份分支
git checkout feature/studio-migration-backup

# 或者回滚到上一个成功的 commit
git reset --hard HEAD~1

# 强制推送（谨慎使用）
git push origin feature/listingkit-refactoring-phase1 --force
```

### 常见问题和解决方案

#### 问题 1: 循环依赖
**症状:** `import cycle not allowed`

**解决:**
- 检查是否有双向引用
- 将共享类型提取到独立的 `types.go` 文件
- 使用接口解耦

#### 问题 2: 未导出类型无法访问
**症状:** `cannot refer to unexported type`

**解决:**
- 将类型首字母大写（导出）
- 或者在同一个包中访问

#### 问题 3: 测试失败
**症状:** 编译成功但测试失败

**解决:**
- 检查测试文件是否也需要更新 import
- 验证 mock 对象是否正确
- 逐步调试失败的测试

## 成功指标

- ✅ 所有 33 个 studio 文件迁移到 `service/studio/`
- ✅ 根目录文件数减少 33 个
- ✅ 编译成功，无错误
- ✅ 所有测试通过（100% 通过率）
- ✅ 测试覆盖率保持 ≥68.4%
- ✅ 无循环依赖
- ✅ Git 历史清晰，每个 Phase 一个 commit

## 时间估算

| Phase | 预计时间 | 复杂度 |
|-------|---------|--------|
| 准备工作 | 30 分钟 | 低 |
| Phase 1: Models | 2-3 小时 | 中 |
| Phase 2: Repositories | 3-4 小时 | 高 |
| Phase 3: Services | 4-6 小时 | 非常高 |
| 清理和优化 | 2-3 小时 | 中 |
| **总计** | **11-16 小时** | - |

**建议:** 分 2-3 天完成，每天专注一个 Phase。

## 下一步行动

1. ✅ 阅读并理解本计划
2. ⏳ 创建备份分支
3. ⏳ 开始执行 Phase 1
4. ⏳ 每完成一个 Phase 后验证并提交
5. ⏳ 遇到问题时参考"常见问题和解决方案"

---

**备注:** 本计划基于当前的代码分析制定。实际执行时可能需要根据具体情况调整。关键是保持小步快跑，每次变更后立即验证。
