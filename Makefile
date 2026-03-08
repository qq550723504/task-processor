# Makefile for Task Processor

# 变量定义
APP_NAME=task-processor
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION=$(shell go version | awk '{print $$3}')

# 构建目录
BUILD_DIR=bin
CMD_DIR=cmd

# Go 相关变量
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# 构建标志
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

.PHONY: all build clean test coverage lint fmt help install-tools

# 默认目标
all: clean fmt lint test build

# 帮助信息
help:
	@echo "Task Processor Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make build          - 编译所有二进制文件"
	@echo "  make build-task     - 编译主任务处理器"
	@echo "  make build-consumer - 编译 RabbitMQ 消费者"
	@echo "  make clean          - 清理构建文件"
	@echo "  make test           - 运行测试"
	@echo "  make coverage       - 生成测试覆盖率报告"
	@echo "  make lint           - 运行代码检查"
	@echo "  make fmt            - 格式化代码"
	@echo "  make install-tools  - 安装开发工具"
	@echo "  make run            - 运行主程序"
	@echo "  make docker-build   - 构建 Docker 镜像"
	@echo "  make docker-up      - 启动 Docker Compose"
	@echo "  make docker-down    - 停止 Docker Compose"

# 编译所有二进制文件
build: build-task build-consumer build-crawler-consumer

# 编译主任务处理器
build-task:
	@echo "Building task processor..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) $(CMD_DIR)/task/main.go

# 编译 RabbitMQ 消费者
build-consumer:
	@echo "Building RabbitMQ consumer..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/rabbitmq-consumer $(CMD_DIR)/rabbitmq-consumer/main.go

# 编译爬虫消费者
build-crawler-consumer:
	@echo "Building crawler consumer..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/crawler-consumer $(CMD_DIR)/crawler-consumer/main.go

# 清理构建文件
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# 运行测试
test:
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

# 生成测试覆盖率报告
coverage:
	@echo "Generating coverage report..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# 运行代码检查
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not found, installing..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

# 格式化代码
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# 安装开发工具
install-tools:
	@echo "Installing development tools..."
	$(GOGET) $(shell go list -f '{{join .Imports " "}}' tools/tools.go)

# 下载依赖
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

# 运行主程序
run: build-task
	@echo "Running task processor..."
	./$(BUILD_DIR)/$(APP_NAME)

# 运行 RabbitMQ 消费者
run-consumer: build-consumer
	@echo "Running RabbitMQ consumer..."
	./$(BUILD_DIR)/rabbitmq-consumer

# Docker 相关命令
docker-build:
	@echo "Building Docker image..."
	docker build -f deployments/docker/Dockerfile -t $(APP_NAME):$(VERSION) .

docker-up:
	@echo "Starting Docker Compose..."
	docker-compose -f deployments/docker/docker-compose.yml up -d

docker-down:
	@echo "Stopping Docker Compose..."
	docker-compose -f deployments/docker/docker-compose.yml down

docker-logs:
	@echo "Showing Docker logs..."
	docker-compose -f deployments/docker/docker-compose.yml logs -f

# 数据库迁移
migrate-up:
	@echo "Running database migrations..."
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	@echo "Rolling back database migrations..."
	migrate -path migrations -database "$(DB_URL)" down 1

migrate-create:
	@echo "Creating new migration: $(name)"
	migrate create -ext sql -dir migrations -seq $(name)

# 生成代码
generate:
	@echo "Generating code..."
	$(GOCMD) generate ./...

# 生成 Swagger 文档
swagger:
	@echo "Generating Swagger documentation..."
	@which swag > /dev/null || (echo "swag not found, installing..." && go install github.com/swaggo/swag/cmd/swag@latest)
	swag init -g cmd/task/main.go -o api/openapi

# 生成 Mock 代码
mock:
	@echo "Generating mock code..."
	@which mockgen > /dev/null || (echo "mockgen not found, installing..." && go install github.com/golang/mock/mockgen@latest)
	$(GOCMD) generate ./...

# 检查代码安全性
security:
	@echo "Running security check..."
	@which gosec > /dev/null || (echo "gosec not found, installing..." && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	gosec ./...

# 性能分析
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

# 查看项目信息
info:
	@echo "Project Information:"
	@echo "  Name:       $(APP_NAME)"
	@echo "  Version:    $(VERSION)"
	@echo "  Build Time: $(BUILD_TIME)"
	@echo "  Go Version: $(GO_VERSION)"

# 完整的 CI 流程
ci: deps fmt lint test build
	@echo "CI pipeline completed successfully!"
