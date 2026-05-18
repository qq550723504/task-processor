package taskstatus

import (
	"context"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/core/logger"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/infra/resilience"
	"task-processor/internal/model"
	"task-processor/internal/pkg/recovery"

	"github.com/sirupsen/logrus"
)

type ImportTaskStatusClient interface {
	UpdateTaskStatus(req *managementapi.ProductImportTaskUpdateReqDTO) error
}

type UpdateInput struct {
	TaskID                int64
	Status                model.TaskStatus
	ErrorMessage          string
	ReasonCode            string
	Stage                 string
	ExpectedCurrentStatus *model.TaskStatus
	IgnoreConflict        bool
	RetryCount            *int
	Priority              *int
}

type Service struct {
	component      string
	log            *logrus.Entry
	clientProvider func() ImportTaskStatusClient
	maxRetries     int
}

func NewService(component string, clientProvider func() ImportTaskStatusClient) *Service {
	log := logger.GetGlobalLogger("app/taskstatus").WithField("component", component)

	return &Service{
		component:      component,
		log:            log,
		clientProvider: clientProvider,
		maxRetries:     3,
	}
}

func (s *Service) UpdateSync(taskID int64, status model.TaskStatus, errorMsg string) error {
	return s.UpdateSyncWithInput(UpdateInput{
		TaskID:       taskID,
		Status:       status,
		ErrorMessage: errorMsg,
	})
}

func (s *Service) UpdateAsync(taskID int64, status model.TaskStatus, errorMsg string) {
	s.UpdateAsyncWithInput(UpdateInput{
		TaskID:       taskID,
		Status:       status,
		ErrorMessage: errorMsg,
	})
}

func (s *Service) UpdateSyncWithInput(input UpdateInput) error {
	return s.updateSync(input)
}

func (s *Service) UpdateAsyncWithInput(input UpdateInput) {
	go func() {
		defer recovery.RecoverWithStack(
			fmt.Sprintf("%s async update task status", s.component),
			s.log.WithFields(logrus.Fields{
				logger.FieldTaskID: input.TaskID,
				logger.FieldStatus: input.Status.String(),
			}),
		)

		if err := s.updateSync(input); err != nil {
			logEntry := s.log.WithError(err).WithFields(logrus.Fields{
				logger.FieldTaskID: input.TaskID,
				logger.FieldStatus: input.Status.String(),
			})
			if isNonRetriableUpdateErr(err) {
				logEntry.Warn("task status async update rejected without retry")
				return
			}
			logEntry.Error("failed to update task status asynchronously")
		}
	}()
}

func (s *Service) TransitionSync(taskID int64, from, to model.TaskStatus, errorMsg string) error {
	return s.TransitionSyncWithInput(from, UpdateInput{
		TaskID:       taskID,
		Status:       to,
		ErrorMessage: errorMsg,
	})
}

func (s *Service) TransitionAsync(taskID int64, from, to model.TaskStatus, errorMsg string) error {
	return s.TransitionAsyncWithInput(from, UpdateInput{
		TaskID:       taskID,
		Status:       to,
		ErrorMessage: errorMsg,
	})
}

func (s *Service) TransitionSyncWithInput(from model.TaskStatus, input UpdateInput) error {
	if err := model.ValidateTaskStatusTransition(from, input.Status); err != nil {
		return err
	}
	input.ExpectedCurrentStatus = &from
	return s.updateSync(input)
}

func (s *Service) TransitionAsyncWithInput(from model.TaskStatus, input UpdateInput) error {
	if err := model.ValidateTaskStatusTransition(from, input.Status); err != nil {
		return err
	}
	input.ExpectedCurrentStatus = &from
	s.UpdateAsyncWithInput(input)
	return nil
}

func (s *Service) TransitionFromCodeSync(taskID int64, fromCode int16, to model.TaskStatus, errorMsg string) error {
	return s.TransitionFromCodeSyncWithInput(fromCode, UpdateInput{
		TaskID:       taskID,
		Status:       to,
		ErrorMessage: errorMsg,
	})
}

func (s *Service) TransitionFromCodeSyncWithInput(fromCode int16, input UpdateInput) error {
	if err := model.ValidateTaskStatusTransitionCode(fromCode, input.Status); err != nil {
		return err
	}
	from, _ := model.ParseTaskStatus(fromCode)
	input.ExpectedCurrentStatus = &from
	return s.updateSync(input)
}

