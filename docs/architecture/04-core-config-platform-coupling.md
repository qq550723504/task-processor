# 问题四：core/config 耦合平台细节

**严重程度**：中

## 问题描述

`internal/core/config/` 是项目的核心配置层，理论上应该只定义通用配置结构。但实际上该包中存在大量平台特定的配置类型：

- `type_amazon.go` — Amazon 专属配置（SP-API、市场、邮编等）
- `type_platform.go` — 平台通用配置（但包含 Temu、Shein、1688 的具体字段）

这违反了开闭原则：每新增一个平台，就必须修改 `core/config` 包。

## 代码证据

**`internal/core/config/type_amazon.go`** — Amazon 专属配置混入核心层：

```go
type AmazonConfig struct {
    Zipcodes          map[string]string  // Amazon 特有：邮编映射
    SPAPI             SPAPIConfig        // Amazon 特有：SP-API 配置
    ...
}

// AWS 凭证出现在 core 层
type SPAPIConfig struct {
    AWSAccessKeyID string
    AWSSecretKey   string
    RefreshToken   string
    ...
}
```

AWS 凭证、SP-API 协议细节这类高度平台特定的信息出现在 `core/` 层，是明显的层级越界。

**`internal/core/config/type_platform.go`** — 平台配置直接枚举具体平台：

```go
type PlatformsConfig struct {
    Temu        PlatformConfig    `yaml:"temu"`
    Shein       PlatformConfig    `yaml:"shein"`
    Alibaba1688 Alibaba1688Config `yaml:"alibaba1688"` // 1688 有独立类型，其他平台没有
}
```

`PlatformsConfig` 直接用字段名枚举了所有平台。新增平台（如 Lazada、Shopee）必须修改这个结构体，违反开闭原则。注意 `Alibaba1688` 有独立的 `Alibaba1688Config` 类型，而 Temu 和 Shein 共用 `PlatformConfig`，说明这个设计本身就不一致。

## 影响分析

1. **违反开闭原则**：新增平台必须修改 `core/config` 包，而 `core/` 应该是最稳定的层。
2. **核心层膨胀**：随着平台增加，`core/config` 会持续膨胀，最终变成包含所有平台细节的大杂烩。
3. **AWS 凭证泄漏到核心层**：`SPAPIConfig` 中的 `AWSAccessKeyID`、`AWSSecretKey` 是 Amazon 平台特有的基础设施细节，不应出现在 `core/` 层。
4. **平台配置不一致**：1688 有独立类型，Temu/Shein 共用类型，说明没有统一的平台配置设计规范。

## 重构建议

**方向一：平台配置下沉到各平台包**

```
internal/core/config/
    config.go       ← 只保留通用配置：数据库、Redis、日志、浏览器等
    type_common.go  ← 通用平台字段抽象（Enabled、SchedulerEnabled 等）

internal/platforms/amazon/
    config.go       ← AmazonConfig、SPAPIConfig 迁移到这里

internal/platforms/temu/
    config.go       ← Temu 特有配置

internal/platforms/shein/
    config.go       ← Shein 特有配置
```

**方向二：用 map 替代枚举字段（适合平台数量多且动态的场景）**

```go
// 修改前：硬编码字段
type PlatformsConfig struct {
    Temu        PlatformConfig
    Shein       PlatformConfig
    Alibaba1688 Alibaba1688Config
}

// 修改后：map 结构，新增平台无需改代码
type PlatformsConfig struct {
    Platforms map[string]PlatformConfig `yaml:"platforms"`
}
```

**短期可行的最小改动**：至少将 `SPAPIConfig`（含 AWS 凭证）从 `core/config` 移到 `internal/platforms/amazon/config.go`，这是最明显的层级越界，改动范围可控。
