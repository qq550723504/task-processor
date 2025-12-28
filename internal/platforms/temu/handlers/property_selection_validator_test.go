// Package handlers 提供属性选择约束验证功能的测试
package handlers

import (
	"testing"

	"task-processor/internal/platforms/temu/types"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestPropertySelectionValidator_ValidateSelectionConstraints(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	validator := NewPropertySelectionValidator(logger)

	tests := []struct {
		name          string
		properties    []types.PropertyItem
		templateProps []types.TemplateRespGoodsProperty
		expectedCount int
		description   string
	}{
		{
			name: "单选属性多个值应该只保留一个",
			properties: []types.PropertyItem{
				{Pid: 134, RefPid: 234, Value: "Hiking", Vid: 18898},
				{Pid: 134, RefPid: 234, Value: "Backpacking", Vid: 21548},
				{Pid: 134, RefPid: 234, Value: "Camping", Vid: 21552},
				{Pid: 134, RefPid: 234, Value: "Travel", Vid: 21554},
			},
			templateProps: []types.TemplateRespGoodsProperty{
				{
					PID:               134,
					RefPID:            234,
					Name:              "Activity Type",
					PropertyValueType: 1, // 选择类型
					ChooseMaxNum:      1, // 单选
					Values: []types.PropertyValue{
						{VID: 18898, Value: "Hiking"},
						{VID: 21548, Value: "Backpacking"},
						{VID: 21552, Value: "Camping"},
						{VID: 21554, Value: "Travel"},
					},
				},
			},
			expectedCount: 1,
			description:   "单选属性应该只保留一个值",
		},
		{
			name: "多选属性超过限制应该截取",
			properties: []types.PropertyItem{
				{Pid: 200, RefPid: 300, Value: "Red", Vid: 1},
				{Pid: 200, RefPid: 300, Value: "Blue", Vid: 2},
				{Pid: 200, RefPid: 300, Value: "Green", Vid: 3},
				{Pid: 200, RefPid: 300, Value: "Yellow", Vid: 4},
			},
			templateProps: []types.TemplateRespGoodsProperty{
				{
					PID:               200,
					RefPID:            300,
					Name:              "Colors",
					PropertyValueType: 1, // 选择类型
					ChooseMaxNum:      2, // 最多选2个
					Values: []types.PropertyValue{
						{VID: 1, Value: "Red"},
						{VID: 2, Value: "Blue"},
						{VID: 3, Value: "Green"},
						{VID: 4, Value: "Yellow"},
					},
				},
			},
			expectedCount: 2,
			description:   "多选属性应该限制在最大选择数内",
		},
		{
			name: "无限制选择属性应该保留所有",
			properties: []types.PropertyItem{
				{Pid: 300, RefPid: 400, Value: "Feature1", Vid: 1},
				{Pid: 300, RefPid: 400, Value: "Feature2", Vid: 2},
				{Pid: 300, RefPid: 400, Value: "Feature3", Vid: 3},
			},
			templateProps: []types.TemplateRespGoodsProperty{
				{
					PID:               300,
					RefPID:            400,
					Name:              "Features",
					PropertyValueType: 1, // 选择类型
					ChooseMaxNum:      0, // 无限制
				},
			},
			expectedCount: 3,
			description:   "无限制选择属性应该保留所有值",
		},
		{
			name: "非选择类型属性应该保持不变",
			properties: []types.PropertyItem{
				{Pid: 400, RefPid: 500, Value: "Text Value 1", Vid: 0},
				{Pid: 400, RefPid: 500, Value: "Text Value 2", Vid: 0},
			},
			templateProps: []types.TemplateRespGoodsProperty{
				{
					PID:               400,
					RefPID:            500,
					Name:              "Description",
					PropertyValueType: 2, // 文本类型
					ChooseMaxNum:      1, // 这个对文本类型无效
				},
			},
			expectedCount: 2,
			description:   "非选择类型属性不受选择约束影响",
		},
		{
			name: "混合属性类型处理",
			properties: []types.PropertyItem{
				// 单选属性 - 应该只保留1个
				{Pid: 134, RefPid: 234, Value: "Hiking", Vid: 18898},
				{Pid: 134, RefPid: 234, Value: "Camping", Vid: 21552},
				// 多选属性 - 应该保留2个
				{Pid: 200, RefPid: 300, Value: "Red", Vid: 1},
				{Pid: 200, RefPid: 300, Value: "Blue", Vid: 2},
				{Pid: 200, RefPid: 300, Value: "Green", Vid: 3},
				// 文本属性 - 保持不变
				{Pid: 400, RefPid: 500, Value: "Description", Vid: 0},
			},
			templateProps: []types.TemplateRespGoodsProperty{
				{
					PID:               134,
					RefPID:            234,
					PropertyValueType: 1, // 选择类型
					ChooseMaxNum:      1, // 单选
					Values: []types.PropertyValue{
						{VID: 18898, Value: "Hiking"},
						{VID: 21552, Value: "Camping"},
					},
				},
				{
					PID:               200,
					RefPID:            300,
					PropertyValueType: 1, // 选择类型
					ChooseMaxNum:      2, // 多选最多2个
					Values: []types.PropertyValue{
						{VID: 1, Value: "Red"},
						{VID: 2, Value: "Blue"},
						{VID: 3, Value: "Green"},
					},
				},
				{
					PID:               400,
					RefPID:            500,
					PropertyValueType: 2, // 文本类型
				},
			},
			expectedCount: 4, // 1 + 2 + 1 = 4
			description:   "混合属性类型应该正确处理",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.ValidateSelectionConstraints(tt.properties, tt.templateProps)
			assert.Equal(t, tt.expectedCount, len(result), tt.description)

			// 验证单选约束
			pidCounts := make(map[int]int)
			for _, prop := range result {
				pidCounts[prop.Pid]++
			}

			for _, templateProp := range tt.templateProps {
				if templateProp.PropertyValueType == 1 && templateProp.ChooseMaxNum > 0 {
					count := pidCounts[templateProp.PID]
					assert.LessOrEqual(t, count, templateProp.ChooseMaxNum,
						"PID %d 的属性数量 %d 超过了最大选择数 %d",
						templateProp.PID, count, templateProp.ChooseMaxNum)
				}
			}
		})
	}
}

