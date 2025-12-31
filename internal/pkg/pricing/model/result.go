// Package model 提供核价结果相关的数据模型。
package model

import "time"

// PricingAction 核价决策动作
type PricingAction string

const (
	ActionAccept   PricingAction = "accept"   // 接受价格
	ActionReject   PricingAction = "reject"   // 拒绝价格
	ActionReappeal PricingAction = "reappeal" // 重新申诉
	ActionSkip     PricingAction = "skip"     // 跳过处理
)

// PricingResult 核价结果
type PricingResult struct {
	// 基础信息
	ProductID   string    `json:"product_id"`
	SkuID       string    `json:"sku_id"`
	ProductName string    `json:"product_name"`
	Timestamp   time.Time `json:"timestamp"`

	// 价格信息
	OriginCostPrice float64 `json:"origin_cost_price"` // 原始成本价
	CurrentPrice    float64 `json:"current_price"`     // 当前价格
	SuggestPrice    float64 `json:"suggest_price"`     // 建议价格
	CalculatedPrice float64 `json:"calculated_price"`  // 计算得出的价格
	AcceptablePrice float64 `json:"acceptable_price"`  // 最低可接受价格

	// 利润信息
	ProfitMargin float64 `json:"profit_margin"` // 实际利润率(%)
	TargetMargin float64 `json:"target_margin"` // 目标利润率(%)
	MinMargin    float64 `json:"min_margin"`    // 最低利润率(%)

	// 决策信息
	Action          PricingAction `json:"action"`            // 决策动作
	Reason          string        `json:"reason"`            // 决策原因
	AppliedRuleName string        `json:"applied_rule_name"` // 应用的规则名称
	AppliedRuleType string        `json:"applied_rule_type"` // 应用的规则类型

	// 平台特定数据
	PlatformData interface{} `json:"platform_data,omitempty"` // 平台特定的结果数据
}

// IsSuccessful 判断核价是否成功
func (pr *PricingResult) IsSuccessful() bool {
	return pr.Action == ActionAccept || pr.Action == ActionReappeal
}

// ShouldProcess 判断是否需要进一步处理
func (pr *PricingResult) ShouldProcess() bool {
	return pr.Action != ActionSkip
}

// CalculateProfitMargin 计算利润率
func (pr *PricingResult) CalculateProfitMargin() {
	if pr.OriginCostPrice > 0 && pr.SuggestPrice > 0 {
		pr.ProfitMargin = (pr.SuggestPrice - pr.OriginCostPrice) / pr.OriginCostPrice * 100
	}
}

// SetDecision 设置决策结果
func (pr *PricingResult) SetDecision(action PricingAction, reason string) {
	pr.Action = action
	pr.Reason = reason
	pr.Timestamp = time.Now()
}

// BatchPricingResult 批量核价结果
type BatchPricingResult struct {
	TotalCount   int              `json:"total_count"`
	SuccessCount int              `json:"success_count"`
	FailCount    int              `json:"fail_count"`
	SkipCount    int              `json:"skip_count"`
	Results      []*PricingResult `json:"results"`
	StartTime    time.Time        `json:"start_time"`
	EndTime      time.Time        `json:"end_time"`
	Duration     time.Duration    `json:"duration"`
}

// AddResult 添加核价结果
func (bpr *BatchPricingResult) AddResult(result *PricingResult) {
	bpr.Results = append(bpr.Results, result)
	bpr.TotalCount++

	switch result.Action {
	case ActionAccept, ActionReappeal:
		bpr.SuccessCount++
	case ActionReject:
		bpr.FailCount++
	case ActionSkip:
		bpr.SkipCount++
	}
}

// Finish 完成批量处理
func (bpr *BatchPricingResult) Finish() {
	bpr.EndTime = time.Now()
	bpr.Duration = bpr.EndTime.Sub(bpr.StartTime)
}
