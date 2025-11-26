package amazon

import (
	"fmt"
	"strings"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// ImageExtractor 图片提取器
type ImageExtractor struct{}

// NewImageExtractor 创建图片提取器实例
func NewImageExtractor() *ImageExtractor {
	return &ImageExtractor{}
}

// Extract 提取产品图片 - 只提取当前选中变体的图片
func (ie *ImageExtractor) Extract(page playwright.Page, product *Product) error {
	var images []string
	var mainImageURL string

	logrus.Infof("开始提取产品图片...")

	// 策略：从页面JavaScript中的colorImages对象提取当前变体的图片
	// colorImages.initial 包含了当前变体的所有图片及其缩略图的映射关系
	result, err := page.Evaluate(`() => {
		const imageData = [];
		
		// 方法1：从页面script标签中提取colorImages对象
		// colorImages.initial 包含了当前变体的图片数据，格式为：
		// [{"hiRes":"...", "thumb":"...", "large":"...", "main":{...}}, ...]
		const scripts = document.querySelectorAll('script');
		
		for (const script of scripts) {
			const text = script.textContent;
			
			// 查找包含 colorImages 的script
			if (text.includes("'colorImages'") && text.includes("'initial'")) {
				// 使用正则提取每个图片对象的 hiRes URL
				// 这些是真正的高分辨率图片URL，不需要手动构造
				const pattern = /\{"hiRes":"([^"]+)"/g;
				let match;
				
				while ((match = pattern.exec(text)) !== null) {
					const hiResUrl = match[1];
					if (hiResUrl && !imageData.includes(hiResUrl)) {
						imageData.push(hiResUrl);
					}
				}
				
				if (imageData.length > 0) {
					return imageData;
				}
			}
		}
		
		// 方法2：如果方法1失败，从缩略图列表和主图提取
		const thumbnails = document.querySelectorAll('#altImages li.imageThumbnail');
		const mainImg = document.querySelector('#landingImage, #imgBlkFront');
		
		// 先添加主图的高分辨率版本
		if (mainImg) {
			const hiRes = mainImg.getAttribute('data-old-hires');
			if (hiRes && !imageData.includes(hiRes)) {
				imageData.push(hiRes);
			}
		}
		
		// 然后尝试从data-a-dynamic-image获取其他尺寸
		if (mainImg && imageData.length === 0) {
			const dynamicImage = mainImg.getAttribute('data-a-dynamic-image');
			if (dynamicImage) {
				try {
					const imageObj = JSON.parse(dynamicImage);
					// 获取最大尺寸的图片
					let maxSize = 0;
					let maxUrl = '';
					for (const url in imageObj) {
						const [width, height] = imageObj[url];
						const size = width * height;
						if (size > maxSize) {
							maxSize = size;
							maxUrl = url;
						}
					}
					if (maxUrl && !imageData.includes(maxUrl)) {
						imageData.push(maxUrl);
					}
				} catch (e) {
					console.log('解析 data-a-dynamic-image 失败:', e);
				}
			}
		}
		
		return imageData;
	}`)

	if err != nil {
		logrus.Infof("提取图片失败: %v", err)
		return err
	}

	// 解析结果
	if resultSlice, ok := result.([]interface{}); ok && len(resultSlice) > 0 {
		logrus.Infof("找到 %d 个图片URL", len(resultSlice))
		for _, item := range resultSlice {
			if url, ok := item.(string); ok && url != "" {
				if !ie.containsURL(images, url) {
					images = append(images, url)
					if mainImageURL == "" {
						mainImageURL = url
					}
				}
			}
		}
	}

	// 如果没有提取到图片，使用备用方法
	if len(images) == 0 {
		logrus.Infof("未提取到图片，尝试备用方法")
		return ie.extractFromMainImage(page, product)
	}

	// 设置产品图片信息
	product.ImageURL = mainImageURL
	product.Images = images
	product.ImagesCount = len(images)

	logrus.Infof("最终提取到 %d 张产品图片", len(images))

	return nil
}

// extractFromMainImage 从主图区域提取图片（备用方法）
func (ie *ImageExtractor) extractFromMainImage(page playwright.Page, product *Product) error {
	var images []string
	var mainImageURL string

	mainImageElement, err := page.QuerySelector("#landingImage")
	if err != nil || mainImageElement == nil {
		logrus.Infof("未找到主图元素")
		return nil
	}

	// 获取 data-a-dynamic-image
	dynamicImage, err := mainImageElement.GetAttribute("data-a-dynamic-image")
	if err == nil && dynamicImage != "" {
		urls := ie.parseDynamicImageJSON(dynamicImage)
		for _, url := range urls {
			if cleanURL := ie.cleanImageURL(url); cleanURL != "" && ie.isMainProductImage(cleanURL) && !ie.containsURL(images, cleanURL) {
				images = append(images, cleanURL)
				if mainImageURL == "" {
					mainImageURL = cleanURL
				}
			}
		}
	}

	// 如果还是没有，尝试 data-old-hires
	if len(images) == 0 {
		hiRes, err := mainImageElement.GetAttribute("data-old-hires")
		if err == nil && hiRes != "" {
			if cleanURL := ie.cleanImageURL(hiRes); cleanURL != "" && ie.isMainProductImage(cleanURL) {
				mainImageURL = cleanURL
				images = append(images, cleanURL)
			}
		}
	}

	// 最后使用 src
	if len(images) == 0 {
		src, err := mainImageElement.GetAttribute("src")
		if err == nil && src != "" {
			if cleanURL := ie.cleanImageURL(src); cleanURL != "" && ie.isMainProductImage(cleanURL) {
				mainImageURL = cleanURL
				images = append(images, cleanURL)
			}
		}
	}

	product.ImageURL = mainImageURL
	product.Images = images
	product.ImagesCount = len(images)

	logrus.Infof("从主图提取到 %d 张产品图片", len(images))
	return nil
}

// cleanImageURL 清理和标准化图片URL
func (ie *ImageExtractor) cleanImageURL(url string) string {
	if url == "" {
		return ""
	}

	// 移除data:image前缀
	if strings.HasPrefix(url, "data:image") {
		return ""
	}

	// 确保是完整的URL
	if strings.HasPrefix(url, "//") {
		url = "https:" + url
	} else if strings.HasPrefix(url, "/") {
		url = "https://m.media-amazon.com" + url
	}

	// 对于已经是高分辨率格式的URL，直接返回，不做清理
	// 例如：_AC_SL1500_.jpg, _AC_.jpg 等
	if strings.Contains(url, "_SL1500_") || strings.Contains(url, "_AC_.jpg") {
		return url
	}

	// 移除URL中的尺寸参数，获取原始大小
	// 处理两种格式：
	// 1. 带 ._ 的格式：image._AC_SX300_.jpg -> image.jpg
	// 2. 带 .SS 的格式：image.SS40_BG85,85,85_BR-120_PKdp-play-icon-overlay__.jpg -> image.jpg
	if strings.Contains(url, "._") {
		parts := strings.Split(url, "._")
		if len(parts) >= 2 {
			// 保留文件扩展名
			lastPart := parts[len(parts)-1]
			if strings.Contains(lastPart, ".") {
				ext := "." + strings.Split(lastPart, ".")[len(strings.Split(lastPart, "."))-1]
				url = parts[0] + ext
			}
		}
	} else if strings.Contains(url, ".SS") || strings.Contains(url, ".AC") {
		// 处理 .SS40_xxx.jpg 或 .AC_xxx.jpg 格式
		// 找到图片ID和扩展名
		parts := strings.Split(url, "/I/")
		if len(parts) >= 2 {
			imageIDPart := parts[1]
			// 提取纯图片ID（第一个点之前的部分）
			if dotIdx := strings.Index(imageIDPart, "."); dotIdx > 0 {
				imageID := imageIDPart[:dotIdx]
				// 提取扩展名（最后一个点之后的部分）
				if lastDotIdx := strings.LastIndex(imageIDPart, "."); lastDotIdx > dotIdx {
					ext := imageIDPart[lastDotIdx:]
					url = parts[0] + "/I/" + imageID + ext
				}
			}
		}
	}

	return url
}

// containsURL 检查URL是否已存在于列表中
func (ie *ImageExtractor) containsURL(urls []string, target string) bool {
	for _, url := range urls {
		if url == target {
			return true
		}
	}
	return false
}

// parseDynamicImageJSON 解析动态图片JSON数据，返回按尺寸排序的URL列表（最大的在前）
func (ie *ImageExtractor) parseDynamicImageJSON(jsonStr string) []string {
	var urls []string
	type imageSize struct {
		url    string
		width  int
		height int
		size   int // width * height
	}
	var imageSizes []imageSize

	// 简单的JSON解析，提取URL和尺寸
	// 格式示例: {"https://url1.jpg":[width,height],"https://url2.jpg":[width,height]}
	jsonStr = strings.TrimSpace(jsonStr)
	jsonStr = strings.Trim(jsonStr, "{}")

	// 按逗号分割，但要注意URL中可能包含逗号
	parts := strings.Split(jsonStr, "\":")
	for i := 0; i < len(parts)-1; i++ {
		urlPart := strings.Trim(parts[i], "\",")
		if !strings.HasPrefix(urlPart, "https://") {
			// 找到https://的位置
			idx := strings.Index(urlPart, "https://")
			if idx >= 0 {
				urlPart = urlPart[idx:]
			} else {
				continue
			}
		}

		// 提取尺寸信息
		sizePart := parts[i+1]
		if strings.Contains(sizePart, "[") && strings.Contains(sizePart, "]") {
			sizeStr := strings.Split(sizePart, "]")[0]
			sizeStr = strings.Trim(sizeStr, "[], ")
			dimensions := strings.Split(sizeStr, ",")
			if len(dimensions) >= 2 {
				var width, height int
				fmt.Sscanf(strings.TrimSpace(dimensions[0]), "%d", &width)
				fmt.Sscanf(strings.TrimSpace(dimensions[1]), "%d", &height)
				imageSizes = append(imageSizes, imageSize{
					url:    urlPart,
					width:  width,
					height: height,
					size:   width * height,
				})
			}
		}
	}

	// 按尺寸排序（从大到小）
	for i := 0; i < len(imageSizes); i++ {
		for j := i + 1; j < len(imageSizes); j++ {
			if imageSizes[j].size > imageSizes[i].size {
				imageSizes[i], imageSizes[j] = imageSizes[j], imageSizes[i]
			}
		}
	}

	// 提取排序后的URL
	for _, img := range imageSizes {
		urls = append(urls, img.url)
	}

	return urls
}

// isMainProductImage 判断是否为主产品图片
func (ie *ImageExtractor) isMainProductImage(url string) bool {
	// 只保留主产品图片，排除其他类型的图片
	if strings.Contains(url, "https://m.media-amazon.com/images/I/") {
		// 排除用户评论图片
		if strings.Contains(url, "aicid=community-reviews") {
			return false
		}

		// 排除视频封面图（包含各种play-button模式）
		if strings.Contains(url, "play-button") ||
			strings.Contains(url, "PKplay") ||
			strings.Contains(url, "PKdp-play") {
			return false
		}

		// 只保留大图，过滤小图
		// 通过图片ID的第一个数字来判断尺寸：8x和7x开头的是大图，6x、5x、4x开头的是大图，3x开头的是小图
		if ie.isLargeImage(url) {
			return true
		}

		return false
	}

	// 排除A+内容图片
	if strings.Contains(url, "aplus-media-library-service-media") {
		return false
	}

	// 排除其他非主产品图片
	if strings.Contains(url, "/sash/") ||
		strings.Contains(url, "icon") ||
		strings.Contains(url, "badge") ||
		strings.Contains(url, "logo") {
		return false
	}

	return false
}

// isLargeImage 判断是否为大图（根据图片ID的第一个数字）
func (ie *ImageExtractor) isLargeImage(url string) bool {
	// 提取图片ID，格式通常为：https://m.media-amazon.com/images/I/81v7BIl-atL.jpg
	parts := strings.Split(url, "/I/")
	if len(parts) < 2 {
		return false
	}

	imageID := parts[1]
	if len(imageID) < 2 {
		return false
	}

	// 检查图片ID的前两个字符，8x、7x、6x、5x、4x开头的是大图，3x开头的是小图
	firstTwo := imageID[:2]
	if strings.HasPrefix(firstTwo, "8") || strings.HasPrefix(firstTwo, "7") || strings.HasPrefix(firstTwo, "6") || strings.HasPrefix(firstTwo, "5") || strings.HasPrefix(firstTwo, "4") {
		return true
	}

	return false
}
