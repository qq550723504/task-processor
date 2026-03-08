# app 目录

## 用途

应用层，负责业务逻辑编排、服务组合、任务调度等。这是最上层，协调各个组件完成业务功能。

## 目录结构

```
app/
├── api/         # API 处理器
├── bootstrap/   # 系统初始化
├── scheduler/   # 任务调度器
├── service/     # 应用服务
├── updater/     # 自动更新器
└── worker/      # 工作池和任务处理器
```

## 子目录说明

### api（API 处理器）
- CLI 命令处理
- HTTP API 处理器
- gRPC 服务实现

**应该放置的文件：**
- `cli_handler.go` - CLI 命令处理器
- `http_handler.go` - HTTP 处理器（如果有）
- `grpc_handler.go` - gRPC 处理器（如果有）

### bootstrap（系统初始化）
- 系统级初始化
- 日志初始化
- Goroutine 管理
- 信号处理

**应该放置的文件：**
- `system_init.go` - 系统初始化器

### scheduler（任务调度器）
- 任务调度逻辑
- 任务执行器
- 任务依赖管理
- 任务监控

**应该放置的文件：**
- `manager.go` - 调度管理器
- `executor.go` - 任务执行器
- `registry.go` - 任务注册表
- `dependency.go` - 任务依赖管理
- `monitor.go` - 任务监控
- `types.go` - 调度类型定义

### service（应用服务）
- 业务服务实现
- 服务编排
- 事务管理

**应该放置的文件：**
- `auth_service.go` - 认证服务
- `config_service.go` - 配置服务
- `crawler_service.go` - 爬虫服务
- `processor_service.go` - 处理器服务
- `scheduler_service.go` - 调度服务
- `updater_service.go` - 更新服务

### updater（自动更新器）
- 版本检查
- 文件下载
- 更新安装
- 版本管理

**应该放置的文件：**
- `updater.go` - 更新器主逻辑
- `version_manager.go` - 版本管理
- `file_downloader.go` - 文件下载器
- `file_manager.go` - 文件管理
- `update_manager.go` - 更新管理器
- `config.go` - 更新配置
- `models.go` - 更新模型

### worker（工作池）
- 工作池管理
- 任务处理器
- 并发控制

**应该放置的文件：**
- `pool.go` - 工作池
- `base_processor.go` - 基础处理器
- `base_task_handler.go` - 基础任务处理器
- `interfaces.go` - 工作器接口
- `types.go` - 工作器类型

## 应用服务设计模式

### 1. 服务接口定义

```go
// service/processor_service.go
package service

type ProcessorService interface {
    StartProcessors(ctx context.Context, cfg *config.Config) error
    StopProcessors() error
    GetStatus() map[string]any
}
```

### 2. 服务实现

```go
type processorServiceImpl struct {
    logger           *logrus.Logger
    lifecycleManager lifecycle.LifecycleManager
    managementClient *management.ClientManager
}

func NewProcessorService(logger *logrus.Logger) ProcessorService {
    return &processorServiceImpl{
        logger:           logger,
        lifecycleManager: lifecycle.NewLifecycleManager(logger),
    }
}

func (s *processorServiceImpl) StartProcessors(ctx context.Context, cfg *config.Config) error {
    // 编排启动逻辑
    return nil
}
```

### 3. 服务编排

```go
// 编排多个服务完成复杂业务
func (s *processorServiceImpl) ProcessTask(ctx context.Context, task *Task) error {
    // 1. 验证任务
    if err := s.validator.Validate(task); err != nil {
        return err
    }
    
    // 2. 获取产品信息
    product, err := s.crawlerService.FetchProduct(task.URL)
    if err != nil {
        return err
    }
    
    // 3. 处理产品数据
    result, err := s.productService.Process(product)
    if err != nil {
        return err
    }
    
    // 4. 保存结果
    return s.repository.Save(result)
}
```

## 任务调度器设计

### 1. 任务注册

```go
// scheduler/registry.go
type TaskRegistry struct {
    tasks map[string]TaskExecutor
}

func (r *TaskRegistry) Register(name string, executor TaskExecutor) {
    r.tasks[name] = executor
}
```

### 2. 任务执行

```go
// scheduler/executor.go
type TaskExecutor interface {
    Execute(ctx context.Context, params map[string]any) error
    GetName() string
    GetDependencies() []string
}
```

### 3. 依赖管理

```go
// scheduler/dependency.go
type DependencyManager struct {
    graph map[string][]string
}

func (d *DependencyManager) ResolveDependencies(taskName string) ([]string, error) {
    // 拓扑排序解析依赖
    return nil, nil
}
```

## 工作池设计

```go
// worker/pool.go
type WorkerPool struct {
    workers    int
    taskQueue  chan Task
    resultChan chan Result
}

func (p *WorkerPool) Start(ctx context.Context) {
    for i := 0; i < p.workers; i++ {
        go p.worker(ctx)
    }
}

func (p *WorkerPool) worker(ctx context.Context) {
    for {
        select {
        case task := <-p.taskQueue:
            result := p.processTask(task)
            p.resultChan <- result
        case <-ctx.Done():
            return
        }
    }
}
```

## 编码规范

1. 应用服务负责业务流程编排，不包含具体业务逻辑
2. 使用依赖注入管理服务依赖
3. 服务方法应该是事务性的
4. 提供清晰的错误处理和日志
5. 考虑并发安全和性能

## 注意事项

- 应用层不直接访问基础设施，通过接口调用
- 保持服务方法的单一职责
- 使用上下文传递请求范围的数据
- 注意资源的正确释放
- 提供完善的监控和日志
