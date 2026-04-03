// Package model 提供Amazon任务上下文模型
package model

import managementapi "task-processor/internal/infra/clients/management/api"

// TaskContext Amazon任务上下文
type TaskContext struct {
	TaskID              string                      `json:"task_id"`
	MarketplaceID       string                      `json:"marketplace_id"`
	LanguageTag         string                      `json:"language_tag"`
	Currency            string                      `json:"currency"`
	Data                map[string]any              `json:"data"`
	StoreInfo           *managementapi.StoreRespDTO `json:"store_info,omitempty"`
	ProductData         *ProductData                `json:"product_data,omitempty"`
	Results             map[string]any              `json:"results,omitempty"`
	DailyQuotaReserved  bool                        `json:"daily_quota_reserved,omitempty"`
	DailyQuotaDate      string                      `json:"daily_quota_date,omitempty"`
	DailyQuotaIncrement int64                       `json:"daily_quota_increment,omitempty"`
}

// SetResult 设置处理结果
func (tc *TaskContext) SetResult(key string, value any) {
	if tc.Results == nil {
		tc.Results = make(map[string]any)
	}
	tc.Results[key] = value
}

// GetResult 获取处理结果
func (tc *TaskContext) GetResult(key string) (any, bool) {
	if tc.Results == nil {
		return nil, false
	}
	value, exists := tc.Results[key]
	return value, exists
}

// SetProductData 设置产品数据
func (tc *TaskContext) SetProductData(data *ProductData) {
	tc.ProductData = data
}

// GetProductData 获取产品数据
func (tc *TaskContext) GetProductData() *ProductData {
	return tc.ProductData
}

func (tc *TaskContext) SetDailyQuotaReservation(date string, increment int64) {
	tc.DailyQuotaReserved = true
	tc.DailyQuotaDate = date
	tc.DailyQuotaIncrement = increment
}

func (tc *TaskContext) ClearDailyQuotaReservation() {
	tc.DailyQuotaReserved = false
	tc.DailyQuotaDate = ""
	tc.DailyQuotaIncrement = 0
}
