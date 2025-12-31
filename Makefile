.PHONY: build clean test run build-prod build-linux build-windows

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

# 默认构建
build:
	go build $(LDFLAGS) -o dist/task-processor.exe ./cmd/task

# 生产构建（优化）
build-prod:
	go build $(LDFLAGS) -ldflags "-s -w" -o dist/task-processor.exe ./cmd/task

# 清理构建产物
clean:
	rm -rf dist/*.exe

# 运行测试
test:
	go test -v ./...

# 运行程序
run:
	go run ./cmd/task

# 交叉编译 - Linux
build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/task-processor-linux ./cmd/task

# 交叉编译 - Windows
build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/task-processor.exe ./cmd/task

# 构建所有平台
build-all: build-linux build-windows

# 检查代码格式
fmt:
	go fmt ./...

# 代码检查
vet:
	go vet ./...

# 依赖管理
tidy:
	go mod tidy

# 完整检查（格式化 + 检查 + 测试 + 构建）
check: fmt vet test build

# 安装依赖
deps:
	go mod download

# 查看帮助
help:
	@echo "可用的命令："
	@echo "  build        - 构建程序"
	@echo "  build-prod   - 生产环境构建（优化）"
	@echo "  clean        - 清理构建产物"
	@echo "  test         - 运行测试"
	@echo "  run          - 运行程序"
	@echo "  build-linux  - 构建 Linux 版本"
	@echo "  build-windows- 构建 Windows 版本"
	@echo "  build-all    - 构建所有平台版本"
	@echo "  fmt          - 格式化代码"
	@echo "  vet          - 代码检查"
	@echo "  tidy         - 整理依赖"
	@echo "  check        - 完整检查"
	@echo "  deps         - 安装依赖"
	@echo "  help         - 显示帮助"