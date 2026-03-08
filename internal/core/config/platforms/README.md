# 平台特定配置实现

## 目录说明

此目录用于存放各个平台的特定配置实现。

## 设计目的

每个平台(TEMU、SHEIN、Amazon等)可以在此目录下创建自己的配置提供者实现,实现`PlatformConfigProvider`接口。

## 使用方式

### 1. 创建平台配置提供者

```go
package platforms

import "task-processor/internal/core/config"

type TemuConfigProvider struct {
    *config.BasePlatformProvider
}

func NewTemuConfigProvider(cfg *types.PlatformConfig) *TemuConfigProvider {
    return &TemuConfigProvider{
        BasePlatformProvider: config.NewBasePlatformProvider("temu", cfg),
    }
}

// 可以重写Validate方法添加平台特定验证
func (p *TemuConfigProvider) Validate() error {
    // TEMU特定验证逻辑
    return p.BasePlatformProvider.Validate()
}
```

### 2. 注册平台

```go
func init() {
    config.RegisterPlatform(NewTemuConfigProvider(cfg))
}
```

## 当前状态

🚧 此目录当前为空,预留用于未来扩展。

如果需要为特定平台添加复杂的配置逻辑,可以在此目录下实现。
