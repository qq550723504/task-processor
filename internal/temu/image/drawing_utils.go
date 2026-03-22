// Package image 提供TEMU平台绘图工具功能
package image

import (
	"image"
	"image/color"

		"task-processor/internal/core/logger"
	"github.com/sirupsen/logrus"
)

// DrawingUtils 绘图工具
type DrawingUtils struct {
	logger *logrus.Entry
}

// NewDrawingUtils 创建绘图工具
func NewDrawingUtils() *DrawingUtils {
	return &DrawingUtils{
		logger: logger.GetGlobalLogger("DrawingUtils"),
	}
}

// DrawLine 绘制线条
func (u *DrawingUtils) DrawLine(img *image.RGBA, x1, y1, x2, y2 int, col color.Color) {
	// 简单的Bresenham算法绘制直线
	dx := u.abs(x2 - x1)
	dy := u.abs(y2 - y1)
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
func (u *DrawingUtils) abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
