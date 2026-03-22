// Package distributed 提供分布式爬虫结果监听器
package distributed

import (
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

// ResultListener 监听爬虫结果队列，将结果投递给 PendingRegistry
type ResultListener struct {
	declarer  QueueDeclarer
	registry  *PendingRegistry
	logger    *logrus.Logger
	queueName string
}

// NewResultListener 创建结果监听器
func NewResultListener(declarer QueueDeclarer, registry *PendingRegistry, logger *logrus.Logger) *ResultListener {
	return &ResultListener{
		declarer: declarer,
		registry: registry,
		logger:   logger,
	}
}

// Start 声明结果队列并启动消费 goroutine。
// 幂等：若已启动则直接返回队列名。
func (l *ResultListener) Start() (string, error) {
	if l.queueName != "" {
		return l.queueName, nil
	}

	// 生成唯一队列名
	name := fmt.Sprintf("crawler.results.node-%d", time.Now().UnixNano())

	l.logger.Infof("声明结果队列: %s", name)
	actualName, err := l.declarer.DeclareExclusiveQueue(name)
	if err != nil {
		return "", fmt.Errorf("声明结果队列失败: %w", err)
	}
	l.queueName = actualName
	l.logger.Infof("结果队列声明成功: %s", l.queueName)

	msgs, err := l.declarer.ConsumeQueue(l.queueName, "crawler-result-consumer")
	if err != nil {
		l.queueName = "" // 重置，允许重试
		return "", fmt.Errorf("启动结果队列消费者失败: %w", err)
	}

	l.logger.Infof("结果队列消费者启动成功: %s", l.queueName)
	go l.consume(msgs)
	return l.queueName, nil
}

// Restart 重连后重新启动（重置队列名，重新声明）
func (l *ResultListener) Restart() error {
	l.queueName = ""
	_, err := l.Start()
	return err
}

// QueueName 返回当前结果队列名（未启动时返回空字符串）
func (l *ResultListener) QueueName() string {
	return l.queueName
}

// consume 消费结果消息（在 goroutine 中运行）
func (l *ResultListener) consume(msgs <-chan amqp.Delivery) {
	l.logger.Infof("开始消费结果队列: %s", l.queueName)
	for msg := range msgs {
		l.handleMessage(msg)
	}
	l.logger.Warnf("结果队列消费者已停止: %s", l.queueName)
}

// handleMessage 处理单条结果消息（可独立测试）
func (l *ResultListener) handleMessage(msg amqp.Delivery) {
	var result CrawlResult
	if err := json.Unmarshal(msg.Body, &result); err != nil {
		l.logger.Errorf("解析爬虫结果消息失败: %v, body=%s", err, string(msg.Body))
		_ = msg.Nack(false, false)
		return
	}

	l.logger.Debugf("收到爬虫结果: TaskID=%s, Success=%v", result.TaskID, result.Success)

	if !l.registry.Deliver(&result) {
		l.logger.Warnf("未找到等待的任务（可能已超时）: TaskID=%s", result.TaskID)
	}

	_ = msg.Ack(false)
}
