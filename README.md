# Task Processor

跨平台电商任务处理系统，支持 Amazon、Shein、Temu 等平台的产品数据同步和监控。

## 功能特性

- 🛒 **多平台支持**：Amazon、Shein、Temu 平台产品数据处理
- 🔄 **自动同步**：定时同步产品信息、价格、库存等数据
- 📊 **实时监控**：产品变化监控和告警
- 🚀 **高性能**：并发处理，支持大规模数据处理
- 🛡️ **反检测**：内置反爬虫检测机制
- 📝 **敏感词过滤**：自动过滤敏感词汇
- 🔧 **可配置**：灵活的配置管理

## 快速开始

### 前置要求

- Go 1.24+
- Chrome 浏览器（用于爬虫功能）
- MySQL 数据库
- Redis 缓存

### 安装依赖

```bash
# 安装 Go 依赖
go mod download

# 安装 Chrome 浏览器（如果系统中没有）
# Windows: 下载并安装 Chrome
# Linux: sudo apt-get install google-chrome-stable
# macOS: brew install --cask google-chrome
```

### 配置

1. 复制配置模板：
```bash
cp config/config-dev.yaml config/config.yaml
```

2. 修改配置文件中的数据库连接、API 密钥等信息

3. 确保 `data/` 目录中的敏感词文件存在

### 构建

```bash
# 使用 Makefile
make build

# 或直接使用 go build
go build -o dist/task-processor.exe ./cmd/task
```

### 运行

```bash
# 运行主程序
./dist/task-processor.exe

# 或使用 Makefile
make run
```

## 项目结构

```
task-processor/
├── cmd/                    # 应用入口
│   ├── task/              # 主任务处理器
│   ├── amazon-crawler/    # Amazon 爬虫
│   └── test-product-fetcher/ # 测试工具
├── config/                # 配置文件
│   └── config-dev.yaml   # 开发环境配置
├── data/                  # 业务数据文件
│   ├── sensitive_words.json
│   ├── sensitive_words_shein.json
│   └── sensitive_words_temu.json
├── internal/              # 内部代码
│   ├── api/              # API 处理
│   ├── auth/             # 认证模块
│   ├── common/           # 公共代码
│   ├── config/           # 配置管理
│   ├── platforms/        # 平台实现
│   │   ├── amazon/       # Amazon 平台
│   │   ├── shein/        # Shein 平台
│   │   └── temu/         # Temu 平台
│   ├── service/          # 业务服务
│   ├── worker/           # 工作池
│   └── utils/            # 工具函数
├── docs/                 # 文档
├── scripts/              # 脚本
└── dist/                 # 编译产物
```

## 开发指南

### 添加新平台

1. 在 `internal/platforms/` 下创建新平台目录
2. 实现平台特定的处理器接口
3. 在配置文件中添加平台配置
4. 更新调度器以支持新平台

### 测试

```bash
# 运行所有测试
make test

# 运行特定包的测试
go test ./internal/platforms/shein/...
```

### 构建选项

```bash
# 开发构建
make build

# 生产构建（优化）
make build-prod

# 交叉编译
make build-linux
make build-windows
```

## 配置说明

主要配置项：

- `database`: 数据库连接配置
- `redis`: Redis 连接配置
- `platforms`: 各平台 API 配置
- `scheduler`: 调度器配置
- `monitoring`: 监控配置

详细配置说明请参考 `config/config-dev.yaml` 中的注释。

## API 文档

系统提供 REST API 用于：

- 任务管理
- 状态查询
- 手动触发同步
- 监控数据获取

API 文档请参考 `docs/api.md`（待完善）。

## 部署

### Docker 部署

```bash
# 构建镜像
docker build -t task-processor .

# 运行容器
docker run -d --name task-processor \
  -v ./config:/app/config \
  -v ./data:/app/data \
  task-processor
```

### 系统服务

参考 `scripts/` 目录中的部署脚本。

## 监控

系统内置监控指标：

- 任务执行状态
- 处理速度
- 错误率
- 资源使用情况

可通过 API 或日志查看监控数据。

## 故障排除

### 常见问题

1. **Chrome 启动失败**
   - 确保 Chrome 浏览器已正确安装
   - 检查 Chrome 路径配置

2. **数据库连接失败**
   - 检查数据库服务是否运行
   - 验证连接配置

3. **API 调用失败**
   - 检查网络连接
   - 验证 API 密钥配置

### 日志

日志文件位置：
- 应用日志：`logs/app.log`
- 错误日志：`logs/error.log`

## 贡献

1. Fork 项目
2. 创建功能分支
3. 提交更改
4. 推送到分支
5. 创建 Pull Request

## 许可证

[MIT License](LICENSE)

## 更新日志

### v2.8.6
- 优化调度器性能
- 修复敏感词过滤问题
- 增强错误处理

### v2.8.5
- 添加 Temu 平台支持
- 改进反检测机制
- 优化内存使用

更多版本信息请查看 [CHANGELOG.md](CHANGELOG.md)。