package bootstrap

import (
	"context"
	"net"
	"strconv"
	"testing"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/sirupsen/logrus"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/listingadmin"
)

type stubStoreRepository struct {
	items []listingadmin.Store
}

func (s *stubStoreRepository) ListStores(_ context.Context, query listingadmin.StoreQuery) (*listingadmin.StorePage, error) {
	items := make([]listingadmin.Store, 0, len(s.items))
	for _, item := range s.items {
		if query.TenantID > 0 && item.TenantID != query.TenantID {
			continue
		}
		if query.Platform != "" && item.Platform != query.Platform {
			continue
		}
		items = append(items, item)
	}
	return &listingadmin.StorePage{Items: items, Total: int64(len(items)), Page: 1, PageSize: len(items)}, nil
}

func (s *stubStoreRepository) GetStore(_ context.Context, tenantID, id int64) (*listingadmin.Store, error) {
	for _, item := range s.items {
		if item.TenantID == tenantID && item.ID == id {
			store := item
			return &store, nil
		}
	}
	return nil, listingadmin.ErrStoreNotFound
}

func (s *stubStoreRepository) CreateStore(context.Context, *listingadmin.Store) (*listingadmin.Store, error) {
	panic("unexpected CreateStore")
}

func (s *stubStoreRepository) UpdateStore(context.Context, *listingadmin.Store) (*listingadmin.Store, error) {
	panic("unexpected UpdateStore")
}

func (s *stubStoreRepository) UpdateStoreStatus(context.Context, int64, int64, int16, string) (*listingadmin.Store, error) {
	panic("unexpected UpdateStoreStatus")
}

func (s *stubStoreRepository) DeleteStore(context.Context, int64, int64) error {
	panic("unexpected DeleteStore")
}

func (s *stubStoreRepository) ListDeletedStores(context.Context, int64) ([]listingadmin.Store, error) {
	panic("unexpected ListDeletedStores")
}

func (s *stubStoreRepository) RestoreStore(context.Context, int64, int64) (*listingadmin.Store, error) {
	panic("unexpected RestoreStore")
}

func (s *stubStoreRepository) PermanentlyDeleteStore(context.Context, int64, int64) error {
	panic("unexpected PermanentlyDeleteStore")
}

func (s *stubStoreRepository) ExtendStoreValidity(context.Context, int64, int64, int) (*listingadmin.Store, error) {
	panic("unexpected ExtendStoreValidity")
}

func TestBuildHandlerReturnsNilWithoutLocalStoreRepository(t *testing.T) {
	t.Parallel()

	result, err := BuildHandler(BuildInput{
		Config: &config.Config{
			Platforms: config.PlatformsConfig{
				Shein: config.PlatformConfig{
					CookieRedis: config.RedisConfig{Host: "127.0.0.1"},
				},
			},
		},
		ManagementClient: management.NewClientManager(&config.ManagementConfig{}),
		AccountRepositoryBuilder: func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
			return nil, nil, nil
		},
	})
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if result != nil {
		t.Fatalf("BuildHandler() result = %#v, want nil", result)
	}
}

func TestBuildHandlerReturnsHandlerAndClose(t *testing.T) {
	t.Parallel()

	redisServer := miniredis.RunT(t)
	host, portText, err := net.SplitHostPort(redisServer.Addr())
	if err != nil {
		t.Fatalf("SplitHostPort() error = %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("Atoi() error = %v", err)
	}

	closed := false
	result, err := BuildHandler(BuildInput{
		Config: &config.Config{
			Browser: config.BrowserConfig{ViewportWidth: 1280, ViewportHeight: 720},
			Platforms: config.PlatformsConfig{
				Shein: config.PlatformConfig{
					CookieRedis: config.RedisConfig{Host: host, Port: port},
				},
			},
		},
		ManagementClient: management.NewClientManager(&config.ManagementConfig{}),
		AccountRepositoryBuilder: func(*config.Config, *logrus.Logger) (listingadmin.StoreRepository, []func() error, error) {
			return &stubStoreRepository{
					items: []listingadmin.Store{{
						ID:       12,
						TenantID: 7,
						Platform: "SHEIN",
						Username: "demo-user",
						Password: "secret",
						LoginURL: "sellerhub.shein.com",
					}},
				}, []func() error{func() error {
					closed = true
					return nil
				}}, nil
		},
	})
	if err != nil {
		t.Fatalf("BuildHandler() error = %v", err)
	}
	if result == nil || result.Handler == nil || result.Service == nil {
		t.Fatalf("BuildHandler() returned incomplete result: %#v", result)
	}
	if result.Close == nil {
		t.Fatal("BuildHandler() close func is nil")
	}
	if err := result.Close(); err != nil {
		t.Fatalf("result.Close() error = %v", err)
	}
	if !closed {
		t.Fatal("expected repository closer to be invoked")
	}
}
