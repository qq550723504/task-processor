#!/usr/bin/env python3
"""
清空 RabbitMQ 队列的 Python 脚本
需要安装: pip install pika
"""

import sys

try:
    import pika
except ImportError:
    print("❌ 错误: 未安装 pika 库")
    print("请运行: pip install pika")
    sys.exit(1)


def purge_queue(queue_name="amazon.tasks.queue"):
    """清空指定队列"""
    
    print("=" * 50)
    print("🗑️  清空 RabbitMQ 队列")
    print("=" * 50)
    print()
    print(f"📋 队列名称: {queue_name}")
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
        
        # 清空队列
        print(f"🗑️  清空队列 {queue_name}...")
        result = channel.queue_purge(queue_name)
        
        print(f"✅ 队列已清空！删除了 {result} 条消息")
        print()
        
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
        print(f"❌ 清空失败: {e}")
        sys.exit(1)
    
    print()


if __name__ == "__main__":
    # 解析命令行参数
    queue_name = sys.argv[1] if len(sys.argv) > 1 else "amazon.tasks.queue"
    
    # 清空队列
    purge_queue(queue_name)
