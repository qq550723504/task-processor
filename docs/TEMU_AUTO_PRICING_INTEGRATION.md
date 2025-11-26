# TEMU 自动核价集成文档

## 概述

参考SHEIN的自动核价实现，为TEMU平台添加了完整的自动核价功能，集成到主程序中，无需独立启动。

## 实现架构

### 核心组件

1. **AutoPricingHandler** (`platforms/temu/handlers/auto_pricing_handler.go`)
   - 自动核价处理器
   - 定时获取待核价商品
   - 根据规则计算目标价格
   - 提交重新报价

2. **APIClientManager** (`common/temu/api_client_manager.go`)
   - API客户端管理器
   - 缓存和复用客户端
   - 支持多店铺管理

3. **PricingAPI** (`common/temu/pricing_api.go`)
   - 获取待核价列表
   - 拒绝平台报价
   - 重新报价
   - 接受平台报价

## 集成方式

### 1. 在Processor中集成

参考SHEIN的实现，在`TemuProcessor`中添加自动核价处理器：

```go
type TemuProcessor struct {
    // ... 其他字段
    autoPricingHandler *handlers.AutoPricingHandler
}
```

### 2. 启动时自动运行

在`Start()`方法中根据配置启动：

```go
func (p *TemuProcessor) Start(ctx context.Context) error {
    // 启动自动核价处理器（如果启用）
    if p.config.Temu.AutoPricing.Enabled {
        autoPricingInterval := time.Duration(p.config.Temu.AutoPricing.Interval) * time.Second
        if autoPricingInterval <= 0 {
            autoPricingInterval = 30 * time.Minute
        }
        p.logger.Infof("[TEMU] 启动自动核价处理器，间隔: %v", autoPricingInterval)
        go p.autoPricingHandler.Start(ctx, autoPricingInterval)
    }
    return nil
}
```

## 配置说明

在`config/config-dev.yaml`中添加：

```yaml
temu:
  autoPricing:
    enabled: true
    interval: 1800  # 30分钟
    batchSize: 100
```

在`common/config/config.go`中添加：

```go
type Config struct {
    // ... 其他字段
    Temu TemuConfig
}

type TemuConfig struct {
    AutoPricing AutoPricingConfig `yaml:"autoPricing"`
}
```

## 核价流程

### 1. 获取店铺列表
```
getAllTemuStores() -> []*StoreRespDTO
```

### 2. 检查店铺配置
- 检查是否启用自动核价 (`EnableAutoPrice`)
- 获取核价规则

### 3. 获取待核价商品
```
GetPendingPriceList(pageNo, pageSize) -> []PendingPriceItem
```

### 4. 处理每个商品
- 获取原始成本价（从产品导入映射）
- 根据规则计算目标价格
- 提交重新报价

### 5. 核价规则应用

支持多种规则类型：
- `fixed`: 固定加价
- `percent`: 加价百分比
- `multiple`: 倍数
- `discount`: 折扣率
- `fixed_price`: 固定价格

## API接口

### 1. 获取待核价列表
```
POST /mms/marigold/sku/v2/search
```

### 2. 拒绝平台报价
```
POST /mms/marigold/sku/offline
```

### 3. 重新报价
```
POST /mms/marigold/price/appeal/order/create
```

### 4. 接受平台报价
```
POST /mms/marigold/price/goods/change
```

## 与SHEIN的差异

### 相同点
1. 都集成到主程序的Processor中
2. 都使用定时任务机制
3. 都支持核价规则配置
4. 都从管理系统获取店铺和规则信息

### 不同点
1. **API接口不同**
   - SHEIN: 使用议价接口 (`BargainPage`, `BatchHandleCostDiscuss`)
   - TEMU: 使用待核价列表和重新报价接口

2. **核价策略不同**
   - SHEIN: 比较建议价格和通过价格，决定接受/拒绝
   - TEMU: 根据规则计算目标价格，提交重新报价

3. **数据结构不同**
   - SHEIN: `BargainPageData` 包含SKU成本价历史
   - TEMU: `PendingPriceItem` 相对简单

## 使用方式

### 启动主程序
```bash
# TEMU自动核价会随主程序自动启动
go run cmd/temu-web/main.go
```

### 配置检查
```yaml
# 确保配置文件中启用了自动核价
temu:
  autoPricing:
    enabled: true
    interval: 1800
```

### 日志监控
```
[TEMU] 启动自动核价处理器，间隔: 30m0s
[TEMU] 开始执行TEMU自动核价任务
[TEMU] 找到 3 个TEMU店铺
[TEMU] 处理店铺: 测试店铺 (ID: 508)
[TEMU] 店铺 测试店铺 获取到 10 个待核价商品
[TEMU] 商品 XXX: 原始价格=50.00, 目标价格=60.00
[TEMU] 成功提交商品 XXX 的重新报价
[TEMU] 店铺 测试店铺 核价完成: 成功=8, 失败=2
[TEMU] TEMU自动核价任务执行完成，耗时: 5.2s
```

## 注意事项

1. **店铺配置**
   - 需要在管理系统中启用店铺的自动核价功能
   - 需要配置核价规则

2. **Cookie管理**
   - 确保店铺Cookie有效
   - Cookie过期会自动重新加载

3. **错误处理**
   - 单个商品失败不影响其他商品
   - 网络错误自动重试
   - Panic自动恢复

4. **性能考虑**
   - 建议间隔时间30分钟以上
   - 支持分页处理大量商品
   - 使用客户端缓存减少重复创建

## 店铺发现机制

参考SHEIN的设计，TEMU自动核价从已创建的API客户端中获取店铺信息：

1. **自动发现**: 当任务处理器处理任务时，会创建API客户端
2. **客户端缓存**: APIClientManager缓存已创建的客户端
3. **店铺提取**: 自动核价从缓存中提取店铺ID
4. **平台过滤**: 只处理platform="temu"的店铺

这种设计的优点：
- 无需额外的店铺列表API
- 只处理活跃的店铺（有任务处理的店铺）
- 自动适应新增店铺
- 减少不必要的API调用

## 后续优化

1. 支持更复杂的核价策略（如竞品价格分析）
2. 添加核价成功率统计
3. 支持手动触发核价
4. 添加核价历史记录
5. 支持按店铺配置不同的核价策略
6. 添加店铺列表API以支持主动发现所有店铺

## 测试

```bash
# 运行单元测试
go test ./common/temu -v
go test ./platforms/temu/handlers -v

# 启动主程序测试
go run cmd/temu-web/main.go
```

## 相关文件

- `platforms/temu/handlers/auto_pricing_handler.go` - 自动核价处理器
- `common/temu/api_client_manager.go` - API客户端管理器
- `common/temu/pricing_api.go` - 核价API接口
- `common/temu/pricing_types.go` - 数据类型定义
- `platforms/temu/processor.go` - TEMU处理器（集成点）
- `common/config/config.go` - 配置定义
- `config/config-dev.yaml` - 配置文件
