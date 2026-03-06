#!/bin/bash

# 发送测试消息到 RabbitMQ 的脚本

echo "=========================================="
echo "📨 发送测试消息到 RabbitMQ"
echo "=========================================="
echo ""

# 默认参数
PLATFORM=${1:-"amazon"}
PRIORITY=${2:-"normal"}
TASK_ID=${3:-"test-$(date +%s)"}

echo "📋 消息参数:"
echo "  平台: $PLATFORM"
echo "  优先级: $PRIORITY"
echo "  任务ID: $TASK_ID"
echo ""

# 构造测试消息
MESSAGE=$(cat <<EOF
{
  "task_id": "$TASK_ID",
  "platform": "$PLATFORM",
  "type": "listing",
  "priority": "$PRIORITY",
  "data": {
    "source_url": "https://www.amazon.com/dp/B0BRKPBMVY",
    "target_store_id": 169,
    "test_mode": true
  },
  "created_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
EOF
)

echo "📝 消息内容:"
echo "$MESSAGE" | jq '.' 2>/dev/null || echo "$MESSAGE"
echo ""

# 发送消息到 RabbitMQ
echo "🚀 发送消息..."

# 使用 docker exec 在 RabbitMQ 容器中执行命令
docker exec task-processor-rabbitmq rabbitmqadmin publish \
  exchange=tasks.exchange \
  routing_key="${PLATFORM}.${PRIORITY}" \
  payload="$MESSAGE" \
  properties='{"content_type":"application/json","delivery_mode":2}' \
  2>&1

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ 消息发送成功！"
    echo ""
    echo "📊 查看消息处理:"
    echo "  - 查看消费者日志: (在 Kiro 中查看进程输出)"
    echo "  - RabbitMQ 管理界面: http://localhost:15672/#/queues"
    echo "  - 队列名称: ${PLATFORM}.tasks.queue"
else
    echo ""
    echo "❌ 消息发送失败"
    echo ""
    echo "💡 提示:"
    echo "  1. 确保 RabbitMQ 容器正在运行: docker ps"
    echo "  2. 检查 rabbitmqadmin 是否可用"
    echo "  3. 或使用 RabbitMQ 管理界面手动发送消息"
fi

echo ""
