package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
)

// ImageFormat 图片格式
type ImageFormat string

const (
	ImageFormatJPEG ImageFormat = "jpeg"
	ImageFormatPNG  ImageFormat = "png"
	ImageFormatAuto ImageFormat = "auto"
)

// ImageToBase64 将图片转换为base64字符串
// format: jpeg/png/auto (auto会自动选择格式)
// quality: JPEG质量 (1-100)，仅对JPEG格式有效
func ImageToBase64(img image.Image, format ImageFormat, quality int) (string, error) {
	if img == nil {
		return "", fmt.Errorf("image is nil")
	}

	var buf bytes.Buffer
	var err error

	switch format {
	case ImageFormatJPEG:
		if quality <= 0 || quality > 100 {
			quality = 85 // 默认质量
		}
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	case ImageFormatPNG:
		err = png.Encode(&buf, img)
	case ImageFormatAuto:
		// 自动选择：优先使用PNG（无损）
		err = png.Encode(&buf, img)
	default:
		return "", fmt.Errorf("unsupported image format: %s", format)
	}

	if err != nil {
		return "", fmt.Errorf("encode image failed: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// ImageToBase64JPEG 将图片转换为JPEG格式的base64字符串
func ImageToBase64JPEG(img image.Image, quality int) (string, error) {
	return ImageToBase64(img, ImageFormatJPEG, quality)
}

// ImageToBase64PNG 将图片转换为PNG格式的base64字符串
func ImageToBase64PNG(img image.Image) (string, error) {
	return ImageToBase64(img, ImageFormatPNG, 0)
}

// Base64ToImage 将base64字符串转换为图片
// 自动检测图片格式（支持JPEG和PNG）
func Base64ToImage(base64Str string) (image.Image, error) {
	if base64Str == "" {
		return nil, fmt.Errorf("base64 string is empty")
	}

	data, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, fmt.Errorf("decode base64 failed: %w", err)
	}

	return BytesToImage(data)
}

// BytesToImage 将字节数组转换为图片
// 自动检测图片格式
func BytesToImage(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}

	reader := bytes.NewReader(data)

	// 尝试解码（image.Decode会自动检测格式）
	img, _, err := image.Decode(reader)
	if err != nil {
		return nil, fmt.Errorf("decode image failed: %w", err)
	}

	return img, nil
}

// BytesToImageWithFormat 将字节数组转换为图片，同时返回格式
func BytesToImageWithFormat(data []byte) (image.Image, string, error) {
	if len(data) == 0 {
		return nil, "", fmt.Errorf("data is empty")
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode image failed: %w", err)
	}

	return img, format, nil
}

// ImageToBytes 将图片转换为字节数组
func ImageToBytes(img image.Image, format ImageFormat, quality int) ([]byte, error) {
	if img == nil {
		return nil, fmt.Errorf("image is nil")
	}

	var buf bytes.Buffer
	var err error

	switch format {
	case ImageFormatJPEG:
		if quality <= 0 || quality > 100 {
			quality = 85
		}
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	case ImageFormatPNG:
		err = png.Encode(&buf, img)
	case ImageFormatAuto:
		err = png.Encode(&buf, img)
	default:
		return nil, fmt.Errorf("unsupported image format: %s", format)
	}

	if err != nil {
		return nil, fmt.Errorf("encode image failed: %w", err)
	}

	return buf.Bytes(), nil
}

// LoadImageFromReader 从Reader加载图片
func LoadImageFromReader(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decode image failed: %w", err)
	}
	return img, nil
}

// GetImageFormat 获取图片格式
func GetImageFormat(data []byte) (string, error) {
	if len(data) == 0 {
		return "", fmt.Errorf("data is empty")
	}

	reader := bytes.NewReader(data)
	_, format, err := image.DecodeConfig(reader)
	if err != nil {
		return "", fmt.Errorf("decode image config failed: %w", err)
	}

	return format, nil
}

// GetImageSize 获取图片尺寸
func GetImageSize(img image.Image) (width, height int) {
	if img == nil {
		return 0, 0
	}
	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy()
}

// EncodeImage 将图片编码到 Writer
// format: jpeg/jpg/png，quality 仅对 JPEG 有效（1-100）
func EncodeImage(w io.Writer, img image.Image, format string, quality int) error {
	if img == nil {
		return fmt.Errorf("image is nil")
	}
	if quality <= 0 || quality > 100 {
		quality = 95
	}
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
	case "png":
		return png.Encode(w, img)
	default:
		// 未知格式默认 JPEG
		return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
	}
}

// GetImageFormatFromBytes 通过魔数字节检测图片格式，支持 jpeg/png/gif/webp
// 与 GetImageFormat 的区别：不需要完整解码，速度更快，且支持 gif/webp
func GetImageFormatFromBytes(data []byte) string {
	if len(data) < 12 {
		return ""
	}
	if data[0] == 0xFF && data[1] == 0xD8 {
		return "jpeg"
	}
	if string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return "png"
	}
	if string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a" {
		return "gif"
	}
	if string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "webp"
	}
	return ""
}

// HasTransparency 采样检测图片是否含有透明通道
func HasTransparency(img image.Image) bool {
	if img == nil {
		return false
	}
	bounds := img.Bounds()
	const sampleStep = 10
	for y := bounds.Min.Y; y < bounds.Max.Y; y += sampleStep {
		for x := bounds.Min.X; x < bounds.Max.X; x += sampleStep {
			_, _, _, a := img.At(x, y).RGBA()
			if a < 65535 {
				return true
			}
		}
	}
	return false
}

// GetOptimalFormat 根据图片内容选择最优编码格式
// PNG 有透明度时保持 PNG，否则用 JPEG（压缩比更好）
func GetOptimalFormat(img image.Image, originalFormat string) string {
	if strings.ToLower(originalFormat) == "png" && HasTransparency(img) {
		return "png"
	}
	return "jpeg"
}
