# 应用服务层 (Application Service Layer)

## 📋 概述

`internal/app/service` 包提供应用服务层功能，负责协调和编排业务流程，是连接表示层和领域层的桥梁。

## 🎯 职责

应用服务层的主要职责：

1. **流程编排** - 协调多个领域服务和基础设施服务完成业务流程
2. **事务管理** - 管理跨多个操作的事务边界
3. **服务协调** - 协调不同子系统之间的交互
4. **生命周期管理** - 管理组件的启动、停止和状态
5. **依赖注入** - 提供依赖注入和服务创建

## 📁 目录结构

```
internal/app/service/
├── processor_service.go              # 处理器服务接口
├── processor_service_impl.go         # 处理器服务实现
├── processor_lifecycle.go            # 处理器生命周期管理
├── processor_manager.go              # 处理器管理器
├── scheduler_service.go              # 调度服务接口
├── scheduler_service_impl.go         # 调度服务实现
├── scheduler_factory_creator.go      # 调度器工厂创建器
├── scheduler_platform_config.go      # 调度器平台配置
├── scheduler_task_starter.go         # 调度任务启动器
├── auth_service.go                   # 认证服务
├── config_service.go                 # 配置服务
├── crawler_service.go                # 爬虫服务
├── updater_service.go                # 更新服务
├── status_monitor.go                 # 状态监控
├── task_submitter_adapter.go         # 任务提交适配器
└── README.md                         # 本文档
```

## 🔧 核心服务

### 1. ProcessorService - 处理器服务

**职责：** 管理所有平台处理器的生命周期

**主要功能：**
- 启动/停止所有处理器
- 初始化处理器依赖
- 管理处理器状态
- 协调任务获取和分发

**接口定义：**
```go
type ProcessorService interface {
    StartProcessors(ctx context.Context, cfg *config.Config, authClient *auth.ClientCredentialsAuthClient) error
    StopProcessors() error
    GetStatus() map[string]any
}
```

**使用示例：**
```go
// 创建处理器服务
service := service.NewProcessorService(logger)

// 启动处理器
err := service.StartProcessors(ctx, cfg, authClient)

// 获取状态
status := service.GetStatus()

// 停止处理器
err = service.StopProcessors()
```

### 2. SchedulerService - 调度服务

**职责：** 管理所有周期性调度任务

**主要功能：**
- 核价任务调度
- 产品同步调度
- 库存同步调度
- 活动报名调度
- 调度任务状态监控

**接口定义：**
```go
type SchedulerService interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    GetStatus() map[string]any
}
```

**支持的调度任务类型：**
- 核价任务 (Pricing)
- 产品同步 (Product Sync)
- 库存同步 (Inventory Sync)
- 活动报名 (Activity Registration)

**使用示例：**
```go
// 创建调度服务
scheduler := service.NewSchedulerService(logger, managementClient, cfg)

// 启动调度
err := scheduler.Start(ctx)

// 获取状态
status := scheduler.GetStatus()

// 停止调度
err = scheduler.Stop(ctx)
```

### 3. AuthService - 认证服务

**职责：** 管理客户端认证和授权

**主要功能：**
- 初始化客户端凭证认证
- 获取访问令牌
- 令牌刷新管理

**使用示例：**
```go
// 创建认证服务
authService := service.NewAuthService(logger)

// 初始化客户端凭证
authClient, err := authService.InitializeClientCredentials(cfg)
```

### 4. ConfigService - 配置服务

**职责：** 管理应用配置

**主要功能：**
- 加载配置
- 验证配置
- 配置热更新

### 5. CrawlerService - 爬虫服务

**职责：** 管理爬虫相关功能

**主要功能：**
- 爬虫任务管理
- 爬虫状态监控
- 爬虫结果处理

### 6. UpdaterService - 更新服务

**职责：** 管理数据更新功能

**主要功能：**
- 数据更新调度
- 更新状态跟踪
- 更新结果处理

## 🏗️ 架构模式

### 依赖方向

```
表示层 (Presentation)
    ↓
应用服务层 (Application Service) ← 当前层
    ↓
领域层 (Domain)
    ↓
基础设施层 (Infrastructure)
```

### 设计原则

1. **单一职责** - 每个服务只负责一个业务领域
2. **依赖倒置** - 依赖接口而非实现
3. **开闭原则** - 对扩展开放，对修改关闭
4. **接口隔离** - 提供细粒度的接口

## 📝 编码规范

### 服务命名

- 接口：`XxxService`
- 实现：`xxxServiceImpl`
- 构造函数：`NewXxxService()`

### 文件组织

- 接口定义：`xxx_service.go`
- 实现代码：`xxx_service_impl.go`
- 辅助功能：`xxx_helper.go`

### 错误处理

```go
// 使用统一的错误包装
if err != nil {
    return errors.Wrap(err, errors.ErrCodeSystem, "操作失败")
}
```

### 日志记录

```go
// 使用结构化日志
s.logger.WithFields(logrus.Fields{
    "service": "processor",
    "action": "start",
}).Info("启动处理器服务")
```

## 🔄 生命周期管理

### 服务启动流程

