package database

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"runtime"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func newTestConnectionProxy(t *testing.T, maxOps int) (*ConnectionProxy, *sql.DB, sqlmock.Sqlmock) {
	return newTestConnectionProxyWithCloseError(t, maxOps, nil)
}

func newTestConnectionProxyWithCloseError(t *testing.T, maxOps int, closeErr error) (*ConnectionProxy, *sql.DB, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("open SQL mock: %v", err)
	}
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		t.Fatalf("open GORM test database: %v", err)
	}

	proxy := &ConnectionProxy{
		master:    db,
		semaphore: make(chan struct{}, maxOps),
		maxOps:    maxOps,
	}
	closeExpectation := mock.ExpectClose()
	if closeErr != nil {
		closeExpectation.WillReturnError(closeErr)
	}
	t.Cleanup(func() {
		if err := proxy.Close(); !errors.Is(err, closeErr) {
			t.Errorf("close proxy error = %v, want %v", err, closeErr)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("SQL expectations: %v", err)
		}
	})
	return proxy, sqlDB, mock
}

func waitForConnectionProxyCloseWaiters(t *testing.T, results <-chan error, want int) {
	t.Helper()

	const attempts = 10_000
	for range attempts {
		select {
		case err := <-results:
			t.Fatalf("Close returned before active Execute was released: %v", err)
		default:
		}

		stack := make([]byte, 64<<10)
		stack = stack[:runtime.Stack(stack, true)]
		waiting := 0
		for _, goroutine := range bytes.Split(stack, []byte("\n\n")) {
			if bytes.Contains(goroutine, []byte("[sync.Mutex.Lock")) &&
				bytes.Contains(goroutine, []byte("database.(*ConnectionProxy).Close")) {
				waiting++
			}
		}
		if waiting == want {
			return
		}
		runtime.Gosched()
	}

	t.Fatalf("concurrent Close waiters did not block in sync.Once; want %d", want)
}

func TestNewConnectionProxyRejectsMissingConfig(t *testing.T) {
	t.Parallel()

	if _, err := NewConnectionProxy(nil); err == nil || !strings.Contains(err.Error(), "connection proxy config is nil") {
		t.Fatalf("NewConnectionProxy(nil) error = %v", err)
	}
	if _, err := NewConnectionProxy(&ConnectionProxyConfig{}); err == nil || !strings.Contains(err.Error(), "database config is nil") {
		t.Fatalf("NewConnectionProxy(empty) error = %v", err)
	}
}

func TestConnectionProxyExecuteLimitsConcurrencyAndReleasesSlots(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		proxy, _, _ := newTestConnectionProxy(t, 2)
		release := make(chan struct{})
		defer close(release)
		started := make(chan struct{}, 2)
		results := make(chan error, 2)

		for range 2 {
			go func() {
				results <- proxy.Execute(t.Context(), func(*gorm.DB) error {
					started <- struct{}{}
					<-release
					return nil
				})
			}()
		}
		<-started
		<-started

		stats := proxy.GetStats()
		if got := stats["max_concurrent_ops"]; got != 2 {
			t.Fatalf("max_concurrent_ops = %v, want 2", got)
		}
		if got := stats["active_operations"]; got != int64(2) {
			t.Fatalf("active_operations = %v, want 2", got)
		}
		if got := stats["semaphore_available"]; got != 2 {
			t.Fatalf("semaphore_available = %v, want 2 occupied slots", got)
		}

		waitingContext, cancelWaiting := context.WithCancel(t.Context())
		waitingCalled := make(chan struct{}, 1)
		waitingResult := make(chan error, 1)
		go func() {
			waitingResult <- proxy.Execute(waitingContext, func(*gorm.DB) error {
				waitingCalled <- struct{}{}
				return nil
			})
		}()
		synctest.Wait()
		cancelWaiting()
		synctest.Wait()
		if err := <-waitingResult; !errors.Is(err, context.Canceled) {
			t.Fatalf("saturated Execute error = %v, want context cancellation", err)
		}
		select {
		case <-waitingCalled:
			t.Fatal("saturated Execute invoked callback after context cancellation")
		default:
		}

		release <- struct{}{}
		if err := <-results; err != nil {
			t.Fatalf("first blocked Execute error = %v", err)
		}
		if err := proxy.Execute(t.Context(), func(*gorm.DB) error { return nil }); err != nil {
			t.Fatalf("Execute after slot release error = %v", err)
		}
		release <- struct{}{}
		if err := <-results; err != nil {
			t.Fatalf("second blocked Execute error = %v", err)
		}

		stats = proxy.GetStats()
		if got := stats["active_operations"]; got != int64(0) {
			t.Fatalf("active_operations after release = %v, want 0", got)
		}
		if got := stats["semaphore_available"]; got != 0 {
			t.Fatalf("semaphore_available after release = %v, want 0 occupied slots", got)
		}
	})
}

