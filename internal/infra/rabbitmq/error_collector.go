// Package rabbitmq 提供RabbitMQ错误收集和报告功能
package rabbitmq

import (
	"fmt"
	"sync"
	"time"
)

// ErrorType 错误类型
type ErrorType int

const (
	ErrorTypeConnection ErrorType = iota
	ErrorTypeConsumer
	ErrorTypeMessage
	ErrorTypePanic
)

func (et ErrorType) String() string {
	switch et {
	case ErrorTypeConnection:
		return "connection"
	case ErrorTypeConsumer:
		return "consumer"
	case ErrorTypeMessage:
		return "message"
	case ErrorTypePanic:
		return "panic"
	default:
		return "unknown"
	}
}

type ErrorRecord struct {
	Type      ErrorType
	QueueName string
	MessageID string
	Error     error
	Timestamp time.Time
	Context   string
}

type ErrorCollector struct {
	errors    []ErrorRecord
	maxSize   int
	mutex     sync.RWMutex
	listeners []ErrorListener
}

type ErrorListener func(record ErrorRecord)

type ErrorStats struct {
	Total     int
	ByType    map[string]int
	ByQueue   map[string]int
	RecentErr *ErrorRecord
}

func NewErrorCollector(maxSize int) *ErrorCollector {
	if maxSize <= 0 {
		maxSize = 1000
	}
	return &ErrorCollector{
		errors:    make([]ErrorRecord, 0, maxSize),
		maxSize:   maxSize,
		listeners: make([]ErrorListener, 0),
	}
}

func (ec *ErrorCollector) Collect(errorType ErrorType, queueName, messageID string, err error, context string) {
	if err == nil {
		return
	}
	record := ErrorRecord{
		Type:      errorType,
		QueueName: queueName,
		MessageID: messageID,
		Error:     err,
		Timestamp: time.Now(),
		Context:   context,
	}
	ec.mutex.Lock()
	if len(ec.errors) >= ec.maxSize {
		ec.errors = ec.errors[1:]
	}
	ec.errors = append(ec.errors, record)
	ec.mutex.Unlock()
	ec.notifyListeners(record)
}

func (ec *ErrorCollector) GetErrors() []ErrorRecord {
	ec.mutex.RLock()
	defer ec.mutex.RUnlock()
	errors := make([]ErrorRecord, len(ec.errors))
	copy(errors, ec.errors)
	return errors
}

func (ec *ErrorCollector) GetRecentErrors(n int) []ErrorRecord {
	ec.mutex.RLock()
	defer ec.mutex.RUnlock()
	if n <= 0 || n > len(ec.errors) {
		n = len(ec.errors)
	}
	start := len(ec.errors) - n
	errors := make([]ErrorRecord, n)
	copy(errors, ec.errors[start:])
	return errors
}

func (ec *ErrorCollector) GetErrorsByType(errorType ErrorType) []ErrorRecord {
	ec.mutex.RLock()
	defer ec.mutex.RUnlock()
	filtered := make([]ErrorRecord, 0)
	for _, record := range ec.errors {
		if record.Type == errorType {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func (ec *ErrorCollector) GetErrorsByQueue(queueName string) []ErrorRecord {
	ec.mutex.RLock()
	defer ec.mutex.RUnlock()
	filtered := make([]ErrorRecord, 0)
	for _, record := range ec.errors {
		if record.QueueName == queueName {
			filtered = append(filtered, record)
		}
	}
	return filtered
}

func (ec *ErrorCollector) GetErrorStats() ErrorStats {
	ec.mutex.RLock()
	defer ec.mutex.RUnlock()
	stats := ErrorStats{
		Total:     len(ec.errors),
		ByType:    make(map[string]int),
		ByQueue:   make(map[string]int),
		RecentErr: nil,
	}
	for _, record := range ec.errors {
		stats.ByType[record.Type.String()]++
		if record.QueueName != "" {
			stats.ByQueue[record.QueueName]++
		}
	}
	if len(ec.errors) > 0 {
		lastErr := ec.errors[len(ec.errors)-1]
		stats.RecentErr = &lastErr
	}
	return stats
}

func (ec *ErrorCollector) Clear() {
	ec.mutex.Lock()
	defer ec.mutex.Unlock()
	ec.errors = make([]ErrorRecord, 0, ec.maxSize)
}

func (ec *ErrorCollector) AddListener(listener ErrorListener) {
	ec.mutex.Lock()
	defer ec.mutex.Unlock()
	ec.listeners = append(ec.listeners, listener)
}

func (ec *ErrorCollector) notifyListeners(record ErrorRecord) {
	ec.mutex.RLock()
	listeners := make([]ErrorListener, len(ec.listeners))
	copy(listeners, ec.listeners)
	ec.mutex.RUnlock()
	for _, listener := range listeners {
		go listener(record)
	}
}

func (ec *ErrorCollector) GenerateReport() string {
	stats := ec.GetErrorStats()
	recentErrors := ec.GetRecentErrors(10)
	report := fmt.Sprintf("=== 错误报告 ===\n总错误数: %d\n\n", stats.Total)
	report += "按类型统计:\n"
	for errType, count := range stats.ByType {
		report += fmt.Sprintf("  %s: %d\n", errType, count)
	}
	report += "\n按队列统计:\n"
	for queue, count := range stats.ByQueue {
		report += fmt.Sprintf("  %s: %d\n", queue, count)
	}
	if stats.RecentErr != nil {
		report += fmt.Sprintf("\n最近错误:\n  类型: %s\n  队列: %s\n  时间: %s\n  错误: %v\n",
			stats.RecentErr.Type.String(), stats.RecentErr.QueueName,
			stats.RecentErr.Timestamp.Format(time.RFC3339), stats.RecentErr.Error)
	}
	report += "\n最近10个错误:\n"
	for i, record := range recentErrors {
		report += fmt.Sprintf("%d. [%s] %s - %s: %v\n", i+1,
			record.Timestamp.Format("15:04:05"), record.Type.String(),
			record.QueueName, record.Error)
	}
	return report
}
