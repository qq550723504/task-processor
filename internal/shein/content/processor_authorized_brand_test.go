package content_test

import (
	"strings"
	"testing"

	"task-processor/internal/model"
	"task-processor/internal/shein/authorizedbrand"
	"task-processor/internal/shein/content"
	sheinctx "task-processor/internal/shein/context"
)

func TestSanitizeDisplayTextWithContext_PreservesAuthorizedAmazonBrand(t *testing.T) {
	t.Parallel()

	service := content.NewSensitiveWordServiceInMemory()
	taskCtx := &sheinctx.TaskContext{
		RuntimeState: sheinctx.RuntimeState{
			AuthorizedBrand: &authorizedbrand.Resolved{
				Enabled: true,
				Name:    "Amazon Basics",
				NameEn:  "Amazon Basics",
			},
		},
	}

	got := service.SanitizeDisplayTextWithContext(taskCtx, "Amazon Basics wireless mouse")
	if !strings.Contains(got, "Amazon Basics") {
		t.Fatalf("sanitized text = %q, want authorized brand preserved", got)
	}
}

func TestSanitizeDisplayTextWithContext_RemovesContextBrandButKeepsAuthorizedBrand(t *testing.T) {
	t.Parallel()

	service := content.NewSensitiveWordServiceInMemory()
	taskCtx := &sheinctx.TaskContext{
		RuntimeState: sheinctx.RuntimeState{
			AuthorizedBrand: &authorizedbrand.Resolved{
				Enabled: true,
				Name:    "Logitech",
				NameEn:  "Logitech",
			},
		},
		ProductState: sheinctx.ProductState{
			AmazonProduct: &model.Product{Brand: "Sony"},
		},
	}

	got := service.SanitizeDisplayTextWithContext(taskCtx, "Logitech mouse by Sony for office")
	if !strings.Contains(got, "Logitech") {
		t.Fatalf("sanitized text = %q, want authorized brand preserved", got)
	}
	if strings.Contains(strings.ToLower(got), "sony") {
		t.Fatalf("sanitized text = %q, want context brand removed", got)
	}
}
