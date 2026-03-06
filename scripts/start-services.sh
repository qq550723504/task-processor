#!/bin/bash

# 启动 RabbitMQ 和相关服务的脚本

echo "=========================================="
echo "🐳 启动 Task Processor 依赖服务"
echo "=========================================="
echo ""

# 检查 Docker 是否运行
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker 未运行，请先启动 Docker"
    exit 1
fi

echo "✅ Docker 正在运行"
echo ""

# 启动服务
echo "🚀 启动服务容器..."
docker compose -f "$(dirname "$0")/docker-compose.yml" up -d

echo ""
echo "⏳ 等待服务启动..."
sleep 5

# 检查服务状态
echo ""
echo "📊 服务状态:"
docker compose -f "$(dirname "$0")/docker-compose.yml" ps

echo ""
echo "=========================================="
echo "✅ 服务启动完成！"
echo "=========================================="
echo ""
echo "📌 访问地址:"
echo "  - RabbitMQ 管理界面: http://localhost:15672"
echo "    用户名: admin"
echo "    密码: admin123"
echo ""
echo "  - RabbitMQ AMQP: amqp://admin:admin123@localhost:5672/"
echo "  - Redis: localhost:6379"
echo ""
echo "🔍 查看日志:"
echo "  docker compose -f scripts/docker-compose.yml logs -f rabbitmq"
echo "  docker compose -f scripts/docker-compose.yml logs -f redis"
echo ""
echo "🛑 停止服务:"
echo "  bash scripts/stop-services.sh"
echo ""
