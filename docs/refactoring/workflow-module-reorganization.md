# Workflow 模块整理计划

## 📊 当前状态分析

### 根目录中的 workflow 文件 (38个)

#### Service 方法文件 (需要保留在根目录,因为依赖 `*service`)
1. `workflow.go` - `runWorkflow()` 主入口
2. `workflow_standard.go` - `runStandardProductWorkflow()` 
3. `workflow_shein.go` - `applyDefaultSheinPricing()`
4. `workflow_platform_adaptation.go` - `runPlatformAdaptation()`
5. `workflow_sds_sync.go` - `syncSDSDesign()`, `syncSDSDesignFromRemote()`

#### 可移动的独立函数/类型文件 (应移到 workflow/ 子模块)

**核心工作流逻辑:**
6. `workflow_state.go` - `workflowRecorder`, `workflowStageHandle` 类型定义
7. `workflow_result.go` - `initResult()`, `markChildTask()`, `appendWarning()`
8. `workflow_requests.go` - `toProductGenerateRequest()`, `toImageProcessRequest()`, `shouldProcessImages()`

**Phase 构建器 (独立逻辑):**
9. `workflow_standard_canonical_phase.go` - `buildStandardWorkflowCanonicalPhase()`, `standardWorkflowCanonicalPhase`
10. `workflow_standard_media_phase.go` - `buildStandardWorkflowMediaPhase()`, `standardWorkflowMediaPhase`
11. `workflow_standard_asset_phase.go` - `buildStandardWorkflowAssetPhase()`, `standardWorkflowAssetPhase`

**平台适配 Phase:**
12. `workflow_platform_finalize_phase.go` - `buildPlatformFinalizePhase()`, `buildPlatformPostprocessPhase()`
13. `workflow_platform_asset_dispatch_phase.go` - `buildPlatformAssetDispatchPhase()`
14. `workflow_platform_asset_dispatch_apply.go` - `applyPlatformAssetDispatchMutation()`
15. `workflow_platform_asset_dispatch_persist.go` - `buildPlatformAssetDispatchPersistPhase()`
16. `workflow_platform_asset_dispatch_inventory_apply.go` - `buildPlatformAssetDispatchInventoryApplyPhase()`
17. `workflow_platform_asset_dispatch_inventory_persist.go` - `buildPlatformAssetDispatchInventoryPersistPhase()`
18. `workflow_platform_asset_dispatch_bundle_apply.go` - `buildPlatformAssetDispatchBundleApplyPhase()`
19. `workflow_platform_asset_dispatch_bundle_reshape.go` - `buildPlatformAssetDispatchBundleReshapePhase()`
20. `workflow_platform_asset_dispatch_task_merge.go` - `buildPlatformAssetDispatchTaskMergePhase()`

**Studio Fallback 逻辑:**
21. `workflow_studio_fallback.go` - `shouldUseStudioProductFallback()`, `buildStudioFallbackCanonicalProduct()`
22. `workflow_studio_sds_metadata.go` - `studioCategoryPath()`, `studioAttributes()`, `firstNonEmptyString()`

**Review State 处理:**
23. `workflow_review_state.go` - `applySheinInspectionReviewToSummary()`, `addSheinReviewWorkflowIssues()`

**SDS 状态管理:**
24. `workflow_sds_state.go` - `finishSDSStageWithError()`, `isSDSAuthRequiredError()`

**测试文件 (跟随源文件移动):**
25. `workflow_state_test.go` → `workflow/workflow_state_test.go`
26. `workflow_review_state_test.go` → `workflow/workflow_review_state_test.go`
27. `workflow_scene_options_test.go` → `workflow/workflow_scene_options_test.go`
28. `workflow_sds_fallback_test.go` → `workflow/workflow_sds_fallback_test.go`
29. `workflow_timeout_test.go` → `workflow/workflow_timeout_test.go`
30. `workflow_assets_test.go` → `workflow/workflow_assets_test.go`
31. `workflow_model_generation_test.go` → `workflow/workflow_model_generation_test.go`
32. `workflow_shein_content_test.go` → `workflow/workflow_shein_content_test.go`
33. `workflow_studio_fallback_test.go` → `workflow/workflow_studio_sds_metadata_test.go` (合并或单独)

---

## 🎯 重构策略

### 方案: 分层移动 (推荐)

#### 第一层: 移动纯工具函数和类型 (无 service 依赖)
- `workflow_state.go` - recorder 和 stage handle
- `workflow_result.go` - result 初始化辅助函数
- `workflow_requests.go` - 请求转换函数
- `workflow_sds_state.go` - SDS 状态辅助函数

**优势:**
- 这些是完全独立的工具代码
- 不依赖 `*service`,可以安全导出
- 立即提升模块清晰度

#### 第二层: 移动 Phase 构建器 (部分依赖 service)
对于需要 `*service` 的 phase 构建器,有两种选择:

**选项 A: 保持分离 (保守)**
- Phase 构建器留在根目录,因为它们需要访问 service 的字段
- 只移动纯逻辑到 `workflow/`
- 优点: 简单,风险低
- 缺点: 模块边界仍不够清晰

