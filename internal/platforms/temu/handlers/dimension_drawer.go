// Package handlers 提供TEMU平台尺寸标注绘制功能
package handlers

import (
	"fmt"
	"image"
	"image/color"

	"github.com/sirupsen/logrus"
)

// DimensionDrawer 尺寸绘制器
type DimensionDrawer struct {
	logger       *logrus.Entry
	textRenderer *TextRenderer
	utils        *DrawingUtils
}

// NewDimensionDrawer 创建尺寸绘制器
func NewDimensionDrawer() *DimensionDrawer {
	return &DimensionDrawer{
		logger:       logrus.WithField("component", "DimensionDrawer"),
		textRenderer: NewTextRenderer(),
		utils:        NewDrawingUtils(),
	}
}

// DrawDimensionAnnotations 绘制尺寸标注（带箭头）
func (d *DimensionDrawer) DrawDimensionAnnotations(img *image.RGBA, dimensions DimensionInfo) error {
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
		d.drawHorizontalArrow(img, productLeft, y, productRight, y, yellow)
		d.textRenderer.DrawTextWithBackground(img, (productLeft+productRight)/2-50, y-25,
			fmt.Sprintf("W: %s\"", dimensions.Width), white, 18.0)
	}

	// 绘制长度标注（右侧垂直箭头）
	if dimensions.Length != "" && dimensions.Length != "0" {
		x := productRight + 30
		d.drawVerticalArrow(img, x, productTop, x, productBottom, yellow)
		d.textRenderer.DrawTextWithBackground(img, x+10, (productTop+productBottom)/2-10,
			fmt.Sprintf("L: %s\"", dimensions.Length), white, 18.0)
	}

	// 绘制高度标注（左侧垂直箭头）
	if dimensions.Height != "" && dimensions.Height != "0" {
		x := productLeft - 30
		d.drawVerticalArrow(img, x, productTop, x, productBottom, yellow)
		d.textRenderer.DrawTextWithBackground(img, x-80, (productTop+productBottom)/2-10,
			fmt.Sprintf("H: %s\"", dimensions.Height), white, 18.0)
	}

	// 在左下角绘制汇总信息
	d.textRenderer.DrawSummaryInfo(img, dimensions)

	return nil
}

// drawHorizontalArrow 绘制水平箭头
func (d *DimensionDrawer) drawHorizontalArrow(img *image.RGBA, x1, y1, x2, y2 int, col color.Color) {
	// 绘制主线
	d.drawThickLine(img, x1, y1, x2, y2, col, 3)

	// 绘制左侧箭头
	arrowSize := 10
	d.drawThickLine(img, x1, y1, x1+arrowSize, y1-arrowSize, col, 2)
	d.drawThickLine(img, x1, y1, x1+arrowSize, y1+arrowSize, col, 2)

	// 绘制右侧箭头
	d.drawThickLine(img, x2, y2, x2-arrowSize, y2-arrowSize, col, 2)
	d.drawThickLine(img, x2, y2, x2-arrowSize, y2+arrowSize, col, 2)
}

// drawVerticalArrow 绘制垂直箭头
func (d *DimensionDrawer) drawVerticalArrow(img *image.RGBA, x1, y1, x2, y2 int, col color.Color) {
	// 绘制主线
	d.drawThickLine(img, x1, y1, x2, y2, col, 3)

	// 绘制顶部箭头
	arrowSize := 10
	d.drawThickLine(img, x1, y1, x1-arrowSize, y1+arrowSize, col, 2)
	d.drawThickLine(img, x1, y1, x1+arrowSize, y1+arrowSize, col, 2)

	// 绘制底部箭头
	d.drawThickLine(img, x2, y2, x2-arrowSize, y2-arrowSize, col, 2)
	d.drawThickLine(img, x2, y2, x2+arrowSize, y2-arrowSize, col, 2)
}

// drawThickLine 绘制粗线条
func (d *DimensionDrawer) drawThickLine(img *image.RGBA, x1, y1, x2, y2 int, col color.Color, thickness int) {
	// 绘制多条平行线来实现粗线效果
	for t := -thickness / 2; t <= thickness/2; t++ {
		if x1 == x2 {
			// 垂直线
			d.utils.DrawLine(img, x1+t, y1, x2+t, y2, col)
		} else if y1 == y2 {
			// 水平线
			d.utils.DrawLine(img, x1, y1+t, x2, y2+t, col)
		} else {
			// 斜线
			d.utils.DrawLine(img, x1+t, y1, x2+t, y2, col)
			d.utils.DrawLine(img, x1, y1+t, x2, y2+t, col)
		}
	}
}
