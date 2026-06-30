package listingkit

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type MemStudioBatchTaskLinkRepository struct {
	mu    sync.Mutex
	links map[string]StudioBatchTaskLinkRecord
}

func NewMemStudioBatchTaskLinkRepository() *MemStudioBatchTaskLinkRepository {
	return &MemStudioBatchTaskLinkRepository{
		links: map[string]StudioBatchTaskLinkRecord{},
	}
}

func (r *MemStudioBatchTaskLinkRepository) GetStudioBatchTaskLinkByCandidateKey(ctx context.Context, candidateKey string) (*StudioBatchTaskLinkRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, link := range r.links {
		if link.CandidateKey != candidateKey || !matchesStudioBatchScope(ctx, link.TenantID, link.UserID) {
			continue
		}
		cloned := link
		return &cloned, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (r *MemStudioBatchTaskLinkRepository) CreateStudioBatchTaskLink(ctx context.Context, link *StudioBatchTaskLinkRecord) error {
	if link == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	row := *link
	applyStudioBatchTaskLinkCreateScope(ctx, &row)
	if strings.TrimSpace(row.ID) == "" {
		return fmt.Errorf("studio batch task link id is required")
	}
	if _, ok := r.links[row.ID]; ok {
		return fmt.Errorf("studio batch task link id already exists")
	}
	if err := r.validateUniqueLocked(row, ""); err != nil {
		return err
	}
	r.links[row.ID] = row
	return nil
}

func (r *MemStudioBatchTaskLinkRepository) UpdateStudioBatchTaskLink(ctx context.Context, link *StudioBatchTaskLinkRecord) error {
	if link == nil {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	existing, ok := r.links[link.ID]
	if !ok || !matchesStudioBatchScope(ctx, existing.TenantID, existing.UserID) {
		return gorm.ErrRecordNotFound
	}

	row := existing
	row.ListingKitTaskID = link.ListingKitTaskID
	row.Status = link.Status
	row.ReasonCode = link.ReasonCode
	row.Message = link.Message
	row.UpdatedAt = link.UpdatedAt
	r.links[row.ID] = row
	return nil
}

func (r *MemStudioBatchTaskLinkRepository) ListStudioBatchTaskLinksByBatchID(ctx context.Context, batchID string) ([]StudioBatchTaskLinkRecord, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	links := make([]StudioBatchTaskLinkRecord, 0)
	for _, link := range r.links {
		if link.BatchID != batchID || !matchesStudioBatchScope(ctx, link.TenantID, link.UserID) {
			continue
		}
		links = append(links, link)
	}
	slices.SortStableFunc(links, func(a, b StudioBatchTaskLinkRecord) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return links, nil
}

func (r *MemStudioBatchTaskLinkRepository) ClaimStudioBatchTaskCandidate(ctx context.Context, candidateKey string, fromStatus string, toStatus string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, link := range r.links {
		if link.CandidateKey != candidateKey || !matchesStudioBatchScope(ctx, link.TenantID, link.UserID) {
			continue
		}
		if link.Status != fromStatus {
			cloned := link
			return &cloned, false, nil
		}
		link.Status = toStatus
		link.UpdatedAt = updatedAt
		r.links[id] = link
		cloned := link
		return &cloned, true, nil
	}
	return nil, false, gorm.ErrRecordNotFound
}

func (r *MemStudioBatchTaskLinkRepository) ClaimStudioBatchTaskCandidateUpdatedAt(ctx context.Context, candidateKey string, fromStatus string, observedUpdatedAt time.Time, toStatus string, updatedAt time.Time) (*StudioBatchTaskLinkRecord, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, link := range r.links {
		if link.CandidateKey != candidateKey || !matchesStudioBatchScope(ctx, link.TenantID, link.UserID) {
			continue
		}
		if link.Status != fromStatus || !link.UpdatedAt.Equal(observedUpdatedAt) {
			cloned := link
			return &cloned, false, nil
		}
		link.Status = toStatus
		link.UpdatedAt = updatedAt
		r.links[id] = link
		cloned := link
		return &cloned, true, nil
	}
	return nil, false, gorm.ErrRecordNotFound
}

func (r *MemStudioBatchTaskLinkRepository) validateUniqueLocked(candidate StudioBatchTaskLinkRecord, existingID string) error {
	for _, link := range r.links {
		if link.ID == existingID || link.TenantID != candidate.TenantID {
			continue
		}
		if link.CandidateKey == candidate.CandidateKey {
			return fmt.Errorf("studio batch task link candidate key already exists")
		}
		if link.BatchID == candidate.BatchID &&
			link.ItemID == candidate.ItemID &&
			link.DesignID == candidate.DesignID &&
			link.SelectionID == candidate.SelectionID &&
			link.CompatibilityFingerprint == candidate.CompatibilityFingerprint &&
			link.SheinStoreID == candidate.SheinStoreID {
			return fmt.Errorf("studio batch task link tuple already exists")
		}
	}
	return nil
}
