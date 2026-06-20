package image

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"time"

	"task-processor/internal/pkg/imagex"
	"task-processor/internal/shein/api"
	"task-processor/internal/shein/client"

	"golang.org/x/image/draw"
)

type imageDownloadProvider interface {
	DownloadImage(url string) ([]byte, error)
}

// Client 图片相关API实现
type Client struct {
	*client.BaseAPIClient
	imageDownloader imageDownloadProvider
}

// NewClient 创建新的图片API客户端
func NewClient(baseClient *client.BaseAPIClient) *Client {
	return NewClientWithImageDownloader(baseClient, nil)
}

// NewClientWithImageDownloader 创建新的图片API客户端，并允许复用外部已配置的下载器。
func NewClientWithImageDownloader(baseClient *client.BaseAPIClient, imageDownloader imageDownloadProvider) *Client {
	if imageDownloader == nil {
		imageDownloader = &plainHTTPImageDownloader{client: &http.Client{Timeout: 180 * time.Second}}
	}
	return &Client{
		BaseAPIClient:   baseClient,
		imageDownloader: imageDownloader,
	}
}

type plainHTTPImageDownloader struct {
	client *http.Client
}

func (d *plainHTTPImageDownloader) DownloadImage(imageURL string) ([]byte, error) {
	if d == nil || d.client == nil {
		return downloadImageWithPlainHTTP(imageURL)
	}
	req, err := http.NewRequest(http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 ListingKit ImageUploader/1.0")
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 30<<20))
}

// UploadImage 下载并上传图片（含尺寸处理）
func (i *Client) UploadImage(imageURL string) (string, error) {
	imageData, err := i.downloadImageToMemory(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}

	processedImageData, err := i.processImageIfNeeded(imageData)
	if err != nil {
		return "", fmt.Errorf("处理图片失败: %w", err)
	}

	uploadedURL, err := i.UploadOriginalImage(processedImageData)
	if err != nil {
		return "", fmt.Errorf("上传图片失败: %w", err)
	}

	return uploadedURL, nil
}

// processImageIfNeeded 检查图片尺寸，不足900x900时填充白边
func (i *Client) processImageIfNeeded(imageData []byte) ([]byte, error) {
	img, format, err := imagex.FromBytesWithFormat(imageData)
	if err != nil {
		return nil, fmt.Errorf("解码图片失败: %w", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() >= 900 && bounds.Dy() >= 900 {
		return imageData, nil
	}

	paddedImg := i.addWhitePadding(img)

	var buf bytes.Buffer
	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, paddedImg, &jpeg.Options{Quality: 95})
	case "png":
		err = png.Encode(&buf, paddedImg)
	default:
		err = jpeg.Encode(&buf, paddedImg, &jpeg.Options{Quality: 95})
	}

	if err != nil {
		return nil, fmt.Errorf("重新编码图片失败: %w", err)
	}

	return buf.Bytes(), nil
}

// addWhitePadding 为图片添加白边填充到900x900
func (i *Client) addWhitePadding(img image.Image) image.Image {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	paddedImg := image.NewRGBA(image.Rect(0, 0, 900, 900))
	draw.Draw(paddedImg, paddedImg.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)

	offsetX := (900 - width) / 2
	offsetY := (900 - height) / 2
	draw.Draw(paddedImg, image.Rect(offsetX, offsetY, offsetX+width, offsetY+height), img, bounds.Min, draw.Src)

	return paddedImg
}

// UploadOriginalImage 上传原始图片数据
func (i *Client) UploadOriginalImage(imageData []byte) (string, error) {
	url := fmt.Sprintf("%s%s", i.GetBaseURL(), client.GetUploadImageEndpoint())

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

	reqClient := i.GetHTTPClient()
	resp, err := reqClient.R().
		SetFileReader("file", "image.jpg", bytes.NewReader(imageData)).
		SetFormData(map[string]string{"x": "0", "y": "0", "image_type": "MAIN"}).
		Post(url)

	if err != nil {
		return "", &api.APIError{StatusCode: 0, Message: fmt.Sprintf("图片上传请求失败: %v", err), URL: url}
	}

	if !resp.IsSuccessState() {
		return "", &api.APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("HTTP请求失败，状态码: %d", resp.StatusCode), URL: url}
	}

	if err := resp.UnmarshalJson(&result); err != nil {
		return "", &api.APIError{StatusCode: 0, Message: fmt.Sprintf("解析响应失败: %v", err), URL: url}
	}

	if err := i.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return "", err
		}
		return "", &api.APIError{StatusCode: 0, Message: fmt.Sprintf("上传原始图片失败: %s", result.Msg), URL: url}
	}

	return result.Info.ImageURL, nil
}

// DownloadAndUploadImage 下载亚马逊图片并上传
func (i *Client) DownloadAndUploadImage(imageURL string) (string, error) {
	imageData, err := i.downloadImageToMemory(imageURL)
	if err != nil {
		return "", fmt.Errorf("下载图片失败: %w", err)
	}

	processedImageData, err := i.processImageIfNeeded(imageData)
	if err != nil {
		return "", fmt.Errorf("处理图片失败: %w", err)
	}

	return i.UploadOriginalImage(processedImageData)
}

func (i *Client) downloadImageToMemory(imageURL string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		data, err := i.imageDownloader.DownloadImage(imageURL)
		if err == nil {
			return data, nil
		}
		if data, fallbackErr := downloadImageWithPlainHTTP(imageURL); fallbackErr == nil {
			return data, nil
		}
		lastErr = err
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}
	return nil, lastErr
}

func downloadImageWithPlainHTTP(imageURL string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 ListingKit ImageUploader/1.0")
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP状态码: %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 30<<20))
}
