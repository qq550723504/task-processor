// Package distributed 提供分布式爬虫结果监听器
package distributed

import (
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// amqpChannel 定义消费所需的最小 channel 接口
type amqpChannel interface {
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	Close() error
}

// startResultListener 启动结果监听器
func (c *DistributedCrawlerClient) startResultListener() error {
	// 为每个客户端创建唯一的结果队列（仅在首次启动时生成队列名）
	if c.resultQueueName == "" {
		nodeID := fmt.Sprintf("node-%d", time.Now().UnixNano())
		c.resultQueueName = fmt.Sprintf("crawler.results.%s", nodeID)
	}

	// 用独立 channel 声明队列，避免与共享 channel 竞争
	ch, err := c.rabbitmqClient.GetConnectionManager().CreateChannel()
	if err != nil {
		return fmt.Errorf("创建独立通道失败: %w", err)
	}

	_, err = ch.QueueDeclare(c.resultQueueName, false, true, true, false, nil)
	if err != nil {
		ch.Close()
		return fmt.Errorf("声明结果队列失败: %w", err)
	}

	c.logger.Infof("队列 %s 声明成功", c.resultQueueName)

	// 开始消费结果消息（传入独立 channel）
	go c.consumeResults(ch, c.resultQueueName)

	c.logger.Infof("爬虫结果监听器已启动，队列: %s", c.resultQueueName)
	return nil
}

// consumeResults 消费结果消息
func (c *DistributedCrawlerClient) consumeResults(ch amqpChannel, queueName string) {
	defer func() {
		ch.Close()
		if r := recover(); r != nil {
			c.logger.Errorf("消费结果消息goroutine发生panic: %v", r)
		}
	}()

	c.logger.Infof("开始消费队列: %s, 消费者: %s", queueName, "crawler-result-consumer")

	msgs, err := ch.Consume(queueName, "crawler-result-consumer", false, false, false, false, nil)
	if err != nil {
		c.logger.Errorf("开始消费结果消息失败: %v", err)
		return
	}

	c.logger.Infof("✅ 结果队列消费者启动成功: %s", queueName)

	for msg := range msgs {
		c.handleResultMessage(msg)
	}

	c.logger.Warnf("结果队列消费者已停止: %s", queueName)
}

// handleResultMessage 处理结果消息
func (c *DistributedCrawlerClient) handleResultMessage(msg amqp.Delivery) {
	defer func() {
		if r := recover(); r != nil {
			c.logger.Errorf("处理结果消息时发生panic: %v", r)
		}
	}()

	// 解析结果消息
	var result CrawlResult
	if err := json.Unmarshal(msg.Body, &result); err != nil {
		c.logger.Errorf("解析结果消息失败: %v", err)
		c.rabbitmqClient.NackMessage(msg, false) // 拒绝消息，不重新入队
		return
	}

	taskID := fmt.Sprintf("%d", result.TaskID)
	c.logger.Debugf("收到爬虫结果消息: TaskID=%s", taskID)

	// 查找等待的任务
	c.mutex.RLock()
	pendingTask, exists := c.pendingTasks[taskID]
	c.mutex.RUnlock()

	if !exists {
		c.logger.Warnf("未找到等待的任务: TaskID=%s", taskID)
		c.rabbitmqClient.AckMessage(msg) // 确认消息
		return
	}

	// 发送结果到等待的任务
	select {
	case pendingTask.ResultChan <- &result:
		c.logger.Debugf("结果已发送到等待任务: TaskID=%s", taskID)
	case <-pendingTask.Context.Done():
		c.logger.Warnf("等待任务已超时: TaskID=%s", taskID)
	default:
		c.logger.Warnf("结果通道已满: TaskID=%s", taskID)
	}

	// 清理等待任务
	c.mutex.Lock()
	delete(c.pendingTasks, taskID)
	c.mutex.Unlock()
	pendingTask.Cancel()

	// 确认消息
	c.rabbitmqClient.AckMessage(msg)
}
