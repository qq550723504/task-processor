// Package monitoring 提供指标操作功能
package monitoring

import (
	"time"
)

// SetCounter 设置计数器指标
func (m *MetricsCollector) SetCounter(name string, value float64, labels map[string]string, description string) {
	m.setMetric(name, MetricTypeCounter, value, labels, description)
}

// SetGauge 设置仪表盘指标
func (m *MetricsCollector) SetGauge(name string, value float64, labels map[string]string, description string) {
	m.setMetric(name, MetricTypeGauge, value, labels, description)
}

// IncrementCounter 增加计数器
func (m *MetricsCollector) IncrementCounter(name string, labels map[string]string, description string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getMetricKey(name, labels)
	if metric, exists := m.metrics[key]; exists {
		metric.Value++
		metric.Timestamp = time.Now()
	} else {
		m.metrics[key] = &Metric{
			Name:        name,
			Type:        MetricTypeCounter,
			Value:       1,
			Labels:      labels,
			Timestamp:   time.Now(),
			Description: description,
		}
	}
}

// setMetric 设置指标
func (m *MetricsCollector) setMetric(name string, metricType MetricType, value float64, labels map[string]string, description string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getMetricKey(name, labels)
	m.metrics[key] = &Metric{
		Name:        name,
		Type:        metricType,
		Value:       value,
		Labels:      labels,
		Timestamp:   time.Now(),
		Description: description,
	}
}

// getMetricKey 获取指标键
func (m *MetricsCollector) getMetricKey(name string, labels map[string]string) string {
	key := name
	for k, v := range labels {
		key += "_" + k + "_" + v
	}
	return key
}
