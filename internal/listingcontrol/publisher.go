package listingcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	taskapp "task-processor/internal/app/task"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/model"

	amqp "github.com/rabbitmq/amqp091-go"
)

const defaultDispatchPlatform = "shein"

type AMQPPublisher interface {
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
}

type DispatchPublisher struct {
	channel  AMQPPublisher
	platform string
	adapter  *taskapp.MessageAdapter
}

type PublishedDispatch struct {
	Queue     string
	MessageID string
	Priority  uint8
}

func NewDispatchPublisher(channel AMQPPublisher, platform string) *DispatchPublisher {
	return &DispatchPublisher{
		channel:  channel,
		platform: normalizeDispatchPlatform(platform),
		adapter:  taskapp.NewMessageAdapter(),
	}
}

func (p *DispatchPublisher) PublishTask(ctx context.Context, task *model.Task) (PublishedDispatch, error) {
	if p == nil {
		return PublishedDispatch{}, errors.New("dispatch publisher is nil")
	}
	if p.channel == nil {
		return PublishedDispatch{}, errors.New("dispatch publisher channel is nil")
	}
	if task == nil {
		return PublishedDispatch{}, errors.New("dispatch task is nil")
	}

	adapter := p.adapter
	if adapter == nil {
		adapter = taskapp.NewMessageAdapter()
	}

	taskMsg, err := adapter.TaskToMessage(task)
	if err != nil {
		return PublishedDispatch{}, fmt.Errorf("convert task to dispatch message: %w", err)
	}
	body, err := json.Marshal(taskMsg)
	if err != nil {
		return PublishedDispatch{}, fmt.Errorf("marshal dispatch message: %w", err)
	}

	queue := rabbitmq.GetStoreQueueName(p.platform, task.StoreID)
	messageID := strconv.FormatInt(task.ID, 10)
	priority := adapter.CalculatePriority(task.Priority)
	publishing := amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: 2,
		MessageId:    messageID,
		Type:         "task",
		Priority:     priority,
		Body:         body,
	}

	if err := p.channel.PublishWithContext(ctx, "", queue, false, false, publishing); err != nil {
		return PublishedDispatch{}, fmt.Errorf("publish dispatch task %s to %s: %w", messageID, queue, err)
	}

	return PublishedDispatch{
		Queue:     queue,
		MessageID: messageID,
		Priority:  priority,
	}, nil
}

func normalizeDispatchPlatform(platform string) string {
	platform = strings.ToLower(strings.TrimSpace(platform))
	if platform == "" {
		return defaultDispatchPlatform
	}
	return platform
}
