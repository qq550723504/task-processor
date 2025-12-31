package impl

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"task-processor/internal/common/shein/api"
	"task-processor/internal/pkg/management/impl"
	"time"

	"golang.org/x/image/draw"
)

// ImageAPI 图片相关API实现
type ImageAPI struct {
	*BaseAPIClient
	imageDownloader *impl.ImageDownloader
}

// NewImageAPI 创建新的图片API实现
func NewImageAPI(baseClient *BaseAPIClient) *ImageAPI {
	// 创建图片下载器，设置60秒超时和重试机制
	imageDownloader := impl.NewImageDownloader(60 * time.Second)

	return &ImageAPI{
		BaseAPIClient:   baseClient,
		imageDownloader: imageDownloader,
	}
}

// UploadImage 上传图片
func (i *ImageAPI) UploadImage(imageURL string) (string, error) {
	// 先下载图片
	imageData, err := i.downloadImageToMemory(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}

	// 检查并处理图片尺寸
	processedImageData, err := i.processImageIfNeeded(imageData)
	if err != nil {
		return "", fmt.Errorf("处理图片失败: %w", err)
	}

	// 上传处理后的图片数据
	uploadedURL, err := i.UploadOriginalImage(processedImageData)
	if err != nil {
		return "", fmt.Errorf("上传图片失败: %w", err)
	}

	return uploadedURL, nil
}

// processImageIfNeeded 检查图片尺寸并在需要时进行白边填充
func (i *ImageAPI) processImageIfNeeded(imageData []byte) ([]byte, error) {
	// 解码图片
	img, format, err := image.Decode(bytes.NewReader(imageData))
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 检查是否需要填充白边
	if width >= 900 && height >= 900 {
		// 图片尺寸满足要求，无需填充
		return imageData, nil
	}

	// 进行白边填充
	paddedImg := i.addWhitePadding(img)

	// 重新编码图片
	var buf bytes.Buffer
	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, paddedImg, &jpeg.Options{Quality: 95})
	case "png":
		err = png.Encode(&buf, paddedImg)
	default:
		// 默认使用JPEG格式
		err = jpeg.Encode(&buf, paddedImg, &jpeg.Options{Quality: 95})
	}

	if err != nil {
		return nil, fmt.Errorf("重新编码图片失败: %v", err)
	}

	return buf.Bytes(), nil
}

// addWhitePadding 为图片添加白边填充到900x900
func (i *ImageAPI) addWhitePadding(img image.Image) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 创建900x900的白色背景图片
	paddedImg := image.NewRGBA(image.Rect(0, 0, 900, 900))

	// 填充白色背景
	draw.Draw(paddedImg, paddedImg.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	// 计算居中位置
	offsetX := (900 - width) / 2
	offsetY := (900 - height) / 2

	// 将原图绘制到中心位置
	draw.Draw(paddedImg, image.Rect(offsetX, offsetY, offsetX+width, offsetY+height), img, bounds.Min, draw.Src)

	return paddedImg
}

// UploadOriginalImage 上传原始图片数据
func (i *ImageAPI) UploadOriginalImage(imageData []byte) (string, error) {
	url := fmt.Sprintf("%s%s", i.GetBaseURL(), uploadImageEndpoint)

	var result struct {
		api.APIResponse
		Info struct {
			ImageURL     string  `json:"image_url"`
			Radio        *string `json:"radio"`
			Width        *int    `json:"width"`
			Height       *int    `json:"height"`
			Size         *int    `json:"size"`
			ImageHexType *string `json:"image_hex_type"`
			ImageMD5     *string `json:"image_md5"`
		} `json:"info"`
	}

	// 使用底层req客户端进行文件上传
	reqClient := i.httpClient
	resp, err := reqClient.R().
		SetFileReader("file", "image.jpg", bytes.NewReader(imageData)).
		SetFormData(map[string]string{
			"x":          "0",
			"y":          "0",
			"image_type": "MAIN",
		}).
		Post(url)

	if err != nil {
		return "", &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("图片上传请求失败: %v", err),
			URL:        url,
		}
	}

	if !resp.IsSuccessState() {
		return "", &api.APIError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("HTTP请求失败，状态码: %d", resp.StatusCode),
			URL:        url,
		}
	}

	if err := resp.UnmarshalJson(&result); err != nil {
		return "", &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("解析响应失败: %v", err),
			URL:        url,
		}
	}

	// 统一错误处理 - 使用 ProcessAPIResponse 检查认证过期
	if err := i.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return "", err
		}
		// 其他错误，包装为 APIError
		return "", &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("上传原始图片失败: %s", result.Msg),
			URL:        url,
		}
	}

	return result.Info.ImageURL, nil
}

// DownloadAndUploadImage 下载亚马逊图片并上传
func (i *ImageAPI) DownloadAndUploadImage(imageURL string) (string, error) {
	// 1. 下载图片到内存
	imageData, err := i.downloadImageToMemory(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}

	// 2. 检查并处理图片尺寸
	processedImageData, err := i.processImageIfNeeded(imageData)
	if err != nil {
		return "", fmt.Errorf("处理图片失败: %w", err)
	}

	// 3. 使用UploadOriginalImage上传图片
	uploadedURL, err := i.UploadOriginalImage(processedImageData)
	if err != nil {
		return "", fmt.Errorf("上传图片失败: %w", err)
	}

	return uploadedURL, nil
}

func (i *ImageAPI) downloadImageToMemory(imageURL string) ([]byte, error) {
	// 直接使用带重试机制和URL清理功能的图片下载器
	// URL清理逻辑已经统一到 ImageDownloader 中
	return i.imageDownloader.DownloadImage(imageURL)
}
