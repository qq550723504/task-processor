// Package handlers 提供属性去重功能的测试
package handlers

import (
	"testing"

	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestPropertyDeduplicator_DeduplicateByPidOnly(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	deduplicator := NewPropertyDeduplicator(logger)

	tests := []struct {
		name       string
		properties []types.PropertyItem
		expected   int // 期望的去重后数量
	}{
		{
			name: "无重复属性",
			properties: []types.PropertyItem{
				{Pid: 1, RefPid: 2421, TemplatePid: 1678536, Value: "Universal", Vid: 26038},
				{Pid: 13, RefPid: 63, TemplatePid: 1678541, Value: "Beige", Vid: 433},
			},
			expected: 2,
		},
		{
			name: "有重复PID的属性",
			properties: []types.PropertyItem{
				{Pid: 1, RefPid: 2421, TemplatePid: 1678538, Value: "Polyester (polyester Fiber)", Vid: 26054},
				{Pid: 67, RefPid: 115, TemplatePid: 1678536, Value: "Universal", Vid: 26038},
				{Pid: 13, RefPid: 63, TemplatePid: 1678541, Value: "Beige", Vid: 433},
				{Pid: 1, RefPid: 2421, TemplatePid: 1678538, Value: "Polyester (polyester Fiber)", Vid: 26054}, // 重复
			},
			expected: 3, // 应该去重为3个
		},
		{
			name: "多个重复PID",
			properties: []types.PropertyItem{
				{Pid: 1, RefPid: 2421, TemplatePid: 1678538, Value: "Value1", Vid: 1},
				{Pid: 1, RefPid: 2421, TemplatePid: 1678538, Value: "Value2", Vid: 2}, // 重复PID
				{Pid: 1, RefPid: 2421, TemplatePid: 1678538, Value: "Value3", Vid: 3}, // 重复PID
				{Pid: 2, RefPid: 100, TemplatePid: 2000, Value: "Different", Vid: 100},
			},
			expected: 2, // 应该去重为2个
		},
		{
			name:       "空属性列表",
			properties: []types.PropertyItem{},
			expected:   0,
		},
		{
			name: "单个属性",
			properties: []types.PropertyItem{
				{Pid: 1, RefPid: 2421, TemplatePid: 1678538, Value: "Single", Vid: 1},
			},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicator.DeduplicateByPidOnly(tt.properties)
			assert.Equal(t, tt.expected, len(result), "去重后的属性数量不符合预期")

			// 验证去重后没有重复的PID
			pidMap := make(map[int]bool)
			for _, prop := range result {
				assert.False(t, pidMap[prop.Pid], "去重后仍存在重复的PID: %d", prop.Pid)
				pidMap[prop.Pid] = true
			}
		})
	}
}

func TestPropertyDeduplicator_DeduplicateProperties(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	deduplicator := NewPropertyDeduplicator(logger)

	tests := []struct {
		name       string
		properties []types.PropertyItem
		expected   int
	}{
		{
			name: "完全相同的属性",
			properties: []types.PropertyItem{
				{Pid: 1, RefPid: 2421, TemplatePid: 1678538, Value: "Same", Vid: 1},
				{Pid: 1, RefPid: 2421, TemplatePid: 1678538, Value: "Same", Vid: 1}, // 完全相同
			},
			expected: 1,
		},
		{
			name: "相同键但不同值",
			properties: []types.PropertyItem{
				{Pid: 1, RefPid: 2421, TemplatePid: 1678538, Value: "Value1", Vid: 1},
				{Pid: 1, RefPid: 2421, TemplatePid: 1678538, Value: "Value2", Vid: 2}, // 相同键，不同值
			},
			expected: 1, // 应该保留最后一个
		},
		{
			name: "不同的属性组合",
			properties: []types.PropertyItem{
				{Pid: 1, RefPid: 2421, TemplatePid: 1678538, Value: "Value1", Vid: 1},
				{Pid: 1, RefPid: 2422, TemplatePid: 1678538, Value: "Value2", Vid: 2}, // 不同RefPid
				{Pid: 1, RefPid: 2421, TemplatePid: 1678539, Value: "Value3", Vid: 3}, // 不同TemplatePid
			},
			expected: 3, // 都应该保留
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicator.DeduplicateProperties(tt.properties)
			assert.Equal(t, tt.expected, len(result), "去重后的属性数量不符合预期")
		})
	}
}
