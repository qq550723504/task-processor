package logger

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoggerHelper_LogOperation(t *testing.T) {
	InitGlobalLogger(&LogConfig{
		Level:   "debug",
		Console: false,
	})

	logger := GetGlobalLogger("test")
	helper := NewLoggerHelper(logger)

	// 测试成功的操作
	err := helper.LogOperation("test_operation", func() error {
		time.Sleep(10 * time.Millisecond)
		return nil
	})
	assert.NoError(t, err)

	// 测试失败的操作
	expectedErr := errors.New("operation failed")
	err = helper.LogOperation("test_operation", func() error {
		return expectedErr
	})
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
}

func TestLoggerHelper_LogOperationWithResult(t *testing.T) {
	InitGlobalLogger(&LogConfig{
		Level:   "debug",
		Console: false,
	})

	logger := GetGlobalLogger("test")
	helper := NewLoggerHelper(logger)

	// 测试成功的操作
	result, err := helper.LogOperationWithResult("test_operation", func() (any, error) {
		return "success", nil
	})
	assert.NoError(t, err)
	assert.Equal(t, "success", result)

	// 测试失败的操作
	expectedErr := errors.New("operation failed")
	result, err = helper.LogOperationWithResult("test_operation", func() (any, error) {
		return nil, expectedErr
	})
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestLoggerHelper_LogProgress(t *testing.T) {
	InitGlobalLogger(&LogConfig{
		Level:   "info",
		Console: false,
	})

	logger := GetGlobalLogger("test")
	helper := NewLoggerHelper(logger)

	// 正常进度
	helper.LogProgress(50, 100, "处理中")

	// 边界情况：total为0
	helper.LogProgress(0, 0, "无数据")

	// 完成
	helper.LogProgress(100, 100, "完成")
}

func TestLoggerHelper_LogRetry(t *testing.T) {
	InitGlobalLogger(&LogConfig{
		Level:   "warn",
		Console: false,
	})

	logger := GetGlobalLogger("test")
	helper := NewLoggerHelper(logger)

	err := errors.New("temporary error")
	helper.LogRetry("fetch_data", 1, 3, err)
}

func TestLoggerHelper_LogTaskStart(t *testing.T) {
	InitGlobalLogger(&LogConfig{
		Level:   "info",
		Console: false,
	})

	logger := GetGlobalLogger("test")
	helper := NewLoggerHelper(logger)

	helper.LogTaskStart(12345, "PROD-001")
}

func TestLoggerHelper_LogTaskComplete(t *testing.T) {
	InitGlobalLogger(&LogConfig{
		Level:   "info",
		Console: false,
	})

	logger := GetGlobalLogger("test")
	helper := NewLoggerHelper(logger)

	helper.LogTaskComplete(12345, 1500*time.Millisecond)
}

func TestLoggerHelper_LogTaskFailed(t *testing.T) {
	InitGlobalLogger(&LogConfig{
		Level:   "error",
		Console: false,
	})

	logger := GetGlobalLogger("test")
	helper := NewLoggerHelper(logger)

	err := errors.New("task processing failed")
	helper.LogTaskFailed(12345, err)
}

func TestLoggerHelper_LogAPICall(t *testing.T) {
	InitGlobalLogger(&LogConfig{
		Level:   "info",
		Console: false,
	})

	logger := GetGlobalLogger("test")
	helper := NewLoggerHelper(logger)

	// 成功的API调用
	helper.LogAPICall("GET", "https://api.example.com/data", 200, 150*time.Millisecond)

	// 失败的API调用
	helper.LogAPICall("POST", "https://api.example.com/data", 500, 200*time.Millisecond)

	// 重定向
	helper.LogAPICall("GET", "https://api.example.com/data", 302, 50*time.Millisecond)
}

func TestLoggerHelper_LogCacheHit(t *testing.T) {
	InitGlobalLogger(&LogConfig{
		Level:   "debug",
		Console: false,
	})

	logger := GetGlobalLogger("test")
	helper := NewLoggerHelper(logger)

	helper.LogCacheHit("user:12345", true)
	helper.LogCacheHit("user:67890", false)
}

func TestLoggerHelper_LogStateChange(t *testing.T) {
	InitGlobalLogger(&LogConfig{
		Level:   "info",
		Console: false,
	})

	logger := GetGlobalLogger("test")
	helper := NewLoggerHelper(logger)

	helper.LogStateChange("task", 12345, "pending", "processing")
}

func TestLoggerHelper_LogMetric(t *testing.T) {
	InitGlobalLogger(&LogConfig{
		Level:   "info",
		Console: false,
	})

	logger := GetGlobalLogger("test")
	helper := NewLoggerHelper(logger)

	tags := map[string]string{
		"service": "api",
		"region":  "us-east-1",
	}
	helper.LogMetric("request_count", 1000, tags)
}

func TestWithComponent(t *testing.T) {
	InitGlobalLogger(nil)

	logger := WithComponent("test_component")
	assert.NotNil(t, logger)
	assert.Equal(t, "test_component", logger.Data[FieldComponent])
}

func TestWithPlatform(t *testing.T) {
	InitGlobalLogger(nil)

	logger := WithPlatform("temu")
	assert.NotNil(t, logger)
	assert.Equal(t, "temu", logger.Data[FieldPlatform])
}

func TestWithTaskContext(t *testing.T) {
	InitGlobalLogger(nil)

	logger := WithTaskContext(12345, "PROD-001")
	assert.NotNil(t, logger)
	assert.Equal(t, int64(12345), logger.Data[FieldTaskID])
	assert.Equal(t, "PROD-001", logger.Data[FieldProductID])
}

func TestWithStoreContext(t *testing.T) {
	InitGlobalLogger(nil)

	logger := WithStoreContext(100, 200)
	assert.NotNil(t, logger)
	assert.Equal(t, int64(100), logger.Data[FieldTenantID])
	assert.Equal(t, int64(200), logger.Data[FieldStoreID])
}

func TestShouldLog(t *testing.T) {
	// 0% 采样率
	assert.False(t, ShouldLog(0.0))

	// 100% 采样率
	assert.True(t, ShouldLog(1.0))
	assert.True(t, ShouldLog(2.0)) // 超过1.0也应该返回true

	// 50% 采样率（多次测试）
	// 注意：由于使用时间戳作为随机源，结果可能不够均匀
	// 这里只做基本的功能测试
	count := 0
	iterations := 10000
	for i := 0; i < iterations; i++ {
		if ShouldLog(0.5) {
			count++
		}
		time.Sleep(1 * time.Nanosecond) // 确保时间戳变化
	}
	// 宽松的范围检查（30%-70%），因为简单的时间戳采样不够随机
	assert.Greater(t, count, iterations*30/100, "采样率过低")
	assert.Less(t, count, iterations*70/100, "采样率过高")
}

func BenchmarkLoggerHelper_LogOperation(b *testing.B) {
	InitGlobalLogger(&LogConfig{
		Level:   "info",
		Console: false,
	})

	logger := GetGlobalLogger("benchmark")
	helper := NewLoggerHelper(logger)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = helper.LogOperation("test_op", func() error {
			return nil
		})
	}
}

func BenchmarkWithTaskContext(b *testing.B) {
	InitGlobalLogger(&LogConfig{
		Level:   "info",
		Console: false,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WithTaskContext(12345, "PROD-001")
	}
}
