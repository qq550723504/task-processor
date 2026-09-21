package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/httproute"
	profileStore "task-processor/internal/integration/persistence/accountprofile"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestAccountBusinessProfilePersistsAcrossReadsWithoutOrganizationLeakage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&profileRowForTest{}, &profileAuditRowForTest{}))
	repository, err := profileStore.New(db)
	require.NoError(t, err)
	module := accountProfileModule{repository: repository}

	request := func(method, body string, userID string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ginContext, _ := gin.CreateTestContext(recorder)
		request := httptest.NewRequest(method, accountBusinessProfilePath, strings.NewReader(body))
		request = request.WithContext(authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: userID, HomeOrganizationID: "home-a", EffectiveOrganizationID: "home-a"}))
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

type accountIdentitySelfServiceSpy struct {
	token     string
	operation zitadel.SelfServiceOperation
	body      []byte
	profile   zitadel.SelfServiceProfile
}

func (s *accountIdentitySelfServiceSpy) Execute(_ context.Context, token string, operation zitadel.SelfServiceOperation, body []byte) error {
	s.token, s.operation, s.body = token, operation, append([]byte(nil), body...)
	return nil
}

func (s *accountIdentitySelfServiceSpy) ReadProfile(context.Context, string) (zitadel.SelfServiceProfile, error) {
	return s.profile, nil
}

func TestAccountIdentityUsesVerifiedUserTokenWithoutOrganizationContext(t *testing.T) {
	spy := &accountIdentitySelfServiceSpy{}
	module := accountIdentityModule{client: spy}
	request := httptest.NewRequest(http.MethodPut, accountIdentityEmailPath, strings.NewReader(`{"email":"user@example.test"}`))
	request = request.WithContext(zitadel.WithBearerToken(authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "user-a"}), "user-token"))
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = request
	module.execute(ginContext)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.Equal(t, "user-token", spy.token)
	require.Equal(t, zitadel.SelfServiceSetEmail, spy.operation)
	require.JSONEq(t, `{"email":"user@example.test"}`, string(spy.body))
	require.Contains(t, recorder.Body.String(), `"state":"verification_pending"`)
}

func TestAccountIdentityRegistersOfficialOperationsWithCorrectMethods(t *testing.T) {
	registry := kernelmodule.NewRegistry()
	require.NoError(t, (accountIdentityModule{client: &accountIdentitySelfServiceSpy{}}).Register(registry))
	want := map[string][]string{
		accountIdentityProfilePath:     {http.MethodGet, http.MethodPut},
		accountIdentityEmailPath:       {http.MethodPut},
		accountIdentityEmailResendPath: {http.MethodPost},
		accountIdentityEmailVerifyPath: {http.MethodPost},
		accountIdentityPhonePath:       {http.MethodPut},
		accountIdentityPhoneResendPath: {http.MethodPost},
		accountIdentityPhoneVerifyPath: {http.MethodPost},
		accountIdentityPasswordPath:    {http.MethodPut},
	}
	for path := range want {
		slices.Sort(want[path])
	}
	got := make(map[string][]string)
	for _, route := range registry.Routes() {
		got[route.Path] = append(got[route.Path], route.Method)
	}
	for path := range got {
		slices.Sort(got[path])
	}
	require.Equal(t, want, got)
}

func TestAccountIdentityReadsProfileUsingVerifiedUserToken(t *testing.T) {
	spy := &accountIdentitySelfServiceSpy{profile: zitadel.SelfServiceProfile{FirstName: "First", LastName: "Last", DisplayName: "Name"}}
	module := accountIdentityModule{client: spy}
	request := httptest.NewRequest(http.MethodGet, accountIdentityProfilePath, nil)
	request = request.WithContext(zitadel.WithBearerToken(authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{UserID: "user-a"}), "user-token"))
	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = request
	module.readProfile(ginContext)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	require.JSONEq(t, `{"schemaVersion":"account-identity-profile-v1","userId":"user-a","firstName":"First","lastName":"Last","nickName":"","displayName":"Name","preferredLanguage":"","gender":"","source":"zitadel_auth_v1"}`, recorder.Body.String())
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
	var read *httproute.Descriptor
	for _, route := range modules.Routes() {
		if route.Method == http.MethodGet && route.Path == accountBusinessProfilePath {
			copy := route
			read = &copy
		}
	}
	require.NotNil(t, read)
	require.Equal(t, httproute.OrganizationAccessPolicyContextRead, read.OrganizationAccessPolicy)
}

// The production repository intentionally keeps the row private. This test
// mirrors its columns so the handler contract can run without a PostgreSQL
// fixture; PostgreSQL schema and permissions are verified by the schema tests.
type profileRowForTest struct {
	OrganizationID   string    `gorm:"column:organization_id;primaryKey"`
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

type profileAuditRowForTest struct {
	ID             uint      `gorm:"column:id;primaryKey;autoIncrement"`
	OrganizationID string    `gorm:"column:organization_id;not null"`
	ActorID        string    `gorm:"column:actor_id;not null"`
	UserID         string    `gorm:"column:user_id;not null"`
	Operation      string    `gorm:"column:operation;not null"`
	Version        int64     `gorm:"column:version;not null"`
	CreatedAt      time.Time `gorm:"column:created_at;not null"`
}

func (profileAuditRowForTest) TableName() string { return "account_business_profile_audit_events" }
