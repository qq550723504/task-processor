package rabbitmq

import (
	"testing"
	"time"
)

func TestFixedDelayStrategy_NextDelay(t *testing.T) {
	strategy := &FixedDelayStrategy{
		Delay:      5 * time.Second,
		MaxRetries: 3,
	}

	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{"第1次重试", 0, 5 * time.Second},
		{"第2次重试", 1, 5 * time.Second},
		{"第3次重试", 2, 5 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strategy.NextDelay(tt.attempt)
			if got != tt.want {
				t.Errorf("NextDelay() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFixedDelayStrategy_ShouldRetry(t *testing.T) {
	strategy := &FixedDelayStrategy{
		Delay:      5 * time.Second,
		MaxRetries: 3,
	}

	tests := []struct {
		name    string
		attempt int
		want    bool
	}{
		{"第1次尝试", 0, true},
		{"第2次尝试", 1, true},
		{"第3次尝试", 2, true},
		{"超过最大次数", 3, false},
		{"远超最大次数", 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strategy.ShouldRetry(tt.attempt, nil)
			if got != tt.want {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExponentialBackoffStrategy_NextDelay(t *testing.T) {
	strategy := &ExponentialBackoffStrategy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		MaxRetries:   10,
	}

	tests := []struct {
		name    string
		attempt int
		want    time.Duration
	}{
		{"第1次重试", 0, 1 * time.Second},
		{"第2次重试", 1, 2 * time.Second},
		{"第3次重试", 2, 4 * time.Second},
		{"第4次重试", 3, 8 * time.Second},
		{"第5次重试", 4, 16 * time.Second},
		{"第6次重试（达到最大延迟）", 5, 30 * time.Second},
		{"第7次重试（保持最大延迟）", 6, 30 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strategy.NextDelay(tt.attempt)
			if got != tt.want {
				t.Errorf("NextDelay() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExponentialBackoffStrategy_ShouldRetry(t *testing.T) {
	strategy := &ExponentialBackoffStrategy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		MaxRetries:   5,
	}

	tests := []struct {
		name    string
		attempt int
		want    bool
	}{
		{"第1次尝试", 0, true},
		{"第3次尝试", 2, true},
		{"第5次尝试", 4, true},
		{"超过最大次数", 5, false},
		{"远超最大次数", 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strategy.ShouldRetry(tt.attempt, nil)
			if got != tt.want {
				t.Errorf("ShouldRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewDefaultRetryStrategy(t *testing.T) {
	maxRetries := 10
	strategy := NewDefaultRetryStrategy(maxRetries)

	// 验证返回的是指数退避策略
	expStrategy, ok := strategy.(*ExponentialBackoffStrategy)
	if !ok {
		t.Fatal("NewDefaultRetryStrategy() 应该返回 ExponentialBackoffStrategy")
	}

	// 验证默认参数
	if expStrategy.InitialDelay != 1*time.Second {
		t.Errorf("InitialDelay = %v, want %v", expStrategy.InitialDelay, 1*time.Second)
	}
	if expStrategy.MaxDelay != 30*time.Second {
		t.Errorf("MaxDelay = %v, want %v", expStrategy.MaxDelay, 30*time.Second)
	}
	if expStrategy.Multiplier != 2.0 {
		t.Errorf("Multiplier = %v, want %v", expStrategy.Multiplier, 2.0)
	}
	if expStrategy.MaxRetries != maxRetries {
		t.Errorf("MaxRetries = %v, want %v", expStrategy.MaxRetries, maxRetries)
	}
}

// 基准测试
func BenchmarkFixedDelayStrategy_NextDelay(b *testing.B) {
	strategy := &FixedDelayStrategy{
		Delay:      5 * time.Second,
		MaxRetries: 10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.NextDelay(i % 10)
	}
}

func BenchmarkExponentialBackoffStrategy_NextDelay(b *testing.B) {
	strategy := &ExponentialBackoffStrategy{
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		MaxRetries:   10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.NextDelay(i % 10)
	}
}
