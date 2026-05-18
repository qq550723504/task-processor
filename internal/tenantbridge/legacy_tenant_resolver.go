package tenantbridge

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"gorm.io/gorm"
)

const defaultMetadataTable = "projections.org_metadata2"
const defaultMetadataKey = "yudao_tenant_id"

// Resolver bridges ZITADEL tenant IDs back to legacy Yudao tenant IDs for
// tables that are still shared with the old system. This is an explicit
// compatibility layer and should be removed once the old system no longer reads
// these legacy tenant-scoped tables.
type Resolver interface {
	ResolveLegacyTenantID(ctx context.Context, tenantID string) (int64, bool, error)
}

type MetadataResolver struct {
	db          *gorm.DB
	tableName   string
	metadataKey string
	cache       sync.Map
}

type metadataRow struct {
	OrgID        string `gorm:"column:org_id"`
	Sequence     int64  `gorm:"column:sequence"`
	Key          string `gorm:"column:key"`
	Value        []byte `gorm:"column:value"`
	OwnerRemoved bool   `gorm:"column:owner_removed"`
}

type Option func(*MetadataResolver)

func WithTableName(name string) Option {
	return func(r *MetadataResolver) {
		if strings.TrimSpace(name) != "" {
			r.tableName = strings.TrimSpace(name)
		}
	}
}

func WithMetadataKey(key string) Option {
	return func(r *MetadataResolver) {
		if strings.TrimSpace(key) != "" {
			r.metadataKey = strings.TrimSpace(key)
		}
	}
}

func NewMetadataResolver(db *gorm.DB, options ...Option) *MetadataResolver {
	resolver := &MetadataResolver{
		db:          db,
		tableName:   defaultMetadataTable,
		metadataKey: defaultMetadataKey,
	}
	for _, option := range options {
		if option != nil {
			option(resolver)
		}
	}
	return resolver
}

func (r *MetadataResolver) ResolveLegacyTenantID(ctx context.Context, tenantID string) (int64, bool, error) {
	if r == nil || r.db == nil {
		return 0, false, nil
	}
	trimmed := strings.TrimSpace(tenantID)
	if trimmed == "" {
		return 0, false, nil
	}
	if cached, ok := r.cache.Load(trimmed); ok {
		return cached.(int64), true, nil
	}

	var row metadataRow
	err := r.db.WithContext(ctx).
		Table(r.tableName).
		Select("org_id", "key", "value", "owner_removed").
		Where("org_id = ? AND key = ? AND owner_removed = ?", trimmed, r.metadataKey, false).
		Order("sequence DESC").
		Take(&row).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("resolve legacy tenant id for %s: %w", trimmed, err)
	}
	value, err := strconv.ParseInt(strings.TrimSpace(string(row.Value)), 10, 64)
	if err != nil || value <= 0 {
		return 0, false, fmt.Errorf("resolve legacy tenant id for %s: invalid metadata value %q", trimmed, strings.TrimSpace(string(row.Value)))
	}
	r.cache.Store(trimmed, value)
	return value, true, nil
}

var resolverState struct {
	mu       sync.RWMutex
	resolver Resolver
}

func ConfigureLegacyTenantResolver(resolver Resolver) func() {
	resolverState.mu.Lock()
	previous := resolverState.resolver
	resolverState.resolver = resolver
	resolverState.mu.Unlock()
	return func() {
		resolverState.mu.Lock()
		resolverState.resolver = previous
		resolverState.mu.Unlock()
	}
}

func ResolveLegacyTenantID(ctx context.Context, tenantID string) (int64, error) {
	trimmed := strings.TrimSpace(tenantID)
	if trimmed == "" {
		return 0, fmt.Errorf("tenant id is required")
	}
	resolverState.mu.RLock()
	current := resolverState.resolver
	resolverState.mu.RUnlock()
	if current != nil {
		value, ok, err := current.ResolveLegacyTenantID(ctx, trimmed)
		if err != nil {
			return 0, err
		}
		if ok && value > 0 {
			return value, nil
		}
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("tenant id is required")
	}
	return value, nil
}
