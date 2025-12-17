package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"golang.org/x/image/font/gofont/goregular"

	openaiClient "task-processor/internal/clients/openai"
)

// ImageDimensionAnnotator 图片尺寸标注器
type ImageDimensionAnnotator struct {
	logger       *logrus.Entry
	openaiClient *openaiClient.Client
}

// NewImageDimensionAnnotator 创建新的图片尺寸标注器
func NewImageDimensionAnnotator() *ImageDimensionAnnotator {
	return &ImageDimensionAnnotator{
		logger: logrus.WithField("component", "ImageDimensionAnnotator"),
	}
}

// NewImageDimensionAnnotatorWithOpenAI 创建带OpenAI支持的图片尺寸标注器
func NewImageDimensionAnnotatorWithOpenAI(client *openaiClient.Client) *ImageDimensionAnnotator {
	return &ImageDimensionAnnotator{
		logger:       logrus.WithField("component", "ImageDimensionAnnotator"),
		openaiClient: client,
	}
}

// DimensionInfo 尺寸信息
type DimensionInfo struct {
	Length string // 长度（英寸）
	Width  string // 宽度（英寸）
	Height string // 高度（英寸）
}

// AnnotateImage 为图片添加尺寸标注（从URL下载）
func (a *ImageDimensionAnnotator) AnnotateImage(imageURL string, dimensions DimensionInfo) ([]byte, error) {
	a.logger.Infof("开始为图片添加尺寸标注: %s", imageURL)

	// 1. 下载图片
	img, format, err := a.downloadImage(imageURL)
	if err != nil {
		return nil, fmt.Errorf("下载图片失败: %w", err)
	}

	return a.annotateImageInternal(img, format, dimensions)
}

// AnnotateImageFromBytes 为图片添加尺寸标注（从字节数据）
func (a *ImageDimensionAnnotator) AnnotateImageFromBytes(imageData []byte, dimensions DimensionInfo) ([]byte, error) {
	a.logger.Info("开始为图片添加尺寸标注（使用字节数据）")

	// 1. 解码图片
	img, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	return a.annotateImageInternal(img, format, dimensions)
}

// annotateImageInternal 内部标注方法
func (a *ImageDimensionAnnotator) annotateImageInternal(img image.Image, format string, dimensions DimensionInfo) ([]byte, error) {
	// 1. 暂时跳过检测，直接添加标注
	a.logger.Info("⚠️ 跳过检测，直接添加尺寸标注")

	// 2. 创建可绘制的图片
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// 3. 绘制尺寸标注
	if err := a.drawDimensionAnnotations(rgba, dimensions); err != nil {
		return nil, fmt.Errorf("绘制标注失败: %w", err)
	}

	// 4. 编码为字节
	buf := new(bytes.Buffer)
	switch format {
	case "jpeg", "jpg":
		if err := jpeg.Encode(buf, rgba, &jpeg.Options{Quality: 95}); err != nil {
			return nil, fmt.Errorf("编码JPEG失败: %w", err)
		}
	case "png":
		if err := png.Encode(buf, rgba); err != nil {
			return nil, fmt.Errorf("编码PNG失败: %w", err)
		}
	default:
		// 默认使用PNG
		if err := png.Encode(buf, rgba); err != nil {
			return nil, fmt.Errorf("编码PNG失败: %w", err)
		}
	}

	a.logger.Infof("✅ 尺寸标注完成，图片大小: %d bytes", buf.Len())
	return buf.Bytes(), nil
}

// DownloadImage 下载图片（公开方法）
func (a *ImageDimensionAnnotator) DownloadImage(imageURL string) (image.Image, string, error) {
	return a.downloadImage(imageURL)
}

// downloadImage 下载图片（内部方法）
func (a *ImageDimensionAnnotator) downloadImage(imageURL string) (image.Image, string, error) {
	resp, err := http.Get(imageURL)
	if err != nil {
		return nil, "", fmt.Errorf("HTTP请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP状态码错误: %d", resp.StatusCode)
	}

	// 读取图片数据
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("读取响应失败: %w", err)
	}

	// 解码图片
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("解码图片失败: %w", err)
	}

	return img, format, nil
}

