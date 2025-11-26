# 自动核价平台分离配置

## 问题描述

之前的配置中，`autoPricing.enabled` 是全局开关，当设置为 `true` 时，TEMU 和 SHEIN 的自动核价都会同时启动，导致冲突。

## 解决方案

将自动核价配置拆分为平台独立配置，每个平台可以单独控制是否启用自动核价。

## 配置结构变更

### 修改前
```yaml
autoPricing:
  enabled: true
  interval: 300
  batchSize: 100
```

### 修改后
```yaml
autoPricing:
  temu:
    enabled: true
    interval: 300
    batchSize: 100
  shein:
    enabled: false
    interval: 300
    batchSize: 100
```

## 代码变更

### 1. 配置结构 (common/config/config.go)

```go
// AutoPricingConfig 自动核价配置
type AutoPricingConfig struct {
	Temu  PlatformAutoPricingConfig
	Shein PlatformAutoPricingConfig
}

// PlatformAutoPricingConfig 平台自动核价配置
type PlatformAutoPricingConfig struct {
	Enabled   bool
	Interval  int
	BatchSize int
}
```

### 2. TEMU Processor (platforms/temu/processor.go)

```go
if p.config.AutoPricing.Temu.Enabled {
    autoPricingInterval := time.Duration(p.config.AutoPricing.Temu.Interval) * time.Second
    // ...
}
```

### 3. SHEIN Processor (platforms/shein/processor.go)

```go
if p.config.AutoPricing.Shein.Enabled {
    autoPricingInterval := time.Duration(p.config.AutoPricing.Shein.Interval) * time.Second
    // ...
}
```

## 使用说明

- **只启用 TEMU 自动核价**：设置 `autoPricing.temu.enabled: true` 和 `autoPricing.shein.enabled: false`
- **只启用 SHEIN 自动核价**：设置 `autoPricing.temu.enabled: false` 和 `autoPricing.shein.enabled: true`
- **同时启用**：两个都设置为 `true`（如果业务需要）
- **全部禁用**：两个都设置为 `false`

## 默认配置

- TEMU：默认启用 (`enabled: true`)
- SHEIN：默认禁用 (`enabled: false`)
