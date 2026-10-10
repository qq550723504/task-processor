package grsai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"task-processor/internal/ai"
)

func TestSynchronousEditKeepsEveryExactSourceInOrder(t *testing.T) {
	var submissions, observations int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		submissions++
		var body struct {
			Images []string `json:"images"`
			Prompt string   `json:"prompt"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		want := []string{
			"data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("original one")),
			"data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte("original two")),
			"data:image/webp;base64," + base64.StdEncoding.EncodeToString([]byte("original three")),
		}
		if !reflect.DeepEqual(body.Images, want) || body.Prompt != "approved detail purpose" {
			t.Errorf("source bundle changed: %+v", body)
		}
		_, _ = w.Write([]byte(`{"id":"result-1","status":"succeeded","results":[{"url":"https://example.com/generated.png"}]}`))
	}))
	defer server.Close()
	client := NewClient(Config{Model: "gpt-image-2.5", SubmitURL: server.URL, MaxAttempts: 3, HTTPClient: server.Client()})
	request := &ai.ImageEditRequest{Model: "gpt-image-2.5", Prompt: "approved detail purpose", Image: []byte("original one"), ImageContentType: "image/png", ReferenceImages: []ai.ImageInlineReference{{Bytes: []byte("original two"), MediaType: "image/jpeg"}, {Bytes: []byte("original three"), MediaType: "image/webp"}}, N: 1}
	if _, err := client.EditImageOnce(context.Background(), request, func(context.Context, GenerationObservation) error { observations++; return nil }); err != nil {
		t.Fatal(err)
	}
	if submissions != 1 || observations != 1 {
		t.Fatalf("submissions=%d observations=%d", submissions, observations)
	}
}

func TestSynchronousEditRejectsInvalidSourceBundleBeforePOST(t *testing.T) {
	var submissions int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { submissions++; http.Error(w, "unexpected", 500) }))
	defer server.Close()
	client := NewClient(Config{Model: "gpt-image-2.5", SubmitURL: server.URL, HTTPClient: server.Client()})
	for _, mode := range []string{"empty", "type", "count", "aggregate"} {
		t.Run(mode, func(t *testing.T) {
			request := &ai.ImageEditRequest{Model: "gpt-image-2.5", Prompt: "approved", Image: []byte("primary"), ImageContentType: "image/png", N: 1}
			switch mode {
			case "empty":
				request.ReferenceImages = []ai.ImageInlineReference{{MediaType: "image/png"}}
			case "type":
				request.ReferenceImages = []ai.ImageInlineReference{{Bytes: []byte("source"), MediaType: "text/plain"}}
			case "count":
				for i := 0; i < 8; i++ {
					request.ReferenceImages = append(request.ReferenceImages, ai.ImageInlineReference{Bytes: []byte("source"), MediaType: "image/png"})
				}
			case "aggregate":
				request.ReferenceImages = []ai.ImageInlineReference{{Bytes: make([]byte, 16<<20), MediaType: "image/png"}}
			}
			if _, err := client.EditImageOnce(context.Background(), request, func(context.Context, GenerationObservation) error { return nil }); err == nil {
				t.Fatal("invalid bundle accepted")
			}
		})
	}
	if submissions != 0 {
		t.Fatalf("invalid bundle submitted %d times", submissions)
	}
}

func TestOrdinaryGRSAIEditRejectsExactAdditionalReferences(t *testing.T) {
	var submissions int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { submissions++; http.Error(w, "unexpected", 500) }))
	defer server.Close()
	client := NewClient(Config{Model: "gpt-image-2.5", SubmitURL: server.URL, HTTPClient: server.Client()})
	_, err := client.EditImage(context.Background(), &ai.ImageEditRequest{Model: "gpt-image-2.5", Prompt: "approved", Image: []byte("primary"), ImageContentType: "image/png", ReferenceImages: []ai.ImageInlineReference{{Bytes: []byte("secondary"), MediaType: "image/png"}}})
	if err == nil || submissions != 0 {
		t.Fatalf("unsupported contract dispatched: err=%v calls=%d", err, submissions)
	}
}
