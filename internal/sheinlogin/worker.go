package sheinlogin

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// RunWorker consumes durable login attempts. It is intentionally hosted by a
// dedicated process: a browser session stays owned by this worker through the
// verification-code window and is never reconstructed in the API process.
func (s *Service) RunWorker(ctx context.Context, workerID string) error {
	if s == nil || s.store == nil {
		return fmt.Errorf("shein login worker is not initialized")
	}
	if err := s.store.EnsureLoginAttemptConsumerGroup(ctx); err != nil {
		return fmt.Errorf("ensure SHEIN login consumer group: %w", err)
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		workerID = defaultLoginWorkerID()
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		messages, err := s.store.ReadLoginAttemptJobs(ctx, workerID, time.Second)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("read SHEIN login attempts: %w", err)
		}
		for _, message := range messages {
			if err := s.processLoginAttemptMessage(ctx, workerID, message.ID, message.Values); err != nil {
				sheinLoginServiceLogger.WithError(err).WithField("worker_id", workerID).Error("process SHEIN login attempt")
			}
		}
		if err := s.interruptStaleLoginAttempts(ctx, workerID); err != nil {
			sheinLoginServiceLogger.WithError(err).WithField("worker_id", workerID).Warn("recover stale SHEIN login attempts")
		}
	}
}

func (s *Service) interruptStaleLoginAttempts(ctx context.Context, workerID string) error {
	messages, err := s.store.ClaimStaleLoginAttemptJobs(ctx, workerID, 45*time.Second)
	if err != nil || len(messages) == 0 {
		return err
	}
	for _, message := range messages {
		attemptID, _ := message.Values["attempt_id"].(string)
		attempt, loadErr := s.store.LoadLoginAttempt(ctx, strings.TrimSpace(attemptID))
		if loadErr != nil {
			return loadErr
		}
		if attempt == nil || !attempt.Status.IsActive() {
			if err := s.store.AcknowledgeLoginAttemptJob(ctx, message.ID); err != nil {
				return err
			}
			continue
		}
		if err := s.completeLoginAttempt(ctx, attempt, LoginAttemptInterrupted, "WORKER_RESTARTED", "worker restarted before the browser session could be recovered", message.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) processLoginAttemptMessage(ctx context.Context, workerID, messageID string, values map[string]any) error {
	attemptID, _ := values["attempt_id"].(string)
	attemptID = strings.TrimSpace(attemptID)
	if attemptID == "" {
		return s.store.AcknowledgeLoginAttemptJob(ctx, messageID)
	}
	attempt, err := s.store.LoadLoginAttempt(ctx, attemptID)
	if err != nil {
		return err
	}
	if attempt == nil || !attempt.Status.IsActive() {
		return s.store.AcknowledgeLoginAttemptJob(ctx, messageID)
	}
	now := time.Now().UTC()
	attempt.Status = LoginAttemptLaunching
	attempt.WorkerID = workerID
	attempt.StartedAt = &now
	if err := s.store.UpdateLoginAttempt(ctx, attempt); err != nil {
		return err
	}

	result, runErr := s.loginInline(ctx, attempt.TenantID, attempt.StoreID, LoginRequest{
		ForceLogin: attempt.ForceLogin,
		Headless:   attempt.Headless,
	})
	if runErr != nil {
		return s.completeLoginAttempt(ctx, attempt, LoginAttemptFailed, "LOGIN_FAILED", runErr.Error(), messageID)
	}
	if result != nil && result.WaitingForVerifyCode {
		attempt.Status = LoginAttemptWaitingVerifyCode
		attempt.Message = result.Message
		attempt.ErrorCode = result.ErrorCode
		if err := s.store.UpdateLoginAttempt(ctx, attempt); err != nil {
			return err
		}
		// Keep the stream entry pending while this process owns the browser
		// session. If the worker exits, another worker can claim the entry and
		// mark the otherwise unrecoverable session as interrupted.
		return s.waitForAttemptVerifyCode(ctx, attempt, messageID)
	}
	if result != nil && result.Success {
		return s.completeLoginAttempt(ctx, attempt, LoginAttemptSucceeded, "", result.Message, messageID)
	}
	message := "SHEIN login failed"
	errorCode := "LOGIN_FAILED"
	if result != nil {
		message = result.Message
		if result.ErrorCode != "" {
			errorCode = result.ErrorCode
		}
	}
	return s.completeLoginAttempt(ctx, attempt, LoginAttemptFailed, errorCode, message, messageID)
}

func (s *Service) waitForAttemptVerifyCode(parent context.Context, attempt *LoginAttempt, messageID string) error {
	if attempt == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	code, ok, cancelled, err := s.store.WaitAndConsumeVerifyCodeOrCancel(ctx, attempt.TenantID, attempt.StoreID, attempt.ID, 10*time.Minute)
	if err != nil {
		if parent.Err() != nil {
			// Do not acknowledge or complete the attempt on worker shutdown. The
			// pending stream job is the durable recovery signal for the next worker.
			return nil
		}
		return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "VERIFY_CODE_WAIT_FAILED", err.Error(), messageID)
	}
	if cancelled {
		s.clearSession(attempt.StoreID)
		_, _ = s.store.CancelVerifyWait(context.Background(), attempt.TenantID, attempt.StoreID)
		return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptCancelled, "LOGIN_CANCELLED", "verification cancelled by user", messageID)
	}
	if !ok {
		s.clearSession(attempt.StoreID)
		_, _ = s.store.CancelVerifyWait(context.Background(), attempt.TenantID, attempt.StoreID)
		return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "VERIFY_CODE_TIMEOUT", "verification code wait expired", messageID)
	}
	if err := s.SubmitVerifyCode(ctx, attempt.TenantID, attempt.StoreID, code, 0); err != nil {
		return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "VERIFY_CODE_SUBMIT_FAILED", err.Error(), messageID)
	}
	status, err := s.Status(ctx, attempt.TenantID, attempt.StoreID)
	if err != nil {
		return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "VERIFY_CODE_STATUS_FAILED", err.Error(), messageID)
	}
	if status.HasCookie {
		return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptSucceeded, "", "login succeeded", messageID)
	}
	return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "VERIFY_CODE_LOGIN_FAILED", "verification code did not produce a usable cookie", messageID)
}

func (s *Service) completeLoginAttempt(ctx context.Context, attempt *LoginAttempt, status LoginAttemptStatus, errorCode, message, messageID string) error {
	if attempt == nil {
		return nil
	}
	now := time.Now().UTC()
	attempt.Status = status
	attempt.ErrorCode = errorCode
	attempt.Message = message
	attempt.CompletedAt = &now
	if status == LoginAttemptFailed {
		s.cacheLastFailureDetail(ctx, attempt.TenantID, attempt.StoreID)
	}
	if err := s.store.UpdateLoginAttempt(ctx, attempt); err != nil {
		return err
	}
	if strings.TrimSpace(messageID) == "" {
		return nil
	}
	return s.store.AcknowledgeLoginAttemptJob(ctx, messageID)
}

func defaultLoginWorkerID() string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "shein-login-worker"
	}
	return host + "-" + strconv.Itoa(os.Getpid())
}
