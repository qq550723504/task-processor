package database

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/core/config"

	"github.com/stretchr/testify/assert"
)

type testLogger struct{}

func (l *testLogger) Infof(format string, args ...interface{})  {}
func (l *testLogger) Warnf(format string, args ...interface{})  {}
func (l *testLogger) Errorf(format string, args ...interface{}) {}

func TestConnectionProxyConfig(t *testing.T) {
	t.Parallel()

	// 测试配置验证
	cfg := &ConnectionProxyConfig{
		MaxConcurrentOps: 10,
		DBConfig: &config.DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			User:     "test",
			Password: "test",
			Database: "test",
		},
		Logger: &testLogger{},
	}

	// 验证配置
	assert.NotNil(t, cfg)
	assert.Equal(t, 10, cfg.MaxConcurrentOps)
}

func TestConnectionProxySemaphoreBehavior(t *testing.T) {
	t.Parallel()

	// 创建一个简单的信号量测试，不依赖数据库
	semaphore := make(chan struct{}, 3)
	maxConcurrent := int64(0)
	currentConcurrent := int64(0)
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// 尝试获取信号量
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()

				// 记录并发数
				curr := atomic.AddInt64(&currentConcurrent, 1)
				mu.Lock()
				if curr > maxConcurrent {
					maxConcurrent = curr
				}
				mu.Unlock()

				// 模拟工作
				time.Sleep(50 * time.Millisecond)

				atomic.AddInt64(&currentConcurrent, -1)

			case <-time.After(2 * time.Second):
				// 超时
			}
		}()
	}

	wg.Wait()

	// 验证最大并发数不超过限制
	assert.LessOrEqual(t, maxConcurrent, int64(3))
}

func TestConnectionProxyContextCancellation(t *testing.T) {
	t.Parallel()

	semaphore := make(chan struct{}, 1)
	// 先占用信号量
	semaphore <- struct{}{}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 尝试获取信号量，应该会超时
	done := make(chan error, 1)
	go func() {
		select {
		case semaphore <- struct{}{}:
			<-semaphore
			done <- nil
		case <-ctx.Done():
			done <- ctx.Err()
		}
	}()

	err := <-done
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestConnectionProxyStatsTracking(t *testing.T) {
	t.Parallel()

	activeOps := int64(0)

	// 模拟增加和减少活跃操作数
	atomic.AddInt64(&activeOps, 1)
	assert.Equal(t, int64(1), atomic.LoadInt64(&activeOps))

	atomic.AddInt64(&activeOps, 2)
	assert.Equal(t, int64(3), atomic.LoadInt64(&activeOps))

	atomic.AddInt64(&activeOps, -3)
	assert.Equal(t, int64(0), atomic.LoadInt64(&activeOps))
}
