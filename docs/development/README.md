# 开发文档目录

## 用途

存放开发相关的文档，包括环境搭建、编码规范、开发流程等。

## 目录结构

```
development/
├── README.md           # 本文件
├── setup.md            # 开发环境搭建
├── coding-standards.md # 编码规范
├── git-workflow.md     # Git 工作流
├── testing-guide.md    # 测试指南
└── troubleshooting.md  # 常见问题
```

## 应该放置的文件

### 1. 开发环境搭建（setup.md）

**模板：**
```markdown
# 开发环境搭建

## 前置要求

### 必需软件

- Go 1.21 或更高版本
- Git
- Chrome/Chromium 浏览器
- RabbitMQ（可选，用于本地测试）

### 推荐工具

- VS Code 或 GoLand
- Postman（API 测试）
- Docker（容器化部署）

## 安装步骤

### 1. 安装 Go

**Windows:**
```bash
# 下载并安装
https://golang.org/dl/

# 验证安装
go version
```

**Linux/Mac:**
```bash
# 使用包管理器安装
# Ubuntu/Debian
sudo apt-get install golang-go

# Mac
brew install go

# 验证安装
go version
```

### 2. 克隆项目

```bash
git clone https://github.com/your-org/task-processor.git
cd task-processor
```

### 3. 安装依赖

```bash
go mod download
```

### 4. 配置环境

```bash
# 复制配置文件
cp .env.example .env
cp config/config-dev.yaml config/config.yaml

# 编辑配置文件
# 修改数据库连接、RabbitMQ 地址等
```

### 5. 运行项目

```bash
# 编译
go build -o bin/task-processor ./cmd/task

# 运行
./bin/task-processor -config config/config.yaml
```

## IDE 配置

### VS Code

推荐安装的扩展：
- Go (官方)
- Go Test Explorer
- GitLens
- YAML

配置文件 `.vscode/settings.json`:
```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "package",
  "editor.formatOnSave": true
}
```

### GoLand

1. 打开项目
2. 配置 Go SDK
3. 启用 Go Modules
4. 配置代码格式化

## 本地开发

### 启动依赖服务

使用 Docker Compose:
```bash
docker-compose up -d rabbitmq
```

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/app/scheduler

# 运行带覆盖率的测试
go test -cover ./...
```

### 调试

VS Code 调试配置 `.vscode/launch.json`:
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch",
      "type": "go",
      "request": "launch",
      "mode": "debug",
      "program": "${workspaceFolder}/cmd/task",
      "args": ["-config", "config/config.yaml"]
    }
  ]
}
```

## 常见问题

### 1. 依赖下载失败

```bash
# 设置 Go 代理
go env -w GOPROXY=https://goproxy.cn,direct
```

### 2. 浏览器启动失败

确保 Chrome 路径配置正确：
```yaml
browser:
  browser_path: "C:/Program Files/Google/Chrome/Application/chrome.exe"
```

### 3. RabbitMQ 连接失败

检查 RabbitMQ 是否运行：
```bash
docker ps | grep rabbitmq
```
```

### 2. 编码规范（coding-standards.md）

**模板：**
```markdown
# 编码规范

## Go 代码规范

### 1. 命名规范

**包名：**
- 使用小写字母
- 简短且有意义
- 避免下划线

```go
// 好的例子
package scheduler
package utils

// 不好的例子
package task_scheduler
package Scheduler
```

**变量名：**
- 使用驼峰命名
- 导出的变量首字母大写
- 私有变量首字母小写

```go
// 导出变量
var DefaultConfig = &Config{}

// 私有变量
var internalCache = make(map[string]interface{})
```

**函数名：**
- 使用驼峰命名
- 动词开头
- 清晰表达意图

```go
// 好的例子
func GetUserByID(id int) (*User, error)
func ValidateProduct(product *Product) error

// 不好的例子
func user(id int) (*User, error)
func check(product *Product) error
```

### 2. 代码格式

使用 `gofmt` 和 `goimports` 格式化代码：

```bash
# 格式化单个文件
gofmt -w file.go

# 格式化整个项目
gofmt -w .

# 使用 goimports
goimports -w .
```

### 3. 注释规范

**包注释：**
```go
// Package scheduler 提供任务调度功能
package scheduler
```

**函数注释：**
```go
// GetUserByID 根据用户ID获取用户信息
// 如果用户不存在，返回 ErrUserNotFound 错误
func GetUserByID(id int) (*User, error) {
    // ...
}
```

**复杂逻辑注释：**
```go
// 计算产品最终价格
// 1. 获取基础价格
// 2. 应用折扣
// 3. 计算税费
// 4. 加上运费
finalPrice := calculateFinalPrice(product)
```

### 4. 错误处理

**返回错误：**
```go
// 好的例子
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

// 不好的例子
if err != nil {
    return err  // 丢失了上下文信息
}
```

**自定义错误：**
```go
var (
    ErrUserNotFound = errors.New("user not found")
    ErrInvalidInput = errors.New("invalid input")
)
```

### 5. 接口设计

**小接口原则：**
```go
// 好的例子 - 单一职责
type Reader interface {
    Read(p []byte) (n int, err error)
}

type Writer interface {
    Write(p []byte) (n int, err error)
}

// 不好的例子 - 接口过大
type FileOperator interface {
    Read(p []byte) (n int, err error)
    Write(p []byte) (n int, err error)
    Close() error
    Seek(offset int64, whence int) (int64, error)
    // ... 更多方法
}
```

### 6. 并发安全

**使用互斥锁：**
```go
type SafeCounter struct {
    mu    sync.Mutex
    count int
}

