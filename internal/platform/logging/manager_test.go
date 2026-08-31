package logging

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogManager(t *testing.T) {
	config := &LogConfig{
		Level:   "info",
		Format:  "json",
		Console: true,
	}

	manager := NewLogManager(config)
	assert.NotNil(t, manager)
	assert.Equal(t, logrus.InfoLevel, manager.level)
}

func TestLogManagerWithFile(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	config := &LogConfig{
		Level:      "debug",
		Format:     "text",
		OutputFile: logFile,
		MaxSize:    1, // 1MB
		Console:    false,
	}

	manager := NewLogManager(config)
	require.NotNil(t, manager)
	defer manager.Close()

	// 写入日志
	logger := manager.GetLogger("test")
	logger.Info("test message")

	// 等待写入完成
	time.Sleep(100 * time.Millisecond)

	// 验证文件存在
	_, err := os.Stat(logFile)
	assert.NoError(t, err)
}

func TestDefaultLogManagerDoesNotCreateRuntimeFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	manager := NewLogManager(nil)
	t.Cleanup(func() { _ = manager.Close() })

	manager.GetLogger("default-no-file").Info("stdout only")
	if _, err := os.Stat("tmp"); !os.IsNotExist(err) {
		t.Fatalf("default logger created a runtime directory: %v", err)
	}
}

func TestLogManagerSetLevel(t *testing.T) {
	manager := NewLogManager(nil)
	defer manager.Close()

	// 初始级别应该是info
	assert.Equal(t, "info", manager.GetLevel())

	// 修改级别
	err := manager.SetLevel("debug")
	assert.NoError(t, err)
	assert.Equal(t, "debug", manager.GetLevel())

	// 修改为error
	err = manager.SetLevel("error")
	assert.NoError(t, err)
	assert.Equal(t, "error", manager.GetLevel())
}

func TestGetGlobalLogger(t *testing.T) {
	// 初始化全局logger
	InitGlobalLogger(&LogConfig{
		Level:   "info",
		Format:  "json",
		Console: false,
	})

	logger := GetGlobalLogger("test_component")
	assert.NotNil(t, logger)

	// 验证字段
	data := logger.Data
	assert.Equal(t, "test_component", data["component"])
}

func TestLogManagerRegistryConcurrentLazyInitializationConstructsOnce(t *testing.T) {
	factoryEntered := make(chan struct{}, 2)
	releaseFactory := make(chan struct{})
	var factoryCalls atomic.Int32
	registry := newLogManagerRegistry(func(*LogConfig) *LogManager {
		factoryCalls.Add(1)
		factoryEntered <- struct{}{}
		<-releaseFactory
		return NewLogManager(&LogConfig{Level: "info", Console: false})
	})

	firstReturned := make(chan *LogManager, 1)
	go func() { firstReturned <- registry.get() }()
	<-factoryEntered

	secondLaunched := make(chan struct{})
	secondReturned := make(chan *LogManager, 1)
	go func() {
		close(secondLaunched)
		secondReturned <- registry.get()
	}()
	<-secondLaunched

	duplicateConstruction := false
	select {
	case <-factoryEntered:
		duplicateConstruction = true
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseFactory)

	first := <-firstReturned
	second := <-secondReturned
	t.Cleanup(func() { _ = first.Close() })
	if duplicateConstruction {
		t.Fatal("a second lazy caller entered the manager factory before the first construction completed")
	}
	assert.Equal(t, int32(1), factoryCalls.Load())
	assert.Same(t, first, second)
}

func TestLogManagerRegistryGetWaitsForInitToPublishCompleteManager(t *testing.T) {
	factoryEntered := make(chan int32, 2)
	releaseInit := make(chan struct{})
	var factoryCalls atomic.Int32
	var initialized *LogManager
	registry := newLogManagerRegistry(func(*LogConfig) *LogManager {
		call := factoryCalls.Add(1)
		factoryEntered <- call
		if call == 1 {
			<-releaseInit
			initialized = NewLogManager(&LogConfig{Level: "error", Console: false})
			return initialized
		}
		return &LogManager{}
	})

	initReturned := make(chan struct{})
	go func() {
		registry.init(&LogConfig{Level: "error", Console: false})
		close(initReturned)
	}()
	require.Equal(t, int32(1), <-factoryEntered)

	getLaunched := make(chan struct{})
	getReturned := make(chan *LogManager, 1)
	go func() {
		close(getLaunched)
		getReturned <- registry.get()
	}()
	<-getLaunched

	var observedBeforePublish *LogManager
	select {
	case observedBeforePublish = <-getReturned:
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseInit)
	<-initReturned
	t.Cleanup(func() { _ = initialized.Close() })

	if observedBeforePublish != nil {
		t.Fatal("Get returned a manager before Init completed construction and publication")
	}
	got := <-getReturned
	assert.Equal(t, int32(1), factoryCalls.Load())
	assert.Same(t, initialized, got)
	require.NotNil(t, got.GetRawLogger())
	assert.Equal(t, "error", got.GetLevel())
}

