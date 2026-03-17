package filter

import "regexp"

// ProhibitedItemsConfig 违禁品配置结构
type ProhibitedItemsConfig struct {
	StaticKeywords   map[string][]string `json:"static_keywords"`
	DynamicPatterns  map[string][]string `json:"dynamic_patterns"`
	CategoryKeywords map[string][]string `json:"category_keywords"`
	LastUpdated      string              `json:"last_updated"`
	Version          string              `json:"version"`
	Platform         string              `json:"platform"`
}

// ProhibitedItemResult 违禁品检测结果
type ProhibitedItemResult struct {
	IsProhibited     bool     `json:"is_prohibited"`
	ViolatedItems    []string `json:"violated_items"`
	ViolatedCategory string   `json:"violated_category"`
	Confidence       float64  `json:"confidence"`
	Reason           string   `json:"reason"`
}

// DetectorConfig 检测器配置
type DetectorConfig struct {
	StaticKeywords   map[string][]string
	DynamicPatterns  map[string][]*regexp.Regexp
	CategoryKeywords map[string][]string
}
