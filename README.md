# Task Processor - 电商爬虫任务处理系统

[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

一个高性能、可扩展的电商平台爬虫任务处理系统，支持 Amazon、TEMU、SHEIN、1688 等多个电商平台的产品数据采集和处理。

## 🌟 核心特性

- **多平台支持**: Amazon、TEMU、SHEIN、1688 等主流电商平台
- **双模式架构**: 
  - RabbitMQ 消费者模式：事件驱动，适合实时任务处理
  - 定时调度模式：周期性任务，适合核价、同步等场景
- **高并发处理**: Worker Pool 模式，支持可配置的并发数
- **容错机制**: 自动重试、死信队列、优雅降级
- **监控告警**: 健康检查、指标监控、负载统计
- **生命周期管理**: 统一的组件启动、停止和依赖注入

## 📋 系统要求

- Go 1.24 或更高版本
- RabbitMQ 3.x（可选，用于分布式爬虫）
- Chrome/Chromium（用于浏览器自动化）

## 🚀 快速开始

### 安装

```bash
# 克隆项目
git clone <repository-url>
cd task-processor

# 安装依赖
go mod download

# 编译
go build -o task-processor cmd/task/main.go
go build -o rabbitmq-consumer cmd/rabbitmq-consumer/main.go
```

### 配置

复制并修改配置文件：

```bash
cp config/config-dev.yaml config/config-prod.yaml
# 编辑 config-prod.yaml，填入你的配置
```

关键配置项：

```yaml
# 管理API配置
management:
  baseURL: "http://your-api-server.com"
  clientID: "your-client-id"
  clientSecret: "your-client-secret"

# 平台配置
platforms:
  temu:
    enabled: true
  shein:
    enabled: true
  alibaba1688:
    enabled: true

# 浏览器配置
browser:
  headless: true
  poolSize: 3
```

### 运行

**方式 1: 定时调度模式**

```bash
./task-processor --config=config/config-prod.yaml
```

**方式 2: RabbitMQ 消费者模式**

```bash
./rabbitmq-consumer \
  --config=config/rabbitmq-config.yaml \
  --app-config=config/config-prod.yaml \
  --platforms=amazon,temu,shein
```

## 📖 架构设计

### 系统架构

```
┌─────────────────────────────────────────┐
│         Application Layer               │
│  ┌──────────┐  ┌──────────┐            │
│  │   Task   │  │ RabbitMQ │            │
│  │Scheduler │  │ Consumer │            │
│  └──────────┘  └──────────┘            │
└─────────────────────────────────────────┘
           │              │
           ▼              ▼
┌─────────────────────────────────────────┐
│         Platform Processors             │
│  ┌────────┐ ┌────────┐ ┌────────┐     │
│  │  TEMU  │ │ SHEIN  │ │ Amazon │     │
│  └────────┘ └────────┘ └────────┘     │
└─────────────────────────────────────────┘
           │              │
           ▼              ▼
┌─────────────────────────────────────────┐
│         Infrastructure Layer            │
│  ┌──────────┐  ┌──────────┐            │
│  │ Browser  │  │   HTTP   │            │
│  │   Pool   │  │  Client  │            │
│  └──────────┘  └──────────┘            │
└─────────────────────────────────────────┘
```

### 核心组件

- **Scheduler Manager**: 任务调度管理器，负责定时任务的创建和执行
- **Platform Processors**: 平台处理器，封装各平台的业务逻辑
- **Worker Pool**: 工作池，管理并发任务执行
- **Browser Pool**: 浏览器池，复用浏览器实例提高性能
- **Lifecycle Manager**: 生命周期管理器，统一管理组件启停

## 🔧 开发指南

### 项目结构

```
task-processor/
├── cmd/                    # 应用入口
│   ├── task/              # 定时调度模式
│   └── rabbitmq-consumer/ # RabbitMQ消费者模式
├── internal/              # 内部代码
│   ├── app/              # 应用层
│   │   ├── scheduler/    # 调度器
│   │   ├── service/      # 服务层
│   │   └── worker/       # Worker池
│   ├── core/             # 核心组件
│   │   ├── config/       # 配置管理
│   │   ├── lifecycle/    # 生命周期管理
│   │   └── logger/       # 日志管理
│   ├── platforms/        # 平台实现
│   │   ├── temu/        # TEMU平台
│   │   ├── shein/       # SHEIN平台
│   │   └── amazon/      # Amazon平台
│   ├── crawler/          # 爬虫实现
│   └── infra/           # 基础设施
├── config/               # 配置文件
├── docs/                 # 文档
└── tests/               # 测试
```

### 添加新平台

1. 在 `internal/platforms/` 下创建新平台目录
2. 实现 `Processor` 接口
3. 在配置文件中添加平台配置
4. 注册处理器到系统

示例：

```go
type MyPlatformProcessor struct {
    *worker.BaseProcessor
}

func (p *MyPlatformProcessor) ProcessTask(ctx context.Context, task *model.Task) error {
    // 实现任务处理逻辑
    return nil
}
```

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/app/scheduler/...

# 运行测试并显示覆盖率
go test -cover ./...
```

## 📊 监控和运维

### 健康检查

```bash
# 健康检查
curl http://localhost:8081/health

# 就绪检查
curl http://localhost:8081/ready
```

### 指标监控

```bash
# Prometheus 格式指标
curl http://localhost:8082/metrics

# 统计信息
curl http://localhost:8082/stats
```

### 日志级别

支持的日志级别：`debug`, `info`, `warn`, `error`, `fatal`

```bash
# 启动时指定日志级别
./task-processor --log-level=debug
```

## 🐛 故障排查

### 常见问题

**问题 1: 连接 RabbitMQ 失败**
- 检查 RabbitMQ 服务是否启动
- 验证连接字符串配置
- 测试网络连通性

**问题 2: 浏览器启动失败**
- 检查 Chrome/Chromium 是否安装
- 验证 `browserPath` 配置
- 检查系统资源是否充足

**问题 3: 任务处理失败**
- 查看错误日志
- 检查外部 API 是否可用
- 验证配置参数是否正确

## 📝 更新日志

### v1.0.0 (2024-03-04)
- ✨ 初始版本发布
- ✅ 支持 TEMU、SHEIN、Amazon 平台
- ✅ 实现双模式架构
- ✅ 添加监控和健康检查

### 最近优化 (feature/code-optimization)
- 🐛 修复 panic 问题，改用错误返回
- 📝 添加项目文档
- 🔧 改进错误处理机制

## 🤝 贡献指南

欢迎贡献代码！请遵循以下步骤：

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📄 许可证

本项目采用 MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

## 📧 联系方式

- 技术支持：tech-support@example.com
- 问题反馈：通过 GitHub Issues

## 🙏 致谢

感谢所有贡献者和开源项目的支持！