// drawDimensionAnnotations 绘制尺寸标注（带箭头）
func (a *ImageDimensionAnnotator) drawDimensionAnnotations(img *image.RGBA, dimensions DimensionInfo) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 定义颜色
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	yellow := color.RGBA{R: 255, G: 255, B: 0, A: 255}

	// 计算产品区域（假设产品在图片中心，占80%）
	productMargin := width / 10
	productLeft := productMargin
	productRight := width - productMargin
	productTop := productMargin
	productBottom := height - productMargin

	// 绘制宽度标注（顶部水平箭头）
	if dimensions.Width != "" && dimensions.Width != "0" {
		y := productTop - 30
		a.drawHorizontalArrow(img, productLeft, y, productRight, y, yellow)
		a.drawTextWithBackground(img, (productLeft+productRight)/2-50, y-25,
			fmt.Sprintf("W: %s\"", dimensions.Width), white, 18.0)
	}

	// 绘制长度标注（右侧垂直箭头）
	if dimensions.Length != "" && dimensions.Length != "0" {
		x := productRight + 30
		a.drawVerticalArrow(img, x, productTop, x, productBottom, yellow)
		a.drawTextWithBackground(img, x+10, (productTop+productBottom)/2-10,
			fmt.Sprintf("L: %s\"", dimensions.Length), white, 18.0)
	}

	// 绘制高度标注（左侧垂直箭头）
	if dimensions.Height != "" && dimensions.Height != "0" {
		x := productLeft - 30
		a.drawVerticalArrow(img, x, productTop, x, productBottom, yellow)
		a.drawTextWithBackground(img, x-80, (productTop+productBottom)/2-10,
			fmt.Sprintf("H: %s\"", dimensions.Height), white, 18.0)
	}

	// 在左下角绘制汇总信息
	a.drawSummaryInfo(img, dimensions)

	return nil
}

// drawHorizontalArrow 绘制水平箭头
func (a *ImageDimensionAnnotator) drawHorizontalArrow(img *image.RGBA, x1, y1, x2, y2 int, col color.Color) {
	// 绘制主线
	a.drawThickLine(img, x1, y1, x2, y2, col, 3)

	// 绘制左侧箭头
	arrowSize := 10
	a.drawThickLine(img, x1, y1, x1+arrowSize, y1-arrowSize, col, 2)
	a.drawThickLine(img, x1, y1, x1+arrowSize, y1+arrowSize, col, 2)

	// 绘制右侧箭头
	a.drawThickLine(img, x2, y2, x2-arrowSize, y2-arrowSize, col, 2)
	a.drawThickLine(img, x2, y2, x2-arrowSize, y2+arrowSize, col, 2)
}

// drawVerticalArrow 绘制垂直箭头
func (a *ImageDimensionAnnotator) drawVerticalArrow(img *image.RGBA, x1, y1, x2, y2 int, col color.Color) {
	// 绘制主线
	a.drawThickLine(img, x1, y1, x2, y2, col, 3)

	// 绘制顶部箭头
	arrowSize := 10
	a.drawThickLine(img, x1, y1, x1-arrowSize, y1+arrowSize, col, 2)
	a.drawThickLine(img, x1, y1, x1+arrowSize, y1+arrowSize, col, 2)

	// 绘制底部箭头
	a.drawThickLine(img, x2, y2, x2-arrowSize, y2-arrowSize, col, 2)
	a.drawThickLine(img, x2, y2, x2+arrowSize, y2-arrowSize, col, 2)
}

// drawThickLine 绘制粗线条
func (a *ImageDimensionAnnotator) drawThickLine(img *image.RGBA, x1, y1, x2, y2 int, col color.Color, thickness int) {
	// 绘制多条平行线来实现粗线效果
	for t := -thickness / 2; t <= thickness/2; t++ {
		if x1 == x2 {
			// 垂直线
			a.drawLine(img, x1+t, y1, x2+t, y2, col)
		} else if y1 == y2 {
			// 水平线
			a.drawLine(img, x1, y1+t, x2, y2+t, col)
		} else {
			// 斜线
			a.drawLine(img, x1+t, y1, x2+t, y2, col)
			a.drawLine(img, x1, y1+t, x2, y2+t, col)
		}
	}
}

