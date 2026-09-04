package shein

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/imroc/req/v3"

	"task-processor/internal/product/catalog/canonical"
)

type failClosedRuntimeClientFactory struct {
	client RuntimeAPIClient
	calls  int
}

func (f *failClosedRuntimeClientFactory) NewAPIClient(context.Context, int64) RuntimeAPIClient {
	f.calls++
	return f.client
}

type failClosedRuntimeClient struct {
	hasCookies bool
	refreshErr error
	refreshes  int
}

func (c *failClosedRuntimeClient) HasCookies() bool { return c.hasCookies }
func (c *failClosedRuntimeClient) ForceRefreshCookies() error {
	c.refreshes++
	return c.refreshErr
}
func (*failClosedRuntimeClient) GetBaseURL() string         { return "https://example.test" }
func (*failClosedRuntimeClient) GetTenantID() int64         { return 17 }
func (*failClosedRuntimeClient) GetHTTPClient() *req.Client { return req.C() }

type runtimeResolutionProbe struct {
	name    string
	resolve func(RuntimeAPIClientFactory, *BuildRequest) any
}

func runtimeResolutionProbes() []runtimeResolutionProbe {
	return []runtimeResolutionProbe{
		{
			name: "category",
			resolve: func(factory RuntimeAPIClientFactory, request *BuildRequest) any {
				return NewRuntimeCategoryResolver(factory, CategoryAIConfig{}).Resolve(request, &canonical.Product{Title: "Remote product"}, &Package{})
			},
		},
		{
			name: "attribute",
			resolve: func(factory RuntimeAPIClientFactory, request *BuildRequest) any {
				return NewRuntimeAttributeResolver(factory, nil).Resolve(request, &canonical.Product{Title: "Remote product"}, &Package{})
			},
		},
		{
			name: "sale attribute",
			resolve: func(factory RuntimeAPIClientFactory, request *BuildRequest) any {
				return NewRuntimeSaleAttributeResolver(factory, nil).Resolve(request, &canonical.Product{Title: "Remote product"}, &Package{})
			},
		},
	}
}

func TestRuntimeResolversReturnNoResolutionWhenRequestCapabilityIsUnavailable(t *testing.T) {
	t.Parallel()

	for _, probe := range runtimeResolutionProbes() {
		probe := probe
		t.Run(probe.name, func(t *testing.T) {
			t.Parallel()

			tests := []struct {
				name        string
				request     *BuildRequest
				factory     RuntimeAPIClientFactory
				wantCalls   int
				wantRefresh int
			}{
				{name: "nil request", request: nil, factory: &failClosedRuntimeClientFactory{}, wantCalls: 0},
				{name: "missing store ID", request: &BuildRequest{}, factory: &failClosedRuntimeClientFactory{}, wantCalls: 0},
				{name: "nil factory", request: &BuildRequest{SheinStoreID: 869}, factory: nil, wantCalls: 0},
				{name: "nil client", request: &BuildRequest{SheinStoreID: 869}, factory: &failClosedRuntimeClientFactory{}, wantCalls: 1},
				{name: "cookie refresh error", request: &BuildRequest{SheinStoreID: 869}, factory: &failClosedRuntimeClientFactory{client: &failClosedRuntimeClient{refreshErr: errors.New("cookie unavailable")}}, wantCalls: 1, wantRefresh: 1},
				{name: "cookie still absent after refresh", request: &BuildRequest{SheinStoreID: 869}, factory: &failClosedRuntimeClientFactory{client: &failClosedRuntimeClient{}}, wantCalls: 1, wantRefresh: 1},
			}

			for _, tt := range tests {
				tt := tt
				t.Run(tt.name, func(t *testing.T) {
					resolution, panicValue := resolveRuntimeCapabilityProbe(probe, tt.factory, tt.request)
					if panicValue != nil {
						t.Fatalf("Resolve() panicked with unavailable capability: %v", panicValue)
					}
					if runtimeResolutionAvailable(resolution) {
						t.Fatalf("Resolve() = %#v, want nil instead of fallback/partial", resolution)
					}
					if factory, ok := tt.factory.(*failClosedRuntimeClientFactory); ok {
						if factory.calls != tt.wantCalls {
							t.Fatalf("factory calls = %d, want %d", factory.calls, tt.wantCalls)
						}
						if client, ok := factory.client.(*failClosedRuntimeClient); ok && client.refreshes != tt.wantRefresh {
							t.Fatalf("cookie refresh calls = %d, want %d", client.refreshes, tt.wantRefresh)
						}
					}
				})
			}
		})
	}
}

func runtimeResolutionAvailable(resolution any) bool {
	if resolution == nil {
		return false
	}
	value := reflect.ValueOf(resolution)
	return value.Kind() != reflect.Pointer || !value.IsNil()
}

func resolveRuntimeCapabilityProbe(probe runtimeResolutionProbe, factory RuntimeAPIClientFactory, request *BuildRequest) (resolution any, panicValue any) {
	defer func() {
		panicValue = recover()
	}()
	return probe.resolve(factory, request), nil
}
