package downloader

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"image/jpeg"
	"image/png"
	"math/rand"
	"time"
)

// PlatformImageMutator 为不同平台生成带扰动的图片字节，避免复用完全相同的文件指纹。
type PlatformImageMutator struct {
	rand *rand.Rand // 随机数生成器
}

// NewPlatformImageMutator 创建平台图片扰动器。
func NewPlatformImageMutator() *PlatformImageMutator {
	return &PlatformImageMutator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// ProcessImageForPlatform 为特定平台处理图片，每次生成不同的MD5值
func (p *PlatformImageMutator) ProcessImageForPlatform(imageData []byte, platform string) ([]byte, error) {
	// 生成随机标识符
	randomID := p.generateRandomID()

	// 检测图片格式并处理
	format := p.detectFormat(imageData)

	switch format {
	case "jpeg":
		return p.processJPEGWithRandom(imageData, platform, randomID)
	case "png":
		return p.processPNGWithRandom(imageData, platform, randomID)
	default:
		// 对于其他格式，使用简单的字节添加方法
		return p.processOtherWithRandom(imageData, platform, randomID)
	}
}

// generateRandomID 生成随机标识符
func (p *PlatformImageMutator) generateRandomID() string {
	// 使用时间戳 + 随机数确保唯一性
	timestamp := time.Now().UnixNano()
	randomNum := p.rand.Int63()
	return fmt.Sprintf("%d_%d", timestamp, randomNum)
}

// processJPEGWithRandom 处理JPEG图片，添加随机性
func (p *PlatformImageMutator) processJPEGWithRandom(imageData []byte, platform, randomID string) ([]byte, error) {
	// 解码图片
	img, err := jpeg.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("解码JPEG失败: %w", err)
	}

	// 使用随机质量参数（在基础质量上加减随机值）
	baseQuality := p.getPlatformQuality(platform)
	randomOffset := p.rand.Intn(5) - 2 // -2 到 +2 的随机偏移
	quality := baseQuality + randomOffset

	// 确保质量在合理范围内
	if quality < 85 {
		quality = 85
	}
	if quality > 98 {
		quality = 98
	}

	// 重新编码
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, fmt.Errorf("编码JPEG失败: %w", err)
	}

	// 在文件末尾添加随机标识
	result := buf.Bytes()
	randomComment := fmt.Sprintf("_%s_%s_", platform, randomID)
	result = append(result, []byte(randomComment)...)

	return result, nil
}

// processPNGWithRandom 处理PNG图片，添加随机性
func (p *PlatformImageMutator) processPNGWithRandom(imageData []byte, platform, randomID string) ([]byte, error) {
	// 解码图片
	img, err := png.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("解码PNG失败: %w", err)
	}

	// 重新编码PNG
	var buf bytes.Buffer
	err = png.Encode(&buf, img)
	if err != nil {
		return nil, fmt.Errorf("编码PNG失败: %w", err)
	}

	// 在PNG文件末尾添加随机标识
	result := buf.Bytes()
	randomComment := fmt.Sprintf("_%s_%s_", platform, randomID)
	result = append(result, []byte(randomComment)...)

	return result, nil
}

// processOtherWithRandom 处理其他格式图片，添加随机性
func (p *PlatformImageMutator) processOtherWithRandom(imageData []byte, platform, randomID string) ([]byte, error) {
	// 在文件末尾添加随机标识
	randomComment := fmt.Sprintf("_%s_%s_", platform, randomID)
	return append(imageData, []byte(randomComment)...), nil
}

// getPlatformQuality 根据平台获取基础JPEG质量参数
func (p *PlatformImageMutator) getPlatformQuality(platform string) int {
	qualityMap := map[string]int{
		"amazon": 95,
		"shein":  94,
		"temu":   93,
		"ebay":   92,
		"shopee": 91,
	}

	if quality, exists := qualityMap[platform]; exists {
		return quality
	}

	// 默认质量
	return 90
}

// detectFormat 检测图片格式
func (p *PlatformImageMutator) detectFormat(data []byte) string {
	if len(data) < 12 {
		return ""
	}

	// JPEG
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return "jpeg"
	}

	// PNG
	if len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return "png"
	}

	// GIF
	if len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a") {
		return "gif"
	}

	// WebP
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "webp"
	}

	return "unknown"
}

// GetImageMD5 计算图片的MD5值
func (p *PlatformImageMutator) GetImageMD5(imageData []byte) string {
	hash := md5.Sum(imageData)
	return fmt.Sprintf("%x", hash)
}

// ProcessAndGetMD5 处理图片并返回新的MD5值
func (p *PlatformImageMutator) ProcessAndGetMD5(imageData []byte, platform string) ([]byte, string, error) {
	processedData, err := p.ProcessImageForPlatform(imageData, platform)
	if err != nil {
		return nil, "", err
	}

	md5Hash := p.GetImageMD5(processedData)
	return processedData, md5Hash, nil
}

// ProcessWithMultipleStrategies 使用多种策略处理图片，确保每次都不同
func (p *PlatformImageMutator) ProcessWithMultipleStrategies(imageData []byte, platform string) ([]byte, string, error) {
	// 策略1: 随机质量 + 随机标识
	processedData, err := p.ProcessImageForPlatform(imageData, platform)
	if err != nil {
		return nil, "", err
	}

	// 策略2: 添加额外的随机字节
	extraRandom := make([]byte, p.rand.Intn(10)+5) // 5-14个随机字节
	p.rand.Read(extraRandom)

	// 将随机字节编码为可见字符
	randomSuffix := fmt.Sprintf("_r_%x_", extraRandom)
	finalData := append(processedData, []byte(randomSuffix)...)

	md5Hash := p.GetImageMD5(finalData)
	return finalData, md5Hash, nil
}
