# Makefile for Task Processor / ListingKit
.PHONY: all build-all clean test test-fast test-all test-coverage help \
	build-listing-control-plane build-product-listing-api build-shein build-temu \
	run-listing-control-plane run-product-listing-api run-shein run-temu lint fmt

# 版本信息
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.appVersion=$(VERSION) -X main.buildTime=$(BUILD_TIME)

# 输出目录
BIN_DIR := bin

# 当前维护的测试包
FAST_TEST_PACKAGES := ./cmd/product-listing-api ./cmd/listing-control-plane ./cmd/shein-listing ./cmd/temu-listing ./internal/app/httpapi ./internal/app/runtime/listingcontrol ./internal/listingkit ./internal/listingadmin ./internal/promptmgmt ./internal/listingsubscription
BOUNDARY_TEST_PACKAGES := ./tests/...
ALL_TEST_PACKAGES := ./cmd/... ./internal/... ./tests/... ./tools/... ./hack/debug/...

# 帮助信息
help:
	@echo "Task Processor / ListingKit 构建工具"
	@echo ""
	@echo "当前正式入口以 docs/development/repository-structure.md 和 tests/repository_structure_test.go 为准。"
	@echo ""
	@echo "使用方法:"
	@echo "  make build-all                 - 构建当前受维护的服务入口"
	@echo "  make build-listing-control-plane - 构建 Listing Control Plane"
	@echo "  make build-product-listing-api - 构建统一 ListingKit API"
	@echo "  make build-shein               - 构建 SHEIN Listing runtime"
	@echo "  make build-temu                - 构建 TEMU Listing runtime"
	@echo "  make clean                     - 清理构建文件"
	@echo "  make test                      - 运行常用快速测试"
	@echo "  make test-fast                 - 运行常用快速测试"
	@echo "  make test-all                  - 运行全部 Go 测试"
	@echo ""

all: build-all

# 构建所有当前受维护服务
build-all: build-listing-control-plane build-product-listing-api build-shein build-temu
	@echo "✅ 所有当前受维护服务构建完成"

# Listing Control Plane
build-listing-control-plane:
	@echo "🔨 构建 Listing Control Plane..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/listing-control-plane ./cmd/listing-control-plane
	@echo "✅ Listing Control Plane 构建完成: $(BIN_DIR)/listing-control-plane"

# 统一 ListingKit API
build-product-listing-api:
	@echo "🔨 构建统一 ListingKit API..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/product-listing-api ./cmd/product-listing-api
	@echo "✅ 统一 ListingKit API 构建完成: $(BIN_DIR)/product-listing-api"

# SHEIN Listing runtime
build-shein:
	@echo "🔨 构建 SHEIN Listing runtime..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/shein-listing ./cmd/shein-listing
	@echo "✅ SHEIN Listing runtime 构建完成: $(BIN_DIR)/shein-listing"

# TEMU Listing runtime
build-temu:
	@echo "🔨 构建 TEMU Listing runtime..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/temu-listing ./cmd/temu-listing
	@echo "✅ TEMU Listing runtime 构建完成: $(BIN_DIR)/temu-listing"

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

# 本地运行 Listing Control Plane
run-listing-control-plane:
	@echo "🚀 启动 Listing Control Plane..."
	go run ./cmd/listing-control-plane -config=config/config-dev.yaml -log-level=info

# 本地运行统一 ListingKit API
run-product-listing-api:
	@echo "🚀 启动统一 ListingKit API..."
	go run ./cmd/product-listing-api --config=config/config-dev.yaml --port=8085

# 本地运行 SHEIN Listing runtime
run-shein:
	@echo "🚀 启动 SHEIN Listing runtime..."
	go run ./cmd/shein-listing --config=config/config-dev.yaml

# 本地运行 TEMU Listing runtime
run-temu:
	@echo "🚀 启动 TEMU Listing runtime..."
	go run ./cmd/temu-listing --config=config/config-dev.yaml
