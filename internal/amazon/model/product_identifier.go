// Package model 提供Amazon产品标识符相关数据模型
package model

// ProductIdentifier 产品标识符
type ProductIdentifier struct {
	Type  string `json:"type"`  // UPC, EAN, GTIN等
	Value string `json:"value"` // 标识符值
}

// ProductIdentifierConfig 产品标识符配置（基于亚马逊官方API规范）
type ProductIdentifierConfig struct {
	UPC   string `json:"upc"`   // UPC码 (Universal Product Code)
	EAN   string `json:"ean"`   // EAN码 (European Article Number)
	GTIN  string `json:"gtin"`  // GTIN码 (Global Trade Item Number)
	ISBN  string `json:"isbn"`  // ISBN码 (International Standard Book Number)
	JAN   string `json:"jan"`   // JAN码 (Japanese Article Number)
	ASIN  string `json:"asin"`  // ASIN码 (Amazon Standard Identification Number)
	FNSKU string `json:"fnsku"` // FNSKU码 (Fulfillment Network Stock Keeping Unit)
}

// HasAnyIdentifier 检查是否有任何标识符
func (c *ProductIdentifierConfig) HasAnyIdentifier() bool {
	return c.UPC != "" || c.EAN != "" || c.GTIN != "" || c.ISBN != "" || c.JAN != "" || c.ASIN != "" || c.FNSKU != ""
}

// GetPreferredIdentifier 获取首选标识符（按优先级）
func (c *ProductIdentifierConfig) GetPreferredIdentifier() (string, string) {
	// 按照亚马逊推荐的优先级返回
	if c.UPC != "" {
		return "UPC", c.UPC
	}
	if c.EAN != "" {
		return "EAN", c.EAN
	}
	if c.GTIN != "" {
		return "GTIN", c.GTIN
	}
	if c.ISBN != "" {
		return "ISBN", c.ISBN
	}
	if c.JAN != "" {
		return "JAN", c.JAN
	}
	if c.FNSKU != "" {
		return "FNSKU", c.FNSKU
	}
	if c.ASIN != "" {
		return "ASIN", c.ASIN
	}
	return "", ""
}

// GetAllIdentifiers 获取所有非空标识符
func (c *ProductIdentifierConfig) GetAllIdentifiers() map[string]string {
	identifiers := make(map[string]string)

	if c.UPC != "" {
		identifiers["UPC"] = c.UPC
	}
	if c.EAN != "" {
		identifiers["EAN"] = c.EAN
	}
	if c.GTIN != "" {
		identifiers["GTIN"] = c.GTIN
	}
	if c.ISBN != "" {
		identifiers["ISBN"] = c.ISBN
	}
	if c.JAN != "" {
		identifiers["JAN"] = c.JAN
	}
	if c.FNSKU != "" {
		identifiers["FNSKU"] = c.FNSKU
	}
	if c.ASIN != "" {
		identifiers["ASIN"] = c.ASIN
	}

	return identifiers
}
