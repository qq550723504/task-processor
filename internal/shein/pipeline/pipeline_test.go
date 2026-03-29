package pipeline_test

import (
	"errors"
	"testing"

	"task-processor/internal/shein"
	"task-processor/internal/shein/pipeline"
)

// mockHandler 用于测试的 StepHandler mock
type mockHandler struct {
	name     string
	handleFn func(ctx *shein.TaskContext) error
	called   bool
}

func (m *mockHandler) Name() string { return m.name }

func (m *mockHandler) Handle(ctx *shein.TaskContext) error {
	m.called = true
	if m.handleFn != nil {
		return m.handleFn(ctx)
	}
	return nil
}

func newMockHandler(name string, fn func(ctx *shein.TaskContext) error) *mockHandler {
	return &mockHandler{name: name, handleFn: fn}
}

// TestNewPipeline 验证 NewPipeline 返回空管道
func TestNewPipeline(t *testing.T) {
	p := pipeline.NewPipeline()
	if p == nil {
		t.Fatal("NewPipeline() returned nil")
	}
	if len(p.Handlers()) != 0 {
		t.Errorf("expected 0 handlers, got %d", len(p.Handlers()))
	}
}

// TestPipeline_AddHandler 验证链式添加处理器
func TestPipeline_AddHandler(t *testing.T) {
	p := pipeline.NewPipeline()
	h1 := newMockHandler("step1", nil)
	h2 := newMockHandler("step2", nil)

	ret := p.AddHandler(h1).AddHandler(h2)

	if ret != p {
		t.Error("AddHandler should return the same pipeline for chaining")
	}
	if len(p.Handlers()) != 2 {
		t.Errorf("expected 2 handlers, got %d", len(p.Handlers()))
	}
}

// TestPipeline_Handlers_ReturnsCopy 验证 Handlers() 返回副本，外部修改不影响内部状态
func TestPipeline_Handlers_ReturnsCopy(t *testing.T) {
	p := pipeline.NewPipeline()
	p.AddHandler(newMockHandler("step1", nil))

	handlers := p.Handlers()
	handlers[0] = newMockHandler("injected", nil)

	// 内部状态不应被修改
	if p.Handlers()[0].Name() != "step1" {
		t.Error("Handlers() should return a copy; external modification should not affect pipeline")
	}
}

// TestPipeline_Process 表驱动测试，覆盖各种执行场景
func TestPipeline_Process(t *testing.T) {
	ctx := shein.NewTaskContext(nil, nil)

	tests := []struct {
		name        string
		handlers    []*mockHandler
		wantErr     bool
		wantErrType string // "filtered" | "regular" | ""
	}{
		{
			name:     "空管道成功执行",
			handlers: nil,
			wantErr:  false,
		},
		{
			name: "所有步骤成功",
			handlers: []*mockHandler{
				newMockHandler("step1", nil),
				newMockHandler("step2", nil),
				newMockHandler("step3", nil),
			},
			wantErr: false,
		},
		{
			name: "第一步失败立即返回错误",
			handlers: []*mockHandler{
				newMockHandler("step1", func(_ *shein.TaskContext) error {
					return errors.New("step1 failed")
				}),
				newMockHandler("step2", nil),
			},
			wantErr:     true,
			wantErrType: "regular",
		},
		{
			name: "中间步骤失败后续步骤不执行",
			handlers: []*mockHandler{
				newMockHandler("step1", nil),
				newMockHandler("step2", func(_ *shein.TaskContext) error {
					return errors.New("step2 failed")
				}),
				newMockHandler("step3", nil),
			},
			wantErr:     true,
			wantErrType: "regular",
		},
		{
			name: "FilteredError 被返回但不视为系统错误",
			handlers: []*mockHandler{
				newMockHandler("step1", nil),
				newMockHandler("step2", func(_ *shein.TaskContext) error {
					return shein.NewFilteredError("低于筛选规则最低价格")
				}),
				newMockHandler("step3", nil),
			},
			wantErr:     true,
			wantErrType: "filtered",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := pipeline.NewPipeline()
			for _, h := range tt.handlers {
				p.AddHandler(h)
			}

			err := p.Process(ctx)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}

			switch tt.wantErrType {
			case "filtered":
				if !shein.IsFilteredError(err) {
					t.Errorf("expected FilteredError, got: %T %v", err, err)
				}
			case "regular":
				if shein.IsFilteredError(err) {
					t.Errorf("expected regular error, got FilteredError: %v", err)
				}
			}
		})
	}
}

// TestPipeline_Process_StepsCalledInOrder 验证处理器按顺序执行
func TestPipeline_Process_StepsCalledInOrder(t *testing.T) {
	ctx := shein.NewTaskContext(nil, nil)
	order := make([]string, 0, 3)

	p := pipeline.NewPipeline()
	for _, name := range []string{"a", "b", "c"} {
		n := name // capture
		p.AddHandler(newMockHandler(n, func(_ *shein.TaskContext) error {
			order = append(order, n)
			return nil
		}))
	}

	if err := p.Process(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"a", "b", "c"}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("step %d: expected %q, got %q", i, v, order[i])
		}
	}
}

// TestPipeline_Process_FailedStepSkipsRemaining 验证失败后后续步骤不被调用
func TestPipeline_Process_FailedStepSkipsRemaining(t *testing.T) {
	ctx := shein.NewTaskContext(nil, nil)

	last := newMockHandler("last", nil)
	p := pipeline.NewPipeline()
	p.AddHandler(newMockHandler("fail", func(_ *shein.TaskContext) error {
		return errors.New("boom")
	}))
	p.AddHandler(last)

	_ = p.Process(ctx)

	if last.called {
		t.Error("handler after failed step should not be called")
	}
}
