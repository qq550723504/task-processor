// Package models 提供TEMU平台的通用数据结构定义
package models

// ImageInfo 图片信息
type ImageInfo struct {
	Type   *int   `json:"type"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ProductExpressInfo 产品物流信息
type ProductExpressInfo struct {
	WeightInfo WeightInfo `json:"weight_info"`
	VolumeInfo VolumeInfo `json:"volume_info"`
}

// VolumeInfo 体积信息
type VolumeInfo struct {
	Height string `json:"height"`
	Length string `json:"length"`
	Width  string `json:"width"`
}

// WeightInfo 重量信息
type WeightInfo struct {
	Weight string `json:"weight"`
}
