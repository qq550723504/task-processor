package listingkit

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productenrich"
)

const canonicalProductCacheFingerprintVersion = "listingkit:canonical_product:v1:"

type CanonicalProductCacheEntry struct {
	Fingerprint  string                        `json:"fingerprint" gorm:"primaryKey;type:varchar(128)"`
	Product      *CanonicalProductCachePayload `json:"product" gorm:"type:text"`
	SourceTaskID string                        `json:"source_task_id,omitempty" gorm:"type:varchar(36);index"`
	CreatedAt    time.Time                     `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time                     `json:"updated_at" gorm:"autoUpdateTime"`
}

func (CanonicalProductCacheEntry) TableName() string {
	return "listing_kit_canonical_product_cache"
}

type CanonicalProductCachePayload canonical.Product

func NewCanonicalProductCacheEntry(fingerprint string, product *canonical.Product, sourceTaskID string) (*CanonicalProductCacheEntry, error) {
	if strings.TrimSpace(fingerprint) == "" {
		return nil, fmt.Errorf("fingerprint cannot be empty")
	}
	payload, err := newCanonicalProductCachePayload(product)
	if err != nil {
		return nil, err
	}
	return &CanonicalProductCacheEntry{
		Fingerprint:  fingerprint,
		Product:      payload,
		SourceTaskID: sourceTaskID,
	}, nil
}

func (e *CanonicalProductCacheEntry) CanonicalProduct() (*canonical.Product, error) {
	if e == nil || e.Product == nil {
		return nil, nil
	}
	return e.Product.CanonicalProduct()
}

func newCanonicalProductCachePayload(product *canonical.Product) (*CanonicalProductCachePayload, error) {
	if product == nil {
		return nil, fmt.Errorf("canonical product cannot be nil")
	}
	raw, err := json.Marshal(product)
	if err != nil {
		return nil, err
	}
	var payload CanonicalProductCachePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (p CanonicalProductCachePayload) Value() (driver.Value, error) {
	return json.Marshal(p)
}

func (p *CanonicalProductCachePayload) Scan(value any) error {
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, p)
}

func (p *CanonicalProductCachePayload) CanonicalProduct() (*canonical.Product, error) {
	if p == nil {
		return nil, nil
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var product canonical.Product
	if err := json.Unmarshal(raw, &product); err != nil {
		return nil, err
	}
	return &product, nil
}

func canonicalProductFingerprintForTask(task *Task) string {
	if task == nil {
		return ""
	}
	return canonicalProductFingerprintFromRequest(toProductGenerateRequest(task))
}

func canonicalProductFingerprintFromRequest(req *productenrich.GenerateRequest) string {
	if req == nil {
		return ""
	}
	source := struct {
		ImageURLs  []string `json:"image_urls,omitempty"`
		Text       string   `json:"text,omitempty"`
		ProductURL string   `json:"product_url,omitempty"`
	}{
		ImageURLs:  normalizedFingerprintStrings(req.ImageURLs),
		Text:       strings.TrimSpace(strings.Join(strings.Fields(req.Text), " ")),
		ProductURL: strings.TrimSpace(req.ProductURL),
	}
	if len(source.ImageURLs) == 0 && source.Text == "" && source.ProductURL == "" {
		return ""
	}
	raw, err := json.Marshal(source)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return fmt.Sprintf("%s%x", canonicalProductCacheFingerprintVersion, sum[:])
}

func normalizedFingerprintStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func (s *service) getCachedCanonicalProduct(ctx context.Context, task *Task) (*canonical.Product, bool, error) {
	cacheRepo, ok := s.repo.(CanonicalProductCacheRepository)
	if !ok {
		return nil, false, nil
	}
	fingerprint := canonicalProductFingerprintForTask(task)
	if fingerprint == "" {
		return nil, false, nil
	}
	product, err := cacheRepo.GetCanonicalProductCache(ctx, fingerprint)
	if err != nil {
		return nil, false, err
	}
	if product == nil {
		return nil, false, nil
	}
	return product, true, nil
}

func (s *service) saveCanonicalProductCache(ctx context.Context, task *Task, product *canonical.Product) error {
	cacheRepo, ok := s.repo.(CanonicalProductCacheRepository)
	if !ok || product == nil {
		return nil
	}
	fingerprint := canonicalProductFingerprintForTask(task)
	if fingerprint == "" {
		return nil
	}
	return cacheRepo.SaveCanonicalProductCache(ctx, fingerprint, product, task.ID)
}
