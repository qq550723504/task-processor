package watermark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"net/http"
	"strings"
	"task-processor/internal/pkg/imageutil"

	"github.com/sirupsen/logrus"
)

// AIDetector AI视觉模型检测器
type AIDetector struct {
	config *Config
	logger *logrus.Logger
	client *http.Client
}

// NewAIDetector 创建AI检测器
func NewAIDetector(config *Config, logger *logrus.Logger) *AIDetector {
	return &AIDetector{
		config: config,
		logger: logger,
		client: &http.Client{},
	}
}

// Detect 使用AI检测水印
func (d *AIDetector) Detect(ctx context.Context, img image.Image) (*DetectionResult, error) {
	result := &DetectionResult{
		Method:  DetectionMethodAI,
		Regions: make([]*WatermarkRegion, 0),
	}

	// 根据配置选择AI提供商
	switch d.config.AI.VisionAPI.Provider {
	case "openai":
		return d.detectWithOpenAI(ctx, img, result)
	case "anthropic":
		return d.detectWithClaude(ctx, img, result)
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", d.config.AI.VisionAPI.Provider)
	}
}

// GetMethod 获取检测方法
func (d *AIDetector) GetMethod() DetectionMethod {
	return DetectionMethodAI
}

// detectWithOpenAI 使用OpenAI GPT-4 Vision检测
func (d *AIDetector) detectWithOpenAI(ctx context.Context, img image.Image, result *DetectionResult) (*DetectionResult, error) {
	// 将图片转换为base64
	base64Img, err := imageutil.ToBase64JPEG(img, 85)
	if err != nil {
		return nil, fmt.Errorf("图片编码失败: %w", err)
	}

	// 构建请求
	prompt := d.buildDetectionPrompt()
	requestBody := map[string]any{
		"model": d.config.AI.VisionAPI.Model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "text",
						"text": prompt,
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": fmt.Sprintf("data:image/jpeg;base64,%s", base64Img),
						},
					},
				},
			},
		},
		"max_tokens": 500,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("请求序列化失败: %w", err)
	}

	// 发送请求
	apiURL := d.config.AI.VisionAPI.BaseURL
	if apiURL == "" {
		apiURL = "https://api.openai.com/v1/chat/completions"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", d.config.AI.VisionAPI.APIKey))

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API返回错误: %d, %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}

	if len(response.Choices) == 0 {
		return result, nil
	}

	// 解析AI返回的水印信息
	content := response.Choices[0].Message.Content
	d.parseAIResponse(content, img, result)

	return result, nil
}

// detectWithClaude 使用Claude Vision检测
func (d *AIDetector) detectWithClaude(ctx context.Context, img image.Image, result *DetectionResult) (*DetectionResult, error) {
	// 将图片转换为base64
	base64Img, err := imageutil.ToBase64JPEG(img, 85)
	if err != nil {
		return nil, fmt.Errorf("图片编码失败: %w", err)
	}

	// 构建请求
	prompt := d.buildDetectionPrompt()
	requestBody := map[string]any{
		"model":      d.config.AI.VisionAPI.Model,
		"max_tokens": 500,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "image",
						"source": map[string]string{
							"type":       "base64",
							"media_type": "image/jpeg",
							"data":       base64Img,
						},
					},
					{
						"type": "text",
						"text": prompt,
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("请求序列化失败: %w", err)
	}

	// 发送请求
	apiURL := d.config.AI.VisionAPI.BaseURL
	if apiURL == "" {
		apiURL = "https://api.anthropic.com/v1/messages"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", d.config.AI.VisionAPI.APIKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API返回错误: %d, %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var response struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}

	if len(response.Content) == 0 {
		return result, nil
	}

	// 解析AI返回的水印信息
	content := response.Content[0].Text
	d.parseAIResponse(content, img, result)

	return result, nil
}

// buildDetectionPrompt 构建检测提示词
func (d *AIDetector) buildDetectionPrompt() string {
	return `请分析这张图片中是否存在水印。如果存在水印，请提供以下信息：

1. 水印位置（top_left/top_right/bottom_left/bottom_right/center/custom）
2. 水印类型（text/logo/pattern）
3. 水印的大致坐标和尺寸（x, y, width, height，以像素为单位）
4. 置信度（0-1之间的数值）
5. 水印描述

请以JSON格式返回，格式如下：
{
  "has_watermark": true/false,
  "watermarks": [
    {
      "position": "bottom_right",
      "type": "text",
      "x": 100,
      "y": 200,
      "width": 150,
      "height": 30,
      "confidence": 0.95,
      "description": "白色文字水印，内容为品牌名称"
    }
  ]
}

如果没有水印，返回：{"has_watermark": false, "watermarks": []}`
}

// parseAIResponse 解析AI响应
func (d *AIDetector) parseAIResponse(content string, img image.Image, result *DetectionResult) {
	// 尝试提取JSON
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")

	if jsonStart == -1 || jsonEnd == -1 {
		d.logger.Warnf("AI响应中未找到JSON格式: %s", content)
		return
	}

	jsonStr := content[jsonStart : jsonEnd+1]

	var aiResult struct {
		HasWatermark bool `json:"has_watermark"`
		Watermarks   []struct {
			Position    string  `json:"position"`
			Type        string  `json:"type"`
			X           int     `json:"x"`
			Y           int     `json:"y"`
			Width       int     `json:"width"`
			Height      int     `json:"height"`
			Confidence  float64 `json:"confidence"`
			Description string  `json:"description"`
		} `json:"watermarks"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &aiResult); err != nil {
		d.logger.Warnf("AI响应JSON解析失败: %v, 内容: %s", err, jsonStr)
		return
	}

	result.HasWatermark = aiResult.HasWatermark

	for _, wm := range aiResult.Watermarks {
		region := &WatermarkRegion{
			X:           wm.X,
			Y:           wm.Y,
			Width:       wm.Width,
			Height:      wm.Height,
			Position:    Position(wm.Position),
			Type:        WatermarkType(wm.Type),
			Confidence:  wm.Confidence,
			Description: wm.Description,
		}
		result.Regions = append(result.Regions, region)
	}
}
