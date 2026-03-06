// Package handlers 提供TEMU平台处理器的数据模型定义
package common

// SpecPriority 规格优先级结构体
// 用于SKU AI映射验证器中的规格排序
type SpecPriority struct {
	ParentSpecID   string `json:"parent_spec_id"`
	ParentSpecName string `json:"parent_spec_name"`
	Priority       int    `json:"priority"`
	Usage          int    `json:"usage"`
}

// CategoryDisclaimResponse 分类免责声明响应结构体
// 用于分类免责声明处理器的API响应
type CategoryDisclaimResponse struct {
	Success   bool `json:"success"`
	ErrorCode int  `json:"error_code"`
	Result    struct {
		DisclaimerDTO struct {
			PromptList []string `json:"prompt_list"`
		} `json:"disclaimer_dto"`
	} `json:"result"`
}
