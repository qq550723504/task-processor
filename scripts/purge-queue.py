#!/usr/bin/env python3
"""
清空 RabbitMQ 队列的 Python 脚本
需要安装: pip install pika requests
"""

import sys
import argparse

try:
    import pika
except ImportError as e:
    print("❌ 错误: 缺少必要的库")
    print("请运行: pip install pika")
    sys.exit(1)


# RabbitMQ 配置（来自 config/config-prod.yaml）
RABBITMQ_URL = 'amqp://admin:RabbitMQ%402026%23Prod@101.33.34.102:30567/'

# 所有队列列表（来自 config/config-prod.yaml rabbitmq.consumer.queues）
ALL_QUEUES = [
    # 上架任务队列（按店铺）
    "amazon.tasks.store.836",
    "temu.tasks.store.836",
    "shein.tasks.store.836",
    # 爬虫任务队列
    "amazon.crawler",
    "1688.crawler",
    # 系统队列
    "tasks.dlq",
    "tasks.delay.queue",
    "tasks.result.queue",
]


def get_connection():
    """获取 RabbitMQ 连接"""
    try:
        parameters = pika.URLParameters(RABBITMQ_URL)
        return pika.BlockingConnection(parameters)
    except Exception as e:
        print(f"❌ 建立连接失败: {type(e).__name__}: {e}")
        raise


def get_queue_info(queue_name, channel=None):
    """通过 AMQP passive declare 获取队列信息"""
    should_close = False
    connection = None
    try:
        if channel is None:
            connection = get_connection()
            channel = connection.channel()
            should_close = True

        result = channel.queue_declare(queue=queue_name, passive=True)
        return {
            'messages': result.method.message_count,
            'messages_ready': result.method.message_count,
            'messages_unacknowledged': 0,
            'consumers': result.method.consumer_count,
        }
    except Exception as e:
        print(f"  ⚠️  {queue_name}: {type(e).__name__}: {e}")
        return None
    finally:
        if should_close and connection and not connection.is_closed:
            connection.close()


def list_queues():
    """列出所有队列及其消息数量"""
    print("=" * 70)
    print("📊 RabbitMQ 队列状态")
    print("=" * 70)
    print()

    connection = None
    try:
        connection = get_connection()
        channel = connection.channel()
    except Exception as e:
        print(f"❌ 连接失败: {type(e).__name__}: {e}")
        return

    total_messages = 0
    queue_stats = []

    for queue_name in ALL_QUEUES:
        # 每次用新 channel，避免 passive declare 失败后 channel 被关闭
        try:
            ch = connection.channel()
            info = get_queue_info(queue_name, channel=ch)
            ch.close()
        except Exception:
            info = None
        if info is not None:
            total_messages += info['messages']
            queue_stats.append((queue_name, info))

    if connection and not connection.is_closed:
        connection.close()

    if not queue_stats:
        print("❌ 所有队列均不存在或无法访问，请确认队列已创建")
        if connection and not connection.is_closed:
            connection.close()
        return

    # 打印表头
    print(f"{'队列名称':<30} {'消息数':<10} {'消费者':<10}")
    print("-" * 55)

    for queue_name, info in queue_stats:
        msg_count = info['messages']
        flag = " ⚠️" if msg_count > 0 else ""
        print(f"{queue_name:<30} {msg_count:<10} {info['consumers']:<10}{flag}")

    print("-" * 55)
    print(f"{'总计':<30} {total_messages:<10}")
    print()


def purge_queue(queue_name, channel=None):
    """清空指定队列
    
    Args:
        queue_name: 队列名称
        channel: 可选的channel对象，如果提供则复用，否则创建新连接
    """
    should_close = False
    connection = None
    
    try:
        # 如果没有传入channel，创建新连接
        if channel is None:
            connection = get_connection()
            channel = connection.channel()
            should_close = True
        
        # 获取清空前的消息数量
        info = get_queue_info(queue_name)
        before_count = info['messages'] if info else '未知'
        
        # 清空队列
        result = channel.queue_purge(queue_name)
        
        # 确保返回值是整数 - pika返回的是Method对象
        if hasattr(result, 'method') and hasattr(result.method, 'message_count'):
            deleted_count = int(result.method.message_count)
        else:
            deleted_count = int(result) if result else 0
        
        print(f"  ✅ {queue_name:<30} 删除了 {deleted_count} 条消息 (清空前: {before_count})")
        
        return deleted_count
        
    except Exception as e:
        print(f"  ❌ {queue_name:<30} 清空失败: {e}")
        return 0
    finally:
        if should_close and connection and not connection.is_closed:
            connection.close()


