//go:build integration

package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMembershipBrowserFixture(t *testing.T) {
	directory := os.Getenv("ISSUE410_BROWSER_FIXTURE_DIR")
	if directory == "" {
		t.Skip("explicit task browser fixture launcher required")
	}
	require.True(t, filepath.IsAbs(directory))
	info, err := os.Stat(directory)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	f := newMembershipFixture(t)
	controlKey := uuid.NewString()
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("X-Fixture-Control") != controlKey {
			w.WriteHeader(403)
			return
		}
		var command struct {
			Mode string `json:"mode"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 256)).Decode(&command) != nil {
			w.WriteHeader(400)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch command.Mode {
		case "normal":
			f.revoked = false
			f.permissionDenied = false
			f.loseUpdate = false
		case "revoked":
			f.revoked = true
		case "permission-denied":
			f.permissionDenied = true
		case "unknown-update":
			f.loseUpdate = true
		default:
			w.WriteHeader(400)
			return
		}
		w.WriteHeader(204)
	}))
	defer control.Close()
	manifest, err := json.Marshal(map[string]any{"goOrigin": f.application.URL, "issuerURL": f.provider.URL, "controlOrigin": control.URL, "controlKey": controlKey, "tokens": []string{"viewer", "operator", "admin"}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(directory, "go.json"), manifest, 0600))
	deadline := time.Now().Add(25 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(directory, "stop-go")); err == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("membership browser fixture timed out without cleanup")
}