func TestConnectionProxyExecuteWithTimeoutWhileSaturated(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		proxy, _, _ := newTestConnectionProxy(t, 1)
		release := make(chan struct{})
		defer close(release)
		started := make(chan struct{})
		blockingResult := make(chan error, 1)
		go func() {
			blockingResult <- proxy.Execute(t.Context(), func(*gorm.DB) error {
				close(started)
				<-release
				return nil
			})
		}()
		<-started

		called := false
		err := proxy.ExecuteWithTimeout(t.Context(), 25*time.Millisecond, func(*gorm.DB) error {
			called = true
			return nil
		})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("ExecuteWithTimeout error = %v, want deadline exceeded", err)
		}
		if called {
			t.Fatal("ExecuteWithTimeout invoked callback while semaphore was saturated")
		}

		release <- struct{}{}
		if err := <-blockingResult; err != nil {
			t.Fatalf("blocking Execute error = %v", err)
		}
	})
}

func TestConnectionProxyExecutePropagatesCallbackErrorAndRestoresStats(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		proxy, _, _ := newTestConnectionProxy(t, 1)
		callbackErr := errors.New("callback failed")

		if err := proxy.Execute(t.Context(), func(*gorm.DB) error { return callbackErr }); !errors.Is(err, callbackErr) {
			t.Fatalf("Execute error = %v, want callback error", err)
		}
		stats := proxy.GetStats()
		if got := stats["active_operations"]; got != int64(0) {
			t.Fatalf("active_operations after callback error = %v, want 0", got)
		}
		if got := stats["semaphore_available"]; got != 0 {
			t.Fatalf("semaphore_available after callback error = %v, want 0 occupied slots", got)
		}
		if err := proxy.Execute(t.Context(), func(*gorm.DB) error { return nil }); err != nil {
			t.Fatalf("Execute after callback error error = %v", err)
		}
	})
}

func TestConnectionProxyCloseWaitsForActiveExecutionThenRejectsNewWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		proxy, sqlDB, _ := newTestConnectionProxy(t, 1)
		release := make(chan struct{})
		started := make(chan struct{})
		executeResult := make(chan error, 1)
		go func() {
			executeResult <- proxy.Execute(t.Context(), func(*gorm.DB) error {
				close(started)
				<-release
				return sqlDB.PingContext(t.Context())
			})
		}()
		<-started

		closeResult := make(chan error, 1)
		go func() { closeResult <- proxy.Close() }()
		synctest.Wait()
		select {
		case err := <-closeResult:
			close(release)
			synctest.Wait()
			<-executeResult
			t.Fatalf("Close returned while callback was active: %v", err)
		default:
		}
		if err := sqlDB.PingContext(t.Context()); err != nil {
			t.Fatalf("database closed while callback was active: %v", err)
		}

		close(release)
		synctest.Wait()
		if err := <-executeResult; err != nil {
			t.Fatalf("active Execute error = %v", err)
		}
		if err := <-closeResult; err != nil {
			t.Fatalf("Close error = %v", err)
		}
		if err := sqlDB.PingContext(t.Context()); err == nil || !strings.Contains(err.Error(), "database is closed") {
			t.Fatalf("Ping after Close error = %v, want database closed", err)
		}

		called := false
		err := proxy.Execute(t.Context(), func(*gorm.DB) error {
			called = true
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "connection proxy is closed") {
			t.Fatalf("Execute after Close error = %v, want closed proxy", err)
		}
		if called {
			t.Fatal("Execute after Close invoked callback")
		}
	})
}

