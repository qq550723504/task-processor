package currentapplication

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	coreconfig "task-processor/internal/core/config"
)

func TestRunClosesSourcePoolWhenCommercialOpenFails(t *testing.T) {
	source := &gorm.DB{}
	want := errors.New("commercial unavailable")
	closed := []*gorm.DB{}
	dependencies := runtimeDependencies{
		OpenSourceAccount: func(DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenCommercial:    func(DatabaseConfig) (*gorm.DB, error) { return nil, want },
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

func TestRunClosesBothPoolsInReverseOrderWhenListenFails(t *testing.T) {
	source, commercial := &gorm.DB{}, &gorm.DB{}
	want := errors.New("address already in use")
	closed := []*gorm.DB{}
	dependencies := runtimeDependencies{
		OpenSourceAccount: func(DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenCommercial:    func(DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
		NewApplication: func(gotSource, gotCommercial *gorm.DB, _ *coreconfig.Config, _ *logrus.Logger) (*http.Server, error) {
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
		OpenSourceAccount: func(DatabaseConfig) (*gorm.DB, error) { return source, nil },
		OpenCommercial:    func(DatabaseConfig) (*gorm.DB, error) { return commercial, nil },
		NewApplication: func(*gorm.DB, *gorm.DB, *coreconfig.Config, *logrus.Logger) (*http.Server, error) {
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

func runtimeTestConfig() *Config {
	return &Config{
		SchemaVersion: manifestSchemaVersion,
		Listen:        ListenConfig{Host: "127.0.0.1", Port: 18443},
		Identity: IdentityConfig{
			IssuerURL: "http://localhost:18080", AuthorizationAPIURL: "http://localhost:18080",
			ClientID: "client", ClientSecret: "secret", ProjectID: "project",
		},
		SourceAccountDatabase: DatabaseConfig{Host: "127.0.0.1", Port: 15432, User: "source", Password: "secret", Database: "task_processor", MaxConnections: 2},
		CommercialDatabase:    DatabaseConfig{Host: "127.0.0.1", Port: 15432, User: "commercial", Password: "secret", Database: "task_processor", MaxConnections: 2},
	}
}
