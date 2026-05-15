package listingkit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"task-processor/internal/infra/storage"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

type s3ImageUploadWriter interface {
	Upload(ctx context.Context, key string, data []byte, contentType string) (string, error)
}

type s3ImageUploadReader interface {
	GetObject(ctx context.Context, params *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
}

type s3ImageUploadDeleter interface {
	DeleteObject(ctx context.Context, params *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type S3ImageUploadStoreConfig struct {
	Bucket     string
	PublicBase string
	Uploader   s3ImageUploadWriter
	Reader     s3ImageUploadReader
	Deleter    s3ImageUploadDeleter
}

type s3ImageUploadStore struct {
	bucket     string
	publicBase string
	uploader   s3ImageUploadWriter
	reader     s3ImageUploadReader
	deleter    s3ImageUploadDeleter
}

func NewS3ImageUploadStore(cfg S3ImageUploadStoreConfig) (ImageUploadStore, error) {
	if strings.TrimSpace(cfg.Bucket) == "" {
		return nil, fmt.Errorf("bucket cannot be empty")
	}
	if cfg.Uploader == nil {
		return nil, fmt.Errorf("uploader cannot be nil")
	}
	if cfg.Reader == nil {
		return nil, fmt.Errorf("reader cannot be nil")
	}
	deleter := cfg.Deleter
	if deleter == nil {
		var ok bool
		deleter, ok = cfg.Reader.(s3ImageUploadDeleter)
		if !ok {
			return nil, fmt.Errorf("deleter cannot be nil")
		}
	}
	return &s3ImageUploadStore{
		bucket:     strings.TrimSpace(cfg.Bucket),
		publicBase: strings.TrimRight(strings.TrimSpace(cfg.PublicBase), "/"),
		uploader:   cfg.Uploader,
		reader:     cfg.Reader,
		deleter:    deleter,
	}, nil
}

func (s *s3ImageUploadStore) Save(ctx context.Context, input *ImageUploadInput) (*StoredUploadedImage, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}
	if len(input.Data) == 0 {
		return nil, fmt.Errorf("input data cannot be empty")
	}

	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(input.Data)
	}
	ext := normalizedImageExtension(input.Filename, contentType, input.Data)
	key := filepath.ToSlash(filepath.Join(time.Now().UTC().Format("20060102"), uuid.NewString()+ext))
	fallbackURL, err := s.uploader.Upload(ctx, key, input.Data, contentType)
	if err != nil {
		return nil, fmt.Errorf("upload to s3: %w", err)
	}

	filename := strings.TrimSpace(input.Filename)
	if filename == "" {
		filename = filepath.Base(key)
	}

	return &StoredUploadedImage{
		Key:          key,
		Filename:     filepath.Base(filename),
		PublicURL:    storage.ResolveObjectURL(s.publicBase, key, fallbackURL),
		ContentType:  contentType,
		Size:         int64(len(input.Data)),
		OriginalName: strings.TrimSpace(input.Filename),
	}, nil
}

func (s *s3ImageUploadStore) Delete(ctx context.Context, key string) error {
	normalizedKey := strings.TrimLeft(strings.TrimSpace(key), "/")
	if normalizedKey == "" {
		return ErrUploadedImageNotFound
	}
	_, err := s.deleter.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(normalizedKey),
	})
	if err != nil {
		return ErrUploadedImageNotFound
	}
	return nil
}

func (s *s3ImageUploadStore) Open(ctx context.Context, key string) (*StoredUploadedImage, error) {
	normalizedKey := strings.TrimLeft(strings.TrimSpace(key), "/")
	if normalizedKey == "" {
		return nil, ErrUploadedImageNotFound
	}

	output, err := s.reader.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(normalizedKey),
	})
	if err != nil {
		return nil, ErrUploadedImageNotFound
	}
	defer output.Body.Close()

	data, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, fmt.Errorf("read s3 object body: %w", err)
	}

	contentType := ""
	if output.ContentType != nil {
		contentType = strings.TrimSpace(*output.ContentType)
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}

	var size int64
	if output.ContentLength != nil {
		size = *output.ContentLength
	} else {
		size = int64(len(data))
	}

	return &StoredUploadedImage{
		Key:         normalizedKey,
		Filename:    filepath.Base(normalizedKey),
		PublicURL:   storage.ResolveObjectURL(s.publicBase, normalizedKey, ""),
		ContentType: contentType,
		Size:        size,
		Data:        data,
	}, nil
}