func (s *Service) updateSync(input UpdateInput) error {
	if !input.Status.IsValid() {
		return fmt.Errorf("invalid task status: %d", input.Status)
	}
	if s.clientProvider == nil {
		return fmt.Errorf("import task client provider is nil")
	}

	client := s.clientProvider()
	if client == nil {
		return fmt.Errorf("import task client is not initialized")
	}

	req := &managementapi.ProductImportTaskUpdateReqDTO{
		ID:           input.TaskID,
		Status:       input.Status.Int16(),
		ErrorMessage: input.ErrorMessage,
		ReasonCode:   firstNonEmpty(strings.TrimSpace(input.ReasonCode), extractReasonCode(input.ErrorMessage)),
		Stage:        firstNonEmpty(strings.TrimSpace(input.Stage), extractStage(input.ErrorMessage)),
		RetryCount:   input.RetryCount,
		Priority:     input.Priority,
	}
	if input.ExpectedCurrentStatus != nil {
		expected := input.ExpectedCurrentStatus.Int16()
		req.ExpectedCurrentStatus = &expected
	}

	err := resilience.Retry(context.Background(), resilience.RetryConfig{
		MaxAttempts:         s.maxRetries,
		InitialDelay:        time.Second,
		MaxDelay:            time.Duration(s.maxRetries-1) * time.Second,
		Multiplier:          2,
		RandomizationFactor: 0,
		IsRetryable: func(err error) bool {
			return !isNonRetriableUpdateErr(err)
		},
		OnRetry: func(_ context.Context, attempt resilience.RetryAttempt) {
			s.log.WithError(attempt.Err).WithFields(logrus.Fields{
				logger.FieldTaskID:     input.TaskID,
				logger.FieldRetryCount: attempt.Attempt,
				logger.FieldStatus:     input.Status.String(),
				"expected_status":      formatExpectedStatus(input.ExpectedCurrentStatus),
				"retry_delay":          attempt.NextDelay.String(),
			}).Warn("retrying task status update")
		},
	}, func(context.Context) error {
		return client.UpdateTaskStatus(req)
	})
	if err == nil {
		s.log.WithFields(logrus.Fields{
			logger.FieldTaskID: input.TaskID,
			logger.FieldStatus: input.Status.String(),
			"expected_status":  formatExpectedStatus(input.ExpectedCurrentStatus),
		}).Info("task status updated")
		return nil
	}

	if isNonRetriableUpdateErr(err) {
		if input.IgnoreConflict {
			s.log.WithError(err).WithFields(logrus.Fields{
				logger.FieldTaskID: input.TaskID,
				logger.FieldStatus: input.Status.String(),
				"expected_status":  formatExpectedStatus(input.ExpectedCurrentStatus),
			}).Debug("task status update conflict ignored")
			return nil
		}
		s.log.WithError(err).WithFields(logrus.Fields{
			logger.FieldTaskID: input.TaskID,
			logger.FieldStatus: input.Status.String(),
			"expected_status":  formatExpectedStatus(input.ExpectedCurrentStatus),
		}).Warn("task status update rejected without retry")
	}

	return fmt.Errorf("update task status failed: %w", err)
}

func isNonRetriableUpdateErr(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "管理端拒绝更新任务状态") ||
		strings.Contains(message, "expectedCurrentStatus") ||
		strings.Contains(message, "Management API error 409")
}

func formatExpectedStatus(status *model.TaskStatus) any {
	if status == nil {
		return nil
	}
	return status.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func extractReasonCode(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}

	for _, token := range strings.Fields(message) {
		if strings.HasPrefix(token, "[") && strings.HasSuffix(token, "]") {
			content := strings.TrimSuffix(strings.TrimPrefix(token, "["), "]")
			if content != "" && !strings.HasPrefix(content, "stage:") {
				return content
			}
		}
	}
	return ""
}

func extractStage(message string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}

	for _, token := range strings.Fields(message) {
		if strings.HasPrefix(token, "[stage:") && strings.HasSuffix(token, "]") {
			return strings.TrimSuffix(strings.TrimPrefix(token, "[stage:"), "]")
		}
	}
	return ""
}
