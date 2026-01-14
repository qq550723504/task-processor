# 编译问题汇总与修复计划

## 当前状态

### ✅ 已完成
- 统一调度器核心框架 (`internal/app/scheduler/`) ✅ 编译通过
- TEMU 平台调度器 (`internal/platforms/temu/scheduler/`) ✅ 编译通过
- SHEIN 平台调度器 (`internal/platforms/shein/scheduler/`) ✅ 编译通过
- 核价服务重构 (`internal/app/service/pricing_service*.go`)
- SHEIN utils 工具包 (`internal/platforms/shein/utils/`) ✅ 编译通过
- SHEIN sync 服务包 (`internal/platforms/shein/service/sync/`) ✅ 编译通过

### 🎉 重大进展

**架构层面代码已全部修复** - 所有调度器相关的架构层面代码现在都可以独立编译

**Service 层部分修复完成** - sync 包已完成修复并可以编译

### 🔧 正在修复: Service 层其他包

已完成:
- ✅ `sync/` - 同步服务 (11个文件全部修复)

待修复:
- 🔧 `pricing/` - 核价服务
- 🔧 `inventory/` - 库存服务
- 🔧 `product/` - 产品服务
- 🔧 `strategy/` - 策略服务
- 🔧 其他服务包

### 📝 修复策略

对于复杂的 service 层代码,采用以下策略:
1. 修复类型引用错误 (`ShopAPIClient` → `*client.APIClient`)
2. 修复字段名错误 (根据实际 DTO 定义)
3. 简化复杂实现,标记 TODO 待后续完善
4. 确保每个包可以独立编译

### 🔧 待修复的编译错误 (service 层历史遗留问题)

## 1. SHEIN Pricing Service 问题

### 文件: `internal/platforms/shein/service/pricing/strategy_service.go`

**错误:**
```
undefined: ShopAPIClient
undefined: ShelfOperationManager
undefined: PriceUpdater
undefined: StockUpdater
```

**原因:** 这些类型未定义或导入路径错误

**修复方案:**
1. 检查这些类型应该从哪里导入
2. 可能需要从 `internal/platforms/shein/repo/client` 导入 `APIClient`
3. 其他类型可能需要重新定义或从正确的包导入

### 文件: `internal/platforms/shein/service/pricing/price_service.go`

**错误:**
```
undefined: ShopAPIClient
undefined: SheinProductResponse
```

**修复方案:**
1. `ShopAPIClient` 应该改为 `*client.APIClient`
2. `SheinProductResponse` 应该从 `internal/platforms/shein/model` 导入

## 2. SHEIN Sync Service 问题

### 文件: `internal/platforms/shein/service/sync/activity_converter_service.go`

**错误:**
```
undefined: marketing.ActivityProduct
```

**修复方案:**
1. 检查 `internal/platforms/shein/api/marketing` 包中是否定义了 `ActivityProduct`
2. 如果没有,需要定义这个类型
3. 或者从其他地方导入正确的类型

### 文件: `internal/platforms/shein/service/sync/activity_fetcher_service.go`

**错误:**
```
not enough arguments in call to client.NewBaseAPIClient
    have (*client.APIClient)
    want (string, int64, int64, *req.Client)
```

**修复方案:**
`NewBaseAPIClient` 的签名可能不正确,需要检查:
1. 查看 `client.NewBaseAPIClient` 的实际签名
2. 可能需要传递更多参数
3. 或者修改 `NewBaseAPIClient` 的实现

### 文件: `internal/platforms/shein/service/sync/activity_service.go`

**错误:**
```
cannot use backendProducts (variable of type []ActivityProductDTO) 
as []*ActivityProductDTO value
```

**修复方案:**
类型不匹配,需要:
1. 修改 `ConvertToBackendFormat` 返回 `[]*ActivityProductDTO`
2. 或者修改 `BatchSaveActivityProducts` 接受 `[]ActivityProductDTO`

### 文件: `internal/platforms/shein/service/sync/data_enricher_service.go`

**错误:**
```
mappingResp.Asin undefined
```

**修复方案:**
1. 检查 `ProductImportMappingRespDTO` 的字段名
2. 可能是 `ASIN` 而不是 `Asin`
3. 或者字段名完全不同

### 文件: `internal/platforms/shein/service/sync/registration_service.go`

**错误:**
```
undefined: ShopAPIClient
```

**修复方案:**
改为使用 `*client.APIClient`

## 3. 其他依赖问题

### 文件: `internal/platforms/shein/utils/data_enricher.go`

**错误:**
```
package task-processor/internal/platforms/shein/modules is not in std
```

**修复方案:**
1. 这个包路径不存在
2. 需要找到正确的导入路径
3. 或者删除这个文件(如果已经不需要)

## 修复优先级

### 高优先级 (影响核心功能)
1. ✅ 统一调度器框架 - 已完成
2. ✅ TEMU 调度器 - 已完成
3. ✅ 核价服务重构 - 已完成
4. 🔧 SHEIN 同步服务类型问题
5. 🔧 SHEIN 定价服务类型问题

### 中优先级 (功能完善)
6. 🔧 ActivityProduct 类型定义
7. 🔧 BaseAPIClient 参数问题
8. 🔧 数据转换类型匹配

### 低优先级 (清理工作)
9. 🔧 删除或修复 data_enricher.go
10. 🔧 清理未使用的导入

## 修复策略

### 阶段1: 类型定义修复
1. 统一 API 客户端类型引用
2. 定义缺失的模型类型
3. 修正导入路径

### 阶段2: 接口适配
1. 修复 `NewBaseAPIClient` 调用
2. 统一数据转换接口
3. 修正类型转换

### 阶段3: 清理优化
1. 删除未使用的文件
2. 优化导入语句
3. 添加必要的文档

## 建议的修复顺序

1. **先修复类型引用问题**
   - 将所有 `ShopAPIClient` 改为 `*client.APIClient`
   - 将所有 `SheinProductResponse` 改为 `*model.SheinProductResponse`

2. **然后修复缺失的类型定义**
   - 定义 `marketing.ActivityProduct`
   - 检查并修复其他缺失的类型

3. **最后修复接口调用问题**
   - 修复 `NewBaseAPIClient` 调用
   - 修复类型转换问题

4. **清理工作**
   - 删除或修复 `data_enricher.go`
   - 清理未使用的导入

## 注意事项

1. **保持业务逻辑不变**: 重构时只修复编译错误,不改变业务逻辑
2. **逐步修复**: 一次修复一个文件或一类问题
3. **测试验证**: 修复后及时编译验证
4. **文档更新**: 重要的修改需要更新文档

## 当前可用的功能

即使有编译错误,以下功能已经可用:

✅ **统一调度器核心**
- Manager: 任务管理
- Registry: 工厂注册
- TaskExecutor: 任务执行
- Types: 类型定义

✅ **TEMU 平台**
- 任务工厂
- 核价任务
- 同步/库存/活动任务框架

✅ **核价服务**
- 使用新调度器的核价服务
- 支持多平台扩展

## 下一步行动

建议按以下顺序进行:

1. 创建类型定义修复的 PR
2. 修复 SHEIN sync service 的编译错误
3. 修复 SHEIN pricing service 的编译错误
4. 清理未使用的文件
5. 完整编译测试
6. 更新文档

## 相关文档

- `docs/SCHEDULER_REFACTORING.md` - 调度器架构重构
- `docs/PRICING_SERVICE_REFACTORING.md` - 核价服务重构
- `internal/app/scheduler/README.md` - 调度器使用文档
