// Package imageutil 提供图片编解码工具方法
package imagex

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

// Format 图片格式
type Format string

const (
	FormatJPEG Format = "jpeg"
	FormatPNG  Format = "png"
	FormatAuto Format = "auto"
)

// ToBase64 将图片转换为base64字符串
func ToBase64(img image.Image, format Format, quality int) (string, error) {
	if img == nil {
		return "", fmt.Errorf("image is nil")
	}

	var buf bytes.Buffer
	var err error

	switch format {
	case FormatJPEG:
		if quality <= 0 || quality > 100 {
			quality = 85
		}
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	case FormatPNG, FormatAuto:
		err = png.Encode(&buf, img)
	default:
		return "", fmt.Errorf("unsupported image format: %s", format)
	}

	if err != nil {
		return "", fmt.Errorf("encode image failed: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// ToBase64JPEG 将图片转换为JPEG格式的base64字符串
func ToBase64JPEG(img image.Image, quality int) (string, error) {
	return ToBase64(img, FormatJPEG, quality)
}

// ToBase64PNG 将图片转换为PNG格式的base64字符串
func ToBase64PNG(img image.Image) (string, error) {
	return ToBase64(img, FormatPNG, 0)
}

// FromBase64 将base64字符串转换为图片
func FromBase64(base64Str string) (image.Image, error) {
	if base64Str == "" {
		return nil, fmt.Errorf("base64 string is empty")
	}

	data, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, fmt.Errorf("decode base64 failed: %w", err)
	}

	return FromBytes(data)
}

// FromBytes 将字节数组转换为图片
func FromBytes(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("data is empty")
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image failed: %w", err)
	}

	return img, nil
}

// FromBytesWithFormat 将字节数组转换为图片，同时返回格式
func FromBytesWithFormat(data []byte) (image.Image, string, error) {
	if len(data) == 0 {
		return nil, "", fmt.Errorf("data is empty")
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", fmt.Errorf("decode image failed: %w", err)
	}

	return img, format, nil
}

// ToBytes 将图片转换为字节数组
func ToBytes(img image.Image, format Format, quality int) ([]byte, error) {
	if img == nil {
		return nil, fmt.Errorf("image is nil")
	}

	var buf bytes.Buffer
	var err error

	switch format {
	case FormatJPEG:
		if quality <= 0 || quality > 100 {
			quality = 85
		}
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	case FormatPNG, FormatAuto:
		err = png.Encode(&buf, img)
	default:
		return nil, fmt.Errorf("unsupported image format: %s", format)
	}

	if err != nil {
		return nil, fmt.Errorf("encode image failed: %w", err)
	}

	return buf.Bytes(), nil
}

// FromReader 从 Reader 加载图片
func FromReader(r io.Reader) (image.Image, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("decode image failed: %w", err)
	}
	return img, nil
}

// Encode 将图片编码到 Writer，format 支持 jpeg/jpg/png，quality 仅对 JPEG 有效
func Encode(w io.Writer, img image.Image, format string, quality int) error {
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
		return jpeg.Encode(w, img, &jpeg.Options{Quality: quality})
	}
}

// Size 获取图片尺寸
func Size(img image.Image) (width, height int) {
	if img == nil {
		return 0, 0
	}
	bounds := img.Bounds()
	return bounds.Dx(), bounds.Dy()
}

// FormatFromBytes 通过魔数字节检测图片格式，支持 jpeg/png/gif/webp
func FormatFromBytes(data []byte) string {
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

// OptimalFormat 根据图片内容选择最优编码格式
func OptimalFormat(img image.Image, originalFormat string) string {
	if strings.ToLower(originalFormat) == "png" && HasTransparency(img) {
		return "png"
	}
	return "jpeg"
}
