# 配置平台化迁移指南

## 概述

本次更新将所有平台相关配置统一到 `platforms` 节点下，每个平台包含自动定价（autoPricing）、产品同步（sync）和产品监控（monitor）三个功能模块的完整配置。

## 配置结构变更

### 新的平台化配置结构

**现在的结构：**
```yaml
platforms:
  # TEMU 平台配置
  temu:
    # 自动定价配置
    autoPricing:
      enabled: false
      interval: 300     # 5分钟
      batchSize: 100
    # 产品同步配置
    sync:
      enabled: true
      storeIDs: []      # TEMU店铺ID列表
      interval: 60      # 同步间隔（分钟）
      batchSize: 50     # 批量处理大小
    # 产品监控配置
    monitor:
      enabled: true
      storeIDs: []
      checkInterval: 1440         # 检查间隔（分钟）
      batchSize: 100              # 批量处理大小
      enablePriceAlert: true      # 启用价格告警
      enableStockAlert: true      # 启用库存告警
      priceChangeThreshold: 10.0  # 价格变化阈值（百分比）
      stockChangeThreshold: 5     # 库存变化阈值

  # SHEIN 平台配置
  shein:
    # 自动定价配置
    autoPricing:
      enabled: false
      interval: 300
      batchSize: 100
    # 产品同步配置
    sync:
      enabled: true
      storeIDs: []
      interval: 60
      batchSize: 50
    # 产品监控配置
    monitor:
      enabled: true
      storeIDs: []
      checkInterval: 1440
      batchSize: 100
      enablePriceAlert: true
      enableStockAlert: true
      priceChangeThreshold: 10.0
      stockChangeThreshold: 5
```

## 代码使用方式

### 1. 获取平台配置

```go
// 获取 TEMU 平台的完整配置
temuConfig := config.GetPlatformConfig("temu")
if temuConfig != nil {
    // 访问各个功能模块
    if temuConfig.Sync.Enabled {
        // 处理 TEMU 同步逻辑
    }
    if temuConfig.Monitor.Enabled {
        // 处理 TEMU 监控逻辑
    }
}

// 获取特定功能的配置
temuSyncConfig := config.GetPlatformSyncConfig("temu")
if temuSyncConfig != nil && temuSyncConfig.Enabled {
    // 处理 TEMU 同步逻辑
}

sheinMonitorConfig := config.GetPlatformMonitorConfig("shein")
if sheinMonitorConfig != nil && sheinMonitorConfig.Enabled {
    // 处理 SHEIN 监控逻辑
}
```

### 2. 检查平台功能状态

```go
// 检查 TEMU 是否启用同步
if config.IsSyncEnabled("temu") {
    // 启动 TEMU 同步服务
}

// 检查 SHEIN 是否启用监控
if config.IsMonitorEnabled("shein") {
    // 启动 SHEIN 监控服务
}

// 检查自动定价是否启用
if config.IsAutoPricingEnabled("temu") {
    // 启动 TEMU 自动定价服务
}
```

### 3. 获取启用的平台列表

```go
// 获取启用同步功能的所有平台
enabledSyncPlatforms := config.GetEnabledPlatforms("sync")
for _, platform := range enabledSyncPlatforms {
    // 为每个平台启动同步服务
    platformConfig := config.GetPlatformConfig(platform)
    // 使用平台特定配置
}

// 获取启用监控功能的所有平台
enabledMonitorPlatforms := config.GetEnabledPlatforms("monitor")
for _, platform := range enabledMonitorPlatforms {
    // 为每个平台启动监控服务
}
```

## 配置验证

新的配置结构包含完整的验证逻辑：

- 验证同步间隔必须大于 0
- 验证批量处理大小必须大于 0
- 验证监控检查间隔必须大于 0
- 验证价格变化阈值不能为负数

## 默认配置值

### 同步配置默认值：
- `enabled`: false
- `storeIDs`: []
- `interval`: 60 分钟
- `batchSize`: 50

### 监控配置默认值：
- `enabled`: false
- `storeIDs`: []
- `checkInterval`: 1440 分钟（24小时）
- `batchSize`: 100
- `enablePriceAlert`: true
- `enableStockAlert`: true
- `priceChangeThreshold`: 10.0%
- `stockChangeThreshold`: 5

## 迁移步骤

1. **更新配置文件**：将现有的配置按新的 `platforms` 结构进行重组

2. **更新代码调用**：
   - 将 `cfg.AutoPricing.Temu` 改为 `cfg.Platforms.Temu.AutoPricing`
   - 将 `cfg.Sync` 相关调用改为使用辅助方法 `cfg.GetPlatformSyncConfig("platform")`
   - 将 `cfg.Monitor` 相关调用改为使用辅助方法 `cfg.GetPlatformMonitorConfig("platform")`

3. **测试验证**：确保各平台的功能能够独立启用和禁用

## 配置优势

### 1. 结构清晰
- 每个平台的所有配置集中在一起，便于管理
- 配置层次结构更加直观和一致

### 2. 扩展性强
- 新增平台时只需在 `platforms` 下添加新节点
- 新增功能模块时可以统一在所有平台下添加

### 3. 维护简单
- 平台相关的所有配置都在同一个位置
- 减少了配置分散导致的维护复杂性

## 注意事项

- 如果某个平台的 `storeIDs` 为空，系统将使用 `management.storeIDs` 作为默认值
- 每个平台可以独立配置不同的同步间隔和批量处理大小
- 监控功能支持价格和库存告警的细粒度控制
- 所有配置都支持热重载，无需重启应用程序
- 配置验证会检查每个平台的配置完整性和有效性