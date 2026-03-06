package utils

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestSafeGo(t *testing.T) {
	ctx := context.Background()
	executed := make(chan bool, 1)

	SafeGo(ctx, "test-task", func(ctx context.Context) {
		executed <- true
	})

	select {
	case <-executed:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("SafeGo did not execute")
	}
}

func TestSafeGoWithTimeout_Success(t *testing.T) {
	ctx := context.Background()

	err := SafeGoWithTimeout(ctx, "test-task", 2*time.Second, func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestSafeGoWithTimeout_Timeout(t *testing.T) {
	ctx := context.Background()

	err := SafeGoWithTimeout(ctx, "test-task", 100*time.Millisecond, func(ctx context.Context) error {
		time.Sleep(1 * time.Second)
		return nil
	})

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}

func TestGoroutinePool(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	pool := NewGoroutinePool(ctx, 3, logger)
	defer pool.Close()

	executed := make(chan int, 10)

	// 提交10个任务
	for i := 0; i < 10; i++ {
		taskID := i
		err := pool.Submit("test-task", func(ctx context.Context) error {
			time.Sleep(10 * time.Millisecond)
			executed <- taskID
			return nil
		})
		if err != nil {
			t.Fatalf("Failed to submit task: %v", err)
		}
	}

	// 等待所有任务完成
	err := pool.WaitWithTimeout(5 * time.Second)
	if err != nil {
		t.Fatalf("Wait timeout: %v", err)
	}

	close(executed)

	// 验证所有任务都执行了
	count := 0
	for range executed {
		count++
	}

	if count != 10 {
		t.Fatalf("Expected 10 tasks executed, got %d", count)
	}
}

func TestAsyncTask_Success(t *testing.T) {
	ctx := context.Background()

	task := NewAsyncTask(ctx, "test-task", func(ctx context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	err := task.Wait()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
}

func TestAsyncTask_Error(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("test error")

	task := NewAsyncTask(ctx, "test-task", func(ctx context.Context) error {
		return expectedErr
	})

	err := task.Wait()
	if err != expectedErr {
		t.Fatalf("Expected error %v, got: %v", expectedErr, err)
	}
}

func TestAsyncTask_Timeout(t *testing.T) {
	ctx := context.Background()

	task := NewAsyncTask(ctx, "test-task", func(ctx context.Context) error {
		time.Sleep(1 * time.Second)
		return nil
	})

	err := task.WaitWithTimeout(100 * time.Millisecond)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}
}

func TestAsyncTask_Cancel(t *testing.T) {
	ctx := context.Background()

	task := NewAsyncTask(ctx, "test-task", func(ctx context.Context) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
			return nil
		}
	})

	// 立即取消
	task.Cancel()

	err := task.Wait()
	if err == nil {
		t.Fatal("Expected context canceled error, got nil")
	}
}

func TestPeriodicTask(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	executed := make(chan int, 10)
	count := 0

	task := NewPeriodicTask(ctx, "test-task", 50*time.Millisecond, func(ctx context.Context) error {
		count++
		executed <- count
		return nil
	}, logger)

	task.Start()

	// 等待执行几次
	time.Sleep(250 * time.Millisecond)

	// 停止任务
	err := task.StopWithTimeout(1 * time.Second)
	if err != nil {
		t.Fatalf("Failed to stop task: %v", err)
	}

	close(executed)

	// 验证至少执行了3次（立即执行1次 + 至少2次周期执行）
	execCount := 0
	for range executed {
		execCount++
	}

	if execCount < 3 {
		t.Fatalf("Expected at least 3 executions, got %d", execCount)
	}
}
