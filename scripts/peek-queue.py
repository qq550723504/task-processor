#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
查看 RabbitMQ 队列中的消息内容（不消费，仅预览）
需要安装: pip install pika
"""

import sys
import json
import argparse

try:
    import pika
except ImportError:
    print("❌ 错误: 缺少必要的库")
    print("请运行: pip install pika")
    sys.exit(1)

# RabbitMQ 连接配置（与 purge-queue.py 保持一致，生产环境地址）
RABBITMQ_URL = 'amqp://admin:RabbitMQ%402026%23Prod@101.33.34.102:30567/'


def peek_queue(queue_name, count=1):
    """预览队列中的消息（使用 basic_get，ack=False 不消费）"""
    connection = None
    messages = []
    try:
        params = pika.URLParameters(RABBITMQ_URL)
        connection = pika.BlockingConnection(params)
        channel = connection.channel()

        print(f"📋 队列: {queue_name}，预览前 {count} 条消息\n")
        print("=" * 80)

        for i in range(count):
            method, props, body = channel.basic_get(queue=queue_name, auto_ack=False)
            if method is None:
                print(f"  队列为空或已无更多消息（已获取 {i} 条）")
                break

            # 解析消息体
            try:
                payload = json.loads(body.decode('utf-8'))
                body_str = json.dumps(payload, ensure_ascii=False, indent=2)
            except Exception:
                body_str = body.decode('utf-8', errors='replace')

            print(f"[消息 {i+1}]")
            print(f"  MessageID : {method.delivery_tag}")
            print(f"  RoutingKey: {method.routing_key}")
            if props.message_id:
                print(f"  MsgID     : {props.message_id}")
            if props.type:
                print(f"  Type      : {props.type}")
            if props.headers:
                print(f"  Headers   : {props.headers}")
            print(f"  Body      :\n{body_str}")
            print("-" * 80)

            messages.append((method, body_str))

            # 重新入队（reject + requeue=True），不消费
            channel.basic_reject(delivery_tag=method.delivery_tag, requeue=True)

    except pika.exceptions.AMQPConnectionError as e:
        print(f"❌ 连接失败: {e}")
        sys.exit(1)
    except Exception as e:
        print(f"❌ 操作失败: {e}")
        sys.exit(1)
    finally:
        if connection and not connection.is_closed:
            connection.close()

    return messages


def main():
    parser = argparse.ArgumentParser(
        description='预览 RabbitMQ 队列消息（不消费）',
        epilog="""
示例:
  %(prog)s shein.tasks.normal          # 预览 1 条
  %(prog)s shein.tasks.normal -n 3     # 预览 3 条
  %(prog)s amazon.tasks.high -n 5      # 预览 5 条
        """
    )
    parser.add_argument('queue', help='队列名称')
    parser.add_argument('-n', '--count', type=int, default=1, help='预览条数（默认 1）')

    args = parser.parse_args()
    peek_queue(args.queue, args.count)


if __name__ == '__main__':
    main()
