package rabbitmq

import (
	"testing"
	"time"
)

func TestNewSlidingWindowStats(t *testing.T) {
	tests := []struct {
		name string
		size int
		want int
	}{
		{"正常大小", 100, 100},
		{"零大小（使用默认值）", 0, 100},
		{"负数大小（使用默认值）", -1, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sws := NewSlidingWindowStats(tt.size)
			if sws.windowSize != tt.want {
				t.Errorf("windowSize = %v, want %v", sws.windowSize, tt.want)
			}
		})
	}
}

func TestSlidingWindowStats_Add(t *testing.T) {
	sws := NewSlidingWindowStats(5)

	// 添加数据
	durations := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		3 * time.Second,
	}

	for _, d := range durations {
		sws.Add(d)
	}

	// 验证数据点数量
	if sws.Count() != 3 {
		t.Errorf("Count() = %v, want %v", sws.Count(), 3)
	}
}

func TestSlidingWindowStats_Average(t *testing.T) {
	sws := NewSlidingWindowStats(5)

	// 添加数据
	sws.Add(1 * time.Second)
	sws.Add(2 * time.Second)
	sws.Add(3 * time.Second)

	// 平均值应该是 2 秒
	avg := sws.Average()
	if avg != 2*time.Second {
		t.Errorf("Average() = %v, want %v", avg, 2*time.Second)
	}
}

func TestSlidingWindowStats_Max(t *testing.T) {
	sws := NewSlidingWindowStats(5)

	sws.Add(1 * time.Second)
	sws.Add(5 * time.Second)
	sws.Add(3 * time.Second)

	max := sws.Max()
	if max != 5*time.Second {
		t.Errorf("Max() = %v, want %v", max, 5*time.Second)
	}
}

func TestSlidingWindowStats_Min(t *testing.T) {
	sws := NewSlidingWindowStats(5)

	sws.Add(3 * time.Second)
	sws.Add(1 * time.Second)
	sws.Add(5 * time.Second)

	min := sws.Min()
	if min != 1*time.Second {
		t.Errorf("Min() = %v, want %v", min, 1*time.Second)
	}
}

func TestSlidingWindowStats_Percentile(t *testing.T) {
	sws := NewSlidingWindowStats(10)

	// 添加 1-10 秒的数据
	for i := 1; i <= 10; i++ {
		sws.Add(time.Duration(i) * time.Second)
	}

	tests := []struct {
		name       string
		percentile float64
		want       time.Duration
	}{
		{"P50（中位数）", 50, 5 * time.Second},
		{"P95", 95, 9 * time.Second}, // int(9 * 0.95) = 8, 索引8是第9个元素
		{"P99", 99, 9 * time.Second}, // int(9 * 0.99) = 8, 索引8是第9个元素
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sws.Percentile(tt.percentile)
			if got != tt.want {
				t.Errorf("Percentile(%v) = %v, want %v", tt.percentile, got, tt.want)
			}
		})
	}
}

func TestSlidingWindowStats_SlidingWindow(t *testing.T) {
	// 测试滑动窗口行为
	sws := NewSlidingWindowStats(3)

	// 添加 4 个数据点，窗口大小为 3
	sws.Add(1 * time.Second)
	sws.Add(2 * time.Second)
	sws.Add(3 * time.Second)
	sws.Add(4 * time.Second) // 这个会覆盖第一个

	// 数量应该是 3（窗口大小）
	if sws.Count() != 3 {
		t.Errorf("Count() = %v, want %v", sws.Count(), 3)
	}

	// 平均值应该是 (2+3+4)/3 = 3
	avg := sws.Average()
	if avg != 3*time.Second {
		t.Errorf("Average() = %v, want %v", avg, 3*time.Second)
	}
}

func TestSlidingWindowStats_Reset(t *testing.T) {
	sws := NewSlidingWindowStats(5)

	// 添加数据
	sws.Add(1 * time.Second)
	sws.Add(2 * time.Second)
	sws.Add(3 * time.Second)

	// 重置
	sws.Reset()

	// 验证已重置
	if sws.Count() != 0 {
		t.Errorf("Count() after Reset() = %v, want %v", sws.Count(), 0)
	}

	if sws.Average() != 0 {
		t.Errorf("Average() after Reset() = %v, want %v", sws.Average(), 0)
	}
}

func TestSlidingWindowStats_GetStats(t *testing.T) {
	sws := NewSlidingWindowStats(10)

	// 添加数据
	for i := 1; i <= 10; i++ {
		sws.Add(time.Duration(i) * time.Second)
	}

	stats := sws.GetStats()

	// 验证统计信息
	if stats.Count != 10 {
		t.Errorf("Count = %v, want %v", stats.Count, 10)
	}

	if stats.Average != 5*time.Second+500*time.Millisecond {
		t.Errorf("Average = %v, want %v", stats.Average, 5*time.Second+500*time.Millisecond)
	}

	if stats.Max != 10*time.Second {
		t.Errorf("Max = %v, want %v", stats.Max, 10*time.Second)
	}

	if stats.Min != 1*time.Second {
		t.Errorf("Min = %v, want %v", stats.Min, 1*time.Second)
	}

	if stats.P50 != 5*time.Second {
		t.Errorf("P50 = %v, want %v", stats.P50, 5*time.Second)
	}
}

func TestSlidingWindowStats_EmptyWindow(t *testing.T) {
	sws := NewSlidingWindowStats(5)

	// 空窗口应该返回零值
	if sws.Count() != 0 {
		t.Errorf("Count() = %v, want %v", sws.Count(), 0)
	}

	if sws.Average() != 0 {
		t.Errorf("Average() = %v, want %v", sws.Average(), 0)
	}

	if sws.Max() != 0 {
		t.Errorf("Max() = %v, want %v", sws.Max(), 0)
	}

	if sws.Min() != 0 {
		t.Errorf("Min() = %v, want %v", sws.Min(), 0)
	}
}

// 并发测试
func TestSlidingWindowStats_Concurrent(t *testing.T) {
	sws := NewSlidingWindowStats(1000)

	// 并发添加数据
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				sws.Add(time.Duration(j) * time.Millisecond)
			}
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据点数量
	count := sws.Count()
	if count != 1000 {
		t.Errorf("Count() = %v, want %v", count, 1000)
	}
}

// 基准测试
func BenchmarkSlidingWindowStats_Add(b *testing.B) {
	sws := NewSlidingWindowStats(1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sws.Add(time.Duration(i) * time.Millisecond)
	}
}

func BenchmarkSlidingWindowStats_Average(b *testing.B) {
	sws := NewSlidingWindowStats(1000)
	for i := 0; i < 1000; i++ {
		sws.Add(time.Duration(i) * time.Millisecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sws.Average()
	}
}

func BenchmarkSlidingWindowStats_GetStats(b *testing.B) {
	sws := NewSlidingWindowStats(1000)
	for i := 0; i < 1000; i++ {
		sws.Add(time.Duration(i) * time.Millisecond)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sws.GetStats()
	}
}
