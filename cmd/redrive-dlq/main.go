// redrive-dlq 临时工具：将死信队列(tasks.dlq)中的消息重新发布到原始队列
// 通过解析消息头中的 x-death 信息获取原始交换机和路由键
package main

import (
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	rabbitURL = "amqp://admin:RabbitMQ%402026%23Prod@101.33.34.102:30567/"
	dlqName   = "tasks.dlq"
)

func main() {
	conn, err := amqp.Dial(rabbitURL)
	if err != nil {
		log.Fatalf("连接 RabbitMQ 失败: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("创建通道失败: %v", err)
	}
	defer ch.Close()

	// 检查死信队列消息数
	q, err := ch.QueueInspect(dlqName)
	if err != nil {
		log.Fatalf("检查死信队列失败: %v", err)
	}
	fmt.Printf("死信队列消息数: %d\n", q.Messages)
	if q.Messages == 0 {
		fmt.Println("死信队列为空，无需处理")
		return
	}

	success, failed := 0, 0

	for i := 0; i < q.Messages; i++ {
		msg, ok, err := ch.Get(dlqName, false)
		if err != nil {
			log.Printf("获取消息失败: %v", err)
			break
		}
		if !ok {
			break
		}

		// 打印完整消息头，用于调试
		fmt.Printf("[%d] 消息头: %#v\n", i+1, msg.Headers)
		fmt.Printf("[%d] Exchange=%q RoutingKey=%q\n", i+1, msg.Exchange, msg.RoutingKey)

		exchange, routingKey := getOriginalRoute(msg)
		if exchange == "" {
			// 无法确定原始路由，跳过并 nack 保留在死信队列
			log.Printf("[%d] 无法获取原始路由信息，跳过: headers=%v", i+1, msg.Headers)
			_ = msg.Nack(false, true)
			failed++
			continue
		}

		// 重新发布，清除 x-death 头避免循环
		newHeaders := cleanDeathHeaders(msg.Headers)
		err = ch.Publish(
			exchange,
			routingKey,
			false,
			false,
			amqp.Publishing{
				ContentType:  msg.ContentType,
				Body:         msg.Body,
				DeliveryMode: amqp.Persistent,
				Priority:     msg.Priority,
				MessageId:    msg.MessageId,
				Type:         msg.Type,
				Headers:      newHeaders,
				Timestamp:    time.Now(),
			},
		)
		if err != nil {
			log.Printf("[%d] 重新发布失败: exchange=%s, routingKey=%s, err=%v", i+1, exchange, routingKey, err)
			_ = msg.Nack(false, true)
			failed++
			continue
		}

		_ = msg.Ack(false)
		success++
		fmt.Printf("[%d] 重新发布成功: exchange=%s, routingKey=%s\n", i+1, exchange, routingKey)
	}

	fmt.Printf("\n完成: 成功=%d, 失败=%d\n", success, failed)
}

// getOriginalRoute 从 x-death 头中提取原始交换机和路由键
func getOriginalRoute(msg amqp.Delivery) (exchange, routingKey string) {
	if msg.Headers == nil {
		return "", ""
	}

	xDeath, ok := msg.Headers["x-death"]
	if !ok {
		return "", ""
	}

	deaths, ok := xDeath.([]interface{})
	if !ok || len(deaths) == 0 {
		return "", ""
	}

	// 取第一条死信记录（最原始的来源）
	death, ok := deaths[0].(amqp.Table)
	if !ok {
		return "", ""
	}

	if ex, ok := death["exchange"].(string); ok {
		exchange = ex
	}
	if keys, ok := death["routing-keys"].([]interface{}); ok && len(keys) > 0 {
		if rk, ok := keys[0].(string); ok {
			routingKey = rk
		}
	}

	return exchange, routingKey
}

// cleanDeathHeaders 清除 x-death 相关头信息，避免消息再次被识别为死信
func cleanDeathHeaders(headers amqp.Table) amqp.Table {
	if headers == nil {
		return amqp.Table{}
	}
	newHeaders := make(amqp.Table, len(headers))
	for k, v := range headers {
		newHeaders[k] = v
	}
	delete(newHeaders, "x-death")
	delete(newHeaders, "x-first-death-exchange")
	delete(newHeaders, "x-first-death-queue")
	delete(newHeaders, "x-first-death-reason")
	return newHeaders
}
