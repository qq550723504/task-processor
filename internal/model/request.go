// Package model 提供数据结构定义
package model

// ProductRequest 产品请求
type ProductRequest struct {
	URL     string
	Zipcode string
}

// ProductResult 产品结果
type ProductResult struct {
	Product *Product
	Error   error
}

// ProductNotFoundError 产品不存在错误（不应触发浏览器重建）
type ProductNotFoundError struct {
	Message string
}

func (e *ProductNotFoundError) Error() string {
	return e.Message
}
