# API Handlers

本包包含 ListingKit 的所有 HTTP handlers,提供 RESTful API 接口。

## 📋 概述

所有 handlers 都是 `handler` 结构体的方法,通过依赖注入模式管理各种服务和服务 repository。

### 核心架构

```
handler (api/handler.go)
├── 30+ 个服务/Repository 字段
├── adminHandlers (嵌入结构)
│   ├── storeAdminHandlers
│   └── catalogAdminHandlers
├── subscriptionDependencies (嵌入结构)
└── settingsService

HandlerOption 函数 → 依赖注入
路由注册 → Gin Router
```

---

## 🗂️ 文件组织

Handlers 按功能域分组到不同文件,使用前缀标识:

### 🔧 Admin (管理后台) - 15个文件

系统管理、配置管理、规则管理相关接口。

**Store Admin (店铺管理)**:
- `admin_store_handler.go` - 店铺 CRUD
- `admin_import_task_handler.go` - 导入任务管理

**Catalog Admin (类目管理)**:
- `admin_category_handler.go` - 类目管理
- `admin_filter_rule_handler.go` - 过滤规则
- `admin_profit_rule_handler.go` - 利润规则
- `admin_pricing_rule_handler.go` - 定价规则
- `admin_operation_strategy_handler.go` - 运营策略
- `admin_sensitive_word_handler.go` - 敏感词管理
- `admin_generation_topic_override_handler.go` - 生成主题覆盖
- `admin_generation_topic_policy_handler.go` - 生成主题策略
- `admin_product_import_mapping_handler.go` - 产品导入映射
- `admin_product_data_handler.go` - 产品数据管理

**依赖注入**:
- `admin_dependencies.go` - Admin 依赖配置
- `admin_handler_options_test.go` - 测试辅助
- `admin_handlers_shape_test.go` - 测试辅助

---

### 🎨 Studio (工作室) - 6个文件

批量处理、会话管理、异步任务相关接口。

- `studio_async_jobs_handler.go` - 异步任务管理
- `studio_batch_runs_handler.go` - 批次运行管理
- `studio_batches_handler_test.go` - 批次测试
- `studio_designs_handler.go` - 设计管理
- `studio_product_images_handler.go` - 产品图片管理
- `studio_sessions_handler.go` - 会话管理

---

### 🛍️ Shein (SHEIN平台) - 9个文件

SHEIN 平台特定功能接口。

- `shein_category_search_handler.go` - 类目搜索
- `shein_customer_flow_handler.go` - 客户流程
- `shein_image_regeneration_handler.go` - 图片重新生成
- `shein_resolution_cache_handler.go` - 分辨率缓存
- `shein_sync_handler.go` - 同步处理 (主接口)
- `shein_sync_summary_handler.go` - 同步摘要和仪表板

**测试辅助**:
- `shein_category_search_handler_test_stubs_test.go`
- `shein_image_regeneration_test_stubs_test.go`
- `shein_resolution_cache_handler_test_stubs_test.go`

---

### ⚙️ Generation (内容生成) - 4个文件

内容生成、导航分发相关接口。

- `generation_navigation_dispatch_handler.go` - 导航分发
- `generation_navigation_dispatch_handler_test.go` - 导航分发测试
- `generation_tasks_handler.go` - 生成任务管理
- `generation_tasks_handler_test.go` - 生成任务测试

---

### 📋 Task (任务管理) - 4个文件

任务恢复、重试、队列管理相关接口。

- `task_recovery_handler.go` - 任务恢复
- `task_recovery_handler_test.go` - 任务恢复测试
- `task_requeue_handler.go` - 任务重入队
- `task_requeue_handler_test.go` - 任务重入队测试

---

### 📜 History (历史记录) - 4个文件

历史查询、详情查看相关接口。

- `history_handler.go` - 历史列表
- `history_handler_test.go` - 历史列表测试
- `history_detail_handler.go` - 历史详情
- `history_detail_handler_test.go` - 历史详情测试

---

### 📝 Revision (版本管理) - 4个文件

版本修订、验证相关接口。

- `revision_handler.go` - 版本修订
- `revision_handler_test.go` - 版本修订测试
- `revision_validate_handler.go` - 版本验证
- `revision_validate_handler_test.go` - 版本验证测试

---

### 📊 SDS (基线管理) - 2个文件

SDS 基线数据管理接口。

- `sds_baseline_handler.go` - SDS 基线管理
- `sds_baseline_handler_test.go` - SDS 基线测试

