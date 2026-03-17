package storage

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
	Bucket string `json:"bucket"`
	Region string `json:"region"`
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
	u.logger.WithFields(logrus.Fields{
		"bucket":       u.bucket,
		"key":          key,
		"size":         len(data),
		"content_type": contentType,
	}).Info("开始上传到S3")

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
	u.logger.WithField("url", url).Info("S3上传成功")

	return url, nil
}

// UploadMultiple 批量上传图片
func (u *S3Uploader) UploadMultiple(
	ctx context.Context,
	prefix string,
	images [][]byte,
) ([]string, error) {
	u.logger.WithFields(logrus.Fields{
		"prefix": prefix,
		"count":  len(images),
	}).Info("开始批量上传到S3")

	urls := make([]string, 0, len(images))
	var errors []error

	for i, imageData := range images {
		// 生成唯一的key
		key := u.generateKey(prefix, i)

		// 检测内容类型
		contentType := u.detectContentType(imageData)

		url, err := u.Upload(ctx, key, imageData, contentType)
		if err != nil {
			u.logger.WithError(err).Warnf("上传图片失败 [%d/%d]", i+1, len(images))
			errors = append(errors, err)
			continue
		}

		urls = append(urls, url)
	}

	if len(urls) == 0 {
		return nil, fmt.Errorf("所有图片上传失败，第一个错误: %w", errors[0])
	}

	u.logger.WithFields(logrus.Fields{
		"success_count": len(urls),
		"total_count":   len(images),
		"error_count":   len(errors),
	}).Info("批量S3上传完成")

	return urls, nil
}

// UploadWithMetadata 上传带元数据的文件
func (u *S3Uploader) UploadWithMetadata(
	ctx context.Context,
	key string,
	data []byte,
	contentType string,
	metadata map[string]string,
) (string, error) {
	u.logger.WithFields(logrus.Fields{
		"bucket":       u.bucket,
		"key":          key,
		"size":         len(data),
		"content_type": contentType,
		"metadata":     metadata,
	}).Info("开始上传带元数据的文件到S3")

	input := &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
		Metadata:    metadata,
	}

	_, err := u.s3Client.PutObject(ctx, input)
	if err != nil {
		return "", fmt.Errorf("上传到S3失败: %w", err)
	}

	url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", u.bucket, key)
	u.logger.WithField("url", url).Info("带元数据的S3上传成功")

	return url, nil
}

// Delete 删除S3对象
func (u *S3Uploader) Delete(ctx context.Context, key string) error {
	u.logger.WithFields(logrus.Fields{
		"bucket": u.bucket,
		"key":    key,
	}).Info("删除S3对象")

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
	}

	_, err := u.s3Client.DeleteObject(ctx, input)
	if err != nil {
		return fmt.Errorf("删除S3对象失败: %w", err)
	}

	u.logger.Info("S3对象删除成功")
	return nil
}

// Exists 检查S3对象是否存在
func (u *S3Uploader) Exists(ctx context.Context, key string) (bool, error) {
	input := &s3.HeadObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
	}

	_, err := u.s3Client.HeadObject(ctx, input)
	if err != nil {
		// 如果是NotFound错误，返回false
		return false, nil
	}

	return true, nil
}

// generateKey 生成S3 key
func (u *S3Uploader) generateKey(prefix string, index int) string {
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("image_%d_%d.jpg", timestamp, index)
	return filepath.Join(prefix, filename)
}

// GenerateUniqueKey 生成唯一的S3 key
func (u *S3Uploader) GenerateUniqueKey(prefix, filename string) string {
	timestamp := time.Now().Unix()
	ext := filepath.Ext(filename)
	name := filename[:len(filename)-len(ext)]
	uniqueFilename := fmt.Sprintf("%s_%d%s", name, timestamp, ext)
	return filepath.Join(prefix, uniqueFilename)
}

// detectContentType 检测内容类型
func (u *S3Uploader) detectContentType(data []byte) string {
	if len(data) < 12 {
		return "image/jpeg"
	}

	// 检测PNG
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}

	// 检测JPEG
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return "image/jpeg"
	}

	// 检测GIF
	if len(data) >= 6 && data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 {
		return "image/gif"
	}

	// 检测WebP
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}

	// 默认JPEG
	return "image/jpeg"
}
