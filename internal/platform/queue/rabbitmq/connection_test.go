package rabbitmq

import (
	"context"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestConnectionManagerConnectUsesInjectedAttemptAndStartsMonitorOnce(t *testing.T) {
	manager := newTestConnectionManager()
	attempts := 0
	monitors := 0
	manager.connectAttempt = func() error {
		attempts++
		return nil
	}
	manager.startMonitor = func() { monitors++ }

	if err := manager.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if attempts != 1 || monitors != 1 {
		t.Fatalf("attempts=%d monitors=%d, want 1 and 1", attempts, monitors)
	}
}

func TestConnectionManagerReconnectRetriesThenRunsCallbacksAndMonitorOnce(t *testing.T) {
	manager := newTestConnectionManager()
	manager.ctx = context.Background()
	manager.maxReconnectTries = 3
	manager.retryStrategy = &FixedDelayStrategy{Delay: 0, MaxRetries: 3}
	attempts := 0
	monitors := 0
	callbacks := 0
	manager.connectAttempt = func() error {
		attempts++
		if attempts == 1 {
			return errors.New("temporary dial failure")
		}
		return nil
	}
	manager.startMonitor = func() { monitors++ }
	manager.RegisterReconnectCallback(func() error {
		callbacks++
		return nil
	})

	manager.reconnect()

	if attempts != 2 || callbacks != 1 || monitors != 1 {
		t.Fatalf("attempts=%d callbacks=%d monitors=%d, want 2, 1, 1", attempts, callbacks, monitors)
	}
}

func TestConnectionManagerReconnectCanceledBeforeFirstAttempt(t *testing.T) {
	manager := newTestConnectionManager()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager.ctx = ctx
	manager.maxReconnectTries = 3
	attempts := 0
	monitors := 0
	callbacks := 0
	manager.connectAttempt = func() error {
		attempts++
		return nil
	}
	manager.startMonitor = func() { monitors++ }
	manager.RegisterReconnectCallback(func() error {
		callbacks++
		return nil
	})

	manager.reconnect()

	if attempts != 0 || callbacks != 0 || monitors != 0 {
		t.Fatalf("attempts=%d callbacks=%d monitors=%d, want all zero", attempts, callbacks, monitors)
	}
}

func TestConnectionManagerReconnectExhaustionDoesNotRunCallbacksOrMonitor(t *testing.T) {
	manager := newTestConnectionManager()
	manager.ctx = context.Background()
	manager.maxReconnectTries = 3
	manager.retryStrategy = &FixedDelayStrategy{Delay: 0, MaxRetries: 3}
	attempts := 0
	monitors := 0
	callbacks := 0
	manager.connectAttempt = func() error {
		attempts++
		return errors.New("broker unavailable")
	}
	manager.startMonitor = func() { monitors++ }
	manager.RegisterReconnectCallback(func() error {
		callbacks++
		return nil
	})

	manager.reconnect()

	if attempts != 3 || callbacks != 0 || monitors != 0 {
		t.Fatalf("attempts=%d callbacks=%d monitors=%d, want 3, 0, 0", attempts, callbacks, monitors)
	}
}

func newTestConnectionManager() *ConnectionManager {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	return NewConnectionManager(ConnectionConfig{
		URL:               "amqp://unused.invalid/",
		ReconnectInterval: 1,
		MaxReconnectTries: 3,
	}, logger)
}
