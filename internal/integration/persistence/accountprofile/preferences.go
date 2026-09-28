package accountprofile

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"task-processor/internal/authidentity"
	"time"
	"unicode"
	"unicode/utf8"
)

type UserPreferences struct {
	UserID    string
	Country   string
	Province  string
	City      string
	UpdatedAt *time.Time
}
type preferencesRow struct {
	UserID    string    `gorm:"column:user_id;primaryKey"`
	Country   string    `gorm:"column:country"`
	Province  string    `gorm:"column:province"`
	City      string    `gorm:"column:city"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (preferencesRow) TableName() string { return "account_user_preferences" }
func (r *Repository) ReadPreferences(ctx context.Context, subject string) (UserPreferences, error) {
	if r == nil || r.db == nil || !authidentity.IsBoundedIdentifier(subject) {
		return UserPreferences{}, errors.New("invalid preferences subject")
	}
	var row preferencesRow
	err := r.db.WithContext(ctx).Where("user_id = ?", subject).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return UserPreferences{UserID: subject}, nil
	}
	if err != nil {
		return UserPreferences{}, err
	}
	return UserPreferences{UserID: row.UserID, Country: row.Country, Province: row.Province, City: row.City, UpdatedAt: &row.UpdatedAt}, nil
}
func (r *Repository) SavePreferences(ctx context.Context, input UserPreferences) (UserPreferences, error) {
	if r == nil || r.db == nil || !authidentity.IsBoundedIdentifier(input.UserID) {
		return UserPreferences{}, errors.New("invalid preferences subject")
	}
	for _, value := range []*string{&input.Country, &input.Province, &input.City} {
		if !utf8.ValidString(*value) || len(*value) > 128 {
			return UserPreferences{}, errors.New("invalid region")
		}
		for _, char := range *value {
			if unicode.IsControl(char) {
				return UserPreferences{}, errors.New("invalid region")
			}
		}
		*value = strings.TrimSpace(*value)
	}
	row := preferencesRow{UserID: input.UserID, Country: input.Country, Province: input.Province, City: input.City, UpdatedAt: time.Now().UTC()}
	if err := r.db.WithContext(ctx).Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "user_id"}}, DoUpdates: clause.AssignmentColumns([]string{"country", "province", "city", "updated_at"})}).Create(&row).Error; err != nil {
		return UserPreferences{}, err
	}
	return UserPreferences{UserID: row.UserID, Country: row.Country, Province: row.Province, City: row.City, UpdatedAt: &row.UpdatedAt}, nil
}
