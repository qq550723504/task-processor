//go:build integration

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"task-processor/internal/authidentity"
)

// This optional lifecycle surrounds the same PostgreSQL fixture and owner path.
// It is compiled only into integration test binaries, never a production server.
type commercialBrowserFixture struct {
	directory string
	tokens    map[string]string
	suspended atomic.Bool
	signedOut atomic.Bool
	slow      atomic.Bool
}

func newCommercialBrowserFixture(t *testing.T) *commercialBrowserFixture {
	t.Helper()
	directory := os.Getenv("ISSUE347_BROWSER_FIXTURE_DIR")
	if directory == "" {
		return nil
	}
	absolute, err := filepath.Abs(directory)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(filepath.Base(absolute), "issue347-browser-"), "requires a launcher-created task directory")
	info, err := os.Stat(absolute)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	return &commercialBrowserFixture{directory: absolute, tokens: map[string]string{"owner": uuid.NewString(), "viewer": uuid.NewString()}}
}

func (f *commercialBrowserFixture) Verify(_ context.Context, token string) (authidentity.AuthenticatedIdentity, error) {
	if f.signedOut.Load() {
		return authidentity.AuthenticatedIdentity{}, errors.New("synthetic external session ended")
	}
	for subject, expected := range f.tokens {
		if token == expected {
			return authidentity.AuthenticatedIdentity{UserID: subject, HomeOrganizationID: "home-A", TokenExpiresAt: time.Now().Add(time.Hour)}, nil
		}
	}
	return authidentity.AuthenticatedIdentity{}, errors.New("unknown synthetic fixture token")
}

func (f *commercialBrowserFixture) IsOrganizationSuspended(context.Context, string) (bool, error) {
	return f.suspended.Load(), nil
}

func (f *commercialBrowserFixture) control(db *gorm.DB, grants *commercialGrantFixture) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("X-Fixture-Control") != f.tokens["owner"] {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		var request struct{ Scenario string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 128)).Decode(&request) != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		switch request.Scenario {
		case "normal", "revoked", "role-downgraded", "provider-unavailable", "suspended", "signed-out", "database-unavailable", "slow":
		default:
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		statement := "GRANT SELECT ON saas_usage_buckets TO commercial_reader"
		if request.Scenario == "database-unavailable" {
			statement = "REVOKE SELECT ON saas_usage_buckets FROM commercial_reader"
		}
		if db.Exec(statement).Error != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		grants.mode.Store(0)
		if request.Scenario == "revoked" {
			grants.mode.Store(1)
		} else if request.Scenario == "provider-unavailable" {
			grants.mode.Store(2)
		} else if request.Scenario == "role-downgraded" {
			grants.mode.Store(4)
		}
		f.suspended.Store(request.Scenario == "suspended")
		f.signedOut.Store(request.Scenario == "signed-out")
		f.slow.Store(request.Scenario == "slow")
		w.WriteHeader(http.StatusNoContent)
	}
}

func (f *commercialBrowserFixture) serve(t *testing.T, origin string) {
	t.Helper()
	seed, err := json.Marshal(map[string]any{"goOrigin": origin, "contextOrigin": origin, "tokens": f.tokens, "defaultOrganization": "org-B"})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(f.directory, "go.json"), seed, 0o600))
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(30 * time.Minute)
	defer deadline.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatal("commercial browser fixture lifetime ended without stop")
		case <-ticker.C:
			if _, err := os.Stat(filepath.Join(f.directory, "stop")); err == nil {
				return
			} else if !errors.Is(err, os.ErrNotExist) {
				require.NoError(t, err)
			}
		}
	}
}
