package listingkit

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

type localImageUploadStore struct {
	rootDir string
}

func NewLocalImageUploadStore(rootDir string) (ImageUploadStore, error) {
	trimmedRootDir := strings.TrimSpace(rootDir)
	if trimmedRootDir == "" {
		return nil, fmt.Errorf("root dir cannot be empty")
	}
	if err := os.MkdirAll(trimmedRootDir, 0o755); err != nil {
		return nil, fmt.Errorf("create root dir: %w", err)
	}
	return &localImageUploadStore{rootDir: trimmedRootDir}, nil
}

func (s *localImageUploadStore) Save(_ context.Context, input *ImageUploadInput) (*StoredUploadedImage, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}
	if len(input.Data) == 0 {
		return nil, fmt.Errorf("input data cannot be empty")
	}

	ext := normalizedImageExtension(input.Filename, input.ContentType, input.Data)
	dayPrefix := time.Now().UTC().Format("20060102")
	key := filepath.ToSlash(filepath.Join(dayPrefix, uuid.NewString()+ext))
	targetPath := filepath.Join(s.rootDir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return nil, fmt.Errorf("create image dir: %w", err)
	}
	if err := os.WriteFile(targetPath, input.Data, 0o644); err != nil {
		return nil, fmt.Errorf("write uploaded image: %w", err)
	}

	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(input.Data)
	}

	filename := filepath.Base(targetPath)
	if baseName := strings.TrimSpace(input.Filename); baseName != "" {
		filename = filepath.Base(baseName)
	}

	return &StoredUploadedImage{
		Key:          key,
		Filename:     filename,
		Path:         targetPath,
		ContentType:  contentType,
		Size:         int64(len(input.Data)),
		OriginalName: strings.TrimSpace(input.Filename),
	}, nil
}

func (s *localImageUploadStore) SaveWithKey(ctx context.Context, key string, input *ImageUploadInput) (*StoredUploadedImage, error) {
	if input == nil {
		return nil, fmt.Errorf("input cannot be nil")
	}
	if len(input.Data) == 0 {
		return nil, fmt.Errorf("input data cannot be empty")
	}
	normalizedKey, targetPath, err := localImageUploadTargetPath(s.rootDir, key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return nil, fmt.Errorf("create image dir: %w", err)
	}
	if err := os.WriteFile(targetPath, input.Data, 0o644); err != nil {
		return nil, fmt.Errorf("write uploaded image: %w", err)
	}

	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = http.DetectContentType(input.Data)
	}
	filename := filepath.Base(targetPath)
	if baseName := strings.TrimSpace(input.Filename); baseName != "" {
		filename = filepath.Base(baseName)
	}
	return &StoredUploadedImage{
		Key:          normalizedKey,
		Filename:     filename,
		Path:         targetPath,
		ContentType:  contentType,
		Size:         int64(len(input.Data)),
		OriginalName: strings.TrimSpace(input.Filename),
	}, nil
}

func (s *localImageUploadStore) Open(_ context.Context, key string) (*StoredUploadedImage, error) {
	normalizedKey, targetPath, err := localImageUploadTargetPath(s.rootDir, key)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrUploadedImageNotFound
		}
		return nil, fmt.Errorf("stat uploaded image: %w", err)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrUploadedImageNotFound
		}
		return nil, fmt.Errorf("read uploaded image header: %w", err)
	}

	return &StoredUploadedImage{
		Key:         normalizedKey,
		Filename:    filepath.Base(targetPath),
		Path:        targetPath,
		ContentType: http.DetectContentType(data),
		Size:        info.Size(),
	}, nil
}

func (s *localImageUploadStore) Delete(_ context.Context, key string) error {
	_, targetPath, err := localImageUploadTargetPath(s.rootDir, key)
	if err != nil {
		return err
	}
	if err := os.Remove(targetPath); err != nil {
		if os.IsNotExist(err) {
			return ErrUploadedImageNotFound
		}
		return fmt.Errorf("delete uploaded image: %w", err)
	}
	return nil
}

func localImageUploadTargetPath(rootDir, key string) (string, string, error) {
	normalizedKey := strings.TrimLeft(strings.TrimSpace(key), "/")
	if normalizedKey == "" {
		return "", "", ErrUploadedImageNotFound
	}
	targetPath := filepath.Join(rootDir, filepath.FromSlash(normalizedKey))
	cleanedRoot := filepath.Clean(rootDir)
	cleanedTarget := filepath.Clean(targetPath)
	if cleanedTarget != cleanedRoot && !strings.HasPrefix(cleanedTarget, cleanedRoot+string(os.PathSeparator)) {
		return "", "", ErrUploadedImageNotFound
	}
	return normalizedKey, cleanedTarget, nil
}

func normalizedImageExtension(filename, contentType string, data []byte) string {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filename)))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return ext
	}

	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/jpeg":
		return ".jpg"
	}

	switch http.DetectContentType(data) {
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".jpg"
	}
}
