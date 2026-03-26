# Makefile for Task Processor
.PHONY: all build-all clean test help

# 版本信息
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.appVersion=$(VERSION) -X main.buildTime=$(BUILD_TIME)

# 输出目录
BIN_DIR := bin

# 帮助信息
help:
	@echo "Task Processor 构建工具"
	@echo ""
	@echo "使用方法:"
	@echo "  make build-all          - 构建所有服务"
	@echo "  make build-temu         - 构建 TEMU 上架服务"
	@echo "  make build-shein        - 构建 SHEIN 上架服务"
	@echo "  make build-amazon-crawler - 构建 Amazon 爬虫服务"
	@echo "  make build-1688-crawler - 构建 1688 爬虫服务"
	@echo "  make clean              - 清理构建文件"
	@echo "  make test               - 运行测试"
	@echo ""

# 构建所有服务
build-all: build-temu build-shein build-amazon-crawler build-1688-crawler build-amazon-crawler-api build-1688-crawler-api
	@echo "✅ 所有服务构建完成"

# TEMU 上架服务
build-temu:
	@echo "🔨 构建 TEMU 上架服务..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/temu-listing cmd/temu-listing/main.go
	@echo "✅ TEMU 上架服务构建完成: $(BIN_DIR)/temu-listing"

# SHEIN 上架服务
build-shein:
	@echo "🔨 构建 SHEIN 上架服务..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/shein-listing cmd/shein-listing/main.go
	@echo "✅ SHEIN 上架服务构建完成: $(BIN_DIR)/shein-listing"

# Amazon 爬虫服务
build-amazon-crawler:
	@echo "🔨 构建 Amazon 爬虫服务..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/amazon-crawler cmd/amazon-crawler/main.go
	@echo "✅ Amazon 爬虫服务构建完成: $(BIN_DIR)/amazon-crawler"

# 1688 爬虫服务
build-1688-crawler:
	@echo "🔨 构建 1688 爬虫服务..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/1688-crawler cmd/1688-crawler/main.go
	@echo "✅ 1688 爬虫服务构建完成: $(BIN_DIR)/1688-crawler"

# Amazon 爬虫 API 服务（不依赖 RabbitMQ）
build-amazon-crawler-api:
	@echo "🔨 构建 Amazon 爬虫 API 服务..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/amazon-crawler-api cmd/amazon-crawler-api/main.go
	@echo "✅ Amazon 爬虫 API 服务构建完成: $(BIN_DIR)/amazon-crawler-api"

# 1688 爬虫 API 服务（不依赖 RabbitMQ）
build-1688-crawler-api:
	@echo "🔨 构建 1688 爬虫 API 服务..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/1688-crawler-api cmd/1688-crawler-api/main.go
	@echo "✅ 1688 爬虫 API 服务构建完成: $(BIN_DIR)/1688-crawler-api"

# 原有的 RabbitMQ Consumer（保留用于兼容）
build-rabbitmq-consumer:
	@echo "🔨 构建 RabbitMQ Consumer（兼容模式）..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/rabbitmq-consumer cmd/rabbitmq-consumer/main.go
	@echo "✅ RabbitMQ Consumer 构建完成: $(BIN_DIR)/rabbitmq-consumer"

# 清理构建文件
clean:
	@echo "🧹 清理构建文件..."
	rm -rf $(BIN_DIR)
	@echo "✅ 清理完成"

# 运行测试
test:
	@echo "🧪 运行测试..."
	go test -v ./...

# 运行测试（带覆盖率）
test-coverage:
	@echo "🧪 运行测试（带覆盖率）..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ 覆盖率报告已生成: coverage.html"

# 代码检查
lint:
	@echo "🔍 运行代码检查..."
	golangci-lint run

# 格式化代码
fmt:
	@echo "✨ 格式化代码..."
	go fmt ./...

# 本地运行 TEMU 服务
run-temu:
	@echo "🚀 启动 TEMU 上架服务..."
	go run cmd/temu-listing/main.go --config=config/config-dev.yaml

# 本地运行 SHEIN 服务
run-shein:
	@echo "🚀 启动 SHEIN 上架服务..."
	go run cmd/shein-listing/main.go --config=config/config-dev.yaml

# 本地运行 Amazon 爬虫
run-amazon-crawler:
	@echo "🚀 启动 Amazon 爬虫服务..."
	go run cmd/amazon-crawler/main.go --config=config/config-dev.yaml

# 本地运行 1688 爬虫
run-1688-crawler:
	@echo "🚀 启动 1688 爬虫服务..."
	go run cmd/1688-crawler/main.go --config=config/config-dev.yaml

# 本地运行 Amazon 爬虫 API
run-amazon-crawler-api:
	@echo "🚀 启动 Amazon 爬虫 API 服务..."
	go run cmd/amazon-crawler-api/main.go --config=config/config-dev.yaml --port=8080

# 本地运行 1688 爬虫 API
run-1688-crawler-api:
	@echo "🚀 启动 1688 爬虫 API 服务..."
	go run cmd/1688-crawler-api/main.go --config=config/config-dev.yaml --port=8083
