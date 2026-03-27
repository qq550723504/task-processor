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
	return s.updateSync(taskID, status, errorMsg)
}

func (s *Service) UpdateAsync(taskID int64, status model.TaskStatus, errorMsg string) {
	go func() {
		defer recovery.RecoverWithStack(
			fmt.Sprintf("%s async update task status", s.component),
			s.log.WithFields(logrus.Fields{
				logger.FieldTaskID: taskID,
				logger.FieldStatus: status.String(),
			}),
		)

		if err := s.updateSync(taskID, status, errorMsg); err != nil {
			s.log.WithError(err).WithFields(logrus.Fields{
				logger.FieldTaskID: taskID,
				logger.FieldStatus: status.String(),
			}).Error("failed to update task status asynchronously")
		}
	}()
}

func (s *Service) TransitionSync(taskID int64, from, to model.TaskStatus, errorMsg string) error {
	if err := model.ValidateTaskStatusTransition(from, to); err != nil {
		return err
	}
	return s.updateSync(taskID, to, errorMsg)
}

func (s *Service) TransitionAsync(taskID int64, from, to model.TaskStatus, errorMsg string) error {
	if err := model.ValidateTaskStatusTransition(from, to); err != nil {
		return err
	}
	s.UpdateAsync(taskID, to, errorMsg)
	return nil
}

func (s *Service) TransitionFromCodeSync(taskID int64, fromCode int16, to model.TaskStatus, errorMsg string) error {
	if err := model.ValidateTaskStatusTransitionCode(fromCode, to); err != nil {
		return err
	}
	return s.updateSync(taskID, to, errorMsg)
}

func (s *Service) updateSync(taskID int64, status model.TaskStatus, errorMsg string) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid task status: %d", status)
	}
	if s.clientProvider == nil {
		return fmt.Errorf("import task client provider is nil")
	}

	client := s.clientProvider()
	if client == nil {
		return fmt.Errorf("import task client is not initialized")
	}

	req := &managementapi.ProductImportTaskUpdateReqDTO{
		ID:           taskID,
		Status:       status.Int16(),
		ErrorMessage: errorMsg,
	}

	var lastErr error
	for i := 0; i < s.maxRetries; i++ {
		if err := client.UpdateTaskStatus(req); err != nil {
			lastErr = err
			if i < s.maxRetries-1 {
				retryDelay := time.Second * time.Duration(i+1)
				s.log.WithError(err).WithFields(logrus.Fields{
					logger.FieldTaskID:     taskID,
					logger.FieldRetryCount: i + 1,
					logger.FieldStatus:     status.String(),
					"retry_delay":          retryDelay.String(),
				}).Warn("retrying task status update")
				time.Sleep(retryDelay)
				continue
			}
			break
		}

		s.log.WithFields(logrus.Fields{
			logger.FieldTaskID: taskID,
			logger.FieldStatus: status.String(),
		}).Info("task status updated")
		return nil
	}

	return fmt.Errorf("update task status failed: %w", lastErr)
}
