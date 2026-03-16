package main

import (
	"fmt"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	// RabbitMQ 连接 URL（密码中的特殊字符需要 URL 编码）
	// @ -> %40, # -> %23
	rabbitURL := "amqp://admin:RabbitMQ%402026%23Prod@101.33.34.102:30567/"

	fmt.Println("正在连接 RabbitMQ...")
	fmt.Printf("连接地址: %s\n\n", rabbitURL)

	// 连接到 RabbitMQ
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		log.Fatalf("连接 RabbitMQ 失败: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	fmt.Println("✓ 连接成功！")

	// 创建通道
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("创建通道失败: %v", err)
	}
	defer func() {
		_ = ch.Close()
	}()

	fmt.Println("✓ 通道创建成功！")
	fmt.Println()

	// 定义要检查的队列列表（从配置文件中获取）
	queueNames := []string{
		"amazon.tasks.high",
		"temu.tasks.high",
		"shein.tasks.high",
		"amazon.tasks.normal",
		"temu.tasks.normal",
		"shein.tasks.normal",
		"amazon.tasks.low",
		"temu.tasks.low",
		"shein.tasks.low",
		"amazon.crawler.high",
		"1688.crawler.high",
		"amazon.crawler.normal",
		"1688.crawler.normal",
		"amazon.crawler.low",
		"1688.crawler.low",
	}

	fmt.Println("=== RabbitMQ 队列状态 ===")
	fmt.Println()

	totalMessages := 0
	existingQueues := 0

	for _, queueName := range queueNames {
		// 使用 QueueInspect 检查队列（不会创建队列）
		queue, err := ch.QueueInspect(queueName)
		if err != nil {
			fmt.Printf("队列: %s - 不存在或无法访问\n", queueName)
			continue
		}

		existingQueues++
		fmt.Printf("队列: %s\n", queueName)
		fmt.Printf("  消息数: %d\n", queue.Messages)
		fmt.Printf("  消费者数: %d\n", queue.Consumers)
		fmt.Println()

		totalMessages += queue.Messages
	}

	fmt.Println("=== 统计信息 ===")
	fmt.Printf("存在的队列数: %d / %d\n", existingQueues, len(queueNames))
	fmt.Printf("总消息数: %d\n", totalMessages)

	if totalMessages > 0 {
		fmt.Println("\n✓ RabbitMQ 中有数据！")
	} else {
		fmt.Println("\n✗ RabbitMQ 中暂无待处理消息")
	}
}