func (c *SafeCounter) Inc() {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.count++
}
```

**使用 channel：**
```go
func worker(jobs <-chan Job, results chan<- Result) {
    for job := range jobs {
        result := process(job)
        results <- result
    }
}
```

### 7. 测试规范

**测试文件命名：**
- 文件名以 `_test.go` 结尾
- 与被测试文件同目录

**测试函数命名：**
```go
func TestGetUserByID(t *testing.T) {
    // 测试逻辑
}

func TestGetUserByID_NotFound(t *testing.T) {
    // 测试用户不存在的情况
}
```

**表驱动测试：**
```go
func TestAdd(t *testing.T) {
    tests := []struct {
        name string
        a, b int
        want int
    }{
        {"positive", 1, 2, 3},
        {"negative", -1, -2, -3},
        {"zero", 0, 0, 0},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := Add(tt.a, tt.b)
            if got != tt.want {
                t.Errorf("Add(%d, %d) = %d, want %d", 
                    tt.a, tt.b, got, tt.want)
            }
        })
    }
}
```

## 项目特定规范

### 1. 分层架构

遵循项目的分层架构：
- core - 核心基础功能
- domain - 领域模型和业务规则
- infra - 基础设施实现
- app - 应用服务
- platforms - 平台特定逻辑

### 2. 依赖注入

使用 DI 容器管理依赖：
```go
// 注册服务
container.RegisterSingleton("userService", func(c di.Container) (any, error) {
    repo, _ := c.Get("userRepo")
    return NewUserService(repo.(UserRepository)), nil
})

// 获取服务
userService, err := container.Get("userService")
```

### 3. 日志记录

使用结构化日志：
```go
logger.WithFields(logrus.Fields{
    "user_id": userID,
    "action": "login",
}).Info("用户登录成功")
```

### 4. 配置管理

使用配置管理器：
```go
config := configManager.GetCurrent()
timeout := config.Worker.TaskTimeout
```

## 代码审查检查清单

- [ ] 代码格式正确（gofmt）
- [ ] 导入已整理（goimports）
- [ ] 有适当的注释
- [ ] 错误处理完整
- [ ] 有单元测试
- [ ] 测试覆盖率 > 80%
- [ ] 无 lint 警告
- [ ] 遵循项目架构
- [ ] 使用依赖注入
- [ ] 日志记录完整

## 工具配置

### golangci-lint

配置文件 `.golangci.yml`:
```yaml
linters:
  enable:
    - gofmt
    - goimports
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - structcheck
    - varcheck
    - ineffassign
    - deadcode

linters-settings:
  gofmt:
    simplify: true
  goimports:
    local-prefixes: task-processor
```

运行 lint:
```bash
golangci-lint run
```
```

### 3. Git 工作流（git-workflow.md）

**模板：**
```markdown
# Git 工作流

## 分支策略

### 主要分支

- `main` - 生产环境代码
- `develop` - 开发环境代码

### 功能分支

- `feature/*` - 新功能开发
- `bugfix/*` - Bug 修复
- `hotfix/*` - 紧急修复
- `refactor/*` - 代码重构

## 工作流程

### 1. 创建功能分支

```bash
# 从 develop 创建功能分支
git checkout develop
git pull origin develop
git checkout -b feature/add-user-api
```

### 2. 开发和提交

```bash
# 进行开发
# ...

# 提交代码
git add .
git commit -m "feat: 添加用户API"
```

### 3. 推送到远程

```bash
git push origin feature/add-user-api
```

### 4. 创建 Pull Request

1. 在 GitHub/GitLab 上创建 PR
2. 填写 PR 描述
3. 请求代码审查
4. 等待审查通过

### 5. 合并到 develop

```bash
# 审查通过后合并
git checkout develop
git merge --no-ff feature/add-user-api
git push origin develop
```

### 6. 删除功能分支

```bash
git branch -d feature/add-user-api
git push origin --delete feature/add-user-api
```

## 提交信息规范

### 格式

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Type 类型

- `feat` - 新功能
- `fix` - Bug 修复
- `docs` - 文档更新
- `style` - 代码格式（不影响功能）
- `refactor` - 重构
- `test` - 测试相关
- `chore` - 构建/工具相关

### 示例

```
feat(scheduler): 添加任务优先级支持

- 在 Task 结构中添加 Priority 字段
- 实现基于优先级的任务调度
- 添加相关单元测试

Closes #123
```

## 代码审查

### 审查者职责

1. 检查代码质量
2. 验证功能正确性
3. 确保符合编码规范
4. 提出改进建议

### 被审查者职责

1. 提供清晰的 PR 描述
2. 响应审查意见
3. 及时修改代码
4. 保持沟通

## 发布流程

### 1. 创建发布分支

```bash
git checkout develop
git checkout -b release/v1.2.0
```

### 2. 更新版本号

```bash
# 更新 version 文件或配置
echo "1.2.0" > VERSION
git commit -am "chore: bump version to 1.2.0"
```

### 3. 合并到 main

```bash
git checkout main
git merge --no-ff release/v1.2.0
git tag -a v1.2.0 -m "Release version 1.2.0"
git push origin main --tags
```

### 4. 合并回 develop

```bash
git checkout develop
git merge --no-ff release/v1.2.0
git push origin develop
```
```

## 注意事项

1. 所有文档使用 Markdown 格式
2. 保持文档简洁实用
3. 提供实际可运行的示例
4. 定期更新文档内容
5. 收集开发者反馈
