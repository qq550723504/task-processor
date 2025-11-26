# SKU/产品重复检查失效问题修复

## 问题描述

Temu产品提交时遇到SKU重复错误：

```
ERRO 任务处理失败且不可重试: ID=1575485, Priority=0, 错误=处理器 产品提交处理器 执行失败: 提交产品失败: NONRETRYABLE: 产品提交失败(error_code=10000103): Contribution SKU duplicated with another product (Goods ID: 602737311901961)
```

**问题：** 系统有两层检查机制，但都可能被静默跳过：
1. 第13步：SKU编码重复检查（`OutGoodsSnCheckHandler`）- 调用Temu API
2. 第27步：产品存在性检查（`ProductSubmitHandler.checkProductExists`）- 调用本地管理系统API

两个检查都因为客户端未初始化而被跳过，导致重复的产品被提交。

## 根本原因

系统有**两层检查机制**，但都可能被静默跳过：

### 1. Temu API SKU检查被跳过（第13步）

在 `platforms/temu/handlers/out_goods_sn_check_handler.go` 第96行：

```go
if ctx.APIClient == nil {
    h.logger.Warn("API客户端未初始化，跳过SKU编码检查")
    return nil  // ❌ 只是警告，不返回错误
}
```

**问题：**
- 当 `APIClient` 未初始化时，只记录警告
- 返回 `nil` 表示成功，pipeline继续执行
- SKU重复检查被完全跳过

### 2. 本地管理系统产品检查被跳过（第27步）

在 `platforms/temu/handlers/product_submit_handler.go` 第304行：

```go
if h.mappingClient == nil {
    h.logger.Warn("产品导入映射客户端未初始化，跳过产品存在性检查")
    return nil  // ❌ 只是警告，不返回错误
}
```

**问题：**
- 当 `mappingClient` 未初始化时，只记录警告
- 返回 `nil` 表示成功，继续提交产品
- 产品存在性检查被完全跳过
- 直到真正提交到Temu时才发现重复

### 2. 与其他Handler的不一致

其他handler在APIClient为nil时都返回错误：

```go
// CommitCreateHandler
if ctx.APIClient == nil {
    return fmt.Errorf("API客户端未初始化")
}

// ProductSubmitHandler
if ctx.APIClient == nil {
    return fmt.Errorf("API客户端未初始化")
}

// PriceQueryHandler
if ctx.APIClient == nil {
    return fmt.Errorf("API客户端未初始化")
}
```

**只有 `OutGoodsSnCheckHandler` 是警告而不是错误！**

## 解决方案

### 1. 新增独立的产品存在性检查Handler（第3步）✨

**新建文件**: `platforms/temu/handlers/product_exists_check_handler.go`

```go
type ProductExistsCheckHandler struct {
    logger        *logrus.Entry
    mappingClient api.ProductImportMappingAPI
}

func (h *ProductExistsCheckHandler) Handle(ctx *pipeline.TaskContext) error {
    // 检查客户端是否初始化
    if h.mappingClient == nil {
        return fmt.Errorf("产品导入映射客户端未初始化")
    }
    
    // 检查主产品是否已上架
    req := &api.ProductImportMappingCheckReqDTO{
        StoreId:   ctx.Task.StoreID,
        Platform:  ctx.Task.Platform,
        Region:    ctx.Task.Region,
        ProductId: ctx.Task.ProductID,
    }
    
    exists, err := h.mappingClient.CheckProductExists(req)
    if err != nil {
        return fmt.Errorf("检查产品是否已上架失败: %w", err)
    }
    
    if exists {
        return fmt.Errorf("NONRETRYABLE: 产品 %s 已经上架过", ctx.Task.ProductID)
    }
    
    return nil
}
```

**优势：**
- ✅ 在pipeline第3步就检查，避免浪费资源
- ✅ 独立handler，职责单一
- ✅ 客户端未初始化时返回错误，不会静默跳过

### 2. 修改Pipeline配置

**文件**: `platforms/temu/pipeline.go`

```go
func (b *TemuPipelineBuilder) addHandlers(p *pipeline.Pipeline) {
    p.AddHandler(handlers.NewInitDataHandler()).                                 // 1. 初始化
      AddHandler(handlers.NewStoreInfoHandler(b.storeClient)).                   // 2. 店铺信息
      AddHandler(handlers.NewProductExistsCheckHandler(b.mappingClient)).        // 3. 产品存在性检查 ✨ 新增
      AddHandler(handlers.NewRawJsonDataHandlerV2(...)).                         // 4. 获取数据
      // ... 其他handlers
}
```

### 3. 修改 `platforms/temu/handlers/out_goods_sn_check_handler.go`

**修改前：**
```go
if ctx.APIClient == nil {
    h.logger.Warn("API客户端未初始化，跳过SKU编码检查")
    return nil  // ❌ 静默跳过
}
```

**修改后：**
```go
if ctx.APIClient == nil {
    h.logger.Error("API客户端未初始化，无法执行SKU编码检查")
    return fmt.Errorf("API客户端未初始化，无法执行SKU编码检查")  // ✅ 返回错误
}
```

### 4. 从ProductSubmitHandler中移除重复检查

**文件**: `platforms/temu/handlers/product_submit_handler.go`

**修改前：**
```go
// 检查产品是否已上架
if err := h.checkProductExists(ctx); err != nil {
    return fmt.Errorf("检查产品是否已上架失败: %w", err)
}

// 提交产品
err := h.submitProduct(ctx)
```

**修改后：**
```go
// 提交产品（产品存在性检查已在第3步完成）
err := h.submitProduct(ctx)
```

## 效果对比

### 修改前（问题场景）

