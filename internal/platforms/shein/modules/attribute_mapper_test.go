package modules

import (
	"testing"
)

func TestConvertToAttributeValueNameMultis(t *testing.T) {
	tests := []struct {
		name   string
		source []struct {
			Language                string `json:"language"`
			AttributeValueNameMulti string `json:"attribute_value_name_multi"`
			WarningType             int    `json:"warning_type"`
		}
		wantLen int
	}{
		{
			name: "正常转换",
			source: []struct {
				Language                string `json:"language"`
				AttributeValueNameMulti string `json:"attribute_value_name_multi"`
				WarningType             int    `json:"warning_type"`
			}{
				{Language: "en", AttributeValueNameMulti: "Lobster", WarningType: 0},
				{Language: "zh", AttributeValueNameMulti: "龙虾", WarningType: 0},
			},
			wantLen: 2,
		},
		{
			name: "空列表",
			source: []struct {
				Language                string `json:"language"`
				AttributeValueNameMulti string `json:"attribute_value_name_multi"`
				WarningType             int    `json:"warning_type"`
			}{},
			wantLen: 0,
		},
		{
			name: "包含空语言代码",
			source: []struct {
				Language                string `json:"language"`
				AttributeValueNameMulti string `json:"attribute_value_name_multi"`
				WarningType             int    `json:"warning_type"`
			}{
				{Language: "", AttributeValueNameMulti: "Test", WarningType: 0},
				{Language: "en", AttributeValueNameMulti: "Valid", WarningType: 0},
			},
			wantLen: 1,
		},
		{
			name: "包含空属性值名称",
			source: []struct {
				Language                string `json:"language"`
				AttributeValueNameMulti string `json:"attribute_value_name_multi"`
				WarningType             int    `json:"warning_type"`
			}{
				{Language: "en", AttributeValueNameMulti: "", WarningType: 0},
				{Language: "zh", AttributeValueNameMulti: "有效值", WarningType: 0},
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToAttributeValueNameMultis(tt.source)
			if len(result) != tt.wantLen {
				t.Errorf("convertToAttributeValueNameMultis() 返回长度 = %d, 期望 %d", len(result), tt.wantLen)
			}

			// 验证转换后的数据
			for _, item := range result {
				if item.Language == "" {
					t.Error("转换后的数据包含空语言代码")
				}
				if item.AttributeValueName == "" {
					t.Error("转换后的数据包含空属性值名称")
				}
			}
		})
	}
}
