package productenrich

import (
	"context"
	"testing"
)

// validateRequest 是 productService 的私有方法，通过白盒测试直接调用
func TestProductService_ValidateRequest(t *testing.T) {
	s := &productService{}

	cases := []struct {
		name    string
		req     *GenerateRequest
		wantErr bool
	}{
		{"empty request", &GenerateRequest{}, true},
		{"too many images", &GenerateRequest{ImageURLs: make([]string, 11)}, true},
		{"text too long", &GenerateRequest{Text: string(make([]byte, 10001))}, true},
		{"valid text", &GenerateRequest{Text: "hello"}, false},
		{"valid image", &GenerateRequest{ImageURLs: []string{"https://example.com/img.jpg"}}, false},
		{"valid product url", &GenerateRequest{ProductURL: "https://1688.com/product/123"}, false},
		{"exactly 10 images", &GenerateRequest{ImageURLs: make([]string, 10)}, false},
		{"exactly 10000 chars", &GenerateRequest{Text: string(make([]byte, 10000))}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.validateRequest(tc.req)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestProductService_GenerateTaskID(t *testing.T) {
	s := &productService{}
	id1 := s.generateTaskID()
	id2 := s.generateTaskID()
	if id1 == "" {
		t.Error("task ID should not be empty")
	}
	if id1 == id2 {
		t.Error("task IDs should be unique")
	}
	// UUID 格式：8-4-4-4-12
	if len(id1) != 36 {
		t.Errorf("task ID length = %d, want 36 (UUID format)", len(id1))
	}
}

func TestProductService_GetTaskResult(t *testing.T) {
	ctx := context.Background()

	t.Run("empty task ID returns error", func(t *testing.T) {
		s := &productService{taskRepo: newMockTaskRepo()}
		_, err := s.GetTaskResult(ctx, "")
		if err == nil {
			t.Error("expected error for empty task ID")
		}
	})

	t.Run("task not found returns error", func(t *testing.T) {
		s := &productService{taskRepo: newMockTaskRepo()}
		_, err := s.GetTaskResult(ctx, "nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent task")
		}
	})

	t.Run("completed task has CompletedAt set", func(t *testing.T) {
		task := &Task{ID: "t1", Status: TaskStatusCompleted, Request: &GenerateRequest{}}
		s := &productService{taskRepo: newMockTaskRepo(task)}
		result, err := s.GetTaskResult(ctx, "t1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.CompletedAt == nil {
			t.Error("CompletedAt should be set for completed task")
		}
	})

	t.Run("pending task has no CompletedAt", func(t *testing.T) {
		task := &Task{ID: "t2", Status: TaskStatusPending, Request: &GenerateRequest{}}
		s := &productService{taskRepo: newMockTaskRepo(task)}
		result, err := s.GetTaskResult(ctx, "t2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.CompletedAt != nil {
			t.Error("CompletedAt should be nil for pending task")
		}
	})
}
