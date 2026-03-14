package watermark

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"net/http"
	"task-processor/internal/pkg/imageutil"

	"github.com/sirupsen/logrus"
)

// AIRemover AI模型去除器（如LaMa）
type AIRemover struct {
	config *Config
	logger *logrus.Logger
	client *http.Client
}

// NewAIRemover 创建AI去除器
func NewAIRemover(config *Config, logger *logrus.Logger) *AIRemover {
	return &AIRemover{
		config: config,
		logger: logger,
		client: &http.Client{},
	}
}

// Remove 使用AI模型去除水印
func (r *AIRemover) Remove(ctx context.Context, img image.Image, regions []*WatermarkRegion) (*RemovalResult, error) {
	result := &RemovalResult{
		Method:  RemovalMethodAI,
		Success: false,
	}

	// 检查是否配置了LaMa模型
	if r.config.AI.LamaModel.ServerURL == "" {
		return nil, fmt.Errorf("LaMa模型服务URL未配置")
	}

	// 创建mask图像（标记水印区域）
	mask := r.createMask(img, regions)

	// 调用LaMa服务
	processedImg, err := r.callLamaService(ctx, img, mask)
	if err != nil {
		r.logger.Errorf("LaMa服务调用失败: %v", err)
		return nil, err
	}

	result.Success = true
	result.Image = processedImg
	result.Quality = 0.95 // AI方法质量很高
	result.Metadata = map[string]interface{}{
		"model":   "lama",
		"regions": len(regions),
	}

	return result, nil
}

// GetMethod 获取去除方法
func (r *AIRemover) GetMethod() RemovalMethod {
	return RemovalMethodAI
}

// createMask 创建水印区域的mask
func (r *AIRemover) createMask(img image.Image, regions []*WatermarkRegion) image.Image {
	bounds := img.Bounds()
	mask := image.NewGray(bounds)

	// 将水印区域标记为白色，其他区域为黑色
	for _, region := range regions {
		for y := region.Y; y < region.Y+region.Height && y < bounds.Max.Y; y++ {
			for x := region.X; x < region.X+region.Width && x < bounds.Max.X; x++ {
				mask.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}

	return mask
}

// callLamaService 调用LaMa服务
func (r *AIRemover) callLamaService(ctx context.Context, img, mask image.Image) (image.Image, error) {
	// 将图片和mask转换为base64
	imgBase64, err := imageutil.ToBase64PNG(img)
	if err != nil {
		return nil, fmt.Errorf("图片编码失败: %w", err)
	}

	maskBase64, err := imageutil.ToBase64PNG(mask)
	if err != nil {
		return nil, fmt.Errorf("mask编码失败: %w", err)
	}

	// 构建请求
	requestBody := map[string]interface{}{
		"image": imgBase64,
		"mask":  maskBase64,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("请求序列化失败: %w", err)
	}

	// 发送请求
	req, err := http.NewRequestWithContext(ctx, "POST", r.config.AI.LamaModel.ServerURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
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
		Image string `json:"image"` // base64编码的结果图片
		Error string `json:"error,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("响应解析失败: %w", err)
	}

	if response.Error != "" {
		return nil, fmt.Errorf("服务返回错误: %s", response.Error)
	}

	// 解码结果图片
	resultImg, err := imageutil.FromBase64(response.Image)
	if err != nil {
		return nil, fmt.Errorf("结果图片解码失败: %w", err)
	}

	return resultImg, nil
}
