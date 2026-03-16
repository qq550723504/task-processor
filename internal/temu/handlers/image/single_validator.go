// Package image 提供TEMU平台单张图片验证功能
package image

import (
	"fmt"
	"path/filepath"
	"strings"
	"task-processor/internal/pkg/downloader"
	temuimage "task-processor/internal/temu/api/image"
	"task-processor/internal/temu/handlers/handlerbase"

	"github.com/sirupsen/logrus"
)

// SingleImageValidator 单张图片验证器
type SingleImageValidator struct {
	logger           *logrus.Entry
	paddingProcessor *ImagePaddingProcessor
	imageDownloader  *downloader.ImageDownloader
}

// NewSingleImageValidator 创建新的单张图片验证器
func NewSingleImageValidator() *SingleImageValidator {
	return &SingleImageValidator{
		logger:           logrus.WithField("component", "SingleImageValidator"),
		paddingProcessor: NewImagePaddingProcessor(),
		imageDownloader:  downloader.NewImageDownloader(),
	}
}

// ValidateSingleImage 验证单张图片
func (v *SingleImageValidator) ValidateSingleImage(imageURL, context string, requirement handlerbase.ImageRequirement) *temuimage.ValidationResult {
	result := &temuimage.ValidationResult{
		URL:         imageURL,
		IsValid:     true,
		Violations:  []string{},
		Suggestions: []string{},
	}

	if imageURL == "" {
		result.IsValid = false
		result.Violations = append(result.Violations, "图片URL为空")
		return result
	}

	// 验证图片格式
	format := v.getImageFormat(imageURL)
	result.Format = format
	if !v.isValidFormat(format) {
		result.IsValid = false
		result.Violations = append(result.Violations, fmt.Sprintf("不支持的图片格式: %s (仅支持JPEG, JPG, PNG)", format))
	}

	// 获取图片信息
	width, height, size, err := v.getImageInfo(imageURL)
	if err != nil {
		v.logger.Errorf("%s 获取图片信息失败，无法进行填充处理: %v", context, err)
		result.IsValid = false
		result.Violations = append(result.Violations, fmt.Sprintf("无法获取图片信息进行验证和填充: %v", err))
		return result
	}

	result.Width = width
	result.Height = height
	result.Size = size
	result.AspectRatio = float64(width) / float64(height)

	// 验证尺寸要求
	if width < requirement.MinWidth {
		result.IsValid = false
		result.Violations = append(result.Violations, fmt.Sprintf("图片宽度不足: %dpx < %dpx", width, requirement.MinWidth))
	}
	if height < requirement.MinHeight {
		result.IsValid = false
		result.Violations = append(result.Violations, fmt.Sprintf("图片高度不足: %dpx < %dpx", height, requirement.MinHeight))
	}

	// 验证宽高比 - TEMU要求严格的比例，不允许容差
	expectedRatio := requirement.AspectRatio

	// 计算如果要达到目标比例，图片应该是什么尺寸
	var targetWidth, targetHeight int
	if result.AspectRatio > expectedRatio {
		// 图片太宽，需要增加高度
		targetWidth = width
		targetHeight = int(float64(width) / expectedRatio)
	} else if result.AspectRatio < expectedRatio {
		// 图片太高，需要增加宽度
		targetHeight = height
		targetWidth = int(float64(height) * expectedRatio)
	} else {
		// 宽高比完全匹配
		targetWidth = width
		targetHeight = height
	}

	// 检查是否需要填充
	needsAspectRatioPadding := (targetWidth != width || targetHeight != height)
	needsSizePadding := width < requirement.MinWidth || height < requirement.MinHeight

	if needsAspectRatioPadding || needsSizePadding {
		paddingResult, err := v.paddingProcessor.PadImageToAspectRatio(
			imageURL,
			expectedRatio,
			requirement.MinWidth,
			requirement.MinHeight,
		)

		if err != nil {
			result.IsValid = false
			result.Violations = append(result.Violations,
				fmt.Sprintf("宽高比不符合要求: %.2f (期望: %.2f 严格匹配)，且自动填充失败",
					result.AspectRatio, expectedRatio))
		} else if paddingResult.Success {
			if paddingResult.NeedsPadding {
				result.NeedsPadding = true
				result.PaddedImage = paddingResult.PaddedImage
				result.PaddedWidth = paddingResult.NewWidth
				result.PaddedHeight = paddingResult.NewHeight
				result.Suggestions = append(result.Suggestions, "图片已自动添加白边以符合要求")
			} else {
				v.logger.Infof("%s 图片无需填充", context)
			}
			// 填充成功，图片有效
			result.IsValid = true
		}
	}

	// 验证文件大小
	maxSizeBytes := int64(requirement.MaxSizeMB * 1024 * 1024)
	if size > maxSizeBytes {
		result.IsValid = false
		result.Violations = append(result.Violations,
			fmt.Sprintf("文件大小超限: %.2fMB > %.1fMB",
				float64(size)/(1024*1024), requirement.MaxSizeMB))
	}

	// 提供优化建议
	recommendedWidth := requirement.MinWidth * 2
	recommendedHeight := requirement.MinHeight * 2
	if width < recommendedWidth || height < recommendedHeight {
		result.Suggestions = append(result.Suggestions,
			fmt.Sprintf("建议使用更高分辨率的图片（推荐: %dx%d）以提高显示质量",
				recommendedWidth, recommendedHeight))
	}

	if result.IsValid {
		v.logger.Debugf("%s 验证通过: %dx%d, %.2fMB, 宽高比%.2f",
			context, width, height, float64(size)/(1024*1024), result.AspectRatio)
	}

	return result
}

// getImageFormat 获取图片格式
func (v *SingleImageValidator) getImageFormat(imageURL string) string {
	ext := strings.ToLower(filepath.Ext(imageURL))
	switch ext {
	case ".jpg", ".jpeg":
		return "JPEG"
	case ".png":
		return "PNG"
	default:
		return ext
	}
}

// isValidFormat 检查是否为有效格式
func (v *SingleImageValidator) isValidFormat(format string) bool {
	validFormats := []string{"JPEG", "JPG", "PNG"}
	for _, valid := range validFormats {
		if strings.EqualFold(format, valid) {
			return true
		}
	}
	return false
}

// getImageInfo 获取图片信息
func (v *SingleImageValidator) getImageInfo(imageURL string) (width, height int, size int64, err error) {
	// 使用真实的下载方法获取图片信息
	return v.getImageInfoByDownload(imageURL)
}

// getImageInfoByDownload 通过统一下载器获取真实图片信息
func (v *SingleImageValidator) getImageInfoByDownload(imageURL string) (width, height int, size int64, err error) {
	return v.imageDownloader.GetImageInfo(imageURL)
}
