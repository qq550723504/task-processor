# API 子模块整理计划

## 📊 当前状态

`api/` 子模块包含 **78 个文件**,职责混杂,需要按功能域分组。

---

## 🔍 文件分类分析

### 1. Admin 管理后台 (12个文件)
**职责**: 系统管理、配置管理、规则管理

```
admin_category_handler.go              - 类目管理
admin_filter_rule_handler.go           - 过滤规则
admin_generation_topic_override_handler.go - 生成主题覆盖
admin_generation_topic_policy_handler.go   - 生成主题策略
admin_import_task_handler.go           - 导入任务管理
admin_operation_strategy_handler.go    - 运营策略
admin_pricing_rule_handler.go          - 定价规则
admin_product_data_handler.go          - 产品数据
admin_product_import_mapping_handler.go - 产品导入映射
admin_profit_rule_handler.go           - 利润规则
admin_sensitive_word_handler.go        - 敏感词管理
admin_store_handler.go                 - 店铺管理
```

**依赖文件**:
- `admin_dependencies.go` - Admin 依赖注入
- `admin_handler_options_test.go` - 测试辅助
- `admin_handlers_shape_test.go` - 测试辅助

**建议子目录**: `api/admin/`

---

### 2. Studio 工作室 (9个文件)
**职责**: 批量处理、会话管理、异步任务

```
studio_async_jobs_handler.go           - 异步任务管理
studio_batch_runs_handler.go           - 批次运行
studio_batches_handler_test.go         - 批次测试
studio_designs_handler.go              - 设计管理
studio_product_images_handler.go       - 产品图片
studio_sessions_handler.go             - 会话管理
```

**相关服务文件**:
- `settings_service.go` - 设置服务 (可能被 studio 使用)

**建议子目录**: `api/studio/`

---

### 3. Shein 平台专用 (7个文件)
**职责**: SHEIN 平台特定功能

```
shein_category_search_handler.go       - 类目搜索
shein_customer_flow_handler.go         - 客户流程
shein_image_regeneration_handler.go    - 图片重新生成
shein_resolution_cache_handler.go      - 分辨率缓存
shein_sync_handler.go                  - 同步处理
shein_sync_summary_handler.go          - 同步摘要
```

**测试辅助文件**:
- `shein_category_search_handler_test_stubs_test.go`
- `shein_image_regeneration_test_stubs_test.go`
- `shein_resolution_cache_handler_test_stubs_test.go`

**建议子目录**: `api/shein/`

---

### 4. Generation 生成相关 (3个文件)
**职责**: 内容生成、导航分发

```
generation_navigation_dispatch_handler.go      - 导航分发
generation_tasks_handler.go                    - 生成任务
generation_tasks_handler_test.go               - 生成任务测试
```

**建议子目录**: `api/generation/`

---

### 5. Task 任务管理 (4个文件)
**职责**: 任务恢复、重试、队列管理

```
task_recovery_handler.go               - 任务恢复
task_recovery_handler_test.go          - 任务恢复测试
task_requeue_handler.go                - 任务重入队
task_requeue_handler_test.go           - 任务重入队测试
```

**建议子目录**: `api/task/`

---

### 6. History 历史记录 (4个文件)
**职责**: 历史查询、详情查看

```
history_handler.go                     - 历史列表
history_handler_test.go                - 历史列表测试
history_detail_handler.go              - 历史详情
history_detail_handler_test.go         - 历史详情测试
```

**建议子目录**: `api/history/`

---

### 7. Revision 版本管理 (4个文件)
**职责**: 版本修订、验证

```
revision_handler.go                    - 版本修订
revision_handler_test.go               - 版本修订测试
revision_validate_handler.go           - 版本验证
revision_validate_handler_test.go      - 版本验证测试
```

**建议子目录**: `api/revision/`

---

### 8. SDS 基线管理 (2个文件)
**职责**: SDS 基线数据管理

