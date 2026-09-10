package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/workbenchcontext"
)

type sourceAccountBrowserActor struct {
	subject string
	role    string
}

// The fixture substitutes only external token verification and grant loading.
// The mounted middleware, authorization, Source Account service/repository and
// PostgreSQL transaction path are production code.
type sourceAccountBrowserIdentity struct {
	actors  map[string]sourceAccountBrowserActor
	revoked atomic.Bool
	live    atomic.Int32
	cached  atomic.Int32
}

func (f *sourceAccountBrowserIdentity) Verify(_ context.Context, token string) (authidentity.AuthenticatedIdentity, error) {
	actor, ok := f.actors[token]
	if !ok {
		return authidentity.AuthenticatedIdentity{}, errors.New("unknown fixture token")
	}
	return authidentity.AuthenticatedIdentity{
		UserID:             actor.subject,
		HomeOrganizationID: "home-org",
		TokenExpiresAt:     time.Now().Add(time.Hour),
	}, nil
}

func (f *sourceAccountBrowserIdentity) Load(_ context.Context, source workbenchcontext.GrantSource, request workbenchcontext.GrantRequest) (workbenchcontext.GrantResult, error) {
	if source == workbenchcontext.GrantLive {
		f.live.Add(1)
	} else {
		f.cached.Add(1)
	}
	role := ""
	for _, actor := range f.actors {
		if actor.subject == request.Subject {
			role = actor.role
			break
		}
	}
	grants := []authidentity.OrganizationGrant{}
	if role != "" {
		for _, organizationID := range []string{"org-b", "org-c"} {
			if f.revoked.Load() && source == workbenchcontext.GrantLive && request.Subject == "actor-a" && organizationID == "org-b" {
				continue
			}
			grants = append(grants, authidentity.OrganizationGrant{
				OrganizationID: organizationID,
				ProjectID:      "project",
				Roles:          []string{role},
			})
		}
	}
	return workbenchcontext.GrantResult{Source: source, Grants: grants}, nil
}

func (*sourceAccountBrowserIdentity) Invalidate(string, string) {}

type sourceAccountFixtureApplication struct {
	server   *http.Server
	listener net.Listener
	done     chan error
	origin   string
}

func startSourceAccountFixtureApplication(db *gorm.DB, identity *sourceAccountBrowserIdentity, resolver *workbenchcontext.Resolver, authorizer *authz.ListingKitAuthorizer) (*sourceAccountFixtureApplication, error) {
	server, err := NewSourceAccountApplication(db, identity, resolver, authorizer)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	running := &sourceAccountFixtureApplication{
		server: server, listener: listener, done: make(chan error, 1),
		origin: "http://" + listener.Addr().String(),
	}
	go func() { running.done <- server.Serve(listener) }()
	return running, nil
}

func (a *sourceAccountFixtureApplication) stop() error {
	if a == nil || a.server == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	shutdownErr := a.server.Shutdown(ctx)
	serveErr := <-a.done
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	a.server = nil
	return errors.Join(shutdownErr, serveErr)
}

type sourceAccountFixtureState struct {
	t             *testing.T
	db            *gorm.DB
	identity      *sourceAccountBrowserIdentity
	resolver      *workbenchcontext.Resolver
	authorizer    *authz.ListingKitAuthorizer
	applicationM  sync.RWMutex
	application   *sourceAccountFixtureApplication
	dropNext      atomic.Bool
	businessPosts atomic.Int32
	canceled      atomic.Int32
	lockM         sync.Mutex
	lock          *sql.Tx
}

func (s *sourceAccountFixtureState) currentOrigin() string {
	s.applicationM.RLock()
	defer s.applicationM.RUnlock()
	return s.application.origin
}

func (s *sourceAccountFixtureState) restart() error {
	s.applicationM.Lock()
	defer s.applicationM.Unlock()
	if err := s.application.stop(); err != nil {
		return err
	}
	application, err := startSourceAccountFixtureApplication(s.db, s.identity, s.resolver, s.authorizer)
	if err != nil {
		return err
	}
	s.application = application
	return nil
}

func (s *sourceAccountFixtureState) close() error {
	s.lockM.Lock()
	if s.lock != nil {
		_ = s.lock.Rollback()
		s.lock = nil
	}
	s.lockM.Unlock()
	s.applicationM.Lock()
	defer s.applicationM.Unlock()
	return s.application.stop()
}

func (s *sourceAccountFixtureState) handle(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/fixture/") {
		s.handleControl(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 9*1024))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	upstream, err := http.NewRequestWithContext(r.Context(), r.Method, s.currentOrigin()+r.URL.RequestURI(), bytes.NewReader(body))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	upstream.Header = r.Header.Clone()
	if r.Method == http.MethodPost {
		s.businessPosts.Add(1)
	}
	response, err := http.DefaultClient.Do(upstream)
	if err != nil {
		if r.Context().Err() != nil {
			s.canceled.Add(1)
		}
		return
	}
	defer response.Body.Close()
	if r.Method == http.MethodPost && response.StatusCode >= 200 && response.StatusCode < 300 && s.dropNext.Swap(false) {
		_, _ = io.Copy(io.Discard, response.Body)
		if hijacker, ok := w.(http.Hijacker); ok {
			connection, _, hijackErr := hijacker.Hijack()
			if hijackErr == nil {
				_ = connection.Close()
				return
			}
		}
		return
	}
	for name, values := range response.Header {
		w.Header()[name] = append([]string(nil), values...)
	}
	w.WriteHeader(response.StatusCode)
	_, _ = io.Copy(w, response.Body)
}

