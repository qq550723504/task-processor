package pricing

import (
	"strings"
	"testing"
	"time"
)

func TestTimeHelper_FormatTime(t *testing.T) {
	h := NewTimeHelper()
	ts := time.Date(2026, 3, 24, 15, 30, 0, 0, time.UTC)

	got := h.FormatTime(ts)
	want := "2026-03-24 15:30:00"
	if got != want {
		t.Errorf("FormatTime() = %q, want %q", got, want)
	}
}

func TestTimeHelper_ParseTime(t *testing.T) {
	h := NewTimeHelper()

	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantY   int
	}{
		{"有效时间字符串", "2026-03-24 15:30:00", false, 2026},
		{"格式错误", "2026/03/24", true, 0},
		{"空字符串", "", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := h.ParseTime(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Year() != tt.wantY {
				t.Errorf("year = %d, want %d", got.Year(), tt.wantY)
			}
		})
	}
}

func TestTimeHelper_GetDefaultTimeRange(t *testing.T) {
	h := NewTimeHelper()
	start, end := h.GetDefaultTimeRange()

	if start == "" || end == "" {
		t.Error("GetDefaultTimeRange should return non-empty strings")
	}

	startT, err := h.ParseTime(start)
	if err != nil {
		t.Fatalf("start time parse error: %v", err)
	}
	endT, err := h.ParseTime(end)
	if err != nil {
		t.Fatalf("end time parse error: %v", err)
	}

	// end 应约为 now+24h，start 约为 end-3个月
	if !startT.Before(endT) {
		t.Error("start should be before end")
	}
	diff := endT.Sub(startT)
	// 3个月约 89~92 天
	if diff < 88*24*time.Hour || diff > 93*24*time.Hour {
		t.Errorf("default range should be ~3 months, got %v", diff)
	}
}

func TestTimeHelper_ValidateTimeRange(t *testing.T) {
	h := NewTimeHelper()

	tests := []struct {
		name        string
		startTime   string
		endTime     string
		wantErr     bool
		errContains string
	}{
		{"空字符串通过", "", "", false, ""},
		{"有效范围通过", "2026-01-01 00:00:00", "2026-03-01 00:00:00", false, ""},
		{"开始晚于结束报错", "2026-03-01 00:00:00", "2026-01-01 00:00:00", true, "开始时间不能晚于结束时间"},
		{"超过三个月报错", "2025-01-01 00:00:00", "2026-03-01 00:00:00", true, "不能超过三个月"},
		{"开始时间格式错误", "bad-format", "2026-03-01 00:00:00", true, ""},
		{"结束时间格式错误", "2026-01-01 00:00:00", "bad-format", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.ValidateTimeRange(tt.startTime, tt.endTime)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestTimeHelper_AdjustTimeRangeToLimit(t *testing.T) {
	h := NewTimeHelper()

	t.Run("空字符串返回默认范围", func(t *testing.T) {
		start, end, err := h.AdjustTimeRangeToLimit("", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if start == "" || end == "" {
			t.Error("should return non-empty default range")
		}
	})

	t.Run("范围在90天内不调整", func(t *testing.T) {
		start, end, err := h.AdjustTimeRangeToLimit("2026-01-01 00:00:00", "2026-03-01 00:00:00")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if start != "2026-01-01 00:00:00" {
			t.Errorf("start should not be adjusted, got %q", start)
		}
		if end != "2026-03-01 00:00:00" {
			t.Errorf("end should not be adjusted, got %q", end)
		}
	})

	t.Run("超过90天自动截断开始时间", func(t *testing.T) {
		start, end, err := h.AdjustTimeRangeToLimit("2025-01-01 00:00:00", "2026-03-01 00:00:00")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		startT, _ := h.ParseTime(start)
		endT, _ := h.ParseTime(end)
		diff := endT.Sub(startT)
		if diff > 91*24*time.Hour {
			t.Errorf("adjusted range should be ≤90 days, got %v", diff)
		}
	})

	t.Run("开始时间格式错误返回错误", func(t *testing.T) {
		_, _, err := h.AdjustTimeRangeToLimit("bad", "2026-03-01 00:00:00")
		if err == nil {
			t.Error("expected error for bad start time format")
		}
	})
}

func TestTimeHelper_GetMaxAllowedTimeRange(t *testing.T) {
	h := NewTimeHelper()
	end := time.Date(2026, 3, 24, 0, 0, 0, 0, time.UTC)
	maxStart := h.GetMaxAllowedTimeRange(end)

	// 应该是 end - 3个月
	expected := end.AddDate(0, -3, 0)
	if !maxStart.Equal(expected) {
		t.Errorf("GetMaxAllowedTimeRange() = %v, want %v", maxStart, expected)
	}
}
