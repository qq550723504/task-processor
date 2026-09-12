package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Standalone task-owned fixture. Only the external verifier/grant provider are
// controlled; Next/Auth.js, BFF, route middleware, service and PostgreSQL are real.
func TestBrowserCaptureWebFixture(t *testing.T) {
	dir := os.Getenv("ISSUE399_FIXTURE_DIR")
	if dir == "" {
		t.Skip("NOT_RUN: standalone Browser Web fixture requires its task launcher")
	}
	require.True(t, filepath.IsAbs(dir))
	require.NotEmpty(t, os.Getenv("ISSUE398_TEST_DSN"))
	f := newAcquisitionHTTPFixture(t)
	var mu sync.RWMutex
	handler := f.browserServer(t).Config.Handler
	var drop atomic.Bool
	var posts atomic.Int32
	controlKey := uuid.NewString()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.URL.Path) >= 12 && r.URL.Path[:12] == "/__issue399/" {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Fixture-Control")), []byte(controlKey)) != 1 {
				w.WriteHeader(403)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			switch r.URL.Path {
			case "/__issue399/observe":
				counts := map[string]int64{}
				for _, table := range []string{"product_acquisition_operations", "product_snapshot_versions", "product_source_publications", "product_source_publication_receipts"} {
					var count int64
					if err := f.owner.Table(table).Count(&count).Error; err != nil {
						w.WriteHeader(500)
						return
					}
					counts[table] = count
				}
				var digest string
				if err := f.owner.Raw("SELECT md5(COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text)::text,'[]')) FROM product_acquisition_operations t").Scan(&digest).Error; err != nil {
					w.WriteHeader(500)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"counts": counts, "stagingDigest": digest, "capturePosts": posts.Load(), "publicFetches": f.fetches.Load(), "cachedGrants": f.grants.cached.Load()})
			case "/__issue399/drop-next":
				drop.Store(true)
				w.WriteHeader(204)
			case "/__issue399/revoke":
				f.grants.revoked.Store(true)
				w.WriteHeader(204)
			case "/__issue399/restore":
				f.grants.revoked.Store(false)
				w.WriteHeader(204)
			case "/__issue399/restart":
				mu.Lock()
				handler = f.browserServer(t).Config.Handler
				mu.Unlock()
				w.WriteHeader(204)
			case "/__issue399/read-only":
				pool, err := f.db.DB()
				if err != nil {
					w.WriteHeader(500)
					return
				}
				pool.SetMaxOpenConns(1)
				if err = f.db.Exec("SET default_transaction_read_only=on").Error; err != nil {
					w.WriteHeader(500)
					return
				}
				w.WriteHeader(204)
			default:
				w.WriteHeader(404)
			}
			return
		}
		mu.RLock()
		current := handler
		mu.RUnlock()
		if r.Method == http.MethodPost && r.URL.Path == browserCaptureBase {
			posts.Add(1)
			if drop.Swap(false) {
				recorder := httptest.NewRecorder()
				current.ServeHTTP(recorder, r)
				if recorder.Code == 200 {
					conn, _, err := w.(http.Hijacker).Hijack()
					if err == nil {
						_ = conn.Close()
					}
					return
				}
				for key, values := range recorder.Header() {
					w.Header()[key] = values
				}
				w.WriteHeader(recorder.Code)
				_, _ = w.Write(recorder.Body.Bytes())
				return
			}
		}
		current.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	data, err := json.Marshal(map[string]any{"goOrigin": server.URL, "controlKey": controlKey})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.json"), data, 0600))
	deadline := time.NewTimer(12 * time.Minute)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatal("Browser fixture maximum lifetime reached")
		case <-ticker.C:
			if _, err := os.Stat(filepath.Join(dir, "stop-go")); err == nil {
				return
			}
		}
	}
}
