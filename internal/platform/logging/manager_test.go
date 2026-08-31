package logging

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type probeRWLocker struct {
	delegate        sync.RWMutex
	lockAttempt     chan struct{}
	lockPermit      chan struct{}
	unlockAttempt   chan struct{}
	unlockPermit    chan struct{}
	readLockAttempt chan struct{}
	readLockPermit  chan struct{}
}

func newProbeRWLocker() *probeRWLocker {
	return &probeRWLocker{
		lockAttempt:     make(chan struct{}),
		lockPermit:      make(chan struct{}),
		unlockAttempt:   make(chan struct{}),
		unlockPermit:    make(chan struct{}),
		readLockAttempt: make(chan struct{}),
		readLockPermit:  make(chan struct{}),
	}
}

func (l *probeRWLocker) Lock() {
	l.lockAttempt <- struct{}{}
	<-l.lockPermit
	l.delegate.Lock()
}

func (l *probeRWLocker) Unlock() {
	l.unlockAttempt <- struct{}{}
	<-l.unlockPermit
	l.delegate.Unlock()
}

func (l *probeRWLocker) RLock() {
	l.readLockAttempt <- struct{}{}
	<-l.readLockPermit
	l.delegate.RLock()
}

func (l *probeRWLocker) RUnlock() {
	l.delegate.RUnlock()
}

func TestNewLogManager(t *testing.T) {
	config := &LogConfig{
		Level:   "info",
		Format:  "json",
		Console: true,
	}

	manager := NewLogManager(config)
	assert.NotNil(t, manager)
	assert.Equal(t, "info", manager.GetLevel())
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

func TestZeroValueLogManagerGetLevel(t *testing.T) {
	var manager LogManager

	assert.Equal(t, "panic", manager.GetLevel())
}

func TestLogManagerFallbackLockSupportsConcurrentSetAndGetLevel(t *testing.T) {
	manager := &LogManager{logger: logrus.New()}
	start := make(chan struct{})
	panicValues := make(chan any, 2)
	var workers sync.WaitGroup
	workers.Add(2)

	go func() {
		defer workers.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				panicValues <- recovered
			}
		}()
		<-start
		for i := 0; i < 100; i++ {
			_ = manager.SetLevel("debug")
		}
	}()

	go func() {
		defer workers.Done()
		defer func() {
			if recovered := recover(); recovered != nil {
				panicValues <- recovered
			}
		}()
		<-start
		for i := 0; i < 100; i++ {
			level := manager.GetLevel()
			if level != "panic" && level != "debug" {
				t.Errorf("concurrent GetLevel returned unexpected level %q", level)
				return
			}
		}
	}()

	close(start)
	workers.Wait()
	close(panicValues)
	for recovered := range panicValues {
		t.Errorf("fallback-lock level operation panicked: %v", recovered)
	}
	assert.Equal(t, "debug", manager.GetLevel())
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
	var factoryCalls atomic.Int32
	probe := newProbeRWLocker()
	registry := newLogManagerRegistry(func(*LogConfig) *LogManager {
		factoryCalls.Add(1)
		return NewLogManager(&LogConfig{Level: "info", Console: false})
	}, probe)

	firstReturned := make(chan *LogManager, 1)
	go func() { firstReturned <- registry.get() }()
	select {
	case <-probe.lockAttempt:
	case first := <-firstReturned:
		t.Fatalf("first lazy Get returned before reaching the lock boundary: %p", first)
	}
	probe.lockPermit <- struct{}{}
	select {
	case <-probe.unlockAttempt:
	case first := <-firstReturned:
		t.Fatalf("first lazy Get returned before reaching the unlock boundary: %p", first)
	}

	secondReturned := make(chan *LogManager, 1)
	go func() { secondReturned <- registry.get() }()
	select {
	case <-probe.lockAttempt:
	case second := <-secondReturned:
		t.Fatalf("second lazy Get returned before reaching the lock boundary: %p", second)
	}
	probe.lockPermit <- struct{}{}
	probe.unlockPermit <- struct{}{}

	first := <-firstReturned
	<-probe.unlockAttempt
	probe.unlockPermit <- struct{}{}
	second := <-secondReturned
	t.Cleanup(func() { _ = first.Close() })
	assert.Equal(t, int32(1), factoryCalls.Load())
	assert.Same(t, first, second)
}

