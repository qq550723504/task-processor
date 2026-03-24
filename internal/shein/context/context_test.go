package context_test

import (
	"context"
	"errors"
	"testing"

	sheinctx "task-processor/internal/shein/context"
)

func newCtx() *sheinctx.TaskContext {
	return sheinctx.NewTaskContext(context.Background(), nil)
}

// TestNewTaskContext 验证初始化状态
func TestNewTaskContext(t *testing.T) {
	ctx := newCtx()

	if ctx == nil {
		t.Fatal("NewTaskContext returned nil")
	}
	if ctx.VariantFilterMap == nil {
		t.Error("VariantFilterMap should be initialized")
	}
	if ctx.AsinSkuMap == nil {
		t.Error("AsinSkuMap should be initialized")
	}
	if ctx.SupplierSkuMap == nil {
		t.Error("SupplierSkuMap should be initialized")
	}
	if ctx.TaskState.ProcessedSensitiveWords == nil {
		t.Error("ProcessedSensitiveWords should be initialized")
	}
}

// TestTaskContext_VariantFilter 验证变体过滤状态的设置和读取
func TestTaskContext_VariantFilter(t *testing.T) {
	tests := []struct {
		name        string
		asin        string
		filteredOut bool
		reason      string
	}{
		{"过滤掉的变体", "B001", true, "价格过低"},
		{"未过滤的变体", "B002", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newCtx()
			ctx.SetVariantFiltered(tt.asin, tt.filteredOut, tt.reason)

			info := ctx.GetVariantFilterInfo(tt.asin)
			if info == nil {
				t.Fatal("GetVariantFilterInfo returned nil")
			}
			if info.FilteredOut != tt.filteredOut {
				t.Errorf("FilteredOut: want %v, got %v", tt.filteredOut, info.FilteredOut)
			}
			if info.FilterReason != tt.reason {
				t.Errorf("FilterReason: want %q, got %q", tt.reason, info.FilterReason)
			}

			if ctx.IsVariantFiltered(tt.asin) != tt.filteredOut {
				t.Errorf("IsVariantFiltered: want %v, got %v", tt.filteredOut, ctx.IsVariantFiltered(tt.asin))
			}
		})
	}
}

// TestTaskContext_IsVariantFiltered_NotFound 未设置的 ASIN 返回 false
func TestTaskContext_IsVariantFiltered_NotFound(t *testing.T) {
	ctx := newCtx()
	if ctx.IsVariantFiltered("UNKNOWN") {
		t.Error("unknown ASIN should not be filtered")
	}
	if ctx.GetVariantFilterInfo("UNKNOWN") != nil {
		t.Error("GetVariantFilterInfo for unknown ASIN should return nil")
	}
}

// TestTaskContext_SetError_GetError 验证错误的设置和读取
func TestTaskContext_SetError_GetError(t *testing.T) {
	ctx := newCtx()

	if ctx.GetError() != nil {
		t.Error("initial error should be nil")
	}

	expected := errors.New("something went wrong")
	ctx.SetError(expected)

	if !errors.Is(ctx.GetError(), expected) {
		t.Errorf("GetError() = %v, want %v", ctx.GetError(), expected)
	}
}

// TestTaskContext_SetData_GetData 验证 SetData/GetData 的 key 路由
func TestTaskContext_SetData_GetData(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value any
		check func(t *testing.T, ctx *sheinctx.TaskContext)
	}{
		{
			name:  "init_error key 存储错误",
			key:   "init_error",
			value: errors.New("init failed"),
			check: func(t *testing.T, ctx *sheinctx.TaskContext) {
				v, ok := ctx.GetData("init_error")
				if !ok {
					t.Error("GetData(init_error) should return ok=true")
				}
				if v == nil {
					t.Error("GetData(init_error) should return non-nil value")
				}
			},
		},
		{
			name:  "completed key 存储布尔值",
			key:   "completed",
			value: true,
			check: func(t *testing.T, ctx *sheinctx.TaskContext) {
				v, ok := ctx.GetData("completed")
				if !ok {
					t.Error("GetData(completed) should return ok=true when true")
				}
				if b, _ := v.(bool); !b {
					t.Error("GetData(completed) should return true")
				}
			},
		},
		{
			name:  "未知 key 返回 false",
			key:   "unknown_key",
			value: "ignored",
			check: func(t *testing.T, ctx *sheinctx.TaskContext) {
				_, ok := ctx.GetData("unknown_key")
				if ok {
					t.Error("GetData(unknown_key) should return ok=false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := newCtx()
			ctx.SetData(tt.key, tt.value)
			tt.check(t, ctx)
		})
	}
}

// TestTaskContext_SetCompleted 验证 SetCompleted/IsCompleted
func TestTaskContext_SetCompleted(t *testing.T) {
	ctx := newCtx()

	// IsCompleted 始终返回 false（当前实现）
	if ctx.IsCompleted() {
		t.Error("IsCompleted should return false initially")
	}

	ctx.SetCompleted(true)
	if !ctx.SkipSheinPipeline {
		t.Error("SetCompleted(true) should set SkipSheinPipeline=true")
	}
}

// TestTaskContext_GetContext 验证 GetContext 返回传入的 context
func TestTaskContext_GetContext(t *testing.T) {
	type ctxKey struct{}
	base := context.WithValue(context.Background(), ctxKey{}, "marker")
	ctx := sheinctx.NewTaskContext(base, nil)

	got := ctx.GetContext()
	if got.Value(ctxKey{}) != "marker" {
		t.Error("GetContext should return the original context")
	}
}