func TestLogManagerSetAndGetLevelWaitForManagerLock(t *testing.T) {
	manager := NewLogManager(&LogConfig{Level: "info", Console: false})
	t.Cleanup(func() { _ = manager.Close() })

	manager.mutex.Lock()
	getLaunched := make(chan struct{})
	getReturned := make(chan string, 1)
	go func() {
		close(getLaunched)
		getReturned <- manager.GetLevel()
	}()
	<-getLaunched

	var levelBeforeUnlock string
	getCompletedBeforeUnlock := false
	select {
	case levelBeforeUnlock = <-getReturned:
		getCompletedBeforeUnlock = true
	case <-time.After(100 * time.Millisecond):
	}
	manager.mutex.Unlock()
	if getCompletedBeforeUnlock {
		t.Errorf("GetLevel returned while the manager lock was held: %q", levelBeforeUnlock)
	} else {
		assert.Equal(t, "info", <-getReturned)
	}

	manager.mutex.Lock()
	setLaunched := make(chan struct{})
	setReturned := make(chan error, 1)
	go func() {
		close(setLaunched)
		setReturned <- manager.SetLevel("debug")
	}()
	<-setLaunched

	var setBeforeUnlock error
	setCompletedBeforeUnlock := false
	select {
	case setBeforeUnlock = <-setReturned:
		setCompletedBeforeUnlock = true
	case <-time.After(100 * time.Millisecond):
	}
	manager.mutex.Unlock()
	if setCompletedBeforeUnlock {
		t.Fatalf("SetLevel returned while the manager lock was held: %v", setBeforeUnlock)
	}
	require.NoError(t, <-setReturned)
	assert.Equal(t, "debug", manager.GetLevel())
}

func TestLazyLogManagerRegistryDoesNotCreateRuntimeFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	registry := newLogManagerRegistry(NewLogManager)
	t.Cleanup(func() { _ = registry.get().Close() })

	registry.get().GetLogger("lazy-no-file").Info("stdout only")
	if _, err := os.Stat("tmp"); !os.IsNotExist(err) {
		t.Fatalf("lazy global logger created a runtime directory: %v", err)
	}
}

func TestLogManagerWithFields(t *testing.T) {
	manager := NewLogManager(&LogConfig{
		Level:   "debug",
		Console: false,
	})
	defer manager.Close()

	logger := manager.GetLoggerWithFields(logrus.Fields{
		"service": "test",
		"version": "1.0",
	})

	assert.NotNil(t, logger)
	assert.Equal(t, "test", logger.Data["service"])
	assert.Equal(t, "1.0", logger.Data["version"])
}

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected logrus.Level
	}{
		{"debug", logrus.DebugLevel},
		{"info", logrus.InfoLevel},
		{"warn", logrus.WarnLevel},
		{"warning", logrus.WarnLevel},
		{"error", logrus.ErrorLevel},
		{"fatal", logrus.FatalLevel},
		{"panic", logrus.PanicLevel},
		{"unknown", logrus.InfoLevel}, // 默认
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level := parseLogLevel(tt.input)
			assert.Equal(t, tt.expected, level)
		})
	}
}

func TestCreateFormatter(t *testing.T) {
	tests := []struct {
		format       string
		reportCaller bool
	}{
		{"json", false},
		{"json", true},
		{"text", false},
		{"text", true},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			formatter := createFormatter(tt.format, tt.reportCaller)
			assert.NotNil(t, formatter)
		})
	}
}

func TestDefaultLogConfig(t *testing.T) {
	config := DefaultLogConfig()
	assert.NotNil(t, config)
	assert.Equal(t, "info", config.Level)
	assert.Equal(t, "json", config.Format)
	assert.Empty(t, config.OutputFile)
	assert.Equal(t, 100, config.MaxSize)
	assert.Equal(t, 10, config.MaxBackups)
	assert.Equal(t, 30, config.MaxAge)
	assert.True(t, config.Compress)
	assert.True(t, config.Console)
}

func TestSetGlobalLogLevel(t *testing.T) {
	InitGlobalLogger(nil)

	err := SetGlobalLogLevel("debug")
	assert.NoError(t, err)

	// 验证级别已更改
	logger := GetGlobalLogger("test")
	assert.Equal(t, logrus.DebugLevel, logger.Logger.Level)
}

func BenchmarkGetGlobalLogger(b *testing.B) {
	InitGlobalLogger(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GetGlobalLogger("benchmark")
	}
}

func BenchmarkLogWithFields(b *testing.B) {
	InitGlobalLogger(&LogConfig{
		Level:   "info",
		Console: false,
	})

	logger := GetGlobalLogger("benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.WithFields(logrus.Fields{
			"task_id":    12345,
			"product_id": "ABC123",
			"iteration":  i,
		}).Info("benchmark message")
	}
}
