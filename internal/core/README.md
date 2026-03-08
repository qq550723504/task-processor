# core 目录

## 用途

核心基础功能模块，提供配置管理、日志、错误处理、生命周期管理等基础能力。这是整个项目的最底层，被所有其他模块依赖。

## 目录结构

```
core/
├── config/      # 配置管理
├── errors/      # 错误处理
├── lifecycle/   # 生命周期管理
└── logger/      # 日志管理
```

## 子目录说明

### config（配置管理）
- 配置文件加载和解析
- 配置热更新
- 多环境配置支持
- 配置验证

**应该放置的文件：**
- `config.go` - 配置结构定义
- `loader.go` - 配置加载器
- `manager.go` - 配置管理器
- `source.go` - 配置源接口
- `validator.go` - 配置验证器

### errors（错误处理）
- 自定义错误类型
- 错误码定义
- 错误包装和追踪
- 错误格式化

**应该放置的文件：**
- `errors.go` - 错误类型定义
- `codes.go` - 错误码常量
- `wrapper.go` - 错误包装器

### lifecycle（生命周期管理）
- 组件启动和停止管理
- 依赖关系管理
- 优雅关闭
- 健康检查

**应该放置的文件：**
- `lifecycle.go` - 生命周期接口
- `manager.go` - 生命周期管理器
- `component.go` - 组件基类

### logger（日志管理）
- 日志初始化和配置
- 日志级别管理
- 日志格式化
- 日志输出管理

**应该放置的文件：**
- `logger.go` - 日志器接口
- `manager.go` - 日志管理器
- `config.go` - 日志配置
- `formatter.go` - 日志格式化器

## 编码规范

1. 核心模块应该保持简单和稳定
2. 不依赖任何其他 internal 模块
3. 只依赖标准库和必要的第三方库
4. 提供清晰的接口定义
5. 充分的单元测试覆盖

## 示例代码

### 配置管理示例

```go
// config/config.go
package config

type Config struct {
    App      AppConfig      `yaml:"app"`
    Server   ServerConfig   `yaml:"server"`
    Database DatabaseConfig `yaml:"database"`
}

type ConfigManager interface {
    Load(source ConfigSource) (*Config, error)
    GetCurrent() *Config
    Watch(callback func(*Config)) error
}
```

### 生命周期管理示例

```go
// lifecycle/lifecycle.go
package lifecycle

type Component interface {
    Name() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    IsRunning() bool
}

type LifecycleManager interface {
    Register(component Component) error
    StartAll(ctx context.Context) error
    StopAll(ctx context.Context) error
}
```

### 日志管理示例

```go
// logger/logger.go
package logger

type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
}

type LogManager interface {
    GetLogger(component string) Logger
    SetLevel(level string) error
}
```

## 注意事项

- 核心模块的变更要谨慎，影响范围大
- 保持向后兼容性
- 提供完善的文档和示例
- 避免引入过多的外部依赖
