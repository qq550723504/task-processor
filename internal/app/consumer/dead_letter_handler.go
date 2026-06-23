package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/rabbitmq"
	"task-processor/internal/model"
	"task-processor/internal/taskstatus"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sirupsen/logrus"
)

const (
	deadLetterReasonCode = "DEAD_LETTER"
	deadLetterStage      = "dead_letter"
	defaultDLQQueueName  = "tasks.dlq"
)

type DeadLetterHandlerConfig struct {
	Runtime taskstatus.RuntimeWithTaskRPC
	Logger  *logrus.Logger
}

type DeadLetterHandler struct {
	runtime taskstatus.RuntimeWithTaskRPC
	logger  *logrus.Logger
}

func NewDeadLetterHandler(cfg DeadLetterHandlerConfig) *DeadLetterHandler {
	return &DeadLetterHandler{
		runtime: cfg.Runtime,
		logger:  cfg.Logger,
	}
}

func (h *DeadLetterHandler) HandleMessage(ctx context.Context, msg *rabbitmq.Message) error {
	_ = ctx
	if h == nil || h.runtime == nil {
		return fmt.Errorf("dead letter runtime is not initialized")
	}
	if msg == nil {
		return fmt.Errorf("dead letter message is nil")
	}

	taskID, err := extractDeadLetterTaskID(msg.Payload)
	if err != nil {
		return err
	}

	status, err := h.runtime.GetTaskStatus(taskID)
	if err != nil {
		return fmt.Errorf("get task status for dead letter task %d: %w", taskID, err)
	}
	if isTerminalTaskStatus(status) {
		if h.logger != nil {
			h.logger.WithFields(logrus.Fields{
				"task_id":          taskID,
				"status_key":       status.StatusKey,
				"canonical_status": status.CanonicalStatus,
				"dead_reason":      firstNonEmptyHeader(msg.Headers, "x-first-death-reason"),
				"dead_queue":       firstNonEmptyHeader(msg.Headers, "x-first-death-queue"),
			}).Info("dead letter message already has terminal task state; skipping status overwrite")
		}
		return nil
	}

	retryCount := extractOptionalInt(msg.Payload, "retryCount")
	statusService := taskstatus.NewService("app/consumer/dead_letter", func() taskstatus.ImportTaskStatusClient {
		return taskstatus.NewManagementClientAdapter(h.runtime)
	})
	return statusService.UpdateSyncWithInput(taskstatus.UpdateInput{
		TaskID:       taskID,
		Status:       model.TaskStatusPendingRetry,
		ErrorMessage: formatDeadLetterMessage(msg),
		ReasonCode:   deadLetterReasonCode,
		Stage:        deadLetterStage,
		RetryCount:   retryCount,
	})
}

func extractDeadLetterTaskID(payload map[string]any) (int64, error) {
	if payload == nil {
		return 0, fmt.Errorf("dead letter payload is empty")
	}
	value, ok := payload["taskId"]
	if !ok {
		value, ok = payload["id"]
	}
	if !ok {
		return 0, fmt.Errorf("dead letter payload missing taskId")
	}
	taskID, ok := anyToInt64(value)
	if !ok || taskID <= 0 {
		return 0, fmt.Errorf("dead letter payload has invalid taskId: %v", value)
	}
	return taskID, nil
}

func extractOptionalInt(payload map[string]any, key string) *int {
	if payload == nil {
		return nil
	}
	value, ok := payload[key]
	if !ok {
		return nil
	}
	parsed, ok := anyToInt64(value)
	if !ok {
		return nil
	}
	result := int(parsed)
	return &result
}

func anyToInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case json.Number:
		parsed, err := strconv.ParseInt(v.String(), 10, 64)
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func formatDeadLetterMessage(msg *rabbitmq.Message) string {
	reason := firstNonEmptyHeader(msg.Headers, "x-first-death-reason")
	queue := firstNonEmptyHeader(msg.Headers, "x-first-death-queue")
	exchange := firstNonEmptyHeader(msg.Headers, "x-first-death-exchange")
	if reason == "" || queue == "" {
		death := firstDeathHeader(msg.Headers)
		if reason == "" {
			reason = stringFromAny(death["reason"])
		}
		if queue == "" {
			queue = stringFromAny(death["queue"])
		}
		if exchange == "" {
			exchange = stringFromAny(death["exchange"])
		}
	}
	reason = firstNonEmpty(reason, "unknown")
	queue = firstNonEmpty(queue, "unknown queue")
	if exchange != "" {
		return fmt.Sprintf("Dead letter: RabbitMQ %s from %s via %s", reason, queue, exchange)
	}
	return fmt.Sprintf("Dead letter: RabbitMQ %s from %s", reason, queue)
}

func firstDeathHeader(headers map[string]any) map[string]any {
	if headers == nil {
		return nil
	}
	raw, ok := headers["x-death"]
	if !ok {
		return nil
	}
	switch deaths := raw.(type) {
	case []any:
		if len(deaths) == 0 {
			return nil
		}
		return mapFromAny(deaths[0])
	case []amqp.Table:
		if len(deaths) == 0 {
			return nil
		}
		return mapFromAny(deaths[0])
	default:
		return nil
	}
}

func mapFromAny(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return typed
	case amqp.Table:
		result := make(map[string]any, len(typed))
		for key, val := range typed {
			result[key] = val
		}
		return result
	default:
		return nil
	}
}

func firstNonEmptyHeader(headers map[string]any, key string) string {
	if headers == nil {
		return ""
	}
	return strings.TrimSpace(stringFromAny(headers[key]))
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

func isTerminalTaskStatus(status *managementapi.TaskStatusRespDTO) bool {
	if status == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(status.CanonicalStatus)) {
	case "completed", "failed", "cancelled":
		return true
	}
	switch strings.ToUpper(strings.TrimSpace(status.StatusKey)) {
	case "CRAWL_FAILED", "PUBLISHED", "DRAFT", "CANCELLED", "TERMINATED":
		return true
	default:
		return false
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
