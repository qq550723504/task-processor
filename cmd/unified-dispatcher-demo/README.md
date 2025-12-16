# 统一任务分发系统演示

## 📋 概述

统一任务分发系统是一个高度模块化的任务处理架构，能够将任务智能分发到Amazon、TEMU、SHEIN三个平台进行处理。

## 🏗️ 架构设计

### 核心组件

```
统一任务分发系统
├── TaskDispatcher (任务分发器)
│   ├── ProcessorRegistry (处理器注册表)
│   ├── TaskRouter (任务路由器)
│   └── MetricsCollector (指标收集器)
├── PlatformAdapters (平台适配器)
│   ├── AmazonAdapter
│   ├── TemuAdapter
│   └── SheinAdapter
├── TaskService (任务服务层)
└── UnifiedTask (统一任务模型)
```

### 设计特点

- **统一接口**: 所有平台使用相同的任务处理接口
- **适配器模式**: 保持现有平台代码不变，通过适配器实现统一
- **智能路由**: 根据目标平台自动分发任务
- **监控管理**: 提供完整的状态监控和指标收集
- **模块化设计**: 按Go最佳实践进行文件拆分和职责分离

## 🚀 使用方法

### 基本用法

```bash
# 运行所有平台演示
go run cmd/unified-dispatcher-demo/main.go

# 指定平台演示
go run cmd/unified-dispatcher-demo/main.go -platform=amazon

# 指定任务数量
go run cmd/unified-dispatcher-demo/main.go -count=5

# 详细日志模式
go run cmd/unified-dispatcher-demo/main.go -verbose
```

### 参数说明

- `-platform`: 目标平台 (amazon, temu, shein, all)
- `-count`: 每个平台的测试任务数量
- `-verbose`: 启用详细日志输出

## 📊 功能演示

### 1. 系统初始化
- 创建任务分发器
- 初始化三个平台处理器
- 注册平台适配器
- 启动分发系统

### 2. 任务分发
- 创建统一任务格式
- 智能路由到目标平台
- 实时状态监控
- 错误处理和重试

### 3. 结果展示
- 处理器状态概览
- 任务处理统计
- 成功率计算
- 性能指标

## 🔧 配置说明

### 分发器配置
```go
type DispatcherConfig struct {
    MaxRetries     int           // 最大重试次数
    RetryDelay     time.Duration // 重试延迟
    BatchSize      int           // 批处理大小
    ProcessTimeout time.Duration // 处理超时时间
    EnableMetrics  bool          // 启用指标收集
}
```

### 任务服务配置
```go
type TaskServiceConfig struct {
    MaxRetries        int           // 最大重试次数
    DefaultTimeout    time.Duration // 默认超时时间
    BatchSize         int           // 批处理大小
    EnablePersistence bool          // 启用持久化
}
```

## 📈 监控指标

### 处理器状态
- 运行状态 (running/stopped/error)
- 可用槽位数量
- 处理任务统计
- 成功率计算
- 错误信息

### 任务指标
- 总任务数
- 成功任务数
- 失败任务数
- 平均处理时间
- 吞吐量 (任务/秒)

## 🔄 工作流程

1. **任务提交**: 客户端提交UnifiedTask到TaskService
2. **任务验证**: 验证任务格式和必要字段
3. **任务路由**: TaskRouter根据规则确定目标平台
4. **适配转换**: 平台适配器将UnifiedTask转换为平台特定格式
5. **任务执行**: 平台处理器执行具体的业务逻辑
6. **状态更新**: 更新任务状态和收集指标
7. **结果返回**: 返回处理结果和状态信息

## 🎯 扩展性

### 添加新平台
1. 实现PlatformProcessor接口
2. 创建平台适配器
3. 注册到TaskDispatcher
4. 添加路由规则

### 自定义路由规则
```go
// 基于地区的路由规则
regionRule := NewRegionBasedRule(map[string]string{
    "US": "amazon",
    "CN": "temu",
    "EU": "shein",
}, 10)

router.AddRule(regionRule)
```

## 🧪 测试验证

演示程序会创建测试任务并展示：
- 系统初始化过程
- 任务分发流程
- 实时处理状态
- 最终统计结果

## 📝 日志输出

系统提供结构化日志输出，包括：
- 系统启动和关闭日志
- 任务分发和处理日志
- 错误和警告信息
- 性能指标统计

## 🔒 错误处理

- 任务验证失败处理
- 平台处理器异常处理
- 网络超时和重试机制
- 资源清理和优雅关闭