package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"task-processor/internal/app/taskstatus"
	"task-processor/internal/core/logger"
	"task-processor/internal/model"
)

const defaultClaimJournalFile = "task-processor-claim-journal.json"

type ClaimJournalEntry struct {
	TaskID       int64            `json:"task_id"`
	ClaimedAt    time.Time        `json:"claimed_at"`
	FromStatus   model.TaskStatus `json:"from_status"`
	ProductID    string           `json:"product_id,omitempty"`
	StoreID      int64            `json:"store_id,omitempty"`
	Platform     string           `json:"platform,omitempty"`
	ErrorMessage string           `json:"error_message,omitempty"`
}

type ClaimJournal struct {
	path string
	mu   sync.Mutex
}

func NewClaimJournal(path string) *ClaimJournal {
	if path == "" {
		path = filepath.Join(os.TempDir(), defaultClaimJournalFile)
	}
	return &ClaimJournal{path: path}
}

func (j *ClaimJournal) Record(entry ClaimJournalEntry) error {
	if j == nil {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	entries, err := j.readLocked()
	if err != nil {
		return err
	}
	entries[fmt.Sprintf("%d", entry.TaskID)] = entry
	return j.writeLocked(entries)
}

func (j *ClaimJournal) Remove(taskID int64) error {
	if j == nil {
		return nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	entries, err := j.readLocked()
	if err != nil {
		return err
	}
	delete(entries, fmt.Sprintf("%d", taskID))
	return j.writeLocked(entries)
}

func (j *ClaimJournal) LoadAll() ([]ClaimJournalEntry, error) {
	if j == nil {
		return nil, nil
	}

	j.mu.Lock()
	defer j.mu.Unlock()

	entries, err := j.readLocked()
	if err != nil {
		return nil, err
	}

	result := make([]ClaimJournalEntry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	return result, nil
}

func (j *ClaimJournal) readLocked() (map[string]ClaimJournalEntry, error) {
	if j.path == "" {
		return map[string]ClaimJournalEntry{}, nil
	}

	data, err := os.ReadFile(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]ClaimJournalEntry{}, nil
		}
		return nil, fmt.Errorf("read claim journal: %w", err)
	}
	if len(data) == 0 {
		return map[string]ClaimJournalEntry{}, nil
	}

	var entries map[string]ClaimJournalEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("decode claim journal: %w", err)
	}
	if entries == nil {
		entries = map[string]ClaimJournalEntry{}
	}
	return entries, nil
}

func (j *ClaimJournal) writeLocked(entries map[string]ClaimJournalEntry) error {
	if j.path == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(j.path), 0o755); err != nil {
		return fmt.Errorf("create claim journal dir: %w", err)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode claim journal: %w", err)
	}
	if err := os.WriteFile(j.path, data, 0o644); err != nil {
		return fmt.Errorf("write claim journal: %w", err)
	}
	return nil
}

func (f *TaskFetcher) recordClaimJournalEntry(taskID int64, entry *ClaimJournalEntry) error {
	if f == nil || f.claimJournal == nil || entry == nil {
		return nil
	}

	if err := f.claimJournal.Record(*entry); err != nil {
		logger.GetGlobalLogger("app/task").WithError(err).Warnf("写入 claim journal 失败: TaskID=%d", taskID)
		return err
	}
	return nil
}

func (f *TaskFetcher) removeClaimJournalEntry(taskID int64) {
	if f == nil || f.claimJournal == nil {
		return
	}

	if err := f.claimJournal.Remove(taskID); err != nil {
		logger.GetGlobalLogger("app/task").WithError(err).Warnf("移除 claim journal 失败: TaskID=%d", taskID)
	}
}

func (f *TaskFetcher) recoverInterruptedClaims() error {
	if f == nil || f.claimJournal == nil {
		return nil
	}

	entries, err := f.claimJournal.LoadAll()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	statusService := f.newTaskStatusService("app/task_fetcher_recovery")
	recovered := 0

	for _, entry := range entries {
		if err := statusService.UpdateSyncWithInput(taskStatusRecoveryInput(entry)); err != nil {
			logger.GetGlobalLogger("app/task").WithError(err).Warnf(
				"恢复中断 claim 失败: TaskID=%d, ClaimedAt=%s",
				entry.TaskID,
				entry.ClaimedAt.Format(time.RFC3339),
			)
			continue
		}

		f.removeClaimJournalEntry(entry.TaskID)
		recovered++
		logger.GetGlobalLogger("app/task").Warnf(
			"已恢复中断 claim 任务为 pending_retry: TaskID=%d, ClaimedAt=%s, ProductID=%s",
			entry.TaskID,
			entry.ClaimedAt.Format(time.RFC3339),
			entry.ProductID,
		)
	}

	if recovered > 0 {
		logger.GetGlobalLogger("app/task").Warnf("启动恢复完成: %d 个中断任务已回收到 pending_retry", recovered)
	}
	return nil
}

func taskStatusRecoveryInput(entry ClaimJournalEntry) taskstatus.UpdateInput {
	return taskStatusRecoveryInputWithFallback(entry, "task processing interrupted, recovered on startup")
}

func taskStatusRecoveryInputWithFallback(entry ClaimJournalEntry, fallback string) taskstatus.UpdateInput {
	errorMsg := entry.ErrorMessage
	if errorMsg == "" {
		errorMsg = fallback
	}

	return taskstatus.UpdateInput{
		TaskID:                entry.TaskID,
		Status:                model.TaskStatusPendingRetry,
		ErrorMessage:          errorMsg,
		ExpectedCurrentStatus: taskStatusPtr(model.TaskStatusProcessing),
	}
}

func taskStatusPtr(status model.TaskStatus) *model.TaskStatus {
	return &status
}
