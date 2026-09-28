package accountprofile

import (
	"context"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func TestPreferencesPersistBySubjectAndNormalizeRegion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&preferencesRow{}))
	repo, err := New(db)
	require.NoError(t, err)
	saved, err := repo.SavePreferences(context.Background(), UserPreferences{UserID: "user-1", Country: " 中国大陆 ", Province: "浙江省", City: "杭州市"})
	require.NoError(t, err)
	require.Equal(t, "中国大陆", saved.Country)
	require.NotNil(t, saved.UpdatedAt)
	reread, err := repo.ReadPreferences(context.Background(), "user-1")
	require.NoError(t, err)
	require.Equal(t, "杭州市", reread.City)
	other, err := repo.ReadPreferences(context.Background(), "user-2")
	require.NoError(t, err)
	require.Empty(t, other.Country)
	require.Nil(t, other.UpdatedAt)
	_, err = repo.SavePreferences(context.Background(), UserPreferences{UserID: "user-1", Country: "bad\ncountry"})
	require.Error(t, err)
	reread, err = repo.ReadPreferences(context.Background(), "user-1")
	require.NoError(t, err)
	require.Equal(t, "中国大陆", reread.Country)
}
