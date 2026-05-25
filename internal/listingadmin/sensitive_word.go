package listingadmin

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrSensitiveWordNotFound = errors.New("sensitive word not found")

type SensitiveWord struct {
	ID          int64      `json:"id"`
	TenantID    int64      `json:"tenantId"`
	Word        string     `json:"word"`
	Language    string     `json:"language"`
	Tags        string     `json:"tags,omitempty"`
	Level       int        `json:"level"`
	ReplaceWord string     `json:"replaceWord,omitempty"`
	Remark      string     `json:"remark,omitempty"`
	Status      int16      `json:"status"`
	CreateTime  *time.Time `json:"createTime,omitempty"`
	UpdateTime  *time.Time `json:"updateTime,omitempty"`
}

type SensitiveWordQuery struct {
	TenantID    int64
	OwnerUserID string
	Page        int
	PageSize    int
	Word        string
	Language    string
	Tags        string
	Level       *int
	Status      *int16
	Remark      string
}

type SensitiveWordPage struct {
	Items    []SensitiveWord `json:"items"`
	Total    int64           `json:"total"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
}

type SensitiveWordRepository interface {
	ListSensitiveWords(ctx context.Context, query SensitiveWordQuery) (*SensitiveWordPage, error)
	GetSensitiveWord(ctx context.Context, tenantID, id int64) (*SensitiveWord, error)
	CreateSensitiveWord(ctx context.Context, word *SensitiveWord) (*SensitiveWord, error)
	UpdateSensitiveWord(ctx context.Context, word *SensitiveWord) (*SensitiveWord, error)
	UpdateSensitiveWordStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*SensitiveWord, error)
	DeleteSensitiveWord(ctx context.Context, tenantID, id int64) error
}

type listingSensitiveWord struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement"`
	TenantID    int64      `gorm:"column:tenant_id;not null;index"`
	OwnerUserID string     `gorm:"column:owner_user_id;type:varchar(128);index"`
	Word        string     `gorm:"column:word;not null;index"`
	Language    string     `gorm:"column:language;not null;index"`
	Tags        string     `gorm:"column:tags"`
	Level       int        `gorm:"column:level;not null;default:1;index"`
	ReplaceWord string     `gorm:"column:replace_word"`
	Remark      string     `gorm:"column:remark"`
	Status      int16      `gorm:"column:status;not null;default:0;index"`
	Creator     string     `gorm:"column:creator"`
	CreatedBy   string     `gorm:"column:created_by;type:varchar(128)"`
	CreateTime  *time.Time `gorm:"column:create_time;autoCreateTime"`
	Updater     string     `gorm:"column:updater"`
	UpdatedBy   string     `gorm:"column:updated_by;type:varchar(128)"`
	UpdateTime  *time.Time `gorm:"column:update_time;autoUpdateTime"`
	Deleted     int16      `gorm:"column:deleted;not null;default:0;index"`
}

func (listingSensitiveWord) TableName() string {
	return "listing_sensitive_word"
}

func (r listingSensitiveWord) toSensitiveWord() SensitiveWord {
	return SensitiveWord{
		ID:          r.ID,
		TenantID:    r.TenantID,
		Word:        r.Word,
		Language:    r.Language,
		Tags:        r.Tags,
		Level:       r.Level,
		ReplaceWord: r.ReplaceWord,
		Remark:      r.Remark,
		Status:      r.Status,
		CreateTime:  r.CreateTime,
		UpdateTime:  r.UpdateTime,
	}
}

func listingSensitiveWordFromSensitiveWord(word *SensitiveWord) listingSensitiveWord {
	if word == nil {
		return listingSensitiveWord{}
	}
	return listingSensitiveWord{
		ID:          word.ID,
		TenantID:    word.TenantID,
		Word:        strings.TrimSpace(word.Word),
		Language:    strings.TrimSpace(word.Language),
		Tags:        strings.TrimSpace(word.Tags),
		Level:       word.Level,
		ReplaceWord: strings.TrimSpace(word.ReplaceWord),
		Remark:      strings.TrimSpace(word.Remark),
		Status:      word.Status,
	}
}

type GormSensitiveWordRepository struct{ db *gorm.DB }

func NewGormSensitiveWordRepository(db *gorm.DB) *GormSensitiveWordRepository {
	return &GormSensitiveWordRepository{db: db}
}

func AutoMigrateSensitiveWordRepository(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	return ensureOwnerAuditColumns(db, (listingSensitiveWord{}).TableName())
}

