package taskstatus

import (
	"fmt"
	"time"

	"task-processor/internal/core/logger"
	managementapi "task-processor/internal/infra/clients/management/api"
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
	ExpectedCurrentStatus *model.TaskStatus
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
			s.log.WithError(err).WithFields(logrus.Fields{
				logger.FieldTaskID: input.TaskID,
				logger.FieldStatus: input.Status.String(),
			}).Error("failed to update task status asynchronously")
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
		RetryCount:   input.RetryCount,
		Priority:     input.Priority,
	}
	if input.ExpectedCurrentStatus != nil {
		expected := input.ExpectedCurrentStatus.Int16()
		req.ExpectedCurrentStatus = &expected
	}

	var lastErr error
	for i := 0; i < s.maxRetries; i++ {
		if err := client.UpdateTaskStatus(req); err != nil {
			lastErr = err
			if i < s.maxRetries-1 {
				retryDelay := time.Second * time.Duration(i+1)
				s.log.WithError(err).WithFields(logrus.Fields{
					logger.FieldTaskID:     input.TaskID,
					logger.FieldRetryCount: i + 1,
					logger.FieldStatus:     input.Status.String(),
					"retry_delay":          retryDelay.String(),
				}).Warn("retrying task status update")
				time.Sleep(retryDelay)
				continue
			}
			break
		}

		s.log.WithFields(logrus.Fields{
			logger.FieldTaskID: input.TaskID,
			logger.FieldStatus: input.Status.String(),
		}).Info("task status updated")
		return nil
	}

	return fmt.Errorf("update task status failed: %w", lastErr)
}