**选项 B: 通过接口解耦 (系统)**
- 定义 workflow 需要的 service 接口
- Phase 构建器移到 `workflow/`,通过接口注入依赖
- 优点: 清晰的模块边界,更好的可测试性
- 缺点: 需要额外的接口定义,工作量较大

**推荐: 先执行第一层,评估效果后再决定是否执行第二层**

---

## 📋 执行计划

### Task 1: 移动核心工具函数 (预计 1-2 小时)

1. 移动以下文件到 `workflow/`:
   - `workflow_state.go` → `workflow/recorder.go` (重命名更清晰)
   - `workflow_result.go` → `workflow/result_helpers.go`
   - `workflow_requests.go` → `workflow/request_converters.go`
   - `workflow_sds_state.go` → `workflow/sds_state_helpers.go`

2. 更新 package 声明为 `package workflow`

3. 导出公共 API (如需要):
   - `workflowRecorder` → `Recorder`
   - `workflowStageHandle` → `StageHandle`
   - `newWorkflowRecorder()` → `NewRecorder()`

4. 在根目录创建包装器或直接引用:
   ```go
   // 在 service.go 或其他文件中
   import "task-processor/internal/listingkit/workflow"
   
   func (s *service) someMethod() {
       recorder := workflow.NewRecorder(result)
       // ...
   }
   ```

5. 运行测试验证

### Task 2: 移动 Studio Fallback 逻辑 (预计 1 小时)

1. 移动文件:
   - `workflow_studio_fallback.go` → `workflow/studio_fallback.go`
   - `workflow_studio_sds_metadata.go` → `workflow/studio_metadata.go`

2. 这些函数不依赖 `*service`,可以直接移动

3. 更新调用处导入

### Task 3: 移动 Review State 处理 (预计 30 分钟)

1. 移动文件:
   - `workflow_review_state.go` → `workflow/review_state.go`

2. 检查是否需要导出

### Task 4: 评估 Phase 构建器 (决策点)

完成前三层后,评估:
- 当前模块清晰度是否足够?
- 是否需要进一步解耦 phase 构建器?
- 团队对当前结构的反馈如何?

如果决定继续,则执行选项 B (接口解耦),否则暂停。

---

## ⚠️ 风险评估

### 低风险项 ✅
- 移动纯工具函数 (Task 1)
- 移动 studio fallback (Task 2)
- 移动 review state (Task 3)

**原因:**
- 这些函数不依赖 `*service`
- 可以轻松导出为公共 API
- 影响范围可控

### 中风险项 ⚠️
- Phase 构建器移动 (如果需要执行)

**缓解措施:**
- 先定义清晰的接口
- 小步移动,每次移动后立即测试
- 保持向后兼容的包装器

### 高风险项 ❌
- 移动 service 方法文件 (`workflow.go`, `workflow_standard.go` 等)

**建议:**
- **不要移动**这些文件
- 它们属于 service 层的编排逻辑
- 保持在根目录是合理的

---

## 📊 预期收益

### 代码组织改进
- ✅ workflow 子模块包含 25+ 个相关文件
- ✅ 根目录减少约 20 个文件
- ✅ 职责边界更清晰

### 可维护性提升
- ✅ 相关逻辑集中在一起
- ✅ 更容易找到特定功能的实现
- ✅ 降低认知负担

### 可测试性增强
- ✅ 独立的工具函数更容易单元测试
- ✅ 可以通过接口 mock service 依赖
- ✅ 测试文件与源文件在同一目录

---

## 🔄 回滚策略

如果移动后发现问题:

1. **立即回滚**: 
   ```bash
   git revert <commit-hash>
   ```

2. **部分回滚**:
   - 保留成功的移动
   - 回滚有问题的文件

3. **兼容层**:
   - 在根目录创建包装函数
   - 逐步迁移调用处

---

## 📝 下一步行动

**建议顺序:**
1. ✅ 执行 Task 1 (核心工具函数)
2. ✅ 运行完整测试套件
3. ✅ 提交并审查
4. ⏸️ 评估是否继续 Task 2-3
5. ⏸️ 根据团队反馈决定是否执行 Task 4

**预计总时间:**
- 仅 Task 1: 1-2 小时
- Task 1-3: 3-4 小时
- 完整执行 (含 Task 4): 1-2 天

---

## 💡 关键决策点

### 决策 1: 是否导出类型?
- `workflowRecorder` 是否需要公开?
- 如果只在 listingkit 内部使用,保持私有
- 如果需要被其他包使用,导出为 `Recorder`

### 决策 2: Phase 构建器的依赖处理
- 是否值得为 phase 构建器定义接口?
- 还是接受它们留在根目录?
- 这取决于团队的架构偏好

### 决策 3: 测试文件的组织
- 测试文件是否跟随源文件移动?
- 还是保持在根目录统一存放?
- 推荐: 跟随源文件,便于维护

---

**准备就绪,等待用户确认后开始执行!**
