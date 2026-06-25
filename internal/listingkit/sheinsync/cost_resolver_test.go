package sheinsync

import (
	"context"
	"errors"
	"testing"
	"time"

	sheinproduct "task-processor/internal/shein/api/product"

	"github.com/stretchr/testify/require"
)

func TestSheinCostResolverRetriesForbiddenCostPriceWithBackoff(t *testing.T) {
	t.Parallel()

	productAPI := &retryingCostPriceProductAPI{
		failuresBeforeSuccess: 2,
		response: makeCostPriceQueryResponse([]sheinproduct.SkcCostData{
			{
				SkcName: "skc-1",
				SkuCostInfoList: []sheinproduct.SkuCostInfo{
					{CostPriceInfo: sheinproduct.CostPrice{CostPrice: "12.30", Currency: "USD"}},
				},
			},
		}),
	}
	var slept []time.Duration
	resolver := &sheinProductCostResolver{
		productAPI: productAPI,
		retryDelays: []time.Duration{
			30 * time.Second,
			time.Minute,
		},
		sleep: func(_ context.Context, delay time.Duration) error {
			slept = append(slept, delay)
			return nil
		},
	}

	resolved, err := resolver.ResolveAutoCosts(context.Background(), sheinproduct.ProductListItem{
		SpuName: "spu-1",
		SkcInfoList: []sheinproduct.SkcInfoItem{
			{SkcName: "skc-1"},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 3, productAPI.calls)
	require.Equal(t, []time.Duration{30 * time.Second, time.Minute}, slept)
	require.NotNil(t, resolved["skc-1"].CostPrice)
	require.Equal(t, 12.30, *resolved["skc-1"].CostPrice)
	require.Equal(t, "USD", resolved["skc-1"].Currency)
}

func TestSheinCostResolverReturnsEmptyCostsWhenForbiddenRetriesExhausted(t *testing.T) {
	t.Parallel()

	productAPI := &retryingCostPriceProductAPI{failuresBeforeSuccess: 99}
	resolver := &sheinProductCostResolver{
		productAPI:  productAPI,
		retryDelays: []time.Duration{0, 0},
		sleep:       func(context.Context, time.Duration) error { return nil },
	}

	resolved, err := resolver.ResolveAutoCosts(context.Background(), sheinproduct.ProductListItem{
		SpuName: "spu-1",
		SkcInfoList: []sheinproduct.SkcInfoItem{
			{SkcName: "skc-1"},
		},
	})
	require.NoError(t, err)
	require.Empty(t, resolved)
	require.Equal(t, 3, productAPI.calls)
}

type retryingCostPriceProductAPI struct {
	sheinSyncServiceProductAPIStub
	calls                 int
	failuresBeforeSuccess int
	response              *sheinproduct.CostPriceQueryResponse
}

func (s *retryingCostPriceProductAPI) QueryCostPrice(string, []string) (*sheinproduct.CostPriceQueryResponse, error) {
	s.calls++
	if s.calls <= s.failuresBeforeSuccess {
		return nil, errors.New("API错误 [403]: Access Denied")
	}
	if s.response == nil {
		return nil, errors.New("missing cost price response")
	}
	return s.response, nil
}

func makeCostPriceQueryResponse(items []sheinproduct.SkcCostData) *sheinproduct.CostPriceQueryResponse {
	resp := &sheinproduct.CostPriceQueryResponse{Code: "0", Msg: "ok"}
	resp.Info.Data = append(resp.Info.Data, items...)
	resp.Info.Meta.Count = len(items)
	return resp
}