---

### ⚙️ Settings (设置管理) - 3个文件

命名空间设置相关接口。

- `settings_namespace_handler.go` - 命名空间设置
- `settings_namespace_handler_test.go` - 命名空间设置测试
- `settings_service.go` - 设置服务实现

---

### 🏪 Store (店铺资料) - 2个文件

店铺资料管理接口。

- `store_profile_handler.go` - 店铺资料管理
- `store_profile_handler_test.go` - 店铺资料测试

---

### 📦 Subscription (订阅管理) - 4个文件

订阅处理相关接口。

- `subscription_handler.go` - 订阅处理 (主接口)
- `subscription_handler_test.go` - 订阅处理测试
- `subscription_dependencies.go` - 订阅依赖配置
- `subscription_dependencies_test.go` - 订阅依赖测试

---

### 📤 Upload (上传管理) - 3个文件

文件上传相关接口。

- `upload_handler.go` - 上传处理
- `upload_handler_test.go` - 上传处理测试
- `upload_file_reader.go` - 文件读取器

---

### 🔗 核心/通用文件

这些文件提供核心基础设施,被多个功能域使用。

- `handler.go` - ⭐ **核心 Handler 结构体和接口定义**
- `conditional_read.go` - 条件读取工具函数
- `tenant_context.go` - 租户上下文管理
- `tenant_context_test.go` - 租户上下文测试
- `tenant_store_handler.go` - 租户店铺关联
- `ai_settings_handler.go` - AI 设置管理
- `export_handler.go` - 数据导出
- `preview_handler.go` - 预览生成
- `submit_handler.go` - 任务提交
- `submit_handler_test.go` - 提交测试
- `child_task_retry_handler_test.go` - 子任务重试测试
- `handler_constructor_test.go` - Handler 构造测试
- `handler_dependencies_test.go` - Handler 依赖测试

---

## 🏗️ 架构设计

### Handler 结构体

```go
type handler struct {
    // 核心服务
    taskLifecycleService       listingkit.TaskLifecycleService
    taskRecoveryService        listingkit.TaskRecoveryService
    taskRequeueService         listingkit.TaskRequeueService
    generationTaskService      listingkit.GenerationTaskService
    
    // Studio 服务
    studioMediaService         listingkit.StudioMediaService
    studioBatchRunService      studioBatchRunHandlerService
    studioSessionService       studioSessionAsyncJobService
    
    // Admin 服务
    storeAdminService          listingkit.StoreAdminService
    storeRepository            listingadmin.StoreRepository
    
    // Shein 服务
    sheinSyncService           listingkit.SheinSyncService
    sheinCandidateService      listingkit.SheinCandidateService
    sheinEnrollmentService     listingkit.SheinEnrollmentService
    sheinSyncRepository        listingkit.SheinSyncRepository
    
    // 嵌入结构
    adminHandlers              // Admin handlers 集合
    subscriptionDependencies   // Subscription 依赖
    
    // 其他
    settingsService *settingsService
    initErr         error
}
```

### 依赖注入模式

使用 `HandlerOption` 函数式选项模式:

```go
type HandlerOption func(h *handler)

func WithTaskLifecycleService(svc listingkit.TaskLifecycleService) HandlerOption {
    return func(h *handler) {
        h.taskLifecycleService = svc
    }
}

// 使用示例
h := newHandler(
    WithTaskLifecycleService(taskService),
    WithStoreRepository(storeRepo),
    WithSheinSyncService(sheinSvc),
)
```

### Admin Handlers 嵌入结构

```go
type adminHandlers struct {
    storeAdminHandlers
    catalogAdminHandlers
}

type storeAdminHandlers struct {
    storeHandler           *listingadmin.StoreHandler
    storeStatisticsHandler *listingadmin.StoreStatisticsHandler
    importTaskHandler      *listingadmin.ImportTaskHandler
}

type catalogAdminHandlers struct {
    filterRuleHandler              *listingadmin.FilterRuleHandler
    profitRuleHandler              *listingadmin.ProfitRuleHandler
    pricingRuleHandler             *listingadmin.PricingRuleHandler
    // ... 更多 handlers
}
```

这种设计允许:
- 模块化组织 Admin handlers
- 通过 `h.categoryHandler` 直接访问
- 在 `admin_dependencies.go` 中集中配置

---

## 🚀 添加新 Handler

### 步骤 1: 确定功能域

