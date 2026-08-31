package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
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
	} else if !strings.Contains(err.Error(), `invalid URL escape "%zz"`) {
		t.Fatalf("Connect() error = %q, want invalid URL escape from the production dial path", err)
	}
}

func TestProductionConnectionRuntimeDelegatesDirectly(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "connection.go", nil, 0)
	if err != nil {
		t.Fatalf("parse connection.go: %v", err)
	}

	tests := []struct {
		name      string
		method    string
		delegate  string
		statement runtimeDelegationStatement
	}{
		{name: "connect", method: "connect", delegate: "connect", statement: runtimeReturnDelegation},
		{name: "start monitor", method: "startMonitor", delegate: "monitorConnection", statement: runtimeGoDelegation},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateDirectRuntimeDelegation(parsed, "productionConnectionRuntime", test.method, test.delegate, test.statement); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateDirectRuntimeDelegation(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		method    string
		delegate  string
		statement runtimeDelegationStatement
		wantValid bool
	}{
		{
			name:      "value receiver and renamed parameter",
			source:    `package rabbitmq; type productionConnectionRuntime struct{}; func (productionConnectionRuntime) connect(cm *ConnectionManager) error { return cm.connect() }`,
			method:    "connect",
			delegate:  "connect",
			statement: runtimeReturnDelegation,
			wantValid: true,
		},
		{
			name:      "pointer receiver and renamed parameter",
			source:    `package rabbitmq; type productionConnectionRuntime struct{}; func (*productionConnectionRuntime) startMonitor(cm *ConnectionManager) { go cm.monitorConnection() }`,
			method:    "startMonitor",
			delegate:  "monitorConnection",
			statement: runtimeGoDelegation,
			wantValid: true,
		},
		{
			name:      "fixed connect error",
			source:    `package rabbitmq; type productionConnectionRuntime struct{}; func (productionConnectionRuntime) connect(cm *ConnectionManager) error { return errors.New("fixed") }`,
			method:    "connect",
			delegate:  "connect",
			statement: runtimeReturnDelegation,
			wantValid: false,
		},
		{
			name:      "monitor hidden in unreachable branch",
			source:    `package rabbitmq; type productionConnectionRuntime struct{}; func (productionConnectionRuntime) startMonitor(cm *ConnectionManager) { if false { go cm.monitorConnection() } }`,
			method:    "startMonitor",
			delegate:  "monitorConnection",
			statement: runtimeGoDelegation,
			wantValid: false,
		},
		{
			name:      "monitor hidden in closure",
			source:    `package rabbitmq; type productionConnectionRuntime struct{}; func (productionConnectionRuntime) startMonitor(cm *ConnectionManager) { func() { go cm.monitorConnection() }() }`,
			method:    "startMonitor",
			delegate:  "monitorConnection",
			statement: runtimeGoDelegation,
			wantValid: false,
		},
		{
			name:      "extra top-level statement",
			source:    `package rabbitmq; type productionConnectionRuntime struct{}; func (productionConnectionRuntime) startMonitor(cm *ConnectionManager) { println("extra"); go cm.monitorConnection() }`,
			method:    "startMonitor",
			delegate:  "monitorConnection",
			statement: runtimeGoDelegation,
			wantValid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			parsed, err := parser.ParseFile(token.NewFileSet(), "", test.source, 0)
			if err != nil {
				t.Fatalf("parse fixture: %v", err)
			}
			err = validateDirectRuntimeDelegation(parsed, "productionConnectionRuntime", test.method, test.delegate, test.statement)
			if test.wantValid && err != nil {
				t.Fatalf("valid direct delegation rejected: %v", err)
			}
			if !test.wantValid && err == nil {
				t.Fatal("invalid delegation accepted")
			}
		})
	}
}

type runtimeDelegationStatement int

const (
	runtimeReturnDelegation runtimeDelegationStatement = iota
	runtimeGoDelegation
)

func validateDirectRuntimeDelegation(
	file *ast.File,
	receiverType string,
	methodName string,
	delegateName string,
	statementType runtimeDelegationStatement,
) error {
	method := findRuntimeMethod(file, receiverType, methodName)
	if method == nil {
		return fmt.Errorf("%s.%s method not found", receiverType, methodName)
	}
	if method.Type.Params == nil || len(method.Type.Params.List) != 1 || len(method.Type.Params.List[0].Names) != 1 {
		return fmt.Errorf("%s.%s must have exactly one named parameter", receiverType, methodName)
	}
	parameterName := method.Type.Params.List[0].Names[0].Name
	if method.Body == nil || len(method.Body.List) != 1 {
		return fmt.Errorf("%s.%s must contain exactly one top-level statement", receiverType, methodName)
	}

	var call *ast.CallExpr
	switch statementType {
	case runtimeReturnDelegation:
		statement, ok := method.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(statement.Results) != 1 {
			return fmt.Errorf("%s.%s must directly return one call", receiverType, methodName)
		}
		call, ok = statement.Results[0].(*ast.CallExpr)
		if !ok {
			return fmt.Errorf("%s.%s must directly return a call", receiverType, methodName)
		}
	case runtimeGoDelegation:
		statement, ok := method.Body.List[0].(*ast.GoStmt)
		if !ok {
			return fmt.Errorf("%s.%s must directly start one goroutine", receiverType, methodName)
		}
		call = statement.Call
	default:
		return fmt.Errorf("unknown delegation statement type %d", statementType)
	}

	if !isDirectParameterCall(call, parameterName, delegateName) {
		return fmt.Errorf("%s.%s must directly call %s.%s()", receiverType, methodName, parameterName, delegateName)
	}
	return nil
}

func findRuntimeMethod(file *ast.File, receiverType string, methodName string) *ast.FuncDecl {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != methodName || function.Recv == nil || len(function.Recv.List) != 1 {
			continue
		}
		if runtimeReceiverBaseName(function.Recv.List[0].Type) == receiverType {
			return function
		}
	}
	return nil
}

func runtimeReceiverBaseName(expression ast.Expr) string {
	switch receiver := expression.(type) {
	case *ast.Ident:
		return receiver.Name
	case *ast.StarExpr:
		return runtimeReceiverBaseName(receiver.X)
	case *ast.ParenExpr:
		return runtimeReceiverBaseName(receiver.X)
	default:
		return ""
	}
}

func isDirectParameterCall(call *ast.CallExpr, parameterName string, delegateName string) bool {
	if call == nil || len(call.Args) != 0 {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != delegateName {
		return false
	}
	receiver, ok := selector.X.(*ast.Ident)
	return ok && receiver.Name == parameterName
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
