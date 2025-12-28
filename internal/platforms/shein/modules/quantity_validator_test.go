// Package modules 提供SHEIN平台数量信息验证工具测试
package modules

import (
	"testing"
)

func TestQuantityValidator_ValidateQuantityMapping(t *testing.T) {
	validator := NewQuantityValidator()

	tests := []struct {
		name         string
		quantityType int
		quantityUnit int
		expectError  bool
	}{
		{"单品-件", 1, 1, false},
		{"单品-双", 1, 2, false},
		{"单品-套", 1, 3, true}, // 应该报错
		{"同款多件-件", 2, 1, false},
		{"同款多件-双", 2, 2, true}, // 应该报错
		{"同款多件-套", 2, 3, true}, // 应该报错
		{"单套-件", 3, 1, true},   // 应该报错
		{"单套-双", 3, 2, true},   // 应该报错
		{"单套-套", 3, 3, false},
		{"多套-件", 4, 1, true}, // 应该报错
		{"多套-双", 4, 2, true}, // 应该报错
		{"多套-套", 4, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateQuantityMapping(tt.quantityType, tt.quantityUnit)
			if tt.expectError && err == nil {
				t.Errorf("期望出现错误，但没有错误")
			}
			if !tt.expectError && err != nil {
				t.Errorf("不期望出现错误，但出现了错误: %v", err)
			}
		})
	}
}

func TestQuantityValidator_GetCorrectQuantityUnit(t *testing.T) {
	validator := NewQuantityValidator()

	tests := []struct {
		name         string
		quantityType int
		expectedUnit int
		expectError  bool
	}{
		{"单品", 1, 1, false},
		{"同款多件", 2, 1, false},
		{"单套", 3, 3, false},
		{"多套", 4, 3, false},
		{"无效类型", 5, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unit, err := validator.GetCorrectQuantityUnit(tt.quantityType)
			if tt.expectError && err == nil {
				t.Errorf("期望出现错误，但没有错误")
			}
			if !tt.expectError && err != nil {
				t.Errorf("不期望出现错误，但出现了错误: %v", err)
			}
			if !tt.expectError && unit != tt.expectedUnit {
				t.Errorf("期望单位 %d，但得到 %d", tt.expectedUnit, unit)
			}
		})
	}
}

func TestQuantityValidator_ValidateQuantity(t *testing.T) {
	validator := NewQuantityValidator()

	tests := []struct {
		name         string
		quantity     int
		quantityType int
		expectError  bool
	}{
		{"单品-数量1", 1, 1, false},
		{"单品-数量2", 2, 1, true},   // 应该报错
		{"同款多件-数量1", 1, 2, true}, // 应该报错
		{"同款多件-数量2", 2, 2, false},
		{"同款多件-数量3", 3, 2, false},
		{"单套-数量1", 1, 3, false},
		{"单套-数量2", 2, 3, true}, // 应该报错
		{"多套-数量1", 1, 4, true}, // 应该报错
		{"多套-数量2", 2, 4, false},
		{"多套-数量3", 3, 4, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateQuantity(tt.quantity, tt.quantityType)
			if tt.expectError && err == nil {
				t.Errorf("期望出现错误，但没有错误")
			}
			if !tt.expectError && err != nil {
				t.Errorf("不期望出现错误，但出现了错误: %v", err)
			}
		})
	}
}