def purge_all_queues(confirm=True):
    """清空所有队列 - 优化版：复用连接"""
    print("=" * 70)
    print("🗑️  批量清空 RabbitMQ 队列")
    print("=" * 70)
    print()
    
    # 显示当前状态
    list_queues()
    
    if confirm:
        print("⚠️  警告: 此操作将清空所有队列中的消息，且无法恢复！")
        response = input("确认继续? (yes/no): ").strip().lower()
        if response not in ['yes', 'y']:
            print("❌ 操作已取消")
            return
        print()
    
    print("🔌 连接到 RabbitMQ...")
    connection = None
    try:
        # 创建一个连接，所有队列复用
        connection = get_connection()
        channel = connection.channel()
        print("✅ 连接成功")
        print()
        
        print("🗑️  开始清空队列...")
        print()
        
        total_deleted = 0
        for queue_name in ALL_QUEUES:
            # 传入channel，复用连接
            deleted = purge_queue(queue_name, channel=channel)
            total_deleted += deleted
        
        print()
        print("=" * 70)
        print(f"✅ 完成！共删除 {total_deleted} 条消息")
        print("=" * 70)
        print()
        
    except pika.exceptions.AMQPConnectionError as e:
        print(f"❌ 连接失败: {e}")
        print()
        print("💡 提示:")
        print("  1. 确保 RabbitMQ 容器正在运行: docker ps")
        print("  2. 检查连接配置是否正确")
        sys.exit(1)
    except Exception as e:
        print(f"❌ 操作失败: {e}")
        sys.exit(1)
    finally:
        if connection and not connection.is_closed:
            connection.close()


def purge_single_queue(queue_name):
    """清空单个队列"""
    print("=" * 70)
    print("🗑️  清空 RabbitMQ 队列")
    print("=" * 70)
    print()
    print(f"📋 队列名称: {queue_name}")
    print()
    
    # 连接到 RabbitMQ
    print("🔌 连接到 RabbitMQ...")
    connection = None
    try:
        connection = get_connection()
        channel = connection.channel()
        print("✅ 连接成功")
        print()
        
        # 获取清空前的信息
        info = get_queue_info(queue_name)
        if info:
            print(f"📊 队列状态:")
            print(f"  - 总消息数: {info['messages']}")
            print(f"  - 就绪消息: {info['messages_ready']}")
            print(f"  - 未确认消息: {info['messages_unacknowledged']}")
            print(f"  - 消费者数: {info['consumers']}")
            print()
        
        # 清空队列
        print(f"🗑️  清空队列 {queue_name}...")
        result = channel.queue_purge(queue_name)
        
        # 确保返回值是整数
        if hasattr(result, 'method') and hasattr(result.method, 'message_count'):
            deleted_count = int(result.method.message_count)
        else:
            deleted_count = int(result) if result else 0
        
        print(f"✅ 队列已清空！删除了 {deleted_count} 条消息")
        print()
        
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
    finally:
        if connection and not connection.is_closed:
            connection.close()


def main():
    parser = argparse.ArgumentParser(
        description='RabbitMQ 队列管理工具',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  %(prog)s --list                          # 列出所有队列状态
  %(prog)s --all                           # 清空所有队列（需要确认）
  %(prog)s --all --yes                     # 清空所有队列（跳过确认）
  %(prog)s --queue amazon.tasks.high       # 清空指定队列
  %(prog)s -q temu.tasks.normal            # 清空指定队列（简写）
        """
    )
    
    parser.add_argument(
        '--list', '-l',
        action='store_true',
        help='列出所有队列及其消息数量'
    )
    
    parser.add_argument(
        '--all', '-a',
        action='store_true',
        help='清空所有队列'
    )
    
    parser.add_argument(
        '--queue', '-q',
        type=str,
        help='清空指定队列'
    )
    
    parser.add_argument(
        '--yes', '-y',
        action='store_true',
        help='跳过确认提示（与 --all 一起使用）'
    )
    
    args = parser.parse_args()
    
    # 如果没有参数，显示帮助
    if len(sys.argv) == 1:
        parser.print_help()
        sys.exit(0)
    
    # 列出队列
    if args.list:
        list_queues()
    
    # 清空所有队列
    elif args.all:
        purge_all_queues(confirm=not args.yes)
    
    # 清空指定队列
    elif args.queue:
        purge_single_queue(args.queue)
    
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
