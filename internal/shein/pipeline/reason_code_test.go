package pipeline

import (
	"errors"
	"testing"

	shein "task-processor/internal/shein"
)

func TestBuildTaskStatusErrorMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "filtered",
			err:  shein.NewFilteredError("低于筛选规则最低价格"),
			want: "[stage:validate_rules] [FILTER_RULE_REJECTED] 低于筛选规则最低价格",
		},
		{
			name: "cookie load",
			err:  shein.NewCookieLoadError(1, 2, "cookie missing"),
			want: "[stage:init_store] [COOKIE_LOAD_FAILED] Cookie加载失败 (租户=1, 店铺=2): cookie missing",
		},
		{
			name: "duplicate sku",
			err:  errors.New("产品发布失败: 卖家SKU重复"),
			want: "[stage:publish_product] [SKU_DUPLICATED] 产品发布失败: 卖家SKU重复",
		},
		{
			name: "generic retryable",
			err:  shein.NewRetryableError("发布产品失败", errors.New("timeout")),
			want: "[stage:publish_product] [RETRYABLE_FAILURE] 发布产品失败: timeout",
		},
		{
			name: "generic non-retryable",
			err:  shein.NewNonRetryableError("保存发布结果失败", errors.New("bad data")),
			want: "[stage:save_publish_result] [NON_RETRYABLE_FAILURE] 保存发布结果失败: bad data",
		},
	}

	stages := []string{
		"validate_rules",
		"init_store",
		"publish_product",
		"publish_product",
		"save_publish_result",
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildTaskStatusErrorMessage(stages[i], tt.err); got != tt.want {
				t.Fatalf("buildTaskStatusErrorMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}
