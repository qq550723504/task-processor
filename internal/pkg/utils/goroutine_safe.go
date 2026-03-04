// Package utils 提供安全的 goroutine 管理工具
package utils

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// SafeGo 安全地启动一个 goroutine，带有 panic 恢复和日志记录
func SafeGo(ctx context.Context, name string, fn func(ctx context.Context)) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("[SafeGo] Goroutine %s panic: %v\n%s", name, r, debug.Stack())
			}
		}()

		fn(ctx)
	}()
}

// SafeGoWithTimeout 安全地启动一个带超时的 goroutine
func SafeGoWithTimeout(ctx context.Context, name string, timeout time.Duration, fn func(ctx context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("[SafeGoWithTimeout] Goroutine %s panic: %v\n%s", name, r, debug.Stack())
				errChan <- fmt.Errorf("panic: %v", r)
			}
		}()

		errChan <- fn(ctx)
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		return fmt.Errorf("goroutine %s timeout after %v", name, timeout)
	}
}

// GoroutinePool 安全的 goroutine 池
type GoroutinePool struct {
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	semaphore  chan struct{}
	maxWorkers int
	logger     *logrus.Logger
}

// NewGoroutinePool 创建新的 goroutine 池
func NewGoroutinePool(ctx context.Context, maxWorkers int, logger *logrus.Logger) *GoroutinePool {
	poolCtx, cancel := context.WithCancel(ctx)

	return &GoroutinePool{
		ctx:        poolCtx,
		cancel:     cancel,
		semaphore:  make(chan struct{}, maxWorkers),
		maxWorkers: maxWorkers,
		logger:     logger,
	}
}

// Submit 提交任务到池中
func (p *GoroutinePool) Submit(name string, fn func(ctx context.Context) error) error {
	select {
	case <-p.ctx.Done():
		return fmt.Errorf("goroutine pool is closed")
	case p.semaphore <- struct{}{}:
		// 获取到信号量，可以执行
	}

	p.wg.Add(1)
	go func() {
		defer func() {
			<-p.semaphore // 释放信号量
			p.wg.Done()

			if r := recover(); r != nil {
				p.logger.Errorf("[GoroutinePool] Task %s panic: %v\n%s", name, r, debug.Stack())
			}
		}()

		if err := fn(p.ctx); err != nil {
			p.logger.Errorf("[GoroutinePool] Task %s error: %v", name, err)
		}
	}()

	return nil
}

// Wait 等待所有任务完成
func (p *GoroutinePool) Wait() {
	p.wg.Wait()
}

// WaitWithTimeout 等待所有任务完成，带超时
func (p *GoroutinePool) WaitWithTimeout(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("wait timeout after %v", timeout)
	}
}

// Close 关闭 goroutine 池
func (p *GoroutinePool) Close() {
	p.cancel()
	p.Wait()
}

// CloseWithTimeout 关闭 goroutine 池，带超时
func (p *GoroutinePool) CloseWithTimeout(timeout time.Duration) error {
	p.cancel()
	return p.WaitWithTimeout(timeout)
}

// AsyncTask 异步任务包装器
type AsyncTask struct {
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	err    error
	mu     sync.Mutex
}

// NewAsyncTask 创建新的异步任务
func NewAsyncTask(ctx context.Context, name string, fn func(ctx context.Context) error) *AsyncTask {
	taskCtx, cancel := context.WithCancel(ctx)

	task := &AsyncTask{
		ctx:    taskCtx,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("[AsyncTask] Task %s panic: %v\n%s", name, r, debug.Stack())
				task.mu.Lock()
				task.err = fmt.Errorf("panic: %v", r)
				task.mu.Unlock()
			}
			close(task.done)
		}()

		err := fn(taskCtx)
		task.mu.Lock()
		task.err = err
		task.mu.Unlock()
	}()

	return task
}

// Wait 等待任务完成
func (t *AsyncTask) Wait() error {
	<-t.done
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

// WaitWithTimeout 等待任务完成，带超时
func (t *AsyncTask) WaitWithTimeout(timeout time.Duration) error {
	select {
	case <-t.done:
		t.mu.Lock()
		defer t.mu.Unlock()
		return t.err
	case <-time.After(timeout):
		t.Cancel()
		return fmt.Errorf("task timeout after %v", timeout)
	}
}

// Cancel 取消任务
func (t *AsyncTask) Cancel() {
	t.cancel()
}

// IsDone 检查任务是否完成
func (t *AsyncTask) IsDone() bool {
	select {
	case <-t.done:
		return true
	default:
		return false
	}
}

// PeriodicTask 周期性任务
type PeriodicTask struct {
	ctx      context.Context
	cancel   context.CancelFunc
	interval time.Duration
	name     string
	fn       func(ctx context.Context) error
	logger   *logrus.Logger
	wg       sync.WaitGroup
}

// NewPeriodicTask 创建新的周期性任务
func NewPeriodicTask(ctx context.Context, name string, interval time.Duration, fn func(ctx context.Context) error, logger *logrus.Logger) *PeriodicTask {
	taskCtx, cancel := context.WithCancel(ctx)

	return &PeriodicTask{
		ctx:      taskCtx,
		cancel:   cancel,
		interval: interval,
		name:     name,
		fn:       fn,
		logger:   logger,
	}
}

// Start 启动周期性任务
func (t *PeriodicTask) Start() {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				t.logger.Errorf("[PeriodicTask] Task %s panic: %v\n%s", t.name, r, debug.Stack())
			}
		}()

		ticker := time.NewTicker(t.interval)
		defer ticker.Stop()

		// 立即执行一次
		if err := t.fn(t.ctx); err != nil {
			t.logger.Errorf("[PeriodicTask] Task %s error: %v", t.name, err)
		}

		for {
			select {
			case <-t.ctx.Done():
				t.logger.Infof("[PeriodicTask] Task %s stopped", t.name)
				return
			case <-ticker.C:
				if err := t.fn(t.ctx); err != nil {
					t.logger.Errorf("[PeriodicTask] Task %s error: %v", t.name, err)
				}
			}
		}
	}()

	t.logger.Infof("[PeriodicTask] Task %s started with interval %v", t.name, t.interval)
}

// Stop 停止周期性任务
func (t *PeriodicTask) Stop() {
	t.cancel()
	t.wg.Wait()
	t.logger.Infof("[PeriodicTask] Task %s stopped", t.name)
}

// StopWithTimeout 停止周期性任务，带超时
func (t *PeriodicTask) StopWithTimeout(timeout time.Duration) error {
	t.cancel()

	done := make(chan struct{})
	go func() {
		t.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.logger.Infof("[PeriodicTask] Task %s stopped", t.name)
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("stop timeout after %v", timeout)
	}
}
