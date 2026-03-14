package watermark_test

import (
	"context"
	"fmt"
	"image"
	"image/color"

	"task-processor/internal/pkg/watermark"

	"github.com/sirupsen/logrus"
)

// Example_basicUsage 基础使用示例
func Example_basicUsage() {
	// 创建配置
	config := watermark.DefaultConfig()
	config.Detection.Method = watermark.DetectionMethodTraditional
	config.Removal.Method = watermark.RemovalMethodInpaint

	// 创建处理器
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel) // 减少日志输出
	processor := watermark.NewProcessor(config, logger)

	// 创建测试图片
	img := createTestImage()

	// 处理图片
	ctx := context.Background()
	result, err := processor.Process(ctx, img)
	if err != nil {
		fmt.Printf("处理失败: %v\n", err)
		return
	}

	fmt.Printf("检测到水印: %v\n", result.Detection.HasWatermark)
	if result.Removal != nil {
		fmt.Printf("去除成功: %v\n", result.Removal.Success)
	}

	// Output:
	// 检测到水印: false
	// 去除成功: true
}

// Example_detectionOnly 仅检测示例
func Example_detectionOnly() {
	config := watermark.DefaultConfig()
	processor := watermark.NewProcessor(config, logrus.New())

	img := createTestImage()
	ctx := context.Background()

	result, err := processor.DetectOnly(ctx, img)
	if err != nil {
		fmt.Printf("检测失败: %v\n", err)
		return
	}

	fmt.Printf("检测方法: %s\n", result.Method)
	fmt.Printf("发现水印: %v\n", result.HasWatermark)
	fmt.Printf("区域数量: %d\n", len(result.Regions))

	// Output:
	// 检测方法: hybrid
	// 发现水印: false
	// 区域数量: 0
}

// Example_customRegions 自定义区域去除示例
func Example_customRegions() {
	config := watermark.DefaultConfig()
	processor := watermark.NewProcessor(config, logrus.New())

	img := createTestImage()
	ctx := context.Background()

	// 手动指定水印区域
	regions := []*watermark.WatermarkRegion{
		{
			X:          10,
			Y:          10,
			Width:      50,
			Height:     20,
			Type:       watermark.WatermarkTypeText,
			Position:   watermark.PositionTopLeft,
			Confidence: 1.0,
		},
	}

	result, err := processor.RemoveOnly(ctx, img, regions)
	if err != nil {
		fmt.Printf("去除失败: %v\n", err)
		return
	}

	fmt.Printf("去除方法: %s\n", result.Method)
	fmt.Printf("去除成功: %v\n", result.Success)
	fmt.Printf("处理质量: %.2f\n", result.Quality)

	// Output:
	// 去除方法: inpaint
	// 去除成功: true
	// 处理质量: 0.95
}

// Example_configUpdate 动态更新配置示例
func Example_configUpdate() {
	config := watermark.DefaultConfig()
	processor := watermark.NewProcessor(config, logrus.New())

	fmt.Printf("初始检测方法: %s\n", processor.GetConfig().Detection.Method)

	// 更新配置
	newConfig := watermark.DefaultConfig()
	newConfig.Detection.Method = watermark.DetectionMethodTraditional
	newConfig.Removal.Method = watermark.RemovalMethodBlur
	processor.UpdateConfig(newConfig)

	fmt.Printf("更新后检测方法: %s\n", processor.GetConfig().Detection.Method)
	fmt.Printf("更新后去除方法: %s\n", processor.GetConfig().Removal.Method)

	// Output:
	// 初始检测方法: hybrid
	// 更新后检测方法: traditional
	// 更新后去除方法: blur
}

// Example_differentMethods 不同方法对比示例
func Example_differentMethods() {
	img := createTestImage()
	ctx := context.Background()
	logger := logrus.New()
	logger.SetLevel(logrus.WarnLevel)

	methods := []watermark.DetectionMethod{
		watermark.DetectionMethodTraditional,
		watermark.DetectionMethodHybrid,
	}

	for _, method := range methods {
		config := watermark.DefaultConfig()
		config.Detection.Method = method
		processor := watermark.NewProcessor(config, logger)

		result, _ := processor.DetectOnly(ctx, img)
		fmt.Printf("方法: %s, 耗时: %.3fs\n", method, result.ProcessTime)
	}

	// Output:
	// 方法: traditional, 耗时: 0.000s
	// 方法: hybrid, 耗时: 0.000s
}

// createTestImage 创建测试图片
func createTestImage() image.Image {
	width, height := 200, 150
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 填充白色背景
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.White)
		}
	}

	return img
}
