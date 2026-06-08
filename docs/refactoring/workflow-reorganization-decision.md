# Workflow 模块整理决策记录

## 📋 背景

在方案 B (系统性重构) 中,我们计划整理 workflow 相关文件,将根目录中的 38 个 workflow_*.go 文件移动到 `workflow/` 子模块。

## 🔍 分析结果

### 文件依赖关系分析

经过详细分析,发现 38 个 workflow 文件的依赖情况如下:

#### 强依赖 service 的文件 (5个) - ❌ 不能移动
- `workflow.go` - `runWorkflow()` 需要访问 service 字段
- `workflow_standard.go` - `runStandardProductWorkflow()` 需要 service 方法
- `workflow_shein.go` - `applyDefaultSheinPricing()` 需要 service 缓存
- `workflow_platform_adaptation.go` - `runPlatformAdaptation()` 需要 service 客户端
- `workflow_sds_sync.go` - `syncSDSDesign()` 需要 service SDS 服务

#### 依赖 listingkit 核心类型的文件 (19个) - ⚠️ 移动成本高
这些文件使用以下类型:
- `ListingKitResult`
- `Task`
- `GenerateRequest`
- `WorkflowStage`
- `WorkflowIssue`
- `WorkflowIssueSeverity`
- `WorkflowStageStatus`

这些类型定义在根目录的 model 文件中,如果移动需要:
1. 将这些类型移到 workflow 包并导出
2. 或者创建大量类型别名
3. 或者接受循环依赖

**风险**: 这会导致:
- 大量的 breaking changes
- 其他包需要更新导入路径
- 测试文件也需要大量修改
- 可能引入循环依赖

#### 相对独立的文件 (14个) - ✅ 可以移动但收益有限
- Phase 构建器函数 (如 `buildStandardWorkflowCanonicalPhase`)
- 工具函数 (如 `applyPlatformAssetDispatchMutation`)
- Studio fallback 逻辑

但这些函数仍然:
- 返回的类型属于 listingkit 包
- 被 service 方法直接调用
- 移动后需要在根目录创建包装器

### 尝试执行的结果

我尝试移动以下文件到 `workflow/`:
1. `workflow_state.go` → `workflow/recorder.go`
2. `workflow_result.go` → `workflow/result_helpers.go`
3. `workflow_sds_state.go` → `workflow/sds_state_helpers.go`

**遇到的问题**:
```
internal\listingkit\workflow\recorder.go:10:10: undefined: ListingKitResult
internal\listingkit\workflow\recorder.go:43:38: undefined: WorkflowIssueSeverity
internal\listingkit\workflow\result_helpers.go:8:23: undefined: Task
...
```

**解决方案选项**:
1. **导出所有相关类型** - 工作量巨大,影响范围广
2. **创建类型别名** - 增加认知负担,不符合 Go 惯例
3. **保持现状** - 最务实的选择

## ✅ 最终决策

### 决定: **不执行大规模文件移动**

**理由:**

1. **技术债务 > 收益**
   - 移动文件需要重构大量类型定义
   - 可能导致循环依赖
   - 测试文件需要同步移动和修改
   - 预计需要 2-3 天,但收益有限

2. **当前结构已经合理**
   - Service 方法留在根目录是正确的 (它们属于 service 层)
   - Phase 构建器虽然多,但职责清晰
   - 文件命名规范 (`workflow_*`),易于查找

3. **更好的替代方案**
   - 通过文档化明确模块边界
   - 添加 package-level 注释说明职责
   - 未来可以通过接口解耦,而不是物理移动

### 替代改进方案

#### 方案 1: 文档化改进 (推荐) ⭐
- 为每个 workflow 文件添加清晰的注释
- 创建模块依赖图
- 编写架构决策记录 (ADR)

**工作量**: 2-3 小时  
**收益**: 提高可理解性,无破坏性更改

#### 方案 2: 小范围优化
只移动真正独立且高价值的文件:
- `workflow_studio_fallback.go` (不依赖 service)
- `workflow_review_state.go` (纯状态处理)

**工作量**: 1-2 小时  
**收益**: 小幅提升模块清晰度

#### 方案 3: 接口化重构 (长期)
- 定义 workflow 需要的 service 接口
- 将 phase 构建器移到 `workflow/`,通过接口注入依赖
- 逐步迁移,保持向后兼容

**工作量**: 3-5 天  
**收益**: 清晰的模块边界,更好的可测试性

## 📊 对比分析

| 方案 | 工作量 | 风险 | 收益 | 推荐度 |
|------|--------|------|------|--------|
| 大规模移动 (原计划) | 2-3天 | 高 | 中 | ❌ |
| 文档化改进 | 2-3小时 | 低 | 中 | ⭐⭐⭐ |
| 小范围优化 | 1-2小时 | 低 | 低 | ⭐⭐ |
| 接口化重构 | 3-5天 | 中 | 高 | ⭐⭐ (长期) |

## 🎯 下一步行动

### 立即执行 (本次会话)
✅ **暂停大规模文件移动**
✅ **提交已完成的 submit_lock 移动** (已完成)
⏸️ **等待团队审查和反馈**

### 短期改进 (下次会话)
1. 为 workflow 子模块添加详细的 README
2. 绘制模块依赖图
3. 为关键文件添加架构注释

### 长期规划 (未来迭代)
1. 评估是否需要接口化重构
2. 收集团队反馈
3. 根据实际需求决定是否进一步拆分

## 💡 经验教训

### 学到的经验

1. **模块边界分析很重要**
   - 在执行重构前,必须深入分析依赖关系
   - 表面的文件命名不足以判断是否应该移动

2. **类型系统的约束**
   - Go 的包系统对跨包类型引用有严格要求
   - 移动文件不仅仅是物理位置变化,还涉及类型可见性

3. **渐进式重构的价值**
   - `submit_lock` 的成功移动证明了小步快跑的可行性
   - 大规模重构风险高,应该分阶段执行

4. **务实优于完美**
   - 理论上完美的架构可能不切实际
   - 在当前代码库约束下,选择最务实的方案

### 后续改进建议

1. **建立模块边界规范**
   - 明确什么应该放在子模块
   - 定义子模块与根目录的交互方式

2. **自动化检查**
   - 添加 lint 规则检测模块边界违规
   - CI/CD 中集成架构检查

3. **团队共识**
   - 与团队讨论模块组织策略
   - 确保所有人理解并遵循规范

## 📝 相关文档

- [模块边界分析](./module-boundary-analysis.md)
- [第二阶段进展报告](./phase2-progress-report.md)
- [ListingKit 重构改进计划](../../plans/ListingKit_重构改进计划_2e5b810c.md)

---

**决策日期**: 2026-06-08  
**决策者**: AI Assistant + User  
**状态**: 已确认  
