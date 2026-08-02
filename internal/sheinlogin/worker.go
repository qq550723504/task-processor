package sheinlogin

import (
	"context"
	"errors"
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
	updated, err := s.updateActiveAttemptState(ctx, attempt, messageID)
	if err != nil {
		return err
	}
	if !updated {
		return nil
	}

	attemptCtx, cancelAttempt := context.WithCancel(ctx)
	defer cancelAttempt()
	control := s.startAttemptControlLoop(attemptCtx, attempt, cancelAttempt)
	defer control.Stop()

	account, err := s.provider.GetAccount(attemptCtx, attempt.TenantID, attempt.StoreID)
	if err != nil {
		if ctx.Err() != nil && !control.Cancelled() {
			return nil
		}
		return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "LOGIN_ACCOUNT_FAILED", err.Error(), messageID)
	}
	if !attempt.ForceLogin {
		if ttl, ok, ttlErr := s.store.CookieTTL(attemptCtx, account.TenantID, account.StoreID); ttlErr == nil && ok && ttl > 0 {
			result := s.existingCookieLoginResult(account, ttl)
			return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptSucceeded, "", result.Message, messageID)
		} else if ttlErr != nil {
			if ctx.Err() != nil && !control.Cancelled() {
				return nil
			}
			return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "LOGIN_STATUS_FAILED", ttlErr.Error(), messageID)
		}
	}

	runResult, session, runErr := s.runLoginStart(attemptCtx, account, LoginRequest{
		ForceLogin: attempt.ForceLogin,
		Headless:   attempt.Headless,
	})
	if runErr != nil {
		if control.Cancelled() || errors.Is(runErr, context.Canceled) && control.Cancelled() {
			return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptCancelled, "LOGIN_CANCELLED", "verification cancelled by user", messageID)
		}
		if ctx.Err() != nil {
			return nil
		}
		return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "LOGIN_FAILED", runErr.Error(), messageID)
	}
	if session != nil {
		defer session.Close()
	}
	if runResult != nil && runResult.WaitingForVerifyCode {
		if control.Cancelled() {
			return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptCancelled, "LOGIN_CANCELLED", "verification cancelled by user", messageID)
		}
		updated, err := s.transitionAttemptToVerifyWait(context.Background(), attempt, account, runResult, messageID)
		if err != nil {
			return err
		}
		if !updated {
			return nil
		}
		// Keep the stream entry pending while this process owns the browser
		// session. If the worker exits, another worker can claim the entry and
		// mark the otherwise unrecoverable session as interrupted.
		return s.waitForAttemptVerifyCode(ctx, attemptCtx, attempt, account, session, control, messageID)
	}
	if runResult != nil && runResult.BrowserState != nil {
		return s.completeSuccessfulAttempt(context.Background(), attempt, account, runResult.BrowserState, "login succeeded", messageID)
	}
	message := "SHEIN login failed"
	errorCode := "LOGIN_FAILED"
	if runResult != nil {
		message = failureMessage(runResult)
		if runResult.ErrorCode != "" {
			errorCode = runResult.ErrorCode
		}
	}
	return s.completeFailedLoginAttempt(context.Background(), attempt, errorCode, message, messageID, runResultFailureSummary(runResult))
}

func (s *Service) updateActiveAttemptState(ctx context.Context, attempt *LoginAttempt, messageID string) (bool, error) {
	if attempt == nil {
		return false, nil
	}
	updated, err := s.store.UpdateLoginAttemptIfActive(ctx, attempt)
	if err != nil {
		return false, err
	}
	if updated {
		return true, nil
	}
	current, err := s.store.LoadLoginAttempt(ctx, attempt.ID)
	if err != nil {
		return false, err
	}
	if current != nil {
		*attempt = *current
	}
	if strings.TrimSpace(messageID) == "" {
		return false, nil
	}
	if current == nil || !current.Status.IsActive() {
		if err := s.store.AcknowledgeLoginAttemptJob(ctx, messageID); err != nil {
			return false, err
		}
	}
	return false, nil
}

type loginAttemptControlEvent struct {
	code      string
	received  bool
	cancelled bool
	err       error
}

type loginAttemptControlLoop struct {
	events    chan loginAttemptControlEvent
	done      chan struct{}
	cancelled chan struct{}
	stop      context.CancelFunc
}

func (l *loginAttemptControlLoop) Cancelled() bool {
	if l == nil {
		return false
	}
	select {
	case <-l.cancelled:
		return true
	default:
		return false
	}
}

func (l *loginAttemptControlLoop) markCancelled() {
	if l == nil {
		return
	}
	select {
	case <-l.cancelled:
	default:
		close(l.cancelled)
	}
}

func (l *loginAttemptControlLoop) Stop() {
	if l == nil {
		return
	}
	l.stop()
	<-l.done
}

