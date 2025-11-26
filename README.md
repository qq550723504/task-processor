# Task Processor - 统一任务处理系统

这是一个重构后的统一任务处理系统，支持多个电商平台（TEMU、SHEIN等）的产品发布任务处理。

## 架构设计

### 项目结构

```
task-processor/
├── common/                 # 共享核心库
│   ├── amazon/            # Amazon爬虫组件
│   ├── auth/              # 认证组件
│   ├── config/            # 配置管理
│   ├── processor/         # 处理器接口
│   ├── types/             # 通用类型定义
│   └── worker/            # 工作池实现
├── platforms/             # 平台特定实现
│   ├── temu/              # TEMU平台实现
│   └── shein/             # SHEIN平台实现（待迁移）
├── cmd/                   # 入口程序
│   ├── temu-web/          # TEMU Web版本
│   ├── temu-cli/          # TEMU 命令行版本（待实现）
│   └── shein-web/         # SHEIN Web版本（待迁移）
├── config/                # 配置文件
│   ├── config-temu-dev.yaml
│   ├── config-temu-prod.yaml
│   ├── config-shein-dev.yaml
│   └── config-shein-prod.yaml
└── data/                  # 数据目录
    └── token.json         # 会话数据
```

### 核心特性

1. **统一架构**: 共享核心组件，减少代码重复
2. **平台独立**: 每个平台有独立的处理逻辑和配置
3. **API驱动**: 通过API获取任务，不依赖Redis
4. **Web界面**: 提供用户友好的Web管理界面
5. **认证系统**: 支持OAuth2和会话管理
6. **Amazon爬虫**: 集成完整的Amazon产品爬虫功能
7. **优雅关闭**: 支持任务处理的优雅停止

## 快速开始

### 环境要求

- Go 1.21+
- 网络连接（用于API调用）

### 安装依赖

```bash
cd go/task-processor
go mod tidy
```

### 配置

1. 复制配置文件模板：
```bash
cp config/config-temu-dev.yaml config/config-temu-prod.yaml
```

2. 修改配置文件中的相关参数：
   - `management.baseURL`: 管理系统API地址
   - `management.clientID`: 客户端ID
   - `management.clientSecret`: 客户端密钥
   - `management.storeIDs`: 店铺ID列表

### 运行

#### TEMU Web版本

```bash
# 开发环境
TASK_PROCESSOR_ENV=dev go run cmd/temu-web/main.go

# 生产环境
TASK_PROCESSOR_ENV=prod go run cmd/temu-web/main.go
```

访问 http://localhost:8081 进行登录和管理。

### 构建

```bash
# 构建TEMU Web版本
go build -o dist/temu-web cmd/temu-web/main.go

# 构建所有版本
./scripts/build.sh
```

## 配置说明

### 主要配置项

- `processor`: 处理器配置（重试次数、超时时间）
- `worker`: 工作池配置（并发数、缓冲区大小、任务间隔）
- `server`: Web服务器配置（端口）
- `management`: 管理系统API配置
- `platform`: 平台特定配置

### 环境变量

- `TASK_PROCESSOR_ENV`: 环境名称（dev/prod）
- `TASK_PROCESSOR_*`: 配置覆盖（如 TASK_PROCESSOR_SERVER_PORT=8081）

## 开发指南

### 添加新平台

1. 在 `platforms/` 下创建新平台目录
2. 实现平台特定的处理器和任务获取器
3. 在 `cmd/` 下创建入口程序
4. 添加平台配置文件

### 扩展功能

1. 在 `common/` 中添加共享组件
2. 在平台特定目录中实现具体逻辑
3. 更新配置结构体和默认值

## API接口

### 任务管理

- `GET /api/processor-status`: 获取处理器状态
- `POST /api/start-processor`: 启动任务处理器
- `POST /api/stop-processor`: 停止任务处理器

### 用户认证

- `POST /api/login`: 用户登录
- `POST /api/logout`: 用户登出

## 监控和日志

- 日志文件: `{platform}-processor.log`
- 任务状态: 通过Web界面查看
- 性能指标: 内置指标收集

## 部署

### Docker部署

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o temu-web cmd/temu-web/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/temu-web .
COPY --from=builder /app/config ./config
CMD ["./temu-web"]
```

### Kubernetes部署

参考 `k8s/` 目录中的部署文件。

## Amazon爬虫集成

### 使用Amazon爬虫

```go
import "task-processor/common/amazon"

// 在平台处理器中使用
amazonProcessor := amazon.NewAmazonProcessor(&config.Amazon)
defer amazonProcessor.Shutdown()

// 处理Amazon产品
product, err := amazonProcessor.Process(amazonURL, zipcode)
if err != nil {
    logrus.Infof("Amazon爬虫处理失败: %v", err)
}
```

### Amazon配置

```yaml
amazon:
  enabled: true
  headless: true
  browserPath: "./chrome/chrome.exe"
  poolSize: 3
  zipcodes:
    US: "10001"
    JP: "153-0064"
  viewportWidth: 1920
  viewportHeight: 1080
```

### 完整的提取器支持

Amazon爬虫现已包含完整的提取器：
- 标题、价格、品牌、评分提取
- 图片、分类、描述提取
- 产品详情、卖家信息提取
- 变体信息、畅销排名提取
- 反检测和指纹管理

详细文档请参考: [Amazon爬虫文档](common/amazon/README.md)

## 迁移指南

### 从旧版本迁移

1. 备份现有配置和数据
2. 更新配置文件格式
3. 迁移自定义处理逻辑
4. 迁移Amazon爬虫相关代码
5. 测试新版本功能

## 故障排除

### 常见问题

1. **配置文件找不到**: 检查 `TASK_PROCESSOR_ENV` 环境变量
2. **API连接失败**: 检查 `management.baseURL` 配置
3. **认证失败**: 检查客户端ID和密钥配置
4. **任务处理失败**: 查看日志文件获取详细错误信息

## 贡献

1. Fork 项目
2. 创建功能分支
3. 提交更改
4. 创建 Pull Request

## 许可证

MIT License