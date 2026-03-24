package validation_test

import (
	"testing"

	"task-processor/internal/shein/validation"
)

func TestQuantityValidator_ValidateQuantityMapping(t *testing.T) {
	v := validation.NewQuantityValidator()

	tests := []struct {
		name         string
		quantityType int
		quantityUnit int
		wantErr      bool
	}{
		// 单品(1)：件(1) 或 双(2) 合法
		{"single_item_unit_piece", 1, 1, false},
		{"single_item_unit_pair", 1, 2, false},
		{"single_item_unit_set_invalid", 1, 3, true},
		// 同款多件(2)：只能件(1)
		{"multi_same_unit_piece", 2, 1, false},
		{"multi_same_unit_pair_invalid", 2, 2, true},
		{"multi_same_unit_set_invalid", 2, 3, true},
		// 单套(3)：只能套(3)
		{"single_set_unit_set", 3, 3, false},
		{"single_set_unit_piece_invalid", 3, 1, true},
		// 多套(4)：只能套(3)
		{"multi_set_unit_set", 4, 3, false},
		{"multi_set_unit_piece_invalid", 4, 1, true},
		// 未知类型
		{"unknown_type", 99, 1, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.ValidateQuantityMapping(tc.quantityType, tc.quantityUnit)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateQuantityMapping(%d, %d) error = %v, wantErr %v",
					tc.quantityType, tc.quantityUnit, err, tc.wantErr)
			}
		})
	}
}

func TestQuantityValidator_GetCorrectQuantityUnit(t *testing.T) {
	v := validation.NewQuantityValidator()

	tests := []struct {
		name         string
		quantityType int
		wantUnit     int
		wantErr      bool
	}{
		{"single_item_returns_piece", 1, 1, false},
		{"multi_same_returns_piece", 2, 1, false},
		{"single_set_returns_set", 3, 3, false},
		{"multi_set_returns_set", 4, 3, false},
		{"unknown_type_returns_error", 99, 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := v.GetCorrectQuantityUnit(tc.quantityType)
			if (err != nil) != tc.wantErr {
				t.Errorf("GetCorrectQuantityUnit(%d) error = %v, wantErr %v", tc.quantityType, err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.wantUnit {
				t.Errorf("GetCorrectQuantityUnit(%d) = %d, want %d", tc.quantityType, got, tc.wantUnit)
			}
		})
	}
}

func TestQuantityValidator_ValidateQuantity(t *testing.T) {
	v := validation.NewQuantityValidator()

	tests := []struct {
		name         string
		quantity     int
		quantityType int
		wantErr      bool
	}{
		// 单品(1)：数量必须=1
		{"single_item_qty1_ok", 1, 1, false},
		{"single_item_qty2_invalid", 2, 1, true},
		// 同款多件(2)：数量必须>=2
		{"multi_same_qty2_ok", 2, 2, false},
		{"multi_same_qty5_ok", 5, 2, false},
		{"multi_same_qty1_invalid", 1, 2, true},
		// 单套(3)：数量必须=1
		{"single_set_qty1_ok", 1, 3, false},
		{"single_set_qty2_invalid", 2, 3, true},
		// 多套(4)：数量必须>=2
		{"multi_set_qty2_ok", 2, 4, false},
		{"multi_set_qty1_invalid", 1, 4, true},
		// 未知类型
		{"unknown_type", 1, 99, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.ValidateQuantity(tc.quantity, tc.quantityType)
			if (err != nil) != tc.wantErr {
				t.Errorf("ValidateQuantity(%d, %d) error = %v, wantErr %v",
					tc.quantity, tc.quantityType, err, tc.wantErr)
			}
		})
	}
}