```
sds_baseline_handler.go                - SDS 基线
sds_baseline_handler_test.go           - SDS 基线测试
```

**建议子目录**: `api/sds/`

---

### 9. Settings 设置管理 (2个文件)
**职责**: 命名空间设置

```
settings_namespace_handler.go          - 命名空间设置
settings_namespace_handler_test.go     - 命名空间设置测试
```

**建议子目录**: `api/settings/`

---

### 10. Store 店铺管理 (2个文件)
**职责**: 店铺资料管理

```
store_profile_handler.go               - 店铺资料
store_profile_handler_test.go          - 店铺资料测试
```

**建议子目录**: `api/store/`

---

### 11. Subscription 订阅管理 (3个文件)
**职责**: 订阅处理

```
subscription_handler.go                - 订阅处理
subscription_handler_test.go           - 订阅处理测试
subscription_dependencies.go           - 订阅依赖
subscription_dependencies_test.go      - 订阅依赖测试
```

**建议子目录**: `api/subscription/`

---

### 12. Upload 上传管理 (3个文件)
**职责**: 文件上传

```
upload_handler.go                      - 上传处理
upload_handler_test.go                 - 上传处理测试
upload_file_reader.go                  - 文件读取器
```

**建议子目录**: `api/upload/`

---

### 13. 核心/通用文件 (保持根目录)

这些文件是 API 层的核心基础设施,应该保持在 `api/` 根目录:

```
handler.go                             - 核心 Handler 接口和实现 ⭐
conditional_read.go                    - 条件读取工具
tenant_context.go                      - 租户上下文
tenant_context_test.go                 - 租户上下文测试
tenant_store_handler.go                - 租户店铺 (可能属于 store/)
ai_settings_handler.go                 - AI 设置 (可能属于 settings/)
export_handler.go                      - 导出 (可能属于 generation/)
preview_handler.go                     - 预览 (可能属于 generation/)
submit_handler.go                      - 提交 (可能属于 task/)
submit_handler_test.go                 - 提交测试
child_task_retry_handler_test.go       - 子任务重试测试
handler_constructor_test.go            - Handler 构造测试
handler_dependencies_test.go           - Handler 依赖测试
```

---

## 🎯 推荐的目录结构

