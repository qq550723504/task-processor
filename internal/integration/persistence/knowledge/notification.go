package knowledge

import (
	"context"
	"github.com/google/uuid"
	k "task-processor/internal/knowledge"
)

func (r *Repository) ListNoticeSources(ctx context.Context, org, after string, limit int) ([]k.Source, string, error) {
	if limit < 1 || limit > 100 || after != "" && uuid.Validate(after) != nil {
		return nil, "", k.ErrInvalid
	}
	query := r.db.WithContext(ctx).Table("public.knowledge_sources s").Select("s.*").Joins("JOIN public.knowledge_bases b ON b.organization_id=s.organization_id AND b.id=s.base_id").Where("s.organization_id=? AND s.state=? AND b.state=?", org, k.Active, k.Active)
	if after != "" {
		query = query.Where("s.id>?", after)
	}
	var rows []k.Source
	if e := query.Order("s.id ASC").Limit(limit + 1).Find(&rows).Error; e != nil {
		return nil, "", k.ErrUnavailable
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		next = rows[len(rows)-1].ID
	}
	for i, row := range rows {
		source, e := r.GetSource(ctx, org, row.ID)
		if e != nil {
			return nil, "", safe(e)
		}
		rows[i] = source
	}
	return rows, next, nil
}
