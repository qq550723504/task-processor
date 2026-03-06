#!/bin/bash

# 停止服务脚本

echo "=========================================="
echo "🛑 停止 Task Processor 依赖服务"
echo "=========================================="
echo ""

# 停止并删除容器
docker compose -f "$(dirname "$0")/docker-compose.yml" down

echo ""
echo "✅ 服务已停止"
echo ""
echo "💡 提示:"
echo "  - 如需删除数据卷: docker compose -f scripts/docker-compose.yml down -v"
echo "  - 如需重新启动: bash scripts/start-services.sh"
echo ""