// drawTextWithBackground 绘制带背景的文本
func (a *ImageDimensionAnnotator) drawTextWithBackground(img *image.RGBA, x, y int, text string, col color.Color, fontSize float64) error {
	// 加载字体
	f, err := a.loadFont()
	if err != nil {
		return err
	}

	// 估算文本宽度和高度
	textWidth := len(text) * int(fontSize*0.6)
	textHeight := int(fontSize * 1.5)

	// 绘制半透明黑色背景
	padding := 5
	a.drawBackground(img, x-padding, y-padding, textWidth+padding*2, textHeight+padding)

	// 绘制文本
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(f)
	c.SetFontSize(fontSize)
	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(image.NewUniform(col))

	pt := freetype.Pt(x, y+int(c.PointToFixed(fontSize)>>6))
	if _, err := c.DrawString(text, pt); err != nil {
		return err
	}

	return nil
}

// drawSummaryInfo 绘制汇总信息
func (a *ImageDimensionAnnotator) drawSummaryInfo(img *image.RGBA, dimensions DimensionInfo) error {
	bounds := img.Bounds()
	height := bounds.Dy()

	// 加载字体
	f, err := a.loadFont()
	if err != nil {
		return fmt.Errorf("加载字体失败: %w", err)
	}

	// 创建字体绘制器
	fontSize := 20.0
	c := freetype.NewContext()
	c.SetDPI(72)
	c.SetFont(f)
	c.SetFontSize(fontSize)
	c.SetClip(bounds)
	c.SetDst(img)
	c.SetSrc(image.White)

	// 绘制标注
	margin := 15
	lineHeight := int(fontSize * 1.5)

	// 计算需要的行数
	lines := 0
	if dimensions.Length != "" && dimensions.Length != "0" {
		lines++
	}
	if dimensions.Width != "" && dimensions.Width != "0" {
		lines++
	}
	if dimensions.Height != "" && dimensions.Height != "0" {
		lines++
	}

	if lines == 0 {
		return nil
	}

	// 绘制半透明背景
	bgHeight := lineHeight*lines + 20
	a.drawBackground(img, 10, height-margin-bgHeight, 250, bgHeight)

	// 绘制尺寸文本
	y := height - margin - lineHeight*lines
	if dimensions.Length != "" && dimensions.Length != "0" {
		text := fmt.Sprintf("Length: %s in", dimensions.Length)
		pt := freetype.Pt(margin, y+int(c.PointToFixed(fontSize)>>6))
		if _, err := c.DrawString(text, pt); err != nil {
			return err
		}
		y += lineHeight
	}

	if dimensions.Width != "" && dimensions.Width != "0" {
		text := fmt.Sprintf("Width: %s in", dimensions.Width)
		pt := freetype.Pt(margin, y+int(c.PointToFixed(fontSize)>>6))
		if _, err := c.DrawString(text, pt); err != nil {
			return err
		}
		y += lineHeight
	}

	if dimensions.Height != "" && dimensions.Height != "0" {
		text := fmt.Sprintf("Height: %s in", dimensions.Height)
		pt := freetype.Pt(margin, y+int(c.PointToFixed(fontSize)>>6))
		if _, err := c.DrawString(text, pt); err != nil {
			return err
		}
	}

	return nil
}

// drawBackground 绘制半透明背景
func (a *ImageDimensionAnnotator) drawBackground(img *image.RGBA, x, y, w, h int) {
	// 黑色半透明背景
	bgColor := color.RGBA{R: 0, G: 0, B: 0, A: 180}
	for i := x; i < x+w && i < img.Bounds().Dx(); i++ {
		for j := y; j < y+h && j < img.Bounds().Dy(); j++ {
			img.Set(i, j, bgColor)
		}
	}
}

