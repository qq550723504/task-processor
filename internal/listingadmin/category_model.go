package listingadmin

import "strings"

func (r listingCategory) toCategory() Category {
	return Category{
		ID:          r.ID,
		TenantID:    r.TenantID,
		Name:        r.Name,
		Code:        r.Code,
		ParentID:    r.ParentID,
		Level:       r.Level,
		Sort:        r.Sort,
		Icon:        r.Icon,
		Image:       r.Image,
		Description: r.Description,
		Status:      r.Status,
		CreateTime:  r.CreateTime,
		UpdateTime:  r.UpdateTime,
	}
}

func listingCategoryFromCategory(category *Category) listingCategory {
	if category == nil {
		return listingCategory{}
	}
	return listingCategory{
		ID:          category.ID,
		TenantID:    category.TenantID,
		Name:        strings.TrimSpace(category.Name),
		Code:        strings.TrimSpace(category.Code),
		ParentID:    category.ParentID,
		Level:       category.Level,
		Sort:        category.Sort,
		Icon:        strings.TrimSpace(category.Icon),
		Image:       strings.TrimSpace(category.Image),
		Description: strings.TrimSpace(category.Description),
		Status:      category.Status,
	}
}

func applyCategoryDefaults(row *listingCategory) {
	if row.Level <= 0 {
		row.Level = 1
	}
}

func applyCategoryAuditFields(row *listingCategory, userID string, includeCreate bool) {
	trimmedUserID := strings.TrimSpace(userID)
	if trimmedUserID == "" {
		return
	}
	row.OwnerUserID = trimmedUserID
	row.Updater = trimmedUserID
	row.UpdatedBy = trimmedUserID
	if includeCreate {
		row.Creator = trimmedUserID
		row.CreatedBy = trimmedUserID
	}
}
