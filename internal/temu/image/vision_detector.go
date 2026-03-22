// Package image 提供TEMU平台Vision API检测功能
package image

import (
	"context"
	"fmt"
	"image"
	"strings"
	"time"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"

	openaiClient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/pkg/imagex"
	"task-processor/internal/pkg/timeout"
)

// VisionDetector Vision API检测器
type VisionDetector struct {
	logger       *logrus.Entry
	openaiClient *openaiClient.Client
}

// NewVisionDetector 创建Vision检测器
func NewVisionDetector(client *openaiClient.Client) *VisionDetector {
	return &VisionDetector{
		logger:       logger.GetGlobalLogger("VisionDetector"),
		openaiClient: client,
	}
}

// HasDimensionAnnotationWithDetails 检测图片是否已包含尺寸标注（带详细信息）
func (v *VisionDetector) HasDimensionAnnotationWithDetails(ctx context.Context, img image.Image) (bool, string) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 使用 OpenAI Vision API 检测
	if v.openaiClient != nil {
		hasAnnotation, visionDetails, err := v.detectWithVisionAPI(ctx, img)
		if err == nil {
			return hasAnnotation, fmt.Sprintf("Vision API检测: %s (图片尺寸: %dx%d)", visionDetails, width, height)
		}
		v.logger.Errorf("Vision API检测失败: %v", err)
		return false, fmt.Sprintf("检测失败: %v (图片尺寸: %dx%d)", err, width, height)
	}

	// 如果没有配置OpenAI客户端，返回未检测到
	v.logger.Warn("未配置OpenAI客户端，无法检测尺寸标注")
	return false, fmt.Sprintf("未配置Vision API (图片尺寸: %dx%d)", width, height)
}

// detectWithVisionAPI 使用OpenAI Vision API检测图片中的尺寸标注
func (v *VisionDetector) detectWithVisionAPI(ctx context.Context, img image.Image) (bool, string, error) {
	// 将图片编码为base64
	base64Image, err := imagex.ToBase64PNG(img)
	if err != nil {
		return false, "", fmt.Errorf("编码图片失败: %w", err)
	}

	// 使用传入的context，如果没有超时则添加默认超时
	ctxWithTimeout := ctx
	if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > timeout.AIShortTimeout {
		var cancel context.CancelFunc
		ctxWithTimeout, cancel = timeout.WithAIShortTimeout(ctx)
		defer cancel()
	}

	// 构建简化的文本请求（避免复杂的多媒体消息）
	prompt := fmt.Sprintf(`请分析这张产品图片，判断是否包含尺寸标注信息。

尺寸标注特征：
1. 包含尺寸单位：cm, mm, inch, in, 厘米, 英寸等
2. 尺寸数字：如2.8cm/1.1inch, 5.2cm, 10mm等
3. 标注线条、箭头、引线等图形元素
4. "Product size", "Size", "Dimensions", "尺寸"等标题

图片数据：data:image/png;base64,%s

请只回答 "YES" 或 "NO"，然后简要说明理由（不超过30字）。
格式：YES/NO - 理由`, base64Image)

	messages := []openaiClient.ChatCompletionMessage{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	maxTokens := 100
	temperature := float32(0.1)

	req := &openaiClient.ChatCompletionRequest{
		Messages:    messages,
		MaxTokens:   &maxTokens,
		Temperature: &temperature,
	}

	resp, err := v.openaiClient.CreateChatCompletion(ctxWithTimeout, req)
	if err != nil {
		return false, "", fmt.Errorf("调用Vision API失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return false, "", fmt.Errorf("vision API返回空响应")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	v.logger.Infof("Vision API响应: %s", content)

	// 解析响应
	hasAnnotation := strings.HasPrefix(strings.ToUpper(content), "YES")

	// 提取理由
	parts := strings.SplitN(content, "-", 2)
	reason := content
	if len(parts) > 1 {
		reason = strings.TrimSpace(parts[1])
	}

	// 直接使用AI的判断结果
	if hasAnnotation {
		return true, fmt.Sprintf("有尺寸标注 - %s", reason), nil
	}

	return false, fmt.Sprintf("无尺寸标注 - %s", reason), nil
}
