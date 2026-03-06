#!/usr/bin/env python3
"""
发送测试消息到 RabbitMQ 的 Python 脚本
需要安装: pip install pika
"""

import json
import sys
from datetime import datetime
from time import time as get_time

try:
    import pika
except ImportError:
    print("❌ 错误: 未安装 pika 库")
    print("请运行: pip install pika")
    sys.exit(1)


def send_test_message(platform="amazon", priority="normal", task_id=None):
    """发送测试消息到 RabbitMQ"""
    
    print("=" * 50)
    print("📨 发送测试消息到 RabbitMQ")
    print("=" * 50)
    print()
    
    # 生成任务ID
    if task_id is None:
        task_id = f"test-{int(get_time())}"
    
    print(f"📋 消息参数:")
    print(f"  平台: {platform}")
    print(f"  优先级: {priority}")
    print(f"  任务ID: {task_id}")
    print()
    
    # 构造测试消息（TaskMessage格式，直接作为消息体）
    timestamp = int(get_time())
    
    # 优先级映射
    priority_map = {
        "urgent": 1,
        "high": 5,
        "normal": 7,
        "low": 9
    }
    priority_value = priority_map.get(priority, 7)
    
    # TaskMessage（直接作为消息体）
    message = {
        "taskId": int(task_id.replace("test-", "")) if task_id.startswith("test-") else 123456,
        "tenantId": 1,
        "storeId": 169,
        "platform": platform,
        "region": "us",
        "categoryId": 1001,
        "productId": "B0BRKPBMVY",
        "priority": priority_value,
        "retryCount": 0,
        "maxRetryCount": 3,
        "createdAt": timestamp
    }
    
    print("📝 消息内容:")
    print(json.dumps(message, indent=2, ensure_ascii=False))
    print()
    
    # 连接到 RabbitMQ
    print("🔌 连接到 RabbitMQ...")
    try:
        credentials = pika.PlainCredentials('admin', 'admin123')
        parameters = pika.ConnectionParameters(
            host='localhost',
            port=5672,
            virtual_host='/',
            credentials=credentials
        )
        connection = pika.BlockingConnection(parameters)
        channel = connection.channel()
        print("✅ 连接成功")
        print()
        
        # 发送消息
        print("🚀 发送消息...")
        routing_key = f"{platform}.{priority}"
        
        channel.basic_publish(
            exchange='tasks.exchange',
            routing_key=routing_key,
            body=json.dumps(message),
            properties=pika.BasicProperties(
                content_type='application/json',
                delivery_mode=2,  # 持久化消息
                message_id=task_id,  # 设置消息ID
                type='task',  # 设置消息类型
            )
        )
        
        print("✅ 消息发送成功！")
        print()
        print(f"📊 消息详情:")
        print(f"  交换机: tasks.exchange")
        print(f"  路由键: {routing_key}")
        print(f"  队列: {platform}.tasks.queue")
        print()
        print("📊 查看消息处理:")
        print("  - 查看消费者日志: (在 Kiro 中查看进程输出)")
        print("  - RabbitMQ 管理界面: http://localhost:15672/#/queues")
        print(f"  - 队列名称: {platform}.tasks.queue")
        
        # 关闭连接
        connection.close()
        
    except pika.exceptions.AMQPConnectionError as e:
        print(f"❌ 连接失败: {e}")
        print()
        print("💡 提示:")
        print("  1. 确保 RabbitMQ 容器正在运行: docker ps")
        print("  2. 检查连接配置是否正确")
        sys.exit(1)
    except Exception as e:
        print(f"❌ 发送失败: {e}")
        sys.exit(1)
    
    print()


if __name__ == "__main__":
    # 解析命令行参数
    platform = sys.argv[1] if len(sys.argv) > 1 else "amazon"
    priority = sys.argv[2] if len(sys.argv) > 2 else "normal"
    task_id = sys.argv[3] if len(sys.argv) > 3 else None
    
    # 验证平台
    valid_platforms = ["amazon", "temu", "shein"]
    if platform not in valid_platforms:
        print(f"❌ 错误: 无效的平台 '{platform}'")
        print(f"支持的平台: {', '.join(valid_platforms)}")
        sys.exit(1)
    
    # 发送消息
    send_test_message(platform, priority, task_id)
