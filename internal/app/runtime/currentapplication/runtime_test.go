package currentapplication

import (
	"context"
	"errors"
	"net"
	"net/http"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	coreconfig "task-processor/internal/core/config"
)

func TestRunReferralPoolFailureAndDisabledLifecycle(t *testing.T) {
	for _, stage := range []string{"disabled", "open", "construct", "listen", "cancel"} {
		t.Run(stage, func(t *testing.T) {
			cfg := referralRuntimeConfig(t)
			cfg.Referrals.Enabled = stage != "disabled"
			source, commercial, referrals := &gorm.DB{}, &gorm.DB{}, &gorm.DB{}
			var closed []*gorm.DB
			opened := 0
			stop := errors.New("fixture stop")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			dependencies := Dependencies{
				IdentityPreflight: func(context.Context, IdentityConfig) error { return nil },
				OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
				OpenCommercial:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
				OpenReferrals: func(context.Context, DatabaseConfig) (*gorm.DB, error) {
					opened++
					if stage == "open" {
						return nil, stop
					}
					if stage == "cancel" {
						cancel()
					}
					return referrals, nil
				},
				NewApplication: func(context.Context, *gorm.DB, *gorm.DB, *coreconfig.Config, *logrus.Logger) (*http.Server, error) {
					return &http.Server{}, nil
				},
				NewReferralsApplication: func(_ context.Context, s, c, r *gorm.DB, core *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
					if s != source || c != commercial || r != referrals || core.Referrals.Prepared == nil {
						t.Fatal("referral composition lost independent dependencies")
					}
					if stage == "construct" {
						return nil, stop
					}
					return &http.Server{}, nil
				},
				Listen:        func(string, string) (net.Listener, error) { return nil, stop },
				CloseDatabase: func(db *gorm.DB) error { closed = append(closed, db); return nil },
			}
			if err := Run(ctx, cfg, logrus.New(), dependencies); err == nil {
				t.Fatal("expected bounded stop")
			}
			want := []*gorm.DB{commercial, source}
			if stage != "disabled" && stage != "open" {
				want = append([]*gorm.DB{referrals}, want...)
			}
			if !reflect.DeepEqual(closed, want) {
				t.Fatal("incorrect pool release order")
			}
			if stage == "disabled" && opened != 0 {
				t.Fatal("disabled referrals opened a pool")
			}
		})
	}
}

func TestRunClosesSourcePoolWhenCommercialOpenFails(t *testing.T) {
	source := &gorm.DB{}
	want := errors.New("commercial unavailable")
	closed := []*gorm.DB{}
	dependencies := runtimeDependencies{
		IdentityPreflight: func(context.Context, IdentityConfig) error { return nil },
		OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenCommercial:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return nil, want },
		CloseDatabase: func(db *gorm.DB) error {
			closed = append(closed, db)
			return nil
		},
	}

	err := run(context.Background(), runtimeTestConfig(), logrus.New(), dependencies)
	if !errors.Is(err, want) {
		t.Fatalf("run() error = %v, want wrapped %v", err, want)
	}
	if len(closed) != 1 || closed[0] != source {
		t.Fatalf("closed databases = %#v", closed)
	}
}

func TestRunRejectsUnavailableIdentityBeforeOpeningDatabase(t *testing.T) {
	want := errors.New("provider unavailable")
	opened := false
	err := run(context.Background(), runtimeTestConfig(), logrus.New(), runtimeDependencies{
		IdentityPreflight: func(context.Context, IdentityConfig) error { return want },
		OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { opened = true; return &gorm.DB{}, nil },
		OpenCommercial:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { opened = true; return &gorm.DB{}, nil },
		CloseDatabase:     func(*gorm.DB) error { return nil },
	})
	if !errors.Is(err, want) || opened {
		t.Fatalf("run() = %v, database opened=%t", err, opened)
	}
}

func TestRunClosesBothPoolsInReverseOrderWhenListenFails(t *testing.T) {
	source, commercial := &gorm.DB{}, &gorm.DB{}
	want := errors.New("address already in use")
	closed := []*gorm.DB{}
	dependencies := runtimeDependencies{
		IdentityPreflight: func(context.Context, IdentityConfig) error { return nil },
		OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenCommercial:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
		NewApplication: func(_ context.Context, gotSource, gotCommercial *gorm.DB, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
			if gotSource != source || gotCommercial != commercial {
				t.Fatal("application received wrong database pools")
			}
			return &http.Server{}, nil
		},
		Listen: func(network, address string) (net.Listener, error) {
			if network != "tcp" || address != "127.0.0.1:18443" {
				t.Fatalf("listen = %s %s", network, address)
			}
			return nil, want
		},
		CloseDatabase: func(db *gorm.DB) error {
			closed = append(closed, db)
			return nil
		},
	}

	err := run(context.Background(), runtimeTestConfig(), logrus.New(), dependencies)
	if !errors.Is(err, want) {
		t.Fatalf("run() error = %v, want wrapped %v", err, want)
	}
	if len(closed) != 2 || closed[0] != commercial || closed[1] != source {
		t.Fatalf("close order = %#v", closed)
	}
}