func TestLogManagerRegistryGetWaitsForInitToPublishCompleteManager(t *testing.T) {
	factoryEntered := make(chan struct{})
	releaseInit := make(chan struct{})
	var factoryCalls atomic.Int32
	var initialized *LogManager
	probe := newProbeRWLocker()
	registry := newLogManagerRegistry(func(*LogConfig) *LogManager {
		call := factoryCalls.Add(1)
		if call == 1 {
			factoryEntered <- struct{}{}
			<-releaseInit
			initialized = NewLogManager(&LogConfig{Level: "error", Console: false})
			return initialized
		}
		return &LogManager{}
	}, probe)

	initReturned := make(chan struct{})
	go func() {
		registry.init(&LogConfig{Level: "error", Console: false})
		close(initReturned)
	}()
	select {
	case <-probe.lockAttempt:
	case <-initReturned:
		t.Fatal("Init returned before reaching the lock boundary")
	}
	probe.lockPermit <- struct{}{}
	<-factoryEntered

	getReturned := make(chan *LogManager, 1)
	go func() { getReturned <- registry.get() }()
	select {
	case <-probe.lockAttempt:
	case observed := <-getReturned:
		t.Fatalf("Get returned before reaching the lock boundary: %p", observed)
	}
	probe.lockPermit <- struct{}{}
	close(releaseInit)
	<-probe.unlockAttempt
	probe.unlockPermit <- struct{}{}
	<-initReturned
	t.Cleanup(func() { _ = initialized.Close() })

	<-probe.unlockAttempt
	probe.unlockPermit <- struct{}{}
	got := <-getReturned
	assert.Equal(t, int32(1), factoryCalls.Load())
	assert.Same(t, initialized, got)
	require.NotNil(t, got.GetRawLogger())
	assert.Equal(t, "error", got.GetLevel())
}

func TestLogManagerSetAndGetLevelWaitForManagerLock(t *testing.T) {
	probe := newProbeRWLocker()
	manager := newLogManager(&LogConfig{Level: "info", Console: false}, probe)
	t.Cleanup(func() { _ = manager.Close() })

	getReturned := make(chan string, 1)
	go func() { getReturned <- manager.GetLevel() }()
	select {
	case <-probe.readLockAttempt:
	case level := <-getReturned:
		t.Fatalf("GetLevel returned before reaching the read-lock boundary: %q", level)
	}
	probe.readLockPermit <- struct{}{}
	assert.Equal(t, "info", <-getReturned)

	setReturned := make(chan error, 1)
	go func() { setReturned <- manager.SetLevel("debug") }()
	select {
	case <-probe.lockAttempt:
	case err := <-setReturned:
		t.Fatalf("SetLevel returned before reaching the lock boundary: %v", err)
	}
	probe.lockPermit <- struct{}{}
	select {
	case <-probe.unlockAttempt:
	case err := <-setReturned:
		t.Fatalf("SetLevel returned before reaching the unlock boundary: %v", err)
	}
	probe.unlockPermit <- struct{}{}
	require.NoError(t, <-setReturned)

	finalLevel := make(chan string, 1)
	go func() { finalLevel <- manager.GetLevel() }()
	select {
	case <-probe.readLockAttempt:
	case level := <-finalLevel:
		t.Fatalf("final GetLevel returned before reaching the read-lock boundary: %q", level)
	}
	probe.readLockPermit <- struct{}{}
	assert.Equal(t, "debug", <-finalLevel)
}

func TestLazyLogManagerRegistryDoesNotCreateRuntimeFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	registry := newLogManagerRegistry(NewLogManager, &sync.RWMutex{})
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
