package listingadmin

import "gorm.io/gorm"

func findPagedRows[T any](db *gorm.DB, page, pageSize int, rows *[]T) (total int64, normalizedPage int, normalizedPageSize int, err error) {
	normalizedPage, normalizedPageSize = normalizePage(page, pageSize)
	if err = db.Count(&total).Error; err != nil {
		return 0, normalizedPage, normalizedPageSize, err
	}
	if err = db.Order("id desc").Offset((normalizedPage - 1) * normalizedPageSize).Limit(normalizedPageSize).Find(rows).Error; err != nil {
		return 0, normalizedPage, normalizedPageSize, err
	}
	return total, normalizedPage, normalizedPageSize, nil
}
