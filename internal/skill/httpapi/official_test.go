package skillhttp

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	d "task-processor/internal/skill"
)

type officialRepo struct {
	d.Repository
	calls int
}

func (r *officialRepo) InstallOfficial(_ context.Context, p d.Principal, resolve d.ResolvePrincipal) (d.OfficialInstallResult, error) {
	r.calls++
	return d.OfficialInstallResult{Installed: []agent.SkillVersionRef{}, Existing: []agent.SkillVersionRef{}}, nil
}

func TestOfficialInstallUsesOnlyVerifiedMaintainerAndServerRelease(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &officialRepo{}
	maintainer := true
	service, err := d.NewService(repo, func(context.Context) (d.Principal, error) {
		return d.Principal{UserID: "verified-user", PlatformMaintainer: maintainer}, nil
	})
	require.NoError(t, err)
	engine := gin.New()
	for _, r := range Routes(&Handler{Service: service}) {
		engine.Handle(r.Method, r.Path, r.Handler)
	}
	request := func(body string) int {
		req := httptest.NewRequest("POST", Base+"/official/install", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", uuid.NewString())
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		return rec.Code
	}
	require.Equal(t, 200, request(`{}`))
	require.Equal(t, 1, repo.calls)
	require.Equal(t, 400, request(`{"owner":"PLATFORM","files":[]}`))
	require.Equal(t, 1, repo.calls)
	maintainer = false
	require.Equal(t, 403, request(`{}`))
	require.Equal(t, 1, repo.calls)
}
