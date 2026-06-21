package shein

import (
	"context"
	"errors"
	"strings"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestRetrySensitiveWordSubmitCleansProductAppendsEventAndRetries(t *testing.T) {
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy Door Curtain"}},
	}
	pkg := &Package{}
	originalErr := errors.New("validation failed")
	var called int

	retryResponse, retryErr, retried := RetrySensitiveWordSubmit(
		context.Background(),
		"task-1",
		pkg,
		"publish",
		"req-1",
		nil,
		product,
		&SubmissionResponse{ValidationNotes: []string{"敏感词：whimsy"}},
		originalErr,
		func(_ sheinproduct.ProductAPI, action string, gotProduct *sheinproduct.Product) (*SubmissionResponse, error) {
			called++
			if action != "publish" {
				t.Fatalf("action = %q, want publish", action)
			}
			if gotProduct != product {
				t.Fatal("executor did not receive original product pointer")
			}
			return &SubmissionResponse{Success: false, Message: "still pending review"}, nil
		},
	)

	if !retried {
		t.Fatal("retried = false, want true")
	}
	if called != 1 {
		t.Fatalf("executor calls = %d, want 1", called)
	}
	if retryResponse == nil || retryResponse.Message != "still pending review" {
		t.Fatalf("retry response = %+v, want executor response", retryResponse)
	}
	if retryErr == nil || !strings.Contains(retryErr.Error(), "still pending review") {
		t.Fatalf("retry error = %v, want response-derived publish error", retryErr)
	}
	if strings.Contains(strings.ToLower(findLocalizedText(product.MultiLanguageNameList, "en")), "whimsy") {
		t.Fatalf("title still contains sensitive word: %+v", product.MultiLanguageNameList)
	}
	if len(pkg.SubmissionEvents) != 1 {
		t.Fatalf("events = %+v, want one retry event", pkg.SubmissionEvents)
	}
	event := pkg.SubmissionEvents[0]
	if event.TaskID != "task-1" || event.RequestID != "req-1" || event.Phase != SubmissionPhaseSubmitRemote {
		t.Fatalf("event = %+v, want retry submit_remote event", event)
	}
	if event.Detail != sensitiveWordRetryDetail {
		t.Fatalf("event detail = %q, want retry detail", event.Detail)
	}
}

func TestRetrySensitiveWordSubmitSkipsNonPublishOrMissingExecutor(t *testing.T) {
	product := &sheinproduct.Product{
		MultiLanguageNameList: []sheinproduct.LanguageContent{{Language: "en", Name: "Whimsy Door Curtain"}},
	}
	response := &SubmissionResponse{ValidationNotes: []string{"敏感词：whimsy"}}
	originalErr := errors.New("validation failed")

	gotResponse, gotErr, retried := RetrySensitiveWordSubmit(context.Background(), "task-1", &Package{}, "save_draft", "req-1", nil, product, response, originalErr, nil)

	if retried {
		t.Fatal("retried = true, want false")
	}
	if gotResponse != response || gotErr != originalErr {
		t.Fatalf("result = (%+v, %v), want original response/error", gotResponse, gotErr)
	}
	if !strings.Contains(strings.ToLower(findLocalizedText(product.MultiLanguageNameList, "en")), "whimsy") {
		t.Fatalf("title was cleaned even though retry was skipped: %+v", product.MultiLanguageNameList)
	}
}
