package models

import (
	"task-processor/common/amazon/model"
)

// NewAmazonProduct 创建新的Amazon产品
func NewAmazonProduct(asin string) *model.Product {
	return &model.Product{
		Asin: asin,
	}
}
