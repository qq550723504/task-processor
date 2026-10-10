package customdata

import (
	"context"
	"github.com/stretchr/testify/require"
	"strings"
	"task-processor/internal/dataservice"
	"testing"
)

func TestDeliveryJSONCSVAreStrictBoundedAndUseProductValidation(t *testing.T) {
	rows, err := ParseDelivery(context.Background(), "json", []byte(`[{"title":"fixture","description":"","images":[],"attributes":{"asin":"B000123456"}}]`))
	require.NoError(t, err)
	require.Equal(t, "B000123456", rows[0].Attributes["asin"])
	rows, err = ParseDelivery(context.Background(), "csv", []byte("title,description,brand,images,sku,currency,price,stock\nfixture,,,,,,,,\n"))
	require.ErrorIs(t, err, dataservice.ErrInvalid, "extra column is rejected")
	rows, err = ParseDelivery(context.Background(), "csv", []byte("title,description,brand,images,sku,currency,price,stock\nfixture,,,,SKU,USD,10,0\n"))
	require.NoError(t, err)
	require.Len(t, rows[0].Variants, 1)
	for _, raw := range []string{`[{"title":"one","title":"two","images":[]}]`, `[{"title":"one","organizationId":"foreign","images":[]}]`, `[{"title":"one","variants":[{"price":-1}],"images":[]}]`, strings.Repeat("x", 2<<20+1)} {
		_, err := ParseDelivery(context.Background(), "json", []byte(raw))
		require.ErrorIs(t, err, dataservice.ErrInvalid)
	}
	_, err = ParseDelivery(context.Background(), "csv", []byte("title,description,brand,images,sku,currency,price,stock\n"+strings.Repeat("fixture,,,,,,,\n", 201)))
	require.ErrorIs(t, err, dataservice.ErrInvalid)
}
