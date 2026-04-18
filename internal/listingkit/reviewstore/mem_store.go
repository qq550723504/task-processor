package reviewstore

import (
	"context"
	"sort"
	"sync"
	"time"
)

type MemRepository struct {
	mu      sync.RWMutex
	nextID  uint
	records []ReviewRecord
}

func NewMemRepository() Repository {
	return &MemRepository{nextID: 1}
}

func (r *MemRepository) SaveReview(_ context.Context, record *ReviewRecord) error {
	if record == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cloned := *record
	if cloned.ReviewedAt.IsZero() {
		cloned.ReviewedAt = time.Now().UTC()
	}
	cloned.ID = r.nextID
	r.nextID++
	r.records = append(r.records, cloned)
	return nil
}

func (r *MemRepository) ListReviews(_ context.Context, taskID string) ([]ReviewRecord, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ReviewRecord, 0, len(r.records))
	for _, item := range r.records {
		if item.TaskID == taskID {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ReviewedAt.Equal(out[j].ReviewedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].ReviewedAt.Before(out[j].ReviewedAt)
	})
	return out, nil
}
