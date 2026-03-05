// Package handlers 提供TEMU平台文本渲染功能
package validation

import (
	"fmt"
	"image"
	"image/color"
	"task-processor/internal/platforms/temu/handlers/common"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"github.com/sirupsen/logrus"
	"golang.org/x/image/font/gofont/goregular"
)

// TextRenderer 文本渲染器
type TextRenderer struct {
	logger *logrus.Entry
}

// NewTextRenderer 创建文本渲染器
func NewTextRenderer() *TextRenderer {
	return &TextRenderer{
		logger: logrus.WithField("component", "TextRenderer"),
	}
}

// DrawTextWithBackground 绘制带背景的文本
func (r *TextRenderer) DrawTextWithBackground(img *image.RGBA, x, y int, text string, col color.Color, fontSize float64) error {
	// 加载字体
	f, err := r.loadFont()
	if err != nil {
		return err
	}

	// 估算文本宽度和高度
	textWidth := len(text) * int(fontSize*0.6)
	textHeight := int(fontSize * 1.5)

	// 绘制半透明黑色背景
	padding := 5
	r.drawBackground(img, x-padding, y-padding, textWidth+padding*2, textHeight+padding)

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

// DrawSummaryInfo 绘制汇总信息
func (r *TextRenderer) DrawSummaryInfo(img *image.RGBA, dimensions common.DimensionInfo) error {
	bounds := img.Bounds()
	height := bounds.Dy()

	// 加载字体
	f, err := r.loadFont()
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
	r.drawBackground(img, 10, height-margin-bgHeight, 250, bgHeight)

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
func (r *TextRenderer) drawBackground(img *image.RGBA, x, y, w, h int) {
	// 黑色半透明背景
	bgColor := color.RGBA{R: 0, G: 0, B: 0, A: 180}
	for i := x; i < x+w && i < img.Bounds().Dx(); i++ {
		for j := y; j < y+h && j < img.Bounds().Dy(); j++ {
			img.Set(i, j, bgColor)
		}
	}
}

// loadFont 加载字体
func (r *TextRenderer) loadFont() (*truetype.Font, error) {
	// 使用Go内置字体
	f, err := truetype.Parse(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("解析字体失败: %w", err)
	}
	return f, nil
}