根据功能选择对应的文件前缀:
- Admin 相关 → `admin_*_handler.go`
- Studio 相关 → `studio_*_handler.go`
- Shein 相关 → `shein_*_handler.go`
- 等等...

### 步骤 2: 添加 Handler 方法

在对应文件中添加方法:

```go
func (h *handler) MyNewHandler(c *gin.Context) {
    // 1. 权限检查
    if !h.requireSubscription(c, listingsubscription.ModuleSomeModule) {
        return
    }
    
    // 2. 参数解析
    var req MyRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // 3. 业务逻辑
    result, err := h.someService.DoSomething(c.Request.Context(), req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    // 4. 返回结果
    c.JSON(http.StatusOK, result)
}
```

### 步骤 3: 添加依赖 (如需要)

如果需要新的服务或 Repository:

1. **添加到 handler 结构体** (`handler.go`):
   ```go
   type handler struct {
       // ... existing fields
       myNewService MyNewService
   }
   ```

2. **创建 With* 选项函数** (`handler.go`):
   ```go
   func WithMyNewService(svc MyNewService) HandlerOption {
       return func(h *handler) {
           h.myNewService = svc
       }
   }
   ```

3. **更新依赖注入** (如属于 Admin):
   在 `admin_dependencies.go` 中添加配置

### 步骤 4: 注册路由

在路由注册代码中添加:

```go
apiGroup.POST("/my-endpoint", h.MyNewHandler)
```

### 步骤 5: 编写测试

创建 `*_test.go` 文件:

```go
func TestMyNewHandler(t *testing.T) {
    // 1. 准备测试数据
    // 2. 创建 handler (使用 mock 服务)
    // 3. 发起请求
    // 4. 验证响应
}
```

---

## 🧪 测试策略

### Mock 服务

使用接口 mock 外部依赖:

```go
type mockTaskService struct {
    listingkit.TaskLifecycleService
}

func (m *mockTaskService) GetTask(ctx context.Context, id string) (*listingkit.Task, error) {
    return &listingkit.Task{ID: id}, nil
}
```

### 测试辅助函数

在 `*_test.go` 文件中定义 stub:

```go
func newTestHandler(t *testing.T) *handler {
    h := &handler{
        taskLifecycleService: &mockTaskService{},
        // ... 其他 mock 服务
    }
    return h
}
```

### 运行测试

```bash
# 运行所有 API 测试
go test ./internal/listingkit/api/... -v

# 运行特定功能域测试
go test ./internal/listingkit/api -run TestAdmin -v
```

---

## 📚 相关文档

- [ListingKit 模块边界分析](../../refactoring/module-boundary-analysis.md)
- [API 模块整理决策](../../refactoring/api-reorganization-decision.md)
- [Handler 依赖注入最佳实践](../../architecture/handler-dependency-injection.md) (待创建)

---

## ❓ 常见问题

### Q: 为什么不将 handlers 拆分到子目录?

**A**: 所有 handlers 都是 `handler` 结构体的方法,该结构体定义了 30+ 个私有字段。Go 的包系统不允许在子包中为父包的未导出类型定义方法。强行拆分会导致:
- 循环依赖
- 需要导出 `handler` 类型 (breaking change)
- 依赖注入系统重构

当前的扁平结构 + 文件命名规范已经提供了足够的组织性。

### Q: 如何快速找到某个功能的 handler?

**A**: 使用文件前缀搜索:
- Admin 相关: `admin_*`
- Studio 相关: `studio_*`
- Shein 相关: `shein_*`
- 或使用 IDE 的全局搜索

### Q: handler.go 文件太大怎么办?

**A**: `handler.go` 目前 328 行,包含:
- 结构体定义 (~50 行)
- 接口定义 (~30 行)
- 构造函数 (~100 行)
- 选项函数 (~100 行)
- 通用方法 (~50 行)

如果继续增长,可以考虑:
- 将选项函数移到 `handler_options.go`
- 将接口定义移到 `interfaces.go`
- 保持构造函数在 `handler.go`

### Q: 如何避免 handler 结构体过度膨胀?

**A**: 
1. 使用嵌入结构组织相关字段 (如 `adminHandlers`)
2. 延迟初始化不常用的服务
3. 考虑将相关服务组合成 facade
4. 定期审查,移除未使用的字段

---

## 🔄 更新日志

- **2026-06-08**: 创建初始文档,记录当前组织结构
- **未来**: 随着功能迭代持续更新

---

**最后更新**: 2026-06-08  
**维护者**: ListingKit Team
