package rabbitmq

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewConnectionManagerBindsProductionRuntimeAndUsesRealDial(t *testing.T) {
	manager := NewConnectionManager(ConnectionConfig{
		URL:               "amqp://%zz",
		ReconnectInterval: 1,
		MaxReconnectTries: 3,
	}, newTestConnectionLogger())

	if manager.runtime == nil {
		t.Fatal("NewConnectionManager() runtime is nil")
	}
	if _, ok := manager.runtime.(productionConnectionRuntime); !ok {
		t.Fatalf("NewConnectionManager() runtime = %T, want productionConnectionRuntime", manager.runtime)
	}
	if err := manager.Connect(context.Background()); err == nil {
		t.Fatal("Connect() error = nil for a syntactically invalid AMQP URL; production dial was not used")
	}
}

func TestProductionConnectionRuntimeStartsConnectionMonitor(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "connection.go", nil, 0)
	if err != nil {
		t.Fatalf("parse connection.go: %v", err)
	}

	var startMonitor *ast.FuncDecl
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "startMonitor" || function.Recv == nil || len(function.Recv.List) != 1 {
			continue
		}
		receiver, ok := function.Recv.List[0].Type.(*ast.Ident)
		if ok && receiver.Name == "productionConnectionRuntime" {
			startMonitor = function
			break
		}
	}
	if startMonitor == nil {
		t.Fatal("productionConnectionRuntime.startMonitor method not found")
	}

	startsMonitor := false
	ast.Inspect(startMonitor.Body, func(node ast.Node) bool {
		goStatement, ok := node.(*ast.GoStmt)
		if !ok {
			return true
		}
		selector, ok := goStatement.Call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "monitorConnection" || len(goStatement.Call.Args) != 0 {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if ok && receiver.Name == "manager" {
			startsMonitor = true
		}
		return true
	})
	if !startsMonitor {
		t.Fatal("productionConnectionRuntime.startMonitor does not start manager.monitorConnection in a goroutine")
	}
}

func TestConnectionManagerConnectUsesRuntimeAndStartsMonitorOnce(t *testing.T) {
	runtime := &fakeConnectionRuntime{}
	manager := newTestConnectionManager(runtime)
	runtime.connectFn = func(*ConnectionManager) error {
		runtime.connectCalls++
		return nil
	}
	runtime.startMonitorFn = func(*ConnectionManager) { runtime.monitorCalls++ }

	if err := manager.Connect(context.Background()); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if runtime.connectCalls != 1 || runtime.monitorCalls != 1 {
		t.Fatalf("connect calls=%d monitor calls=%d, want 1 and 1", runtime.connectCalls, runtime.monitorCalls)
	}
}

func TestConnectionManagerReconnectRetriesThenRunsCallbacksAndMonitorOnce(t *testing.T) {
	runtime := &fakeConnectionRuntime{}
	manager := newTestConnectionManager(runtime)
	manager.ctx = context.Background()
	manager.maxReconnectTries = 3
	manager.retryStrategy = &FixedDelayStrategy{Delay: 0, MaxRetries: 3}
	callbacks := 0
	runtime.connectFn = func(*ConnectionManager) error {
		runtime.connectCalls++
		if runtime.connectCalls == 1 {
			return errors.New("temporary dial failure")
		}
		return nil
	}
	runtime.startMonitorFn = func(*ConnectionManager) { runtime.monitorCalls++ }
	manager.RegisterReconnectCallback(func() error {
		callbacks++
		return nil
	})

	manager.reconnect()

	if runtime.connectCalls != 2 || callbacks != 1 || runtime.monitorCalls != 1 {
		t.Fatalf("connect calls=%d callbacks=%d monitor calls=%d, want 2, 1, 1", runtime.connectCalls, callbacks, runtime.monitorCalls)
	}
}

func TestConnectionManagerReconnectCanceledBeforeFirstAttempt(t *testing.T) {
	runtime := &fakeConnectionRuntime{}
	manager := newTestConnectionManager(runtime)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	manager.ctx = ctx
	manager.maxReconnectTries = 3
	callbacks := 0
	runtime.connectFn = func(*ConnectionManager) error {
		runtime.connectCalls++
		return nil
	}
	runtime.startMonitorFn = func(*ConnectionManager) { runtime.monitorCalls++ }
	manager.RegisterReconnectCallback(func() error {
		callbacks++
		return nil
	})

	manager.reconnect()

	if runtime.connectCalls != 0 || callbacks != 0 || runtime.monitorCalls != 0 {
		t.Fatalf("connect calls=%d callbacks=%d monitor calls=%d, want all zero", runtime.connectCalls, callbacks, runtime.monitorCalls)
	}
}

func TestConnectionManagerReconnectExhaustionDoesNotRunCallbacksOrMonitor(t *testing.T) {
	runtime := &fakeConnectionRuntime{}
	manager := newTestConnectionManager(runtime)
	manager.ctx = context.Background()
	manager.maxReconnectTries = 3
	manager.retryStrategy = &FixedDelayStrategy{Delay: 0, MaxRetries: 3}
	callbacks := 0
	runtime.connectFn = func(*ConnectionManager) error {
		runtime.connectCalls++
		return errors.New("broker unavailable")
	}
	runtime.startMonitorFn = func(*ConnectionManager) { runtime.monitorCalls++ }
	manager.RegisterReconnectCallback(func() error {
		callbacks++
		return nil
	})

	manager.reconnect()

	if runtime.connectCalls != 3 || callbacks != 0 || runtime.monitorCalls != 0 {
		t.Fatalf("connect calls=%d callbacks=%d monitor calls=%d, want 3, 0, 0", runtime.connectCalls, callbacks, runtime.monitorCalls)
	}
}

type fakeConnectionRuntime struct {
	connectFn      func(*ConnectionManager) error
	startMonitorFn func(*ConnectionManager)
	connectCalls   int
	monitorCalls   int
}

func (runtime *fakeConnectionRuntime) connect(manager *ConnectionManager) error {
	return runtime.connectFn(manager)
}

func (runtime *fakeConnectionRuntime) startMonitor(manager *ConnectionManager) {
	runtime.startMonitorFn(manager)
}

func newTestConnectionManager(runtime connectionRuntime) *ConnectionManager {
	return newConnectionManager(ConnectionConfig{
		URL:               "amqp://unused.invalid/",
		ReconnectInterval: 1,
		MaxReconnectTries: 3,
	}, newTestConnectionLogger(), runtime)
}

func newTestConnectionLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetLevel(logrus.ErrorLevel)
	return logger
}
