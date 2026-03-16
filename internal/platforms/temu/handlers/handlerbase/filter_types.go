// Package handlerbase 提供TEMU平台处理器的共享类型和基础功能
package handlerbase

// FilterCheckResult 筛选检查结果
type FilterCheckResult struct {
	Passed        bool        // 是否通过筛选
	FailedRule    string      // 失败的规则名称
	FailureReason string      // 失败原因
	ProductValue  interface{} // 产品的实际值
	RuleValue     interface{} // 规则要求的值
}