func TestConnectionProxyQueuedExecutionIsRejectedWhenCloseStartsBeforeSlotRelease(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		proxy, _, _ := newTestConnectionProxy(t, 1)
		releaseFirst := make(chan struct{}, 1)
		defer close(releaseFirst)
		firstStarted := make(chan struct{})
		firstResult := make(chan error, 1)
		go func() {
			firstResult <- proxy.Execute(t.Context(), func(*gorm.DB) error {
				close(firstStarted)
				<-releaseFirst
				return nil
			})
		}()
		<-firstStarted

		queuedCalled := make(chan struct{}, 1)
		queuedResult := make(chan error, 1)
		go func() {
			queuedResult <- proxy.Execute(t.Context(), func(*gorm.DB) error {
				queuedCalled <- struct{}{}
				return nil
			})
		}()
		synctest.Wait()

		closeResult := make(chan error, 1)
		go func() { closeResult <- proxy.Close() }()
		synctest.Wait()

		releaseFirst <- struct{}{}
		synctest.Wait()
		if err := <-firstResult; err != nil {
			t.Fatalf("first Execute error = %v", err)
		}
		if err := <-queuedResult; err == nil || !strings.Contains(err.Error(), "connection proxy is closed") {
			t.Fatalf("queued Execute error = %v, want closed proxy", err)
		}
		select {
		case <-queuedCalled:
			t.Fatal("queued Execute invoked callback after Close started")
		default:
		}
		if err := <-closeResult; err != nil {
			t.Fatalf("Close error = %v", err)
		}
	})
}

func TestConnectionProxyConcurrentCloseSharesUnderlyingErrorAndRejectsNewWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		closeErr := errors.New("driver close failed")
		proxy, _, mock := newTestConnectionProxyWithCloseError(t, 1, closeErr)
		releaseExecute := make(chan struct{}, 1)
		defer close(releaseExecute)
		executeStarted := make(chan struct{})
		executeResult := make(chan error, 1)
		go func() {
			executeResult <- proxy.Execute(t.Context(), func(*gorm.DB) error {
				close(executeStarted)
				<-releaseExecute
				return nil
			})
		}()
		<-executeStarted

		const closeCalls = 8
		results := make(chan error, closeCalls)
		go func() { results <- proxy.Close() }()
		synctest.Wait()

		for range closeCalls - 1 {
			go func() {
				results <- proxy.Close()
			}()
		}
		waitForConnectionProxyCloseWaiters(t, results, closeCalls-1)

		releaseExecute <- struct{}{}
		synctest.Wait()
		if err := <-executeResult; err != nil {
			t.Fatalf("active Execute error = %v", err)
		}

		for i := range closeCalls {
			if err := <-results; !errors.Is(err, closeErr) {
				t.Fatalf("Close result %d = %v, want shared close error", i, err)
			}
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("underlying Close calls: %v", err)
		}

		called := false
		err := proxy.Execute(t.Context(), func(*gorm.DB) error {
			called = true
			return nil
		})
		if err == nil || !strings.Contains(err.Error(), "connection proxy is closed") {
			t.Fatalf("Execute after failed Close error = %v, want closed proxy", err)
		}
		if called {
			t.Fatal("Execute after failed Close invoked callback")
		}
	})
}
