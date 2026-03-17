// Package extractor 提供Amazon产品视频信息提取功能
package extractor

import (
	"fmt"
	"regexp"
	"strings"
	"task-processor/internal/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// VideoExtractor 视频信息提取器
type VideoExtractor struct{}

// NewVideoExtractor 创建视频提取器实例
func NewVideoExtractor() *VideoExtractor {
	return &VideoExtractor{}
}

// Extract 提取产品视频信息
func (e *VideoExtractor) Extract(page playwright.Page, product *model.Product) error {
	logrus.Debug("开始提取视频信息")

	// 使用JavaScript提取视频URL
	videos, err := e.extractVideoURLs(page)
	if err != nil {
		logrus.Debugf("提取视频URL失败: %v", err)
		return nil // 视频不是必需的，不返回错误
	}

	if len(videos) > 0 {
		product.VideoURLs = videos
		logrus.Infof("成功提取 %d 个视频", len(videos))
	} else {
		logrus.Debug("未找到产品视频")
	}

	return nil
}

// extractVideoURLs 从页面提取视频URL列表
func (e *VideoExtractor) extractVideoURLs(page playwright.Page) ([]model.VideoInfo, error) {
	// JavaScript代码：查找所有视频相关的网络请求
	jsCode := `() => {
		const videos = [];
		const videoMap = new Map();
		
		// 方法1: 从video标签获取
		const videoElements = document.querySelectorAll('video');
		videoElements.forEach((video, index) => {
			const src = video.src || video.currentSrc;
			if (src) {
				const videoId = 'video_' + index;
				videoMap.set(videoId, {
					video_id: videoId,
					video_url: src,
					thumbnail_url: video.poster || '',
					title: video.title || video.getAttribute('aria-label') || ''
				});
			}
		});
		
		// 方法2: 从页面脚本中查找m3u8链接
		const scripts = document.querySelectorAll('script');
		const m3u8Regex = /https:\/\/[^"'\s]+\.m3u8/g;
		const videoIdRegex = /([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})/i;
		
		scripts.forEach(script => {
			const content = script.textContent || script.innerHTML;
			const matches = content.match(m3u8Regex);
			
			if (matches) {
				matches.forEach(url => {
					// 提取视频ID
					const idMatch = url.match(videoIdRegex);
					const videoId = idMatch ? idMatch[1] : '';
					
					if (videoId && !videoMap.has(videoId)) {
						videoMap.set(videoId, {
							video_id: videoId,
							video_url: url,
							thumbnail_url: '',
							title: ''
						});
					}
				});
			}
		});
		
		// 方法3: 从data属性中查找
		const videoContainers = document.querySelectorAll('[data-video-url], [data-video], [data-src*="video"]');
		videoContainers.forEach((container, index) => {
			const videoUrl = container.getAttribute('data-video-url') || 
			                 container.getAttribute('data-video') || 
			                 container.getAttribute('data-src');
			
			if (videoUrl && videoUrl.includes('m3u8')) {
				const videoId = 'data_video_' + index;
				if (!videoMap.has(videoId)) {
					videoMap.set(videoId, {
						video_id: videoId,
						video_url: videoUrl,
						thumbnail_url: '',
						title: ''
					});
				}
			}
		});
		
		// 转换为数组
		videoMap.forEach(video => videos.push(video));
		
		return videos;
	}`

	result, err := page.Evaluate(jsCode)
	if err != nil {
		return nil, fmt.Errorf("执行JavaScript提取视频失败: %w", err)
	}

	// 解析结果
	videos := e.parseVideoResult(result)

	// 如果没有找到视频，尝试从网络请求中查找
	if len(videos) == 0 {
		videos = e.extractFromNetworkRequests(page)
	}

	return videos, nil
}

// parseVideoResult 解析JavaScript返回的视频数据
func (e *VideoExtractor) parseVideoResult(result any) []model.VideoInfo {
	videos := make([]model.VideoInfo, 0)

	if result == nil {
		return videos
	}

	// 尝试转换为数组
	videoArray, ok := result.([]any)
	if !ok {
		logrus.Debug("视频数据格式不正确")
		return videos
	}

	for _, item := range videoArray {
		videoMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		video := model.VideoInfo{}

		if videoID, ok := videoMap["video_id"].(string); ok {
			video.VideoID = videoID
		}

		if videoURL, ok := videoMap["video_url"].(string); ok {
			video.VideoURL = e.cleanVideoURL(videoURL)
		}

		if thumbnailURL, ok := videoMap["thumbnail_url"].(string); ok {
			video.ThumbnailURL = thumbnailURL
		}

		if title, ok := videoMap["title"].(string); ok {
			video.Title = title
		}

		// 只添加有效的视频URL
		if video.VideoURL != "" && e.isValidVideoURL(video.VideoURL) {
			videos = append(videos, video)
		}
	}

	return videos
}

// extractFromNetworkRequests 从页面源码中提取视频URL
func (e *VideoExtractor) extractFromNetworkRequests(page playwright.Page) []model.VideoInfo {
	videos := make([]model.VideoInfo, 0)

	content, err := page.Content()
	if err != nil {
		return videos
	}

	// 使用正则表达式查找m3u8链接
	m3u8Regex := regexp.MustCompile(`https://[^"'\s]+\.m3u8`)
	videoIDRegex := regexp.MustCompile(`([a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})`)

	matches := m3u8Regex.FindAllString(content, -1)

	// 去重
	uniqueURLs := make(map[string]bool)
	for _, url := range matches {
		if !uniqueURLs[url] && e.isValidVideoURL(url) {
			uniqueURLs[url] = true

			// 提取视频ID
			idMatches := videoIDRegex.FindStringSubmatch(url)
			videoID := ""
			if len(idMatches) > 1 {
				videoID = idMatches[1]
			}

			videos = append(videos, model.VideoInfo{
				VideoID:  videoID,
				VideoURL: e.cleanVideoURL(url),
			})
		}
	}

	return videos
}

// cleanVideoURL 清理视频URL
func (e *VideoExtractor) cleanVideoURL(url string) string {
	// 移除多余的空格和换行符
	url = strings.TrimSpace(url)
	url = strings.ReplaceAll(url, "\n", "")
	url = strings.ReplaceAll(url, "\r", "")

	return url
}

// isValidVideoURL 检查视频URL是否有效
func (e *VideoExtractor) isValidVideoURL(url string) bool {
	if url == "" {
		return false
	}

	// 检查是否为有效的HTTP(S) URL
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return false
	}

	// 检查是否包含视频文件扩展名
	validExtensions := []string{".m3u8", ".mp4", ".webm", ".mov"}
	for _, ext := range validExtensions {
		if strings.Contains(url, ext) {
			return true
		}
	}

	return false
}