```
api/
├── admin/                    # 管理后台 (12 handlers + 3 deps/tests)
│   ├── category_handler.go
│   ├── filter_rule_handler.go
│   ├── generation_topic_override_handler.go
│   ├── generation_topic_policy_handler.go
│   ├── import_task_handler.go
│   ├── operation_strategy_handler.go
│   ├── pricing_rule_handler.go
│   ├── product_data_handler.go
│   ├── product_import_mapping_handler.go
│   ├── profit_rule_handler.go
│   ├── sensitive_word_handler.go
│   ├── store_handler.go
│   ├── dependencies.go
│   ├── handler_options_test.go
│   └── handlers_shape_test.go
│
├── studio/                   # 工作室 (6 handlers + tests)
│   ├── async_jobs_handler.go
│   ├── async_jobs_handler_test.go
│   ├── batch_runs_handler.go
│   ├── batch_runs_handler_test.go
│   ├── batches_handler_test.go
│   ├── designs_handler.go
│   ├── product_images_handler.go
│   └── sessions_handler.go
│
├── shein/                    # SHEIN 平台 (6 handlers + 3 stubs)
│   ├── category_search_handler.go
│   ├── category_search_handler_test_stubs_test.go
│   ├── customer_flow_handler.go
│   ├── image_regeneration_handler.go
│   ├── image_regeneration_test_stubs_test.go
│   ├── resolution_cache_handler.go
│   ├── resolution_cache_handler_test_stubs_test.go
│   ├── sync_handler.go
│   ├── sync_handler_test.go
│   ├── sync_summary_handler.go
│   └── sync_summary_handler_test.go (如果存在)
│
├── generation/               # 生成相关 (2 handlers + tests)
│   ├── navigation_dispatch_handler.go
│   ├── navigation_dispatch_handler_test.go
│   ├── tasks_handler.go
│   └── tasks_handler_test.go
│
├── task/                     # 任务管理 (2 handlers + tests)
│   ├── recovery_handler.go
│   ├── recovery_handler_test.go
│   ├── requeue_handler.go
│   └── requeue_handler_test.go
│
├── history/                  # 历史记录 (2 handlers + tests)
│   ├── handler.go
│   ├── handler_test.go
│   ├── detail_handler.go
│   └── detail_handler_test.go
│
├── revision/                 # 版本管理 (2 handlers + tests)
│   ├── handler.go
│   ├── handler_test.go
│   ├── validate_handler.go
│   └── validate_handler_test.go
│
├── sds/                      # SDS 基线 (1 handler + test)
│   ├── baseline_handler.go
│   └── baseline_handler_test.go
│
├── settings/                 # 设置管理 (1 handler + test + service)
│   ├── namespace_handler.go
│   ├── namespace_handler_test.go
│   └── service.go
│
├── store/                    # 店铺管理 (1 handler + test)
│   ├── profile_handler.go
│   └── profile_handler_test.go
│
├── subscription/             # 订阅管理 (1 handler + deps + tests)
│   ├── handler.go
│   ├── handler_test.go
│   ├── dependencies.go
│   └── dependencies_test.go
│
├── upload/                   # 上传管理 (1 handler + reader + test)
│   ├── handler.go
│   ├── handler_test.go
│   └── file_reader.go
│
├── handler.go                # ⭐ 核心 Handler 接口
├── conditional_read.go       # 条件读取工具
├── tenant_context.go         # 租户上下文
├── tenant_context_test.go    # 租户上下文测试
├── ai_settings_handler.go    # AI 设置 (待决定归属)
├── export_handler.go         # 导出 (待决定归属)
├── preview_handler.go        # 预览 (待决定归属)
├── submit_handler.go         # 提交 (待决定归属)
├── submit_handler_test.go    # 提交测试
├── child_task_retry_handler_test.go  # 子任务重试测试
├── handler_constructor_test.go       # Handler 构造测试
├── handler_dependencies_test.go      # Handler 依赖测试
└── tmp/                      # 临时文件
```

---

## 📋 执行计划

### Phase 1: 创建子目录结构 (30分钟)

```bash
mkdir -p api/admin
mkdir -p api/studio
mkdir -p api/shein
mkdir -p api/generation
mkdir -p api/task
mkdir -p api/history
mkdir -p api/revision
mkdir -p api/sds
mkdir -p api/settings
mkdir -p api/store
mkdir -p api/subscription
mkdir -p api/upload
```

### Phase 2: 移动 Admin 相关文件 (1小时)

1. 移动 12 个 admin handler 文件
2. 移动 admin_dependencies.go
3. 移动 admin 测试辅助文件
4. 更新 package 声明 (如果需要)
5. 运行测试验证

### Phase 3: 移动 Studio 相关文件 (45分钟)

1. 移动 6 个 studio handler 文件
2. 移动相关测试文件
3. 验证测试通过

### Phase 4: 移动 Shein 相关文件 (45分钟)

1. 移动 6 个 shein handler 文件
2. 移动测试 stub 文件
3. 验证测试通过

### Phase 5: 移动其他功能域 (2小时)

按以下顺序移动:
- generation/ (2 handlers)
- task/ (2 handlers)
- history/ (2 handlers)
- revision/ (2 handlers)
- sds/ (1 handler)
- settings/ (1 handler + service)
- store/ (1 handler)
- subscription/ (1 handler + deps)
- upload/ (1 handler + reader)

每移动一组后立即运行测试。

### Phase 6: 处理边界文件 (30分钟)

决定以下文件的归属:
- `tenant_store_handler.go` → store/ 或保持根目录?
- `ai_settings_handler.go` → settings/ 或保持根目录?
- `export_handler.go` → generation/ 或保持根目录?
- `preview_handler.go` → generation/ 或保持根目录?
- `submit_handler.go` → task/ 或保持根目录?

