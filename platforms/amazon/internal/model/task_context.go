// Package model 提供Amazon任务上下文模型
package model

// TaskContext Amazon任务上下文
type TaskContext struct {
	TaskID        string                 `json:"task_id"`
	MarketplaceID string                 `json:"marketplace_id"`
	LanguageTag   string                 `json:"language_tag"`
	Currency      string                 `json:"currency"`
	Data          map[string]interface{} `json:"data"`
	ProductData   *ProductData           `json:"product_data,omitempty"`
	Results       map[string]interface{} `json:"results,omitempty"`
}

// SetResult 设置处理结果
func (tc *TaskContext) SetResult(key string, value interface{}) {
	if tc.Results == nil {
		tc.Results = make(map[string]interface{})
	}
	tc.Results[key] = value
}

// GetResult 获取处理结果
func (tc *TaskContext) GetResult(key string) (interface{}, bool) {
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