// loadFont 加载字体
func (a *ImageDimensionAnnotator) loadFont() (*truetype.Font, error) {
	// 使用Go内置字体
	f, err := truetype.Parse(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("解析字体失败: %w", err)
	}
	return f, nil
}

// drawLine 绘制线条
func (a *ImageDimensionAnnotator) drawLine(img *image.RGBA, x1, y1, x2, y2 int, col color.Color) {
	// 简单的Bresenham算法绘制直线
	dx := abs(x2 - x1)
	dy := abs(y2 - y1)
	sx := 1
	if x1 > x2 {
		sx = -1
	}
	sy := 1
	if y1 > y2 {
		sy = -1
	}
	err := dx - dy

	for {
		img.Set(x1, y1, col)
		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x1 += sx
		}
		if e2 < dx {
			err += dx
			y1 += sy
		}
	}
}

// abs 绝对值
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// HasDimensionAnnotationWithDetails 检测图片是否已包含尺寸标注（带详细信息，公开方法）
func (a *ImageDimensionAnnotator) HasDimensionAnnotationWithDetails(img image.Image) (bool, string) {
	return a.hasDimensionAnnotationWithDetails(img)
}

// hasDimensionAnnotationWithDetails 检测图片是否已包含尺寸标注（带详细信息，内部方法）
func (a *ImageDimensionAnnotator) hasDimensionAnnotationWithDetails(img image.Image) (bool, string) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 使用 OpenAI Vision API 检测
	if a.openaiClient != nil {
		hasAnnotation, visionDetails, err := a.detectWithVisionAPI(img)
		if err == nil {
			return hasAnnotation, fmt.Sprintf("Vision API检测: %s (图片尺寸: %dx%d)", visionDetails, width, height)
		}
		a.logger.Errorf("Vision API检测失败: %v", err)
		return false, fmt.Sprintf("检测失败: %v (图片尺寸: %dx%d)", err, width, height)
	}

	// 如果没有配置OpenAI客户端，返回未检测到
	a.logger.Warn("未配置OpenAI客户端，无法检测尺寸标注")
	return false, fmt.Sprintf("未配置Vision API (图片尺寸: %dx%d)", width, height)
}

// detectWithVisionAPI 使用OpenAI Vision API检测图片中的尺寸标注
func (a *ImageDimensionAnnotator) detectWithVisionAPI(img image.Image) (bool, string, error) {
	// 将图片编码为base64
	buf := new(bytes.Buffer)
	if err := png.Encode(buf, img); err != nil {
		return false, "", fmt.Errorf("编码图片失败: %w", err)
	}
	base64Image := base64.StdEncoding.EncodeToString(buf.Bytes())

	// 构建Vision API请求
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleUser,
			MultiContent: []openai.ChatMessagePart{
				{
					Type: openai.ChatMessagePartTypeText,
					Text: `请仔细检查这张图片，判断图片中是否包含产品尺寸标注信息。

尺寸标注的特征包括：
1. 包含尺寸单位的文字，如：cm, mm, inch, in, 厘米, 英寸等
2. 尺寸数字，如：2.8cm/1.1inch, 5.2cm, 10mm等
3. 标注线条、箭头、引线等指示尺寸的图形元素
4. "Product size", "Size", "Dimensions", "尺寸" 等标题文字

请只回答 "YES" 或 "NO"，然后简要说明理由（不超过30字）。
格式：YES/NO - 理由`,
				},
				{
					Type: openai.ChatMessagePartTypeImageURL,
					ImageURL: &openai.ChatMessageImageURL{
						URL: fmt.Sprintf("data:image/png;base64,%s", base64Image),
					},
				},
			},
		},
	}

	req := &openaiClient.ChatCompletionRequest{
		Model:    a.openaiClient.GetDefaultModel(),
		Messages: messages,
	}

	resp, err := a.openaiClient.CreateChatCompletion(ctx, req)
	if err != nil {
		return false, "", fmt.Errorf("调用Vision API失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return false, "", fmt.Errorf("vision API返回空响应")
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)
	a.logger.Infof("Vision API响应: %s", content)

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
