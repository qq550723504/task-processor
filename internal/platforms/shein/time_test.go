package shein

import (
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantNil bool
	}{
		{
			name:    "正常时间格式",
			input:   "2025-11-25 11:07:09",
			wantErr: false,
			wantNil: false,
		},
		{
			name:    "空字符串",
			input:   "",
			wantErr: false,
			wantNil: true,
		},
		{
			name:    "错误格式",
			input:   "2025/11/25 11:07:09",
			wantErr: true,
			wantNil: true, // 错误时返回 nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTime(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if (got == nil) != tt.wantNil {
				t.Errorf("ParseTime() got = %v, wantNil %v", got, tt.wantNil)
				return
			}

			if !tt.wantErr && !tt.wantNil {
				t.Logf("解析成功: %s -> %v", tt.input, got)

				// 验证时间格式化
				formatted := got.Format("2006-01-02T15:04:05")
				t.Logf("格式化为 ISO: %s", formatted)

				// 验证不是 1970-01-01
				if got.Year() == 1970 {
					t.Errorf("解析后的时间是 1970 年，可能有问题: %v", got)
				}
			}
		})
	}
}

func TestTimeZone(t *testing.T) {
	// 测试时区问题
	timeStr := "2025-11-25 11:07:09"
	parsed, err := ParseTime(timeStr)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	t.Logf("原始字符串: %s", timeStr)
	t.Logf("解析后时间: %v", parsed)
	t.Logf("时区: %s", parsed.Location())
	t.Logf("Unix 时间戳: %d", parsed.Unix())
	t.Logf("格式化 ISO: %s", parsed.Format("2006-01-02T15:04:05"))
	t.Logf("格式化带时区: %s", parsed.Format(time.RFC3339))
}
