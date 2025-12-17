package shein

import (
	"encoding/json"
	"testing"
	"time"
)

// 测试时间格式化和 JSON 序列化
func TestTimeFormatAndJSON(t *testing.T) {
	// 模拟 SHEIN API 返回的时间字符串
	timeStr := "2025-11-25 11:07:09"

	// 解析时间
	parsedTime, err := ParseTime(timeStr)
	if err != nil {
		t.Fatalf("解析时间失败: %v", err)
	}

	t.Logf("原始字符串: %s", timeStr)
	t.Logf("解析后: %v", parsedTime)
	t.Logf("Unix 时间戳: %d", parsedTime.Unix())

	// 格式化为 ISO 格式（后端期望的格式）
	isoFormat := parsedTime.Format("2006-01-02T15:04:05")
	t.Logf("ISO 格式: %s", isoFormat)

	// 测试 JSON 序列化
	data := map[string]interface{}{
		"publishTime": isoFormat,
		"shelfTime":   isoFormat,
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("JSON 序列化失败: %v", err)
	}

	t.Logf("JSON 输出: %s", string(jsonBytes))

	// 验证不是 1970 年
	if parsedTime.Year() == 1970 {
		t.Errorf("时间是 1970 年，有问题！")
	}
}

// 测试空字符串的处理
func TestEmptyTimeString(t *testing.T) {
	emptyStr := ""

	parsedTime, err := ParseTime(emptyStr)
	if err != nil {
		t.Fatalf("解析空字符串失败: %v", err)
	}

	if parsedTime != nil {
		t.Errorf("空字符串应该返回 nil，但得到: %v", parsedTime)
	}

	t.Log("空字符串正确返回 nil")
}

// 测试零值时间的处理
func TestZeroTime(t *testing.T) {
	zeroTime := time.Time{}

	if !zeroTime.IsZero() {
		t.Error("零值时间应该是 IsZero()")
	}

	t.Logf("零值时间: %v", zeroTime)
	t.Logf("零值时间 Unix: %d", zeroTime.Unix())
	t.Logf("零值时间格式化: %s", zeroTime.Format("2006-01-02T15:04:05"))
}
