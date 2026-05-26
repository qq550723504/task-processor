# Makefile for Task Processor
.PHONY: all build-all clean test test-fast test-all test-coverage help \
	build-1688-crawler-api build-amazon-crawler-api build-amazon-listing \
	build-product-listing-api build-productenrich-api build-shein-address-copy \
	build-shein build-temu

# 版本信息
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.appVersion=$(VERSION) -X main.buildTime=$(BUILD_TIME)

# 输出目录
BIN_DIR := bin

# 常用测试包
FAST_TEST_PACKAGES := ./cmd/product-listing-api ./internal/app/httpapi ./internal/crawler/alibaba1688 ./internal/listingkit ./internal/listingadmin ./internal/promptmgmt ./internal/listingsubscription
BOUNDARY_TEST_PACKAGES := ./tests/...
ALL_TEST_PACKAGES := ./cmd/... ./internal/... ./tests/... ./tools/... ./hack/debug/...

# 帮助信息
help:
	@echo "Task Processor 构建工具"
	@echo ""
	@echo "使用方法:"
	@echo "  make build-all          - 构建当前受维护的服务入口"
	@echo "  make build-temu         - 构建 TEMU 上架服务"
	@echo "  make build-shein        - 构建 SHEIN 上架服务"
	@echo "  make build-amazon-listing - 构建 Amazon 上架服务"
	@echo "  make build-amazon-crawler-api - 构建 Amazon 爬虫 API"
	@echo "  make build-1688-crawler-api - 构建 1688 爬虫 API"
	@echo "  make build-product-listing-api - 构建统一 ListingKit API"
	@echo "  make build-productenrich-api - 构建兼容 productenrich API"
	@echo "  make clean              - 清理构建文件"
	@echo "  make test               - 运行常用快速测试"
	@echo "  make test-fast          - 运行常用快速测试"
	@echo "  make test-all           - 运行全部 Go 测试"
	@echo ""

# 构建所有服务
build-all: build-temu build-shein build-amazon-listing build-amazon-crawler-api build-1688-crawler-api build-product-listing-api build-productenrich-api
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

# Amazon 上架服务
build-amazon-listing:
	@echo "🔨 构建 Amazon 上架服务..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/amazon-listing ./cmd/amazon-listing
	@echo "✅ Amazon 上架服务构建完成: $(BIN_DIR)/amazon-listing"

# Amazon 爬虫 API 服务（不依赖 RabbitMQ）
build-amazon-crawler-api:
	@echo "🔨 构建 Amazon 爬虫 API 服务..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/amazon-crawler-api ./cmd/amazon-crawler-api
	@echo "✅ Amazon 爬虫 API 服务构建完成: $(BIN_DIR)/amazon-crawler-api"

# 1688 爬虫 API 服务（不依赖 RabbitMQ）
build-1688-crawler-api:
	@echo "🔨 构建 1688 爬虫 API 服务..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/1688-crawler-api ./cmd/1688-crawler-api
	@echo "✅ 1688 爬虫 API 服务构建完成: $(BIN_DIR)/1688-crawler-api"

# 统一 ListingKit API
build-product-listing-api:
	@echo "🔨 构建统一 ListingKit API..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/product-listing-api ./cmd/product-listing-api
	@echo "✅ 统一 ListingKit API 构建完成: $(BIN_DIR)/product-listing-api"

# 兼容 productenrich API
build-productenrich-api:
	@echo "🔨 构建兼容 productenrich API..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/productenrich-api ./cmd/productenrich-api
	@echo "✅ 兼容 productenrich API 构建完成: $(BIN_DIR)/productenrich-api"

# SHEIN 地址复制工具
build-shein-address-copy:
	@echo "🔨 构建 SHEIN 地址复制工具..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/shein-address-copy ./cmd/shein-address-copy
	@echo "✅ SHEIN 地址复制工具构建完成: $(BIN_DIR)/shein-address-copy"

# 清理构建文件
clean:
	@echo "🧹 清理构建文件..."
	rm -rf $(BIN_DIR)
	@echo "✅ 清理完成"

# 运行测试
test: test-fast

# 运行常用快速测试
test-fast:
	@echo "🧪 运行常用快速测试..."
	go test -v $(FAST_TEST_PACKAGES)
	go test -v $(BOUNDARY_TEST_PACKAGES)

# 运行全部 Go 测试
test-all:
	@echo "🧪 运行全部 Go 测试..."
	go test -v $(ALL_TEST_PACKAGES)

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
	go run ./cmd/temu-listing --config=config/config-dev.yaml

# 本地运行 SHEIN 服务
run-shein:
	@echo "🚀 启动 SHEIN 上架服务..."
	go run ./cmd/shein-listing --config=config/config-dev.yaml

# 本地运行 Amazon 上架服务
run-amazon-listing:
	@echo "🚀 启动 Amazon 上架服务..."
	go run ./cmd/amazon-listing --config=config/config-dev.yaml

# 本地运行 Amazon 爬虫 API
run-amazon-crawler-api:
	@echo "🚀 启动 Amazon 爬虫 API 服务..."
	go run ./cmd/amazon-crawler-api --config=config/config-dev.yaml --port=8080

# 本地运行 1688 爬虫 API
run-1688-crawler-api:
	@echo "🚀 启动 1688 爬虫 API 服务..."
	go run ./cmd/1688-crawler-api --config=config/config-dev.yaml --port=8083