```
1. 创建服务实例
   ↓
2. 注入依赖
   ↓
3. 初始化资源
   ↓
4. 启动子服务
   ↓
5. 注册到生命周期管理器
   ↓
6. 标记为运行状态
```

### 服务停止流程

```
1. 检查运行状态
   ↓
2. 停止接收新请求
   ↓
3. 等待当前请求完成
   ↓
4. 停止子服务
   ↓
5. 释放资源
   ↓
6. 标记为停止状态
```

## 🧪 测试指南

### 单元测试

```go
func TestProcessorService_Start(t *testing.T) {
    // 准备
    logger := logrus.New()
    service := NewProcessorService(logger)
    
    // 执行
    err := service.StartProcessors(context.Background(), cfg, authClient)
    
    // 断言
    assert.NoError(t, err)
}
```

### 集成测试

```go
func TestProcessorService_Integration(t *testing.T) {
    // 创建真实依赖
    cfg := loadTestConfig()
    authClient := createTestAuthClient()
    
    // 测试完整流程
    service := NewProcessorService(logger)
    err := service.StartProcessors(context.Background(), cfg, authClient)
    assert.NoError(t, err)
    
    // 验证状态
    status := service.GetStatus()
    assert.True(t, status["running"].(bool))
    
    // 清理
    service.StopProcessors()
}
```

## 📊 监控指标

### 关键指标

- **服务状态** - 运行/停止
- **请求计数** - 成功/失败
- **响应时间** - 平均/P95/P99
- **错误率** - 错误数/总请求数
- **资源使用** - CPU/内存/goroutine

### 健康检查

```go
// 实现健康检查接口
func (s *processorServiceImpl) HealthCheck() error {
    if !s.running {
        return errors.New("服务未运行")
    }
    return nil
}
```

## 🔗 依赖关系

### 依赖的包

- `internal/core/config` - 配置管理
- `internal/core/lifecycle` - 生命周期管理
- `internal/core/logger` - 日志记录
- `internal/domain/task` - 任务领域模型
- `internal/app/task` - 任务应用服务
- `internal/infra/auth` - 认证基础设施
- `internal/infra/rabbitmq` - 消息队列
- `internal/pkg/management` - 管理客户端

### 被依赖的包

- `cmd/crawler-consumer` - 爬虫消费者
- `cmd/task-processor` - 任务处理器
- `internal/infra/bootstrap` - 应用启动器

## 🚀 使用示例

### 完整启动流程

```go
package main

import (
    "context"
    "task-processor/internal/app/service"
    "task-processor/internal/core/config"
    "github.com/sirupsen/logrus"
)

func main() {
    // 1. 初始化日志
    logger := logrus.New()
    
    // 2. 加载配置
    cfg, err := config.LoadConfig()
    if err != nil {
        logger.Fatal(err)
    }
    
    // 3. 创建认证服务
    authService := service.NewAuthService(logger)
    authClient, err := authService.InitializeClientCredentials(cfg)
    if err != nil {
        logger.Fatal(err)
    }
    
    // 4. 创建处理器服务
    processorService := service.NewProcessorService(logger)
    
    // 5. 启动处理器
    ctx := context.Background()
    if err := processorService.StartProcessors(ctx, cfg, authClient); err != nil {
        logger.Fatal(err)
    }
    
    // 6. 创建调度服务
    schedulerService := service.NewSchedulerService(logger, managementClient, cfg)
    
    // 7. 启动调度
    if err := schedulerService.Start(ctx); err != nil {
        logger.Fatal(err)
    }
    
    // 8. 等待信号
    // ... 信号处理逻辑
    
    // 9. 优雅关闭
    schedulerService.Stop(ctx)
    processorService.StopProcessors()
}
```

## 📚 相关文档

- [架构说明](../../../README_ARCHITECTURE.md)
- [重构状态](../../../docs/REFACTORING_STATUS.md)
- [任务应用层](../task/README.md)
- [消息应用层](../messaging/README.md)
- [生命周期管理](../../core/lifecycle/README.md)

## 🤝 贡献指南

### 添加新服务

1. 定义服务接口 (`xxx_service.go`)
2. 实现服务逻辑 (`xxx_service_impl.go`)
3. 添加单元测试 (`xxx_service_test.go`)
4. 更新本文档
5. 提交代码审查

### 代码审查要点

- ✅ 是否遵循单一职责原则
- ✅ 是否正确处理错误
- ✅ 是否添加了日志
- ✅ 是否实现了生命周期接口
- ✅ 是否添加了测试
- ✅ 是否更新了文档

## ⚠️ 注意事项

1. **避免业务逻辑** - 应用服务层只做编排，不包含业务规则
2. **避免直接依赖基础设施** - 通过接口依赖
3. **避免循环依赖** - 保持清晰的依赖方向
4. **避免全局状态** - 使用依赖注入
5. **避免阻塞操作** - 使用异步或超时控制

## 📞 获取帮助

如有问题：
1. 查看相关文档
2. 查看代码示例
3. 提交 Issue
4. 联系团队成员

---

**创建日期：** 2024-01  
**维护者：** Task Processor Team  
**版本：** 1.0.0