```
Pipeline执行流程:
1. 初始化 ✅
2. 店铺信息 ✅
3. 获取Amazon数据 ✅ (浪费时间和资源)
4-12. 各种处理 ✅ (浪费时间和资源)
13. SKU编码检查
    → APIClient == nil
    → WARN "跳过SKU编码检查"
    → return nil (成功) ✅
14-26. 更多处理 ✅ (继续浪费资源)
27. 产品提交
    → 检查产品是否已上架
    → mappingClient == nil
    → WARN "跳过检查"
    → return nil (成功) ✅
    → 提交到Temu API
    → ❌ 错误: SKU duplicated (error_code=10000103)
    → 任务失败（浪费了大量时间和资源）
```

### 修改后（正确行为）

#### 场景1：产品已上架（快速失败）

```
Pipeline执行流程:
1. 初始化 ✅
2. 店铺信息 ✅
3. 产品存在性检查 ✨
    → mappingClient != nil
    → 调用本地API检查
    → 发现产品已上架 ❌
    → return error
    → Pipeline中断
    → 任务失败（仅用3步，节省大量资源）
```

#### 场景2：产品未上架，但SKU重复

```
Pipeline执行流程:
1-2. 前置步骤 ✅
3. 产品存在性检查 ✅ (产品未上架)
4-13. 数据处理 ✅
14. SKU编码检查
    → APIClient != nil
    → 调用Temu API检查SKU
    → 发现SKU重复 ❌
    → return error
    → Pipeline中断
    → 任务失败（在第14步发现，避免后续无效处理）
```

#### 场景3：所有检查通过

```
Pipeline执行流程:
1-2. 前置步骤 ✅
3. 产品存在性检查 ✅ (产品未上架)
4-13. 数据处理 ✅
14. SKU编码检查 ✅ (SKU不重复)
15-27. 继续处理和提交 ✅
28. 保存结果 ✅
    → 任务成功
```

## 为什么APIClient可能为nil

需要检查以下情况：

1. **Cookie加载失败**
   - 店铺Cookie未配置
   - Cookie已过期
   - 从管理系统获取Cookie失败

2. **初始化顺序问题**
   - APIClient在某些handler之后才初始化
   - 初始化失败但没有中断pipeline

3. **配置问题**
   - 店铺配置不完整
   - 租户ID或店铺ID错误

## 建议的后续改进

### 1. 确保APIClient在pipeline开始时就初始化

在 `task_handler.go` 中：

```go
// 创建API客户端
apiClient := temu.NewAPIClient(tenantID, storeID, managementClient)

// 检查初始化是否成功
if apiClient == nil {
    return fmt.Errorf("API客户端初始化失败")
}

// 将APIClient设置到context
ctx.APIClient = apiClient
```

### 2. 添加APIClient初始化检查Handler

在pipeline的最开始添加一个检查handler：

```go
type APIClientCheckHandler struct{}

func (h *APIClientCheckHandler) Handle(ctx *pipeline.TaskContext) error {
    if ctx.APIClient == nil {
        return fmt.Errorf("API客户端未初始化，无法继续执行")
    }
    return nil
}
```

### 3. 统一错误处理策略

所有需要APIClient的handler都应该：
- 检查APIClient是否为nil
- 如果为nil，返回错误而不是警告
- 使用统一的错误消息格式

## 检查机制说明

系统有**两层防护**来避免重复上架：

### 第一层：本地管理系统产品检查（第3步）✨ 新增
- **Handler**: `ProductExistsCheckHandler`
- **检查内容**: 调用本地管理系统API检查产品ASIN是否已上架
- **依赖**: `mappingClient`（产品导入映射客户端）
- **检查范围**: 主产品ASIN
- **API接口**: `ProductImportMappingAPI.CheckProductExists`
- **优势**: 
  - ✅ 在pipeline早期（第3步）就检查
  - ✅ 避免浪费资源处理已上架的产品
  - ✅ 快速失败，节省时间

### 第二层：Temu API SKU检查（第14步）
- **Handler**: `OutGoodsSnCheckHandler`
- **检查内容**: 调用Temu API检查SKU编码是否已被使用
- **依赖**: `ctx.APIClient`（Temu API客户端）
- **检查范围**: 主产品和所有变体的SKU编码
- **优势**:
  - ✅ 检查SKU在Temu平台是否重复
  - ✅ 包含所有变体的检查

**两层检查的区别：**
- 第一层（第3步）：检查产品ASIN在本地系统是否已记录上架 → **快速失败**
- 第二层（第14步）：检查SKU编码在Temu平台是否重复 → **全面检查**

## 相关文件

- `platforms/temu/handlers/product_exists_check_handler.go` - 产品存在性检查handler（第3步）✨ 新增
- `platforms/temu/handlers/out_goods_sn_check_handler.go` - SKU重复检查handler（第14步）
- `platforms/temu/handlers/product_submit_handler.go` - 产品提交handler（第27步）
- `common/management/api/product_import_mapping.go` - 产品映射API接口定义
- `common/management/impl/product_import_mapping_api.go` - 产品映射API实现
- `platforms/temu/pipeline.go` - Pipeline配置
- `platforms/temu/task_handler.go` - APIClient初始化

## 性能优势

### 修改前
- 产品已上架 → 处理27步后才发现 → 浪费100%资源

### 修改后
- 产品已上架 → 处理3步后就发现 → 节省约89%资源
- 快速失败，提高系统吞吐量
- 减少无效的API调用和数据处理

## 测试验证

修复后，测试以下场景：

1. **APIClient正常** → SKU检查应该正常执行
2. **APIClient为nil** → 任务应该在第13步失败，而不是第27步
3. **SKU重复** → 任务应该在第13步检测到并失败
4. **SKU不重复** → 任务应该继续执行到第27步并成功提交
