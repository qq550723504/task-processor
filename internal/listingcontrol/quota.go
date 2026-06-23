package listingcontrol

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrRuntimeKeyNotFound = errors.New("runtime key not found")
	ErrQuotaInvalid       = errors.New("quota invalid")
)

const (
	ReasonQuotaExhausted = "quota_exhausted"
	ReasonQuotaInvalid   = "quota_invalid"
	ReasonQuotaStale     = "quota_stale"

	QuotaSourceLegacy = "legacy"
)

type StringRuntime interface {
	Get(ctx context.Context, key string) (string, error)
	Exists(ctx context.Context, key string) (bool, error)
	TTL(ctx context.Context, key string) (time.Duration, error)
}

type QuotaConfig struct {
	EnableLegacyQuotaKeys bool
}

type QuotaService struct {
	runtime StringRuntime
	config  QuotaConfig
}

type QuotaResult struct {
	Remaining int
	Blocked   bool
	Reason    string
	Key       string
	TTL       time.Duration
	Source    string
}

type structuredQuotaValue struct {
	Remaining int    `json:"remaining"`
	Source    string `json:"source"`
	UpdatedAt string `json:"updatedAt"`
	ExpiresAt string `json:"expiresAt"`
}

func NewQuotaService(runtime StringRuntime, config QuotaConfig) *QuotaService {
	return &QuotaService{
		runtime: runtime,
		config:  config,
	}
}

func (s *QuotaService) Check(ctx context.Context, tenantID, storeID int64) (QuotaResult, error) {
	structuredKey := structuredQuotaKey(tenantID, storeID)
	value, err := s.runtime.Get(ctx, structuredKey)
	if err != nil && !errors.Is(err, ErrRuntimeKeyNotFound) {
		return QuotaResult{}, err
	}
	if err == nil {
		return s.checkStructured(ctx, structuredKey, value)
	}

	if !s.config.EnableLegacyQuotaKeys {
		return QuotaResult{}, nil
	}

	legacyKey := legacyQuotaKey(tenantID, storeID)
	value, err = s.runtime.Get(ctx, legacyKey)
	if err != nil {
		if errors.Is(err, ErrRuntimeKeyNotFound) {
			return QuotaResult{}, nil
		}
		return QuotaResult{}, err
	}
	return s.checkLegacy(ctx, legacyKey, value)
}

func (s *QuotaService) checkStructured(ctx context.Context, key, value string) (QuotaResult, error) {
	var decoded structuredQuotaValue
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return QuotaResult{
			Blocked: true,
			Reason:  ReasonQuotaInvalid,
			Key:     key,
		}, fmt.Errorf("%w: structured quota %s: %v", ErrQuotaInvalid, key, err)
	}

	ttl, err := s.ttl(ctx, key)
	if err != nil {
		return QuotaResult{}, err
	}
	result := QuotaResult{
		Remaining: decoded.Remaining,
		Key:       key,
		TTL:       ttl,
		Source:    decoded.Source,
	}
	if ttl <= 0 {
		result.Reason = ReasonQuotaStale
		return result, nil
	}
	if strings.TrimSpace(decoded.ExpiresAt) != "" {
		expiresAt, err := time.Parse(time.RFC3339, decoded.ExpiresAt)
		if err != nil {
			result.Blocked = true
			result.Reason = ReasonQuotaInvalid
			return result, fmt.Errorf("%w: structured quota %s expiresAt: %v", ErrQuotaInvalid, key, err)
		}
		if !expiresAt.After(time.Now()) {
			result.Reason = ReasonQuotaStale
			return result, nil
		}
	}
	if decoded.Remaining <= 0 {
		result.Blocked = true
		result.Reason = ReasonQuotaExhausted
	}
	return result, nil
}

func (s *QuotaService) checkLegacy(ctx context.Context, key, value string) (QuotaResult, error) {
	remaining, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return QuotaResult{
			Blocked: true,
			Reason:  ReasonQuotaInvalid,
			Key:     key,
			Source:  QuotaSourceLegacy,
		}, fmt.Errorf("%w: legacy quota %s: %v", ErrQuotaInvalid, key, err)
	}

	ttl, err := s.ttl(ctx, key)
	if err != nil {
		return QuotaResult{}, err
	}
	result := QuotaResult{
		Remaining: remaining,
		Key:       key,
		TTL:       ttl,
		Source:    QuotaSourceLegacy,
	}
	if remaining <= 0 {
		result.Blocked = true
		result.Reason = ReasonQuotaExhausted
	}
	return result, nil
}

func (s *QuotaService) ttl(ctx context.Context, key string) (time.Duration, error) {
	ttl, err := s.runtime.TTL(ctx, key)
	if err != nil {
		if errors.Is(err, ErrRuntimeKeyNotFound) {
			return 0, nil
		}
		return 0, err
	}
	return ttl, nil
}

func structuredQuotaKey(tenantID, storeID int64) string {
	return fmt.Sprintf("listing:remaining:quota:v2:%d:%d", tenantID, storeID)
}

func legacyQuotaKey(tenantID, storeID int64) string {
	return fmt.Sprintf("listing:remaining:quota:%d:%d", tenantID, storeID)
}
