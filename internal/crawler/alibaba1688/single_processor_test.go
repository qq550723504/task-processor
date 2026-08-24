package alibaba1688

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/stretchr/testify/require"
)

func TestSingleProcessorCreatesFreshPublicBrowserManagerPerAttempt(t *testing.T) {
	processor := &SingleProcessor{config: config.NewDefaultConfig()}

	first := processor.newPublicBrowserManager()
	second := processor.newPublicBrowserManager()

	if first == nil || second == nil {
		t.Fatal("public browser manager factory returned nil")
	}
	if first == second {
		t.Fatal("public browser attempts must not share a browser manager")
	}
}

func TestClassifyProductValidationFailurePreservesAccountFallbackForMissingFields(t *testing.T) {
	checker := NewProductChecker()

	tests := []struct {
		name    string
		product *model.Product1688
		want    PublicAccessFailureKind
	}{
		{
			name:    "missing required fields",
			product: &model.Product1688{},
			want:    PublicAccessFailureMissingFields,
		},
		{
			name: "deterministic validation",
			product: &model.Product1688{
				URL:              "https://detail.1688.com/offer/1.html",
				Title:            "假货",
				MinPrice:         18.8,
				MinOrderQuantity: 1,
				Supplier:         model.SupplierInfo{Name: "Supplier"},
				MainImage:        "https://img.example/product.jpg",
			},
			want: PublicAccessFailureValidation,
		},
		{
			name: "negative price",
			product: &model.Product1688{
				URL:              "https://detail.1688.com/offer/1.html",
				Title:            "shirt",
				MinPrice:         -1,
				MinOrderQuantity: 1,
				Supplier:         model.SupplierInfo{Name: "Supplier"},
				MainImage:        "https://img.example/product.jpg",
			},
			want: PublicAccessFailureValidation,
		},
		{
			name: "negative minimum order quantity",
			product: &model.Product1688{
				URL:              "https://detail.1688.com/offer/1.html",
				Title:            "shirt",
				MinPrice:         18.8,
				MinOrderQuantity: -1,
				Supplier:         model.SupplierInfo{Name: "Supplier"},
				MainImage:        "https://img.example/product.jpg",
			},
			want: PublicAccessFailureValidation,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checker.ValidateProduct(tt.product)
			if err == nil {
				t.Fatal("ValidateProduct() error = nil, want validation error")
			}
			if got := classifyProductValidationFailure(err); got != tt.want {
				t.Fatalf("classifyProductValidationFailure() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCloseOnContextDoneClosesOnce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var closes atomic.Int32
	finish := closeOnContextDone(ctx, func() {
		closes.Add(1)
	})

	cancel()

	require.Eventually(t, func() bool {
		return closes.Load() == 1
	}, time.Second, time.Millisecond)

	finish()

	require.Equal(t, int32(1), closes.Load())
}
