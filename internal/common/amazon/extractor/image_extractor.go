// Package amazon 提供Amazon产品图片提取功能
package extractor

import (
	"task-processor/internal/common/amazon/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// ImageExtractor 图片提取器 - 只提取轮播图
type ImageExtractor struct{}

// NewImageExtractor 创建图片提取器实例
func NewImageExtractor() *ImageExtractor {
	return &ImageExtractor{}
}

// Extract 提取产品轮播图 - 从colorImages提取，优先hiRes，fallback到large
func (ie *ImageExtractor) Extract(page playwright.Page, product *model.Product) error {
	logrus.Infof("开始提取产品轮播图...")

	// 从colorImages提取图片，优先hiRes，如果为null则使用large并转换为高分辨率
	result, err := page.Evaluate(`() => {
		const carouselImages = [];
		
		const scripts = document.querySelectorAll('script');
		for (const script of scripts) {
			const text = script.textContent;
			if (!text) continue;
			
			// 查找包含 colorImages 和 initial 的script
			if ((text.includes("'colorImages'") || text.includes('"colorImages"')) && text.includes("'initial'")) {
				// 匹配完整的图片对象：{"hiRes":xxx,"thumb":"xxx","large":"xxx"
				const imagePattern = /\{"hiRes":(null|"[^"]*"),"thumb":"[^"]*","large":"([^"]*)"/g;
				let match;
				while ((match = imagePattern.exec(text)) !== null) {
					const hiRes = match[1];
					const large = match[2];
					
					// 优先使用 hiRes
					if (hiRes && hiRes !== 'null') {
						const url = hiRes.replace(/^"|"$/g, '');
						if (url.includes('media-amazon.com')) {
							carouselImages.push(url);
							continue;
						}
					}
					
					// hiRes 为 null 时，使用 large 并转换为高分辨率
					if (large && large.includes('media-amazon.com')) {
						let highResUrl = large;
						// _AC_.jpg -> _AC_SL1500_.jpg
						if (large.includes('._AC_.')) {
							highResUrl = large.replace('._AC_.', '._AC_SL1500_.');
						}
						carouselImages.push(highResUrl);
					}
				}
				
				if (carouselImages.length > 0) {
					return carouselImages;
				}
			}
		}
		
		// 备用方案：只提取 hiRes（兼容旧逻辑）
		for (const script of scripts) {
			const text = script.textContent;
			if (!text) continue;
			
			if (text.includes("'colorImages'") || text.includes('"colorImages"')) {
				const hiResPattern = /"hiRes"\s*:\s*"([^"]+)"/g;
				let match;
				while ((match = hiResPattern.exec(text)) !== null) {
					const url = match[1];
					if (url && url !== 'null' && url.includes('media-amazon.com')) {
						carouselImages.push(url);
					}
				}
				
				if (carouselImages.length > 0) {
					return carouselImages;
				}
			}
		}
		
		return carouselImages;
	}`)

	if err != nil {
		logrus.Errorf("提取轮播图失败: %v", err)
		return err
	}

	// 解析结果
	var images []string
	if resultSlice, ok := result.([]any); ok && len(resultSlice) > 0 {
		for _, item := range resultSlice {
			if url, ok := item.(string); ok && url != "" {
				images = append(images, url)
			}
		}
	}

	// 设置产品图片信息
	if len(images) > 0 {
		product.ImageURL = images[0]
		product.Images = images
		product.ImagesCount = len(images)
		logrus.Infof("提取到 %d 张轮播图", len(images))
	} else {
		logrus.Infof("未提取到轮播图")
	}

	return nil
}
