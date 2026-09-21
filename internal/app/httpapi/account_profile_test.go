package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	"task-processor/internal/httproute"
	profileStore "task-processor/internal/integration/persistence/accountprofile"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestAccountBusinessProfilePersistsAcrossReadsWithoutOrganizationLeakage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&profileRowForTest{}))
	repository, err := profileStore.New(db)
	require.NoError(t, err)
	module := accountProfileModule{repository: repository}

	request := func(method, body string, userID string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(recorder)
		request := httptest.NewRequest(method, accountBusinessProfilePath, strings.NewReader(body))
		request = request.WithContext(authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: userID, HomeOrganizationID: "home-a"}))
		ginContext.Request = request
		if method == http.MethodPut {
			module.update(ginContext)
		} else {
			module.read(ginContext)
		}
		return recorder
	}

	initial := request(http.MethodGet, "", "user-a")
	require.Equal(t, http.StatusOK, initial.Code)
	require.Contains(t, initial.Body.String(), `"userId":"user-a"`)
	require.Contains(t, initial.Body.String(), `"userRole":null`)

	saved := request(http.MethodPut, `{"userRole":"品牌方","shopSituation":"已有店铺","factorySituation":"无工厂","platforms":["1688"],"sites":["中国"],"shopType":"品牌店","services":["选品"]}`, "user-a")
	require.Equal(t, http.StatusOK, saved.Code, saved.Body.String())
	var payload map[string]any
	require.NoError(t, json.Unmarshal(saved.Body.Bytes(), &payload))
	require.Equal(t, "品牌方", payload["userRole"])

	other := request(http.MethodGet, "", "user-b")
	require.Equal(t, http.StatusOK, other.Code)
	require.Contains(t, other.Body.String(), `"userId":"user-b"`)
	require.Contains(t, other.Body.String(), `"userRole":null`)
	require.NotContains(t, other.Body.String(), "品牌方")
}

func TestAccountBusinessProfileMutationRequiresLiveOrganizationResolution(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	repository, err := profileStore.New(db)
	require.NoError(t, err)
	modules := kernelmodule.NewRegistry()
	require.NoError(t, (accountProfileModule{repository: repository}).Register(modules))
	var mutation *httproute.Descriptor
	for _, route := range modules.Routes() {
		if route.Method == http.MethodPut && route.Path == accountBusinessProfilePath {
			copy := route
			mutation = &copy
		}
	}
	require.NotNil(t, mutation)
	require.Equal(t, httproute.OrganizationAccessPolicyLiveWrite, mutation.OrganizationAccessPolicy)
	require.NotNil(t, mutation.OrganizationTargetResolver)
}

// The production repository intentionally keeps the row private. This test
// mirrors its columns so the handler contract can run without a PostgreSQL
// fixture; PostgreSQL schema and permissions are verified by the schema tests.
type profileRowForTest struct {
	UserID           string    `gorm:"column:user_id;primaryKey"`
	UserRole         string    `gorm:"column:user_role"`
	ShopSituation    string    `gorm:"column:shop_situation"`
	FactorySituation string    `gorm:"column:factory_situation"`
	Platforms        string    `gorm:"column:platforms"`
	Sites            string    `gorm:"column:sites"`
	ShopType         string    `gorm:"column:shop_type"`
	Services         string    `gorm:"column:services"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (profileRowForTest) TableName() string { return profileStore.TableName }
