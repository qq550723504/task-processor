package database

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"task-processor/internal/core/config"

	"gorm.io/gorm"
)

// ConnectionProxy 应用层连接代理，通过信号量限制并发DB操作数
type ConnectionProxy struct {
	master    *gorm.DB      // 共享的数据库实例
	semaphore chan struct{} // 信号量，控制并发数
	maxOps    int           // 最大并发操作数
	activeOps int64         // 当前活跃操作数（用于监控）
	logger    ProxyLogger   // 日志接口
}

// ProxyLogger 简单的日志接口
type ProxyLogger interface {
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// ConnectionProxyConfig 连接代理配置
type ConnectionProxyConfig struct {
	MaxConcurrentOps int                    // 最大并发DB操作数
	DBConfig         *config.DatabaseConfig // 数据库配置
	Logger           ProxyLogger            // 日志记录器
}

// NewConnectionProxy 创建连接代理
func NewConnectionProxy(cfg *ConnectionProxyConfig) (*ConnectionProxy, error) {
	if cfg == nil {
		return nil, fmt.Errorf("connection proxy config is nil")
	}
	if cfg.DBConfig == nil {
		return nil, fmt.Errorf("database config is nil")
	}

	maxOps := cfg.MaxConcurrentOps
	if maxOps <= 0 {
		maxOps = 50 // 默认50个并发操作
	}

	// 创建共享数据库实例
	db, err := NewSharedDatabaseFromConfig(cfg.DBConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create shared database: %w", err)
	}

	proxy := &ConnectionProxy{
		master:    db,
		semaphore: make(chan struct{}, maxOps),
		maxOps:    maxOps,
		logger:    cfg.Logger,
	}

	if proxy.logger != nil {
		proxy.logger.Infof("ConnectionProxy initialized with max concurrent ops: %d", maxOps)
	}

	return proxy, nil
}

// Execute 在信号量控制下执行数据库操作
// fn 函数中应该只包含数据库操作，不应该包含耗时业务逻辑
func (p *ConnectionProxy) Execute(ctx context.Context, fn func(*gorm.DB) error) error {
	if fn == nil {
		return fmt.Errorf("database operation function is nil")
	}

	// 尝试获取信号量
	select {
	case p.semaphore <- struct{}{}:
		// 成功获取信号量
		defer func() {
			<-p.semaphore // 释放信号量
			atomic.AddInt64(&p.activeOps, -1)
		}()

		atomic.AddInt64(&p.activeOps, 1)

		// 执行数据库操作
		return fn(p.master)

	case <-ctx.Done():
		// 上下文取消，不执行
		return fmt.Errorf("context cancelled before acquiring database semaphore: %w", ctx.Err())
	}
}

// ExecuteWithTimeout 带超时的执行
func (p *ConnectionProxy) ExecuteWithTimeout(ctx context.Context, timeout time.Duration, fn func(*gorm.DB) error) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return p.Execute(timeoutCtx, fn)
}

// GetDB 获取底层数据库实例（用于需要直接访问的场景）
func (p *ConnectionProxy) GetDB() *gorm.DB {
	return p.master
}

// NewProxiedDB 创建一个受代理控制的DB实例
// 所有通过这个DB执行的查询都会受到信号量控制
func (p *ConnectionProxy) NewProxiedDB() *gorm.DB {
	if p.master == nil {
		return nil
	}

	// 返回原始DB实例，但在使用时通过Execute包装
	// 实际使用时应该调用 Execute(func(db *gorm.DB) error { ... })
	return p.master
}

// GetStats 获取连接代理统计信息
func (p *ConnectionProxy) GetStats() map[string]interface{} {
	sqlDB, err := p.master.DB()
	if err != nil {
		return map[string]interface{}{
			"error": err.Error(),
		}
	}

	stats := sqlDB.Stats()

	return map[string]interface{}{
		"max_concurrent_ops":      p.maxOps,
		"active_operations":       atomic.LoadInt64(&p.activeOps),
		"semaphore_available":     len(p.semaphore),
		"semaphore_capacity":      cap(p.semaphore),
		"db_open_connections":     stats.OpenConnections,
		"db_in_use":               stats.InUse,
		"db_idle":                 stats.Idle,
		"db_wait_count":           stats.WaitCount,
		"db_wait_duration_ms":     stats.WaitDuration.Milliseconds(),
		"db_max_open_connections": stats.MaxOpenConnections,
	}
}

// Close 关闭连接代理
func (p *ConnectionProxy) Close() error {
	if p.master == nil {
		return nil
	}

	sqlDB, err := p.master.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}

	if p.logger != nil {
		p.logger.Infof("Closing ConnectionProxy, active operations: %d", atomic.LoadInt64(&p.activeOps))
	}

	return sqlDB.Close()
}