func TestRunShutsDownApplicationAndPreservesPoolsUntilServeStops(t *testing.T) {
	source, commercial := &gorm.DB{}, &gorm.DB{}
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	closed := []*gorm.DB{}
	listening := make(chan struct{})
	dependencies := runtimeDependencies{
		IdentityPreflight: func(context.Context, IdentityConfig) error { return nil },
		OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenCommercial:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
		NewApplication: func(context.Context, *gorm.DB, *gorm.DB, *coreconfig.Config, *logrus.Logger) (*http.Server, error) {
			return &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) })}, nil
		},
		Listen: func(string, string) (net.Listener, error) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err == nil {
				close(listening)
			}
			return listener, err
		},
		CloseDatabase: func(db *gorm.DB) error {
			mu.Lock()
			defer mu.Unlock()
			closed = append(closed, db)
			return nil
		},
	}

	done := make(chan error, 1)
	go func() { done <- run(ctx, runtimeTestConfig(), logrus.New(), dependencies) }()
	<-listening
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run() error = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(closed) != 2 || closed[0] != commercial || closed[1] != source {
		t.Fatalf("close order = %#v", closed)
	}
}

func TestRunForcesCloseAfterBoundedShutdownExpires(t *testing.T) {
	source, commercial := &gorm.DB{}, &gorm.DB{}
	ctx, cancel := context.WithCancel(context.Background())
	listening := make(chan string, 1)
	requestStarted := make(chan struct{})
	closed := 0
	dependencies := runtimeDependencies{
		IdentityPreflight: func(context.Context, IdentityConfig) error { return nil },
		OpenSourceAccount: func(context.Context, DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenCommercial:    func(context.Context, DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
		NewApplication: func(context.Context, *gorm.DB, *gorm.DB, *coreconfig.Config, *logrus.Logger) (*http.Server, error) {
			return &http.Server{Handler: http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
				close(requestStarted)
				<-request.Context().Done()
			})}, nil
		},
		Listen: func(string, string) (net.Listener, error) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err == nil {
				listening <- listener.Addr().String()
			}
			return listener, err
		},
		CloseDatabase:   func(*gorm.DB) error { closed++; return nil },
		ShutdownTimeout: 20 * time.Millisecond,
	}
	done := make(chan error, 1)
	go func() { done <- run(ctx, runtimeTestConfig(), logrus.New(), dependencies) }()
	go func() { _, _ = http.Get("http://" + <-listening) }()
	<-requestStarted
	cancel()
	err := <-done
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("run() error = %v, want shutdown deadline exceeded", err)
	}
	if closed != 2 {
		t.Fatalf("closed databases = %d, want 2", closed)
	}
}

func TestRunPassesOneBoundedStartupContextToBothOpeners(t *testing.T) {
	want := errors.New("context-aware source opener")
	err := run(context.Background(), runtimeTestConfig(), logrus.New(), runtimeDependencies{
		IdentityPreflight: func(ctx context.Context, _ IdentityConfig) error {
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("identity preflight did not receive a deadline")
			}
			return nil
		},
		OpenSourceAccount: func(ctx context.Context, _ DatabaseConfig) (*gorm.DB, error) {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) > 15*time.Second {
				t.Fatalf("source opener context deadline = %v, ok=%t", deadline, ok)
			}
			return nil, want
		},
		OpenCommercial: func(context.Context, DatabaseConfig) (*gorm.DB, error) {
			t.Fatal("commercial opener ran after source failure")
			return nil, nil
		},
		CloseDatabase: func(*gorm.DB) error { return nil },
	})
	if !errors.Is(err, want) {
		t.Fatalf("run() error = %v, want wrapped %v", err, want)
	}
}

func runtimeTestConfig() *Config {
	return &Config{
		SchemaVersion: manifestSchemaVersion,
		Listen:        ListenConfig{Host: "127.0.0.1", Port: 18443},
		Identity: IdentityConfig{
			IssuerURL: "http://localhost:18080", AuthorizationAPIURL: "http://localhost:18080",
			ClientID: "client", ClientSecret: "secret", ProjectID: "project",
		},
		SourceAccountDatabase: DatabaseConfig{Host: "127.0.0.1", Port: 15432, User: "source_account_runtime", Password: "secret", Database: "task_processor", MaxConnections: 2},
		CommercialDatabase:    DatabaseConfig{Host: "127.0.0.1", Port: 15432, User: "commercial_reader", Password: "secret", Database: "task_processor", MaxConnections: 2},
	}
}
