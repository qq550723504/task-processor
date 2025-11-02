package models

import (
	"task-processor/common/amazon"
)

// NewAmazonProduct 创建新的Amazon产品
func NewAmazonProduct(asin string) *amazon.Product {
	return &amazon.Product{
		Asin: asin,
	}
}
