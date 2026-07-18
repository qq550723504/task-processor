package listingkit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
)

func (s *service) syncSDSDesignFromUploadedImagePath(ctx context.Context, task *Task, imageURL string, syncInput sdsusecase.SyncInput) (*sdsworkflow.SyncResult, bool, error) {
	key, ok := uploadedListingKitImageKeyFromURL(imageURL)
	if !ok {
		return nil, false, nil
	}
	return s.syncSDSDesignFromUploadedImageKey(ctx, task, key, syncInput, sdsDesignSyncTimeout)
}

func (s *service) syncSDSDesignFromUploadedImageKey(ctx context.Context, task *Task, key string, syncInput sdsusecase.SyncInput, timeout time.Duration) (*sdsworkflow.SyncResult, bool, error) {
	syncService := resolveSDSSyncService(s)
	if syncService == nil {
		return nil, true, fmt.Errorf("sds sync service is not configured")
	}
	uploadStore := resolveStudioUploadStore(s)
	if uploadStore == nil {
		return nil, true, fmt.Errorf("uploaded image store is not configured")
	}
	storageKey, err := s.resolveUploadedImageStorageKey(ctx, key)
	if err != nil {
		return nil, true, err
	}
	stored, err := uploadStore.Open(ctx, storageKey)
	if err != nil {
		return nil, true, err
	}
	data := stored.Data
	if len(data) == 0 {
		data, err = os.ReadFile(stored.Path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, true, ErrUploadedImageNotFound
			}
			return nil, true, fmt.Errorf("read uploaded image: %w", err)
		}
	}
	tempDir := os.TempDir()
	fileName := strings.TrimSpace(stored.Filename)
	if fileName == "" {
		fileName = studioSDSMaterialFileName(task)
	}
	tempPattern := "listingkit-sds-*-" + filepath.Base(fileName)
	tempFile, err := os.CreateTemp(tempDir, tempPattern)
	if err != nil {
		return nil, true, fmt.Errorf("create temp uploaded image: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return nil, true, fmt.Errorf("write temp uploaded image: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, true, fmt.Errorf("close temp uploaded image: %w", err)
	}
	syncCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	syncResult, err := syncService.SyncFromLocalFile(syncCtx, sdsusecase.LocalFileInput{
		Sync: syncInput,
		File: sdsworkflow.FileSource{
			Path:        tempPath,
			FileName:    filepath.Base(fileName),
			ContentType: strings.TrimSpace(stored.ContentType),
		},
	})
	if err != nil {
		return nil, true, err
	}
	return syncResult, true, nil
}

func (s *service) resolveUploadedImageStorageKey(ctx context.Context, uploadID string) (string, error) {
	repo := resolveUploadedImageRepository(s)
	if repo == nil {
		return uploadID, nil
	}
	record, err := repo.GetUploadedImage(ctx, uploadID)
	if err != nil {
		if errors.Is(err, ErrUploadedImageNotFound) {
			return uploadID, nil
		}
		return "", fmt.Errorf("resolve uploaded image metadata: %w", err)
	}
	if record == nil {
		return uploadID, nil
	}
	if storageKey := strings.TrimSpace(record.StorageKey); storageKey != "" {
		return storageKey, nil
	}
	if key := strings.TrimSpace(record.Key); key != "" {
		return key, nil
	}
	return uploadID, nil
}

func uploadedListingKitImageKeyFromURL(rawURL string) (string, bool) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", false
	}
	const prefix = "/api/v1/listing-kits/uploads/files/"
	if strings.HasPrefix(trimmed, prefix) {
		return strings.TrimPrefix(trimmed, prefix), true
	}
	const localhostPrefix = "http://localhost:3000/api/v1/listing-kits/uploads/files/"
	if strings.HasPrefix(trimmed, localhostPrefix) {
		return strings.TrimPrefix(trimmed, localhostPrefix), true
	}
	const localhostSecurePrefix = "https://localhost:3000/api/v1/listing-kits/uploads/files/"
	if strings.HasPrefix(trimmed, localhostSecurePrefix) {
		return strings.TrimPrefix(trimmed, localhostSecurePrefix), true
	}
	return "", false
}

func studioSDSMaterialFileName(task *Task) string {
	if task == nil || strings.TrimSpace(task.ID) == "" {
		return "listingkit-studio-design.png"
	}
	taskID := strings.TrimSpace(task.ID)
	if len(taskID) > 8 {
		taskID = taskID[:8]
	}
	return fmt.Sprintf("listingkit-studio-design-%s.png", taskID)
}