func (s *Service) startAttemptControlLoop(ctx context.Context, attempt *LoginAttempt, cancelAttempt context.CancelFunc) *loginAttemptControlLoop {
	loopCtx, stop := context.WithCancel(ctx)
	loop := &loginAttemptControlLoop{
		events:    make(chan loginAttemptControlEvent, 4),
		done:      make(chan struct{}),
		cancelled: make(chan struct{}),
		stop:      stop,
	}
	go func() {
		defer close(loop.done)
		defer close(loop.events)
		if attempt == nil {
			return
		}
		for {
			code, received, cancelled, err := s.store.WaitAndConsumeVerifyCodeOrCancel(loopCtx, attempt.TenantID, attempt.StoreID, attempt.ID, time.Second)
			if err != nil {
				if loopCtx.Err() != nil {
					return
				}
				select {
				case loop.events <- loginAttemptControlEvent{err: err}:
				case <-loopCtx.Done():
				}
				return
			}
			if cancelled {
				loop.markCancelled()
				select {
				case loop.events <- loginAttemptControlEvent{cancelled: true}:
				case <-loopCtx.Done():
				}
				cancelAttempt()
				return
			}
			if received {
				select {
				case loop.events <- loginAttemptControlEvent{code: code, received: true}:
				case <-loopCtx.Done():
					return
				}
			}
		}
	}()
	return loop
}

type loginAttemptWaitResult struct {
	result *AutomationResult
	err    error
}

func waitForLoginResult(ctx context.Context, session VerifySession) <-chan loginAttemptWaitResult {
	watcher, ok := session.(VerifySessionLoginWatcher)
	if !ok || watcher == nil {
		return nil
	}
	ch := make(chan loginAttemptWaitResult, 1)
	go func() {
		result, err := watcher.WaitForLogin(ctx)
		ch <- loginAttemptWaitResult{result: result, err: err}
		close(ch)
	}()
	return ch
}

func (s *Service) waitForAttemptVerifyCode(parentCtx, attemptCtx context.Context, attempt *LoginAttempt, account *Account, session VerifySession, control *loginAttemptControlLoop, messageID string) error {
	if attempt == nil || account == nil || session == nil {
		return nil
	}
	verifyCtx, cancel := context.WithTimeout(attemptCtx, 10*time.Minute)
	defer cancel()
	waitCh := waitForLoginResult(verifyCtx, session)
	var controlEvents <-chan loginAttemptControlEvent
	if control != nil {
		controlEvents = control.events
	}
	for {
		select {
		case <-parentCtx.Done():
			if control.Cancelled() {
				return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptCancelled, "LOGIN_CANCELLED", "verification cancelled by user", messageID)
			}
			// Do not acknowledge or complete the attempt on worker shutdown. The
			// pending stream job is the durable recovery signal for the next worker.
			return nil
		case <-verifyCtx.Done():
			if control.Cancelled() {
				return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptCancelled, "LOGIN_CANCELLED", "verification cancelled by user", messageID)
			}
			if parentCtx.Err() != nil {
				return nil
			}
			if errors.Is(verifyCtx.Err(), context.DeadlineExceeded) {
				return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "VERIFY_CODE_TIMEOUT", "verification code wait expired", messageID)
			}
			return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "VERIFY_CODE_WAIT_FAILED", verifyCtx.Err().Error(), messageID)
		case event, ok := <-controlEvents:
			if !ok {
				controlEvents = nil
				continue
			}
			if event.err != nil {
				if control.Cancelled() {
					return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptCancelled, "LOGIN_CANCELLED", "verification cancelled by user", messageID)
				}
				if parentCtx.Err() != nil {
					return nil
				}
				return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "VERIFY_CODE_WAIT_FAILED", event.err.Error(), messageID)
			}
			if event.cancelled {
				return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptCancelled, "LOGIN_CANCELLED", "verification cancelled by user", messageID)
			}
			if !event.received {
				continue
			}
			runResult, err := session.SubmitCode(verifyCtx, event.code)
			if err != nil {
				if control != nil && control.Cancelled() {
					return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptCancelled, "LOGIN_CANCELLED", "verification cancelled by user", messageID)
				}
				if parentCtx.Err() != nil {
					return nil
				}
				if errors.Is(verifyCtx.Err(), context.DeadlineExceeded) {
					return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "VERIFY_CODE_TIMEOUT", "verification code wait expired", messageID)
				}
				return s.completeLoginAttempt(context.Background(), attempt, LoginAttemptFailed, "VERIFY_CODE_SUBMIT_FAILED", err.Error(), messageID)
			}
			done, finishErr := s.finishAttemptVerifyResult(attempt, account, runResult, messageID)
			if finishErr != nil || done {
				return finishErr
			}
		case waitResult, ok := <-waitCh:
			if !ok {
				waitCh = nil
				continue
			}
			if waitResult.err != nil || waitResult.result == nil || waitResult.result.BrowserState == nil {
				waitCh = nil
				continue
			}
			return s.completeSuccessfulAttempt(context.Background(), attempt, account, waitResult.result.BrowserState, "login succeeded", messageID)
		}
	}
}

