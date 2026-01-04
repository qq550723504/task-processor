// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"strings"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// ImageExtractor 图片提取器
type ImageExtractor struct{}

// NewImageExtractor 创建图片提取器
func NewImageExtractor() *ImageExtractor {
	return &ImageExtractor{}
}

// Extract 提取轮播图大图 - 支持两种数据结构
func (ie *ImageExtractor) Extract(page playwright.Page, product *model.Product1688) error {
	// 从结构化数据中获取图片，支持两种数据结构
	imagesResult, err := page.Evaluate(`() => {
		const result = {
			images: [],
			mainImage: ''
		};
		
		// 方案1：优先尝试从window.context结构化数据中获取（普通商品）
		if (window.context && window.context.result && window.context.result.data && 
			window.context.result.data.gallery && window.context.result.data.gallery.fields) {
			
			const galleryFields = window.context.result.data.gallery.fields;
			
			// 获取所有图片
			if (galleryFields.offerImgList && Array.isArray(galleryFields.offerImgList)) {
				result.images = galleryFields.offerImgList.filter(url => url && typeof url === 'string');
			}
			
			// 设置主图（取第一张图片）
			if (result.images.length > 0) {
				result.mainImage = result.images[0];
			}
		}
		// 方案2：备选方案 - 从window.__INIT_DATA获取（定制商品）
		else if (window.__INIT_DATA && window.__INIT_DATA.data) {
			const data = window.__INIT_DATA.data;
			
			// 遍历所有数据块，查找图片信息
			for (let key in data) {
				const item = data[key];
				if (!item || !item.data) continue;
				
				const itemData = item.data;
				
				// 提取图片列表
				if (itemData.offerImgList && Array.isArray(itemData.offerImgList)) {
					result.images = itemData.offerImgList.filter(url => url && typeof url === 'string');
					
					// 设置主图
					if (itemData.mainImage) {
						result.mainImage = itemData.mainImage;
					} else if (result.images.length > 0) {
						result.mainImage = result.images[0];
					}
					break;
				}
			}
		}
		
		return result;
	}`, nil)

	if err != nil {
		logrus.Debugf("提取图片失败: %v", err)
		return err
	}

	var images []string
	var mainImage string

	if imagesResult != nil {
		if resultMap, ok := imagesResult.(map[string]interface{}); ok {
			// 处理图片列表
			if imageArray, ok := resultMap["images"].([]interface{}); ok {
				for _, img := range imageArray {
					if imgUrl, ok := img.(string); ok && imgUrl != "" {
						fullURL := ie.processImageURL(imgUrl)
						if fullURL != "" {
							images = append(images, fullURL)
						}
					}
				}
			}

			// 处理主图
			if mainImg, ok := resultMap["mainImage"].(string); ok && mainImg != "" {
				mainImage = ie.processImageURL(mainImg)
			}

			logrus.Debugf("提取到 %d 张轮播图", len(images))
		}
	}

	product.Images = images
	if mainImage != "" {
		product.MainImage = mainImage
	} else if len(images) > 0 {
		product.MainImage = images[0]
	}

	return nil
}

// processImageURL 处理图片URL
func (ie *ImageExtractor) processImageURL(url string) string {
	if url == "" {
		return ""
	}

	// 如果是相对URL，添加协议
	if strings.HasPrefix(url, "//") {
		url = "https:" + url
	}

	return url
}