func (s *sourceAccountFixtureState) handleControl(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/fixture/observe":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		resources, operations := s.counts()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"resources": resources, "operations": operations,
			"businessPosts": s.businessPosts.Load(), "canceled": s.canceled.Load(),
			"tables": sourceAccountFixtureTables(s.t, s.db),
		})
	case "/fixture/activity":
		_ = json.NewEncoder(w).Encode(map[string]any{"businessPosts": s.businessPosts.Load(), "canceled": s.canceled.Load()})
	case "/fixture/drop-next":
		s.dropNext.Store(true)
		w.WriteHeader(http.StatusNoContent)
	case "/fixture/revoke":
		s.identity.revoked.Store(true)
		w.WriteHeader(http.StatusNoContent)
	case "/fixture/restore":
		s.identity.revoked.Store(false)
		w.WriteHeader(http.StatusNoContent)
	case "/fixture/restart":
		if err := s.restart(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case "/fixture/lock":
		if err := s.acquireOperationLock(); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case "/fixture/release":
		if err := s.releaseOperationLock(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *sourceAccountFixtureState) counts() (int64, int64) {
	var resources, operations int64
	require.NoError(s.t, s.db.Raw(`SELECT count(*) FROM public.source_account_resources`).Scan(&resources).Error)
	require.NoError(s.t, s.db.Raw(`SELECT count(*) FROM public.source_account_operations`).Scan(&operations).Error)
	return resources, operations
}

func (s *sourceAccountFixtureState) acquireOperationLock() error {
	s.lockM.Lock()
	defer s.lockM.Unlock()
	if s.lock != nil {
		return errors.New("operation table is already locked")
	}
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	tx, err := sqlDB.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`LOCK TABLE public.source_account_operations IN ACCESS EXCLUSIVE MODE`); err != nil {
		_ = tx.Rollback()
		return err
	}
	s.lock = tx
	return nil
}

func (s *sourceAccountFixtureState) releaseOperationLock() error {
	s.lockM.Lock()
	defer s.lockM.Unlock()
	if s.lock == nil {
		return errors.New("operation table is not locked")
	}
	err := s.lock.Rollback()
	s.lock = nil
	return err
}

// TestSourceAccountBrowserFixture is launched only by the task-owned acceptance
// script. It refuses shared/non-loopback PostgreSQL and never initializes schema.
func TestSourceAccountBrowserFixture(t *testing.T) {
	dir, dsn := os.Getenv("ISSUE370_FIXTURE_DIR"), os.Getenv("ISSUE370_FIXTURE_DSN")
	if dir == "" && dsn == "" {
		t.Skip("standalone fixture: use web/listingkit-ui/scripts/source-account-final-acceptance.mjs")
	}
	require.True(t, filepath.IsAbs(dir))
	require.True(t, strings.HasPrefix(dsn, "host=127.0.0.1 ") && strings.Contains(dsn, " dbname=issue370_fixture "), "requires task-owned loopback PostgreSQL")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent), NowFunc: func() time.Time { return time.Now().UTC() }})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Ping())
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.Equal(t, []string{"goose_source_account_registry_version", "source_account_operations", "source_account_resources"}, sourceAccountFixtureTables(t, db))

	identity := &sourceAccountBrowserIdentity{actors: map[string]sourceAccountBrowserActor{}}
	tokens := map[string]string{}
	for name, actor := range map[string]sourceAccountBrowserActor{
		"actor-a": {subject: "actor-a", role: "listingkit_operator"},
		"actor-c": {subject: "actor-c", role: "listingkit_operator"},
		"viewer":  {subject: "viewer", role: "listingkit_viewer"},
	} {
		token := uuid.NewString()
		tokens[name], identity.actors[token] = token, actor
	}
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	resolver := workbenchcontext.NewResolver(identity, "project", "v1", nil)
	application, err := startSourceAccountFixtureApplication(db, identity, resolver, authorizer)
	require.NoError(t, err)
	state := &sourceAccountFixtureState{t: t, db: db, identity: identity, resolver: resolver, authorizer: authorizer, application: application}
	t.Cleanup(func() { _ = state.close() })

	public := httptest.NewUnstartedServer(http.HandlerFunc(state.handle))
	public.Listener, err = net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	public.Start()
	t.Cleanup(public.Close)
	data, err := json.Marshal(map[string]any{"goOrigin": public.URL, "tokens": tokens})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.json"), data, 0600))

	deadline := time.NewTimer(29 * time.Minute)
	defer deadline.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline.C:
			t.Fatal("fixture maximum lifetime reached")
		case <-ticker.C:
			if _, err := os.Stat(filepath.Join(dir, "stop-go")); err == nil {
				if _, evidenceErr := os.Stat(filepath.Join(dir, "evidence.json")); evidenceErr == nil {
					resources, operations := state.counts()
					require.EqualValues(t, 4, resources)
					require.EqualValues(t, 7, operations)
				}
				require.NoError(t, state.close())
				return
			}
		}
	}
}

func sourceAccountFixtureTables(t *testing.T, db *gorm.DB) []string {
	t.Helper()
	var tables []string
	require.NoError(t, db.Raw(`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name`).Scan(&tables).Error)
	sort.Strings(tables)
	return tables
}