### Phase 7: 更新导入路径 (1小时)

搜索并更新所有引用这些 handler 的代码:
```bash
grep -r "listingkit/api" --include="*.go" | grep -v "^Binary"
```

可能需要更新:
- HTTP 路由注册代码
- 依赖注入代码
- 测试代码

### Phase 8: 最终验证 (30分钟)

1. 运行完整测试套件
2. 检查编译错误
3. 验证 HTTP 路由正常工作
4. Git 提交

---

## ⚠️ 风险评估

### 低风险 ✅
- 移动 handler 文件本身 (纯 HTTP 层)
- 每个子目录独立,无交叉依赖
- 测试文件跟随源文件移动

### 中风险 ⚠️
- 更新导入路径可能影响路由注册
- 需要确保所有引用都正确更新
- 可能有外部包引用这些 handler

### 高风险 ❌
- 如果 handler 之间有复杂的跨文件依赖
- 如果 `handler.go` 中的类型被广泛引用

**缓解措施**:
- 小步移动,每次移动后立即测试
- 保持向后兼容的 import 别名
- 详细记录所有更改

---

## 📊 预期收益

### 代码组织改进
- ✅ 78 个文件分组到 12 个子目录
- ✅ 每个子目录职责清晰
- ✅ 更容易找到特定功能的 handler

### 可维护性提升
- ✅ 相关功能集中在一起
- ✅ 降低认知负担
- ✅ 便于添加新功能

### 团队协作改善
- ✅ 不同团队可以负责不同子目录
- ✅ Code review 更聚焦
- ✅ 减少合并冲突

---

## 🔄 回滚策略

如果移动后发现问题:

1. **立即回滚**:
   ```bash
   git revert <commit-hash>
   ```

2. **部分回滚**:
   - 保留成功的移动
   - 回滚有问题的子目录

3. **兼容层**:
   - 在根目录创建类型别名
   - 逐步迁移调用处

---

## 💡 关键决策点

### 决策 1: 是否重命名文件?
当前文件名如 `admin_category_handler.go`,移动到 `admin/` 后可以简化为 `category_handler.go`

**选项 A**: 保持完整名称 (`admin_category_handler.go`)
- 优点: 文件名自描述,即使单独看也知道属于 admin
- 缺点: 冗余,目录已经表明了归属

**选项 B**: 简化名称 (`category_handler.go`)
- 优点: 简洁,符合 Go 惯例
- 缺点: 单独看文件时不知道属于哪个模块

**推荐**: 选项 B (简化名称),因为目录已经提供了上下文

### 决策 2: 边界文件的归属
对于 `tenant_store_handler.go`, `ai_settings_handler.go` 等文件:

**原则**:
- 如果只被一个功能域使用 → 移到该功能域
- 如果被多个功能域使用 → 保持根目录
- 如果是核心基础设施 → 保持根目录

### 决策 3: 是否更新导入路径?
移动后,其他包的导入需要从:
```go
import "task-processor/internal/listingkit/api"
```
改为:
```go
import "task-processor/internal/listingkit/api/admin"
```

**策略**:
- 先移动文件
- 编译失败会提示需要更新的导入
- 逐一修复

---

## 📝 下一步行动

**准备就绪,等待用户确认后开始执行!**

建议执行顺序:
1. ✅ 创建子目录结构
2. ✅ 移动 Admin 文件 (最大的一组,先试水)
3. ✅ 运行测试验证
4. ✅ 继续移动其他组
5. ✅ 最终验证和提交

预计总时间: **5-6 小时**

---

**您希望我:**
1. **立即开始执行** - 从 Phase 1 开始?
2. **先审查计划** - 讨论目录结构和分组?
3. **调整方案** - 有不同的分组想法?

请告诉我您的决定! 🚀
