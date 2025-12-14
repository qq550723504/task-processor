// Package model 提供数据结构定义
package model

import (
	"encoding/json"
	"time"
)

// Description 产品描述信息
type Description struct {
	Text string `json:"text"`
	Type string `json:"type"`
	URL  string `json:"url,omitempty"`
}

// PriceBreakdown 价格明细
type PriceBreakdown struct {
	TypicalPrice *float64 `json:"typical_price,omitempty"`
	ListPrice    *float64 `json:"list_price,omitempty"`
	DealType     *string  `json:"deal_type,omitempty"`
}

// BuyboxPrices 购买框价格信息
type BuyboxPrices struct {
	FinalPrice float64 `json:"final_price"`
	UnitPrice  *string `json:"unit_price,omitempty"`
}

// Subcategory 子分类排名信息
type Subcategory struct {
	SubcategoryName string `json:"subcategory_name"`
	SubcategoryRank int    `json:"subcategory_rank"`
}

// CustomersSay 客户评价信息
type CustomersSay struct {
	Text     *string           `json:"text,omitempty"`
	Keywords CustomersKeywords `json:"keywords"`
}

// CustomersKeywords 客户评价关键词
type CustomersKeywords struct {
	Positive *[]string `json:"positive,omitempty"`
	Negative *[]string `json:"negative,omitempty"`
	Mixed    *[]string `json:"mixed,omitempty"`
}

// InactiveBuyBox 非活跃购买框信息
type InactiveBuyBox struct {
	Price *float64 `json:"price,omitempty"`
}

// Variation 变体信息
type Variation struct {
	Name       string                 `json:"name"`
	Asin       string                 `json:"asin"`
	Price      float64                `json:"price"`
	Currency   string                 `json:"currency"`
	Image      string                 `json:"image,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// VariationValue 变体值（维度名称及其可选值）
type VariationValue struct {
	VariantName string   `json:"variant_name"` // 维度名称（如 Color, Size）
	Values      []string `json:"values"`       // 该维度的所有可选值
}

// UnmarshalJSON 自定义JSON解析，支持多种字段名格式
func (vv *VariationValue) UnmarshalJSON(data []byte) error {
	// 定义一个临时结构体，包含所有可能的字段名
	type Alias VariationValue
	aux := &struct {
		VariantNameWithSpace string `json:"variant name"` // 支持带空格的字段名
		*Alias
	}{
		Alias: (*Alias)(vv),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	// 如果 variant_name 为空但 variant name 有值，使用后者
	if vv.VariantName == "" && aux.VariantNameWithSpace != "" {
		vv.VariantName = aux.VariantNameWithSpace
	}

	return nil
}

// ProductDetail 产品详情
type ProductDetail struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// NullableTime 可空时间类型，支持空字符串解析为nil
type NullableTime struct {
	*time.Time
}

// UnmarshalJSON 自定义JSON解析，将空字符串解析为nil
func (nt *NullableTime) UnmarshalJSON(data []byte) error {
	// 去除引号
	str := string(data)
	if str == "null" || str == `""` || str == "" {
		nt.Time = nil
		return nil
	}

	// 解析时间
	var t time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}
	nt.Time = &t
	return nil
}

// MarshalJSON 自定义JSON序列化
func (nt NullableTime) MarshalJSON() ([]byte, error) {
	if nt.Time == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*nt.Time)
}
