package service

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"
)

// S3Uploader S3上传器
type S3Uploader struct {
	s3Client *s3.Client
	bucket   string
	logger   *logrus.Entry
}

// S3Config S3配置
type S3Config struct {
	Bucket string
	Region string
}

// NewS3Uploader 创建S3上传器
func NewS3Uploader(s3Client *s3.Client, bucket string) *S3Uploader {
	return &S3Uploader{
		s3Client: s3Client,
		bucket:   bucket,
		logger:   logrus.WithField("service", "S3Uploader"),
	}
}

// Upload 上传图片到S3
func (u *S3Uploader) Upload(
	ctx context.Context,
	key string,
	data []byte,
	contentType string,
) (string, error) {
	u.logger.Infof("开始上传到S3: bucket=%s, key=%s, size=%d",
		u.bucket, key, len(data))

	input := &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}

	_, err := u.s3Client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("上传到S3失败: %w", err)
	}

	// 构建S3 URL
	url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", u.bucket, key)
	u.logger.Infof("上传成功: %s", url)

	return url, nil
}

// UploadMultiple 批量上传图片
func (u *S3Uploader) UploadMultiple(
	ctx context.Context,
	prefix string,
	images [][]byte,
) ([]string, error) {
	urls := make([]string, 0, len(images))

	for i, imageData := range images {
		// 生成唯一的key
		key := u.generateKey(prefix, i)

		// 检测内容类型
		contentType := u.detectContentType(imageData)

		url, err := u.Upload(ctx, key, imageData, contentType)
		if err != nil {
			u.logger.Warnf("上传图片失败 [%d/%d]: %v", i+1, len(images), err)
			continue
		}

		urls = append(urls, url)
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("所有图片上传失败")
	}

	u.logger.Infof("成功上传 %d/%d 张图片", len(urls), len(images))
	return urls, nil
}

// generateKey 生成S3 key
func (u *S3Uploader) generateKey(prefix string, index int) string {
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("image_%d_%d.jpg", timestamp, index)
	return filepath.Join(prefix, filename)
}

// detectContentType 检测内容类型
func (u *S3Uploader) detectContentType(data []byte) string {
	if len(data) < 12 {
		return "image/jpeg"
	}

	// 检测PNG
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}

	// 检测JPEG
	if data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}

	// 检测GIF
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}

	// 默认JPEG
	return "image/jpeg"
}
