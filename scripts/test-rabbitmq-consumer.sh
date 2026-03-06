#!/bin/bash

# RabbitMQ Consumer 测试脚本
# 用于测试 rabbitmq-consumer 是否正常工作

echo "=========================================="
echo "🧪 RabbitMQ Consumer 测试脚本"
echo "=========================================="
echo ""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 测试结果统计
PASSED=0
FAILED=0

# 测试函数
test_step() {
    echo -e "${YELLOW}[测试]${NC} $1"
}

test_pass() {
    echo -e "${GREEN}[✓]${NC} $1"
    ((PASSED++))
}

test_fail() {
    echo -e "${RED}[✗]${NC} $1"
    ((FAILED++))
}

# 1. 检查 Go 环境
test_step "检查 Go 环境..."
if command -v go &> /dev/null; then
    GO_VERSION=$(go version)
    test_pass "Go 已安装: $GO_VERSION"
else
    test_fail "Go 未安装"
    exit 1
fi
echo ""

# 2. 检查配置文件
test_step "检查配置文件..."
if [ -f "config/rabbitmq-config.yaml" ]; then
    test_pass "RabbitMQ 配置文件存在"
else
    test_fail "RabbitMQ 配置文件不存在"
fi

if [ -f "config/config-dev.yaml" ]; then
    test_pass "应用配置文件存在"
else
    test_fail "应用配置文件不存在"
fi
echo ""

# 3. 检查依赖
test_step "检查 Go 模块依赖..."
if go mod verify &> /dev/null; then
    test_pass "Go 模块依赖验证通过"
else
    test_fail "Go 模块依赖验证失败"
fi
echo ""

# 4. 编译测试
test_step "编译 rabbitmq-consumer..."
if go build -o bin/rabbitmq-consumer-test ./cmd/rabbitmq-consumer 2>&1; then
    test_pass "编译成功"
else
    test_fail "编译失败"
    exit 1
fi
echo ""

# 5. 检查 RabbitMQ 连接
test_step "检查 RabbitMQ 服务..."
RABBITMQ_URL=$(grep "url:" config/rabbitmq-config.yaml | awk '{print $2}' | tr -d '"')
if [ -z "$RABBITMQ_URL" ]; then
    test_fail "无法从配置文件读取 RabbitMQ URL"
else
    # 提取主机和端口
    RABBITMQ_HOST=$(echo $RABBITMQ_URL | sed -n 's/.*@\(.*\):.*/\1/p')
    RABBITMQ_PORT=$(echo $RABBITMQ_URL | sed -n 's/.*:\([0-9]*\)\/.*/\1/p')
    
    if [ -z "$RABBITMQ_HOST" ] || [ -z "$RABBITMQ_PORT" ]; then
        test_fail "无法解析 RabbitMQ 地址"
    else
        echo "  RabbitMQ 地址: $RABBITMQ_HOST:$RABBITMQ_PORT"
        
        # 检查端口是否可访问
        if timeout 3 bash -c "cat < /dev/null > /dev/tcp/$RABBITMQ_HOST/$RABBITMQ_PORT" 2>/dev/null; then
            test_pass "RabbitMQ 服务可访问"
        else
            test_fail "RabbitMQ 服务不可访问 ($RABBITMQ_HOST:$RABBITMQ_PORT)"
            echo "  提示: 请确保 RabbitMQ 服务正在运行"
        fi
    fi
fi
echo ""

# 6. 语法检查
test_step "Go 代码语法检查..."
if go vet ./cmd/rabbitmq-consumer/... 2>&1; then
    test_pass "代码语法检查通过"
else
    test_fail "代码语法检查失败"
fi
echo ""

# 7. 显示帮助信息
test_step "测试命令行参数..."
if ./bin/rabbitmq-consumer-test -h &> /dev/null; then
    test_pass "命令行参数正常"
else
    test_fail "命令行参数异常"
fi
echo ""

# 8. 测试总结
echo "=========================================="
echo "📊 测试总结"
echo "=========================================="
echo -e "${GREEN}通过: $PASSED${NC}"
echo -e "${RED}失败: $FAILED${NC}"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ 所有测试通过!${NC}"
    echo ""
    echo "🚀 启动建议:"
    echo "  ./bin/rabbitmq-consumer-test --config config/rabbitmq-config.yaml --app-config config/config-dev.yaml --log-level debug"
    echo ""
    echo "或者使用默认参数:"
    echo "  ./bin/rabbitmq-consumer-test"
    exit 0
else
    echo -e "${RED}❌ 有 $FAILED 个测试失败${NC}"
    exit 1
fi