func (s *Service) finishAttemptVerifyResult(attempt *LoginAttempt, account *Account, runResult *AutomationResult, messageID string) (bool, error) {
	if attempt == nil || account == nil {
		return true, nil
	}
	if runResult != nil && runResult.BrowserState != nil {
		return true, s.completeSuccessfulAttempt(context.Background(), attempt, account, runResult.BrowserState, "login succeeded", messageID)
	}
	if runResult != nil && runResult.WaitingForVerifyCode {
		updated, err := s.transitionAttemptToVerifyWait(context.Background(), attempt, account, runResult, messageID)
		if err != nil {
			return true, err
		}
		if !updated {
			return true, nil
		}
		return false, nil
	}
	return true, s.completeFailedLoginAttempt(
		context.Background(),
		attempt,
		failureCode(runResult),
		failureMessage(runResult),
		messageID,
		runResultFailureSummary(runResult),
	)
}

func (s *Service) completeLoginAttempt(ctx context.Context, attempt *LoginAttempt, status LoginAttemptStatus, errorCode, message, messageID string) error {
	return s.completeLoginAttemptWithFailureSummary(ctx, attempt, status, errorCode, message, messageID, nil)
}

func (s *Service) completeFailedLoginAttempt(ctx context.Context, attempt *LoginAttempt, errorCode, message, messageID string, summary *FailureSummary) error {
	return s.completeLoginAttemptWithFailureSummary(ctx, attempt, LoginAttemptFailed, errorCode, message, messageID, summary)
}

func (s *Service) completeLoginAttemptWithFailureSummary(ctx context.Context, attempt *LoginAttempt, status LoginAttemptStatus, errorCode, message, messageID string, summary *FailureSummary) error {
	if attempt == nil {
		return nil
	}
	statusBeforeCompletion := attempt.Status
	now := time.Now().UTC()
	attempt.Status = status
	attempt.ErrorCode = errorCode
	attempt.Message = message
	attempt.CompletedAt = &now
	completed, err := s.store.CompleteLoginAttemptIfActive(ctx, attempt)
	if err != nil {
		return err
	}
	if completed && status == LoginAttemptFailed {
		s.persistCommittedWorkerFailure(ctx, attempt, statusBeforeCompletion, errorCode, message, summary)
	}
	if !completed {
		current, loadErr := s.store.LoadLoginAttempt(ctx, attempt.ID)
		if loadErr != nil {
			return loadErr
		}
		if current != nil {
			*attempt = *current
		}
	}
	if strings.TrimSpace(messageID) == "" {
		return nil
	}
	return s.store.AcknowledgeLoginAttemptJob(ctx, messageID)
}

func (s *Service) transitionAttemptToVerifyWait(ctx context.Context, attempt *LoginAttempt, account *Account, runResult *AutomationResult, messageID string) (bool, error) {
	if attempt == nil || account == nil {
		return false, nil
	}
	summary := runResultFailureSummary(runResult)
	if summary == nil {
		summary = verifyCodeFailureSummary(account)
	}
	attempt.Status = LoginAttemptWaitingVerifyCode
	attempt.Message = summary.ErrorMessage
	attempt.ErrorCode = summary.ErrorCode
	updated, err := s.store.PersistVerifyWaitAndUpdateAttemptIfActive(ctx, attempt, summary, 10*time.Minute, 30*24*time.Hour)
	if err != nil {
		return false, err
	}
	if updated {
		return true, nil
	}
	current, loadErr := s.store.LoadLoginAttempt(ctx, attempt.ID)
	if loadErr != nil {
		return false, loadErr
	}
	if current != nil {
		*attempt = *current
	}
	if strings.TrimSpace(messageID) == "" {
		return false, nil
	}
	return false, s.store.AcknowledgeLoginAttemptJob(ctx, messageID)
}

func (s *Service) completeSuccessfulAttempt(ctx context.Context, attempt *LoginAttempt, account *Account, browserState map[string]any, message, messageID string) error {
	if attempt == nil || account == nil {
		return nil
	}
	now := time.Now().UTC()
	attempt.Status = LoginAttemptSucceeded
	attempt.ErrorCode = ""
	attempt.Message = message
	attempt.CompletedAt = &now
	completed, err := s.store.CompleteSuccessfulLoginAttemptIfActive(ctx, attempt, browserState, now)
	if err != nil {
		return err
	}
	if completed {
		s.syncStoreIDAfterLogin(ctx, *account)
	} else {
		current, loadErr := s.store.LoadLoginAttempt(ctx, attempt.ID)
		if loadErr != nil {
			return loadErr
		}
		if current != nil {
			*attempt = *current
		}
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