func TestPropertySelectionValidator_SelectBestSingleChoice(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	validator := NewPropertySelectionValidator(logger)

	templateProp := types.TemplateRespGoodsProperty{
		PID:               134,
		RefPID:            234,
		Name:              "Activity Type",
		PropertyValueType: 1,
		ChooseMaxNum:      1,
		Values: []types.PropertyValue{
			{VID: 18898, Value: "Hiking"},
			{VID: 21548, Value: "Backpacking"},
		},
	}

	tests := []struct {
		name        string
		propGroup   []types.PropertyItem
		expectedVID int
		description string
	}{
		{
			name: "选择有效VID的属性",
			propGroup: []types.PropertyItem{
				{Pid: 134, RefPid: 234, Value: "Invalid", Vid: 99999},
				{Pid: 134, RefPid: 234, Value: "Hiking", Vid: 18898}, // 有效VID
				{Pid: 134, RefPid: 234, Value: "Other", Vid: 0},
			},
			expectedVID: 18898,
			description: "应该选择有效VID的属性",
		},
		{
			name: "选择候选值匹配的属性",
			propGroup: []types.PropertyItem{
				{Pid: 134, RefPid: 234, Value: "Unknown", Vid: 0},
				{Pid: 134, RefPid: 234, Value: "Backpacking", Vid: 0}, // 候选值匹配
			},
			expectedVID: 0, // VID可能为0，但值匹配
			description: "应该选择候选值匹配的属性",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.selectBestSingleChoice(tt.propGroup, templateProp)
			assert.Equal(t, 1, len(result), "单选应该只返回一个属性")
			if len(result) > 0 {
				assert.Equal(t, tt.expectedVID, result[0].Vid, tt.description)
			}
		})
	}
}

func TestPropertySelectionValidator_GetSelectionConstraintSummary(t *testing.T) {
	logger := logrus.NewEntry(logrus.New())
	validator := NewPropertySelectionValidator(logger)

	templateProps := []types.TemplateRespGoodsProperty{
		{PID: 1, PropertyValueType: 1, ChooseMaxNum: 1}, // 单选
		{PID: 2, PropertyValueType: 1, ChooseMaxNum: 3}, // 多选
		{PID: 3, PropertyValueType: 1, ChooseMaxNum: 0}, // 无限制选择
		{PID: 4, PropertyValueType: 2, ChooseMaxNum: 1}, // 文本类型
		{PID: 5, PropertyValueType: 3, ChooseMaxNum: 1}, // 数值类型
	}

	summary := validator.GetSelectionConstraintSummary(templateProps)

	assert.Equal(t, 5, summary["total_properties"])
	assert.Equal(t, 3, summary["selection_properties"])
	assert.Equal(t, 1, summary["single_choice"])
	assert.Equal(t, 1, summary["multiple_choice"])
	assert.Equal(t, 1, summary["unlimited_choice"])
}
