# TEMU 模块

## 功能概述

TEMU模块提供与TEMU卖家平台的API交互功能，包括：

- Cookie管理
- API客户端封装
- 自动拒绝平台报价
- 定时任务调度

## 快速开始

### 1. 单次拒绝报价

```go
import (
    "task-processor/common/management"
    "task-processor/common/temu"
)

// 创建管理客户端
managementClient := management.NewClientManager(nil)

// 创建API客户端
apiClient := temu.NewAPIClient(tenantID, storeID, managementClient)

// 拒绝单个商品报价
err := apiClient.RejectPrice("602906963083875", []string{"42895046380613"})
if err != nil {
    log.Fatal(err)
}
```

### 1.5. 重新报价

```go
// 构造SKU信息
skuInfoList := []temu.ReappealSkuInfo{
    {
        SkuID:                       "37921474236098",
        SupplierPriceStr:            "213.96",
        RecommendedSupplierPriceStr: "10.60",
        TargetSupplierPriceStr:      "100.00",
        Currency:                    "USD",
    },
}

// 提交重新报价
appealReasons := []string{"LOWER_THAN_SIMILAR"}
err := apiClient.ReappealPrice("602408746850061", skuInfoList, 100, appealReasons)
if err != nil {
    log.Fatal(err)
}
```

### 2. 自动拒绝所有待核价商品

```go
// 自动处理所有待核价商品
err := apiClient.AutoRejectAllPendingPrices()
if err != nil {
    log.Fatal(err)
}
```

### 2. 接受平台报价

```go
// 构造SKU列表
skuList := []temu.AcceptPriceSkuInfo{
    {
        SkuID:                  "41735405200193",
        Currency:               "USD",
        TargetSupplierPriceStr: "55.67", // 或空字符串表示接受平台推荐价格
    },
}

// 接受报价 (scene=2表示价格健康页面)
err := apiClient.AcceptPrice("602204735908247", skuList, 2)
if err != nil {
    log.Fatal(err)
}
```

### 3. 启动定时任务

```go
import "time"

// 创建调度器（每30分钟执行一次，拒绝报价）
scheduler := temu.NewPricingScheduler(apiClient, 30*time.Minute, temu.ActionReject)

// 或创建接受报价的调度器
// scheduler := temu.NewPricingScheduler(apiClient, 30*time.Minute, temu.ActionAccept)

// 启动调度器
scheduler.Start()

// 停止调度器
defer scheduler.Stop()
```

### 4. 多店铺管理

```go
// 创建调度器管理器（拒绝报价）
schedulerManager := temu.NewSchedulerManager(managementClient, 30*time.Minute, temu.ActionReject)

// 或创建接受报价的管理器
// schedulerManager := temu.NewSchedulerManager(managementClient, 30*time.Minute, temu.ActionAccept)

// 添加多个店铺
schedulerManager.AddStore(1001, 2001)
schedulerManager.AddStore(1001, 2002)

// 停止所有调度器
defer schedulerManager.StopAll()
```

## 命令行工具

```bash
# 单店铺模式 - 拒绝报价
go run cmd/temu-pricing/main.go -tenant=1001 -store=2001 -interval=30m -action=reject

# 单店铺模式 - 接受报价
go run cmd/temu-pricing/main.go -tenant=1001 -store=2001 -interval=30m -action=accept

# 多店铺模式
go run cmd/temu-pricing/main.go -interval=30m -action=reject
```

## 配置

在 `config/config-dev.yaml` 中配置：

```yaml
temu:
  autoPricing:
    enabled: true
    interval: 1800  # 30分钟
    batchSize: 100
```

## 注意事项

1. 需要先在管理系统中配置店铺Cookie
2. 当前实现为自动拒绝所有待核价商品
3. 建议间隔时间设置为30分钟以上
4. 支持优雅退出（Ctrl+C）

## 错误处理

- Cookie过期：自动重新加载并设置暂停键
- 网络错误：自动重试3次
- Panic恢复：调度器内置恢复机制

详细文档请参考：[TEMU自动核价功能文档](../../docs/TEMU_AUTO_PRICING.md)
