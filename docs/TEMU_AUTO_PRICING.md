# TEMU 自动核价功能文档

## 概述

TEMU自动核价功能参考SHEIN的实现，提供定时自动拒绝平台报价服务，支持单店铺和多店铺模式。

**注意：** 当前实现为自动拒绝所有待核价商品的平台报价。

## 架构设计

### 核心组件

1. **APIClient** (`common/temu/api_client.go`)
   - 封装TEMU API调用
   - 管理Cookie和认证
   - 提供重试机制

2. **PricingAPI** (`common/temu/pricing_api.go`)
   - 获取待核价列表
   - 批量核价操作
   - 自动核价所有商品

3. **PricingScheduler** (`common/temu/scheduler.go`)
   - 单店铺定时任务调度
   - 支持启动/停止
   - Panic恢复机制

4. **SchedulerManager** (`common/temu/scheduler_manager.go`)
   - 多店铺调度管理
   - 动态添加/移除店铺
   - 统一生命周期管理

## API接口

### 1. 获取待核价列表

```go
func (c *APIClient) GetPendingPriceList(pageNo, pageSize int) (*PendingPriceListResponse, error)
```

**请求参数：**
- `pageNo`: 页码（从1开始）
- `pageSize`: 每页数量

**响应数据：**
```json
{
  "success": true,
  "result": {
    "list": [...],
    "total": 100
  }
}
```

### 2. 拒绝平台报价

```go
func (c *APIClient) RejectPrice(goodsID string, skuIDs []string) (*RejectPriceResponse, error)
```

**请求参数：**
- `goodsID`: 商品ID
- `skuIDs`: SKU ID列表

**响应数据：**
```json
{
  "success": true,
  "error_code": 1000000,
  "result": {}
}
```

### 3. 重新报价

```go
func (c *APIClient) ReappealPrice(goodsID string, skuInfoList []ReappealSkuInfo, appealSource int, appealReasons []string) (*ReappealPriceResponse, error)
```

**请求参数：**
- `goodsID`: 商品ID
- `skuInfoList`: SKU信息列表
- `appealSource`: 申诉来源（100表示价格健康页面）
- `appealReasons`: 申诉原因列表

**SKU信息结构：**
```go
type ReappealSkuInfo struct {
    SkuID                       string // SKU ID
    SupplierPriceStr            string // 当前供应商价格
    RecommendedSupplierPriceStr string // 平台推荐价格
    TargetSupplierPriceStr      string // 目标报价
    Currency                    string // 货币单位
}
```

**响应数据：**
```json
{
  "success": true,
  "error_code": 1000000,
  "result": {}
}
```

### 4. 接受平台报价

```go
func (c *APIClient) AcceptPrice(goodsID string, skuList []AcceptPriceSkuInfo, scene int) (*AcceptPriceResponse, error)
```

**请求参数：**
- `goodsID`: 商品ID
- `skuList`: SKU信息列表
- `scene`: 场景（2表示价格健康页面）

**SKU信息结构：**
```go
type AcceptPriceSkuInfo struct {
    SkuID                  string // SKU ID
    Currency               string // 货币单位
    TargetSupplierPriceStr string // 目标价格，空字符串表示接受平台推荐价格
}
```

**响应数据：**
```json
{
  "success": true,
  "error_code": 1000000,
  "result": {}
}
```

### 5. 自动拒绝所有待核价商品

```go
func (c *APIClient) AutoRejectAllPendingPrices() error
```

自动分页获取所有待核价商品并逐个拒绝报价。

### 6. 自动接受所有待核价商品

```go
func (c *APIClient) AutoAcceptAllPendingPrices() error
```

自动分页获取所有待核价商品并逐个接受报价（使用平台推荐价格）。

## 使用方式

### 方式1: 命令行工具

```bash
# 单店铺模式 - 拒绝报价
go run cmd/temu-pricing/main.go -tenant=1001 -store=2001 -interval=30m -action=reject

# 单店铺模式 - 接受报价
go run cmd/temu-pricing/main.go -tenant=1001 -store=2001 -interval=30m -action=accept

# 多店铺模式
go run cmd/temu-pricing/main.go -interval=30m -action=reject
```

### 方式2: 代码集成

```go
import (
    "time"
    "task-processor/common/management"
    "task-processor/common/temu"
)

// 创建管理客户端
managementClient := management.NewClientManager()

// 创建调度器管理器（拒绝报价）
schedulerManager := temu.NewSchedulerManager(managementClient, 30*time.Minute, temu.ActionReject)

// 或创建接受报价的管理器
// schedulerManager := temu.NewSchedulerManager(managementClient, 30*time.Minute, temu.ActionAccept)

// 添加店铺
schedulerManager.AddStore(1001, 2001)

// 停止所有调度器
defer schedulerManager.StopAll()
```

## 配置说明

在 `config/config-dev.yaml` 中添加：

```yaml
temu:
  autoPricing:
    enabled: true
    interval: 1800  # 30分钟
    batchSize: 100
```

## 错误处理

### 1. Cookie过期

当检测到Cookie过期时：
- 自动尝试重新加载Cookie
- 设置暂停键通知管理系统
- 返回 `AuthExpiredError`

### 2. 网络错误

- 自动重试3次
- 使用指数退避策略
- 记录详细错误日志

### 3. Panic恢复

调度器内置Panic恢复机制，确保单次失败不影响后续执行。

## 日志说明

```
[INFO] 启动自动核价定时任务，间隔: 30m0s
[INFO] 开始执行自动拒绝报价任务
[INFO] 获取待核价列表: pageNo=1, pageSize=100
[INFO] 成功获取待核价列表: 总数=150, 当前页数量=100
[INFO] 拒绝平台报价: goodsID=602906963083875, skuIDs数量=1
[INFO] 成功拒绝平台报价
[INFO] 自动拒绝报价完成: 总处理=150, 成功=145, 失败=5
[INFO] 自动拒绝报价任务执行成功，耗时: 5.2s
```

## 最佳实践

1. **间隔时间设置**
   - 建议30分钟以上
   - 避免频繁请求导致限流

2. **批量大小**
   - 建议100个/批次
   - 根据实际情况调整

3. **监控告警**
   - 监控成功率
   - 关注Cookie过期
   - 记录异常日志

4. **优雅退出**
   - 支持SIGINT/SIGTERM信号
   - 30秒超时保护
   - 确保任务完整性

## 测试

```bash
# 运行单元测试
go test ./common/temu -v

# 运行特定测试
go test ./common/temu -run TestPendingPriceListRequest -v
```

## 注意事项

1. 需要先在管理系统中配置店铺Cookie
2. 确保网络连接稳定
3. 定期检查日志和监控指标
4. 生产环境建议使用systemd或supervisor管理进程
