// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"fmt"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/mxschmitt/playwright-go"
)

// DetailExtractor 商品详情提取器
type DetailExtractor struct{}

// NewDetailExtractor 创建商品详情提取器
func NewDetailExtractor() *DetailExtractor {
	return &DetailExtractor{}
}

// Extract 提取商品详情信息 - 支持两种数据结构
func (de *DetailExtractor) Extract(page playwright.Page, product *model.Product1688) error {
	logger.GetGlobalLogger("crawler/alibaba1688").Debug("开始提取商品详情信息")

	// 从结构化数据中获取详情信息，支持两种数据结构
	detailResult, err := page.Evaluate(`() => {
		const result = {
			detailUrl: '',
			detailVideoId: '',
			detailImages: [],
			videos: []
		};
		
		// 方案1：优先尝试从window.context结构化数据中获取（普通商品）
		if (window.context && window.context.result && window.context.result.data) {
			const data = window.context.result.data;
			
			// 获取详情URL和视频ID
			if (data.description && data.description.fields) {
				result.detailUrl = data.description.fields.detailUrl || '';
				result.detailVideoId = data.description.fields.detailVideoId || '';
			}
			
			// 获取商品视频信息
			if (data.Root && data.Root.fields && data.Root.fields.dataJson) {
				const dataJson = data.Root.fields.dataJson;
				
				// 从tempModel中获取视频信息
				if (dataJson.tempModel && dataJson.tempModel.video) {
					const video = dataJson.tempModel.video;
					result.videos.push({
						videoId: video.videoId || 0,
						title: video.title || '商品视频',
						videoUrl: video.videoUrl || '',
						coverUrl: video.coverUrl || '',
						state: video.state || 0
					});
				}
				
				// 获取商品图片
				if (dataJson.images) {
					const images = dataJson.images;
					
					// 提取fullPathImageURI作为详情图片
					images.forEach(img => {
						if (img.fullPathImageURI) {
							// 构建完整的图片URL
							let imageUrl = img.fullPathImageURI;
							if (!imageUrl.startsWith('http')) {
								imageUrl = 'https://cbu01.alicdn.com/' + imageUrl;
							}
							result.detailImages.push(imageUrl);
						}
					});
				}
			}
		}
		// 方案2：备选方案 - 从window.__INIT_DATA获取（定制商品）
		else if (window.__INIT_DATA && window.__INIT_DATA.data) {
			const data = window.__INIT_DATA.data;
			
			// 遍历所有数据块，查找视频和详情信息
			for (let key in data) {
				const item = data[key];
				if (!item || !item.data) continue;
				
				const itemData = item.data;
				
				// 提取视频信息
				if (itemData.video) {
					result.videos.push({
						videoId: itemData.video.videoId || 0,
						title: itemData.video.title || '商品视频',
						videoUrl: itemData.video.videoUrl || '',
						coverUrl: itemData.video.coverUrl || '',
						state: itemData.video.state || 0
					});
				}
				
				// 提取详情URL
				if (itemData.detailUrl) {
					result.detailUrl = itemData.detailUrl;
				}
				
				// 提取详情图片（如果有的话）
				if (itemData.detailImages && Array.isArray(itemData.detailImages)) {
					result.detailImages = itemData.detailImages;
				}
				
				// 提取商品图片作为详情图片
				if (itemData.offerImgList && Array.isArray(itemData.offerImgList)) {
					result.detailImages = result.detailImages.concat(itemData.offerImgList);
				}
			}
		}
		
		return result;
	}`, nil)

	if err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取商品详情失败: %v", err)
		return err
	}

	if detailResult != nil {
		if detailData, ok := detailResult.(map[string]any); ok {
			var productDetails []model.ProductDetail

			// 处理视频信息
			if videos, ok := detailData["videos"].([]any); ok && len(videos) > 0 {
				var videoList []model.Video
				for _, videoInterface := range videos {
					if videoData, ok := videoInterface.(map[string]any); ok {
						video := model.Video{}

						if videoId, ok := videoData["videoId"].(float64); ok {
							video.VideoID = int64(videoId)
						}
						if title, ok := videoData["title"].(string); ok {
							video.Title = title
						}
						if videoUrl, ok := videoData["videoUrl"].(string); ok {
							video.VideoURL = videoUrl
						}
						if coverUrl, ok := videoData["coverUrl"].(string); ok {
							video.CoverURL = coverUrl
						}
						if state, ok := videoData["state"].(float64); ok {
							video.State = int(state)
						}

						if video.VideoURL != "" {
							videoList = append(videoList, video)
						}
					}
				}

				if len(videoList) > 0 {
					product.Videos = videoList
					logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取到 %d 个商品视频", len(videoList))
				}
			}

			// 处理详情URL
			if detailUrl, ok := detailData["detailUrl"].(string); ok && detailUrl != "" {
				productDetails = append(productDetails, model.ProductDetail{
					Section: "详情链接",
					Content: detailUrl,
					Images:  []string{},
				})
			}

			// 处理详情视频
			if videoId, ok := detailData["detailVideoId"].(string); ok && videoId != "" {
				productDetails = append(productDetails, model.ProductDetail{
					Section: "产品视频",
					Content: fmt.Sprintf("视频ID: %s", videoId),
					Images:  []string{},
				})
			}

			// 处理详情图片
			if images, ok := detailData["detailImages"].([]any); ok && len(images) > 0 {
				var imageUrls []string
				for _, img := range images {
					if imgStr, ok := img.(string); ok && imgStr != "" {
						imageUrls = append(imageUrls, imgStr)
					}
				}

				if len(imageUrls) > 0 {
					productDetails = append(productDetails, model.ProductDetail{
						Section: "商品详情图片",
						Content: fmt.Sprintf("包含 %d 张详情图片", len(imageUrls)),
						Images:  imageUrls,
					})
					logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取到 %d 张详情图片", len(imageUrls))
				}
			}

			product.ProductDetails = productDetails
			logger.GetGlobalLogger("crawler/alibaba1688").Debugf("商品详情提取完成，共 %d 个部分", len(productDetails))
		}
	}

	return nil
}
