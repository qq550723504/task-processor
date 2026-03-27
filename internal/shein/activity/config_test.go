package activity

import (
	"errors"
	"testing"
	"time"
)

// TestDefaultTimeLimitedDiscountConfig 验证默认配置的关键字段
func TestDefaultTimeLimitedDiscountConfig(t *testing.T) {
	cfg := DefaultTimeLimitedDiscountConfig()

	if cfg.TimeZone != "America/Los_Angeles" {
		t.Errorf("TimeZone: want %q, got %q", "America/Los_Angeles", cfg.TimeZone)
	}
	if cfg.DiscountRate != 0.4 {
		t.Errorf("DiscountRate: want 0.4, got %v", cfg.DiscountRate)
	}
	if cfg.MinProfitRate != 0.15 {
		t.Errorf("MinProfitRate: want 0.15, got %v", cfg.MinProfitRate)
	}
	if cfg.PriceMode != "DISCOUNT" {
		t.Errorf("PriceMode: want %q, got %q", "DISCOUNT", cfg.PriceMode)
	}
	if cfg.Currency != "USD" {
		t.Errorf("Currency: want %q, got %q", "USD", cfg.Currency)
	}
	if cfg.PageSize != 100 {
		t.Errorf("PageSize: want 100, got %d", cfg.PageSize)
	}
}

// TestTimeLimitedDiscountConfig_Validate 表驱动测试验证配置校验逻辑
func TestTimeLimitedDiscountConfig_Validate(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)

	validBase := func() TimeLimitedDiscountConfig {
		return TimeLimitedDiscountConfig{
			ActivityName: "test-activity",
			TimeZone:     "America/Los_Angeles",
			StartTime:    now,
			EndTime:      later,
		}
	}

	tests := []struct {
		name    string
		mutate  func(*TimeLimitedDiscountConfig)
		wantErr error
	}{
		{
			name:    "有效配置",
			mutate:  func(_ *TimeLimitedDiscountConfig) {},
			wantErr: nil,
		},
		{
			name:    "活动名称为空",
			mutate:  func(c *TimeLimitedDiscountConfig) { c.ActivityName = "" },
			wantErr: ErrInvalidActivityName,
		},
		{
			name:    "开始时间为零值",
			mutate:  func(c *TimeLimitedDiscountConfig) { c.StartTime = time.Time{} },
			wantErr: ErrInvalidActivityTime,
		},
		{
			name:    "结束时间为零值",
			mutate:  func(c *TimeLimitedDiscountConfig) { c.EndTime = time.Time{} },
			wantErr: ErrInvalidActivityTime,
		},
		{
			name: "开始时间晚于结束时间",
			mutate: func(c *TimeLimitedDiscountConfig) {
				c.StartTime = later
				c.EndTime = now
			},
			wantErr: ErrInvalidActivityTimeRange,
		},
		{
			name:    "时区为空",
			mutate:  func(c *TimeLimitedDiscountConfig) { c.TimeZone = "" },
			wantErr: ErrInvalidTimeZone,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validBase()
			tt.mutate(&cfg)

			err := cfg.Validate()

			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// TestTimeLimitedDiscountConfig_Validate_StartEqualsEnd 开始时间等于结束时间应通过
func TestTimeLimitedDiscountConfig_Validate_StartEqualsEnd(t *testing.T) {
	ts := time.Now()
	cfg := TimeLimitedDiscountConfig{
		ActivityName: "test",
		TimeZone:     "UTC",
		StartTime:    ts,
		EndTime:      ts,
	}
	// StartTime.After(EndTime) 为 false，应通过
	if err := cfg.Validate(); err != nil {
		t.Errorf("equal start/end time should pass Validate, got: %v", err)
	}
}