func (r *GormSensitiveWordRepository) ListSensitiveWords(ctx context.Context, query SensitiveWordQuery) (*SensitiveWordPage, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("sensitive word repository database is not configured")
	}
	db := applySensitiveWordQuery(r.db.WithContext(ctx).Table("listing_sensitive_word"), query)
	var rows []listingSensitiveWord
	total, page, pageSize, err := findPagedRows(db, query.Page, query.PageSize, &rows)
	if err != nil {
		return nil, err
	}
	items := make([]SensitiveWord, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.toSensitiveWord())
	}
	return &SensitiveWordPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

func (r *GormSensitiveWordRepository) GetSensitiveWord(ctx context.Context, tenantID, id int64) (*SensitiveWord, error) {
	var row listingSensitiveWord
	err := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_sensitive_word").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrSensitiveWordNotFound
	}
	if err != nil {
		return nil, err
	}
	word := row.toSensitiveWord()
	return &word, nil
}

func (r *GormSensitiveWordRepository) CreateSensitiveWord(ctx context.Context, word *SensitiveWord) (*SensitiveWord, error) {
	row := listingSensitiveWordFromSensitiveWord(word)
	applySensitiveWordDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		row.OwnerUserID = ownerUserID
		row.Creator = ownerUserID
		row.CreatedBy = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
	}
	if err := r.db.WithContext(ctx).Table("listing_sensitive_word").Create(&row).Error; err != nil {
		return nil, err
	}
	created := row.toSensitiveWord()
	return &created, nil
}

func (r *GormSensitiveWordRepository) UpdateSensitiveWord(ctx context.Context, word *SensitiveWord) (*SensitiveWord, error) {
	row := listingSensitiveWordFromSensitiveWord(word)
	applySensitiveWordDefaults(&row)
	if ownerUserID := requestUserIDFromContext(ctx); ownerUserID != "" {
		row.OwnerUserID = ownerUserID
		row.Updater = ownerUserID
		row.UpdatedBy = ownerUserID
	}
	updates := map[string]any{
		"owner_user_id": row.OwnerUserID,
		"word":          row.Word,
		"language":      row.Language,
		"tags":          row.Tags,
		"level":         row.Level,
		"replace_word":  row.ReplaceWord,
		"remark":        row.Remark,
		"status":        row.Status,
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_sensitive_word").Where("tenant_id = ? AND id = ? AND deleted = 0", row.TenantID, row.ID),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrSensitiveWordNotFound
	}
	return r.GetSensitiveWord(ctx, row.TenantID, row.ID)
}

func (r *GormSensitiveWordRepository) UpdateSensitiveWordStatus(ctx context.Context, tenantID, id int64, status int16, remark string) (*SensitiveWord, error) {
	updates := map[string]any{"status": status}
	if strings.TrimSpace(remark) != "" {
		updates["remark"] = strings.TrimSpace(remark)
	}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_sensitive_word").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrSensitiveWordNotFound
	}
	return r.GetSensitiveWord(ctx, tenantID, id)
}

func (r *GormSensitiveWordRepository) DeleteSensitiveWord(ctx context.Context, tenantID, id int64) error {
	updates := map[string]any{"deleted": 1}
	if updatedBy := requestUserIDFromContext(ctx); updatedBy != "" {
		updates["updater"] = updatedBy
		updates["updated_by"] = updatedBy
	}
	res := applyOwnerScope(
		r.db.WithContext(ctx).Table("listing_sensitive_word").Where("tenant_id = ? AND id = ? AND deleted = 0", tenantID, id),
		ctx,
		"owner_user_id",
	).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrSensitiveWordNotFound
	}
	return nil
}

func applySensitiveWordDefaults(row *listingSensitiveWord) {
	if row.Language == "" {
		row.Language = "en"
	}
	if row.Level <= 0 {
		row.Level = 1
	}
}

func applySensitiveWordQuery(db *gorm.DB, query SensitiveWordQuery) *gorm.DB {
	db = applyOwnedTenantQuery(db, query.TenantID, strings.TrimSpace(query.OwnerUserID))
	if query.Word != "" {
		db = db.Where("word LIKE ?", "%"+query.Word+"%")
	}
	if query.Language != "" {
		db = db.Where("language = ?", query.Language)
	}
	if query.Tags != "" {
		db = db.Where("tags LIKE ?", "%"+query.Tags+"%")
	}
	if query.Level != nil {
		db = db.Where("level = ?", *query.Level)
	}
	if query.Status != nil {
		db = db.Where("status = ?", *query.Status)
	}
	if query.Remark != "" {
		db = db.Where("remark LIKE ?", "%"+query.Remark+"%")
	}
	return db
}
