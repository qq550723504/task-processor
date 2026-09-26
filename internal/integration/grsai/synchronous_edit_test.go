package grsai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"task-processor/internal/ai"
)

func TestSynchronousProductEditUsesExactGRSAIWireBeforeDownloads(t *testing.T) {
	for _, mode := range []string{"success", "observer_failure", "running", "partial_response", "alternate_schema", "oversized_response", "307", "308"} {
		t.Run(mode, func(t *testing.T) {
			var submits, queries, downloads, observations atomic.Int32
			var observed GenerationObservation
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/image" {
					downloads.Add(1)
					if observations.Load() != 1 {
						t.Error("output fetched before durable observer")
					}
					_, _ = w.Write([]byte("controlled image"))
					return
				}
				if r.Method == http.MethodGet {
					queries.Add(1)
					http.Error(w, "no query", 500)
					return
				}
				submits.Add(1)
				if r.URL.Path != "/v1/api/generate" {
					t.Errorf("path=%s", r.URL.Path)
				}
				var body map[string]json.RawMessage
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				for key, want := range map[string]string{"model": "gpt-image-2.5", "aspectRatio": "1024x1024", "quality": "auto", "replyType": "json", "prompt": "white background"} {
					var got string
					_ = json.Unmarshal(body[key], &got)
					if got != want {
						t.Errorf("%s=%q want %q", key, got, want)
					}
				}
				if len(body) != 6 {
					t.Errorf("unexpected wire fields: %v", body)
				}
				var images []string
				_ = json.Unmarshal(body["images"], &images)
				wantImage := "data:image/png;base64," + base64.StdEncoding.EncodeToString([]byte("exact source bytes"))
				if len(images) != 1 || images[0] != wantImage {
					t.Errorf("not exact bytes: %v", images)
				}
				switch mode {
				case "alternate_schema":
					_, _ = w.Write([]byte(`{"data":[{"url":"https://example.com/image.png"}]}`))
					return
				case "oversized_response":
					_, _ = w.Write([]byte(`{"id":"result-1","status":"succeeded","extra":"` + strings.Repeat("x", 1<<20) + `"}`))
					return
				case "running":
					_, _ = w.Write([]byte(`{"id":"job-1","status":"running"}`))
					return
				case "partial_response":
					w.Header().Set("Content-Length", "100")
					_, _ = w.Write([]byte("{"))
					return
				case "307", "308":
					status := 307
					if mode == "308" {
						status = 308
					}
					http.Redirect(w, r, "/redirect", status)
					return
				}
				w.Header().Set("X-Request-Id", "request-1")
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "result-1", "status": "succeeded", "results": []map[string]string{{"url": server.URL + "/image"}}})
			}))
			defer server.Close()
			client := NewClient(Config{Model: "gpt-image-2.5", SubmitURL: server.URL, MaxAttempts: 3, HTTPClient: server.Client()})
			var pinned ai.RouteBoundImageGenerator
			_, err := NewSynchronousProductImageAdapter(ProductImageAdapterConfig{
				Client: client, ImageModel: "gpt-image-2.5", RouteReference: "route-1", CredentialReference: "credential-1", ConfigurationVersion: "config-1",
				Build: func(bound ai.RouteBoundImageGenerator) (ProductImageProvider, error) {
					pinned = bound
					return &grsaiProductImageProviderStub{}, nil
				},
			}, func(_ context.Context, o GenerationObservation) error {
				observations.Add(1)
				observed = o
				if mode == "observer_failure" {
					return errors.New("DB unavailable")
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			response, err := pinned.EditImageWithRoute(context.Background(), &ai.ImageEditRequest{Model: "gpt-image-2.5", Prompt: "white background", Image: []byte("exact source bytes"), ImageContentType: "image/png", N: 1}, ai.ImageRouteSelection{CredentialReference: "credential-1", ConfigurationVersion: "config-1"})
			if (err == nil) != (mode == "success") {
				t.Fatalf("response=%v error=%v mode=%s", response, err, mode)
			}
			if submits.Load() != 1 || queries.Load() != 0 {
				t.Fatalf("submits=%d queries=%d", submits.Load(), queries.Load())
			}
			wantObserved := int32(0)
			if mode == "success" || mode == "observer_failure" {
				wantObserved = 1
			}
			if observations.Load() != wantObserved {
				t.Fatalf("observations=%d want %d", observations.Load(), wantObserved)
			}
			if wantObserved == 1 && (observed.ResponseID != "result-1" || observed.RequestID != "request-1" || len(observed.ResultDigest) != 64 || observed.UsageKnown || observed.Usage != (ai.Usage{})) {
				t.Fatalf("invalid effect fact: %+v", observed)
			}
			if downloads.Load() != 0 {
				t.Fatal("output must be fetched only by the existing safe product-image consumer")
			}
			if wantObserved == 1 {
				encoded, _ := json.Marshal(observed)
				var fact map[string]any
				_ = json.Unmarshal(encoded, &fact)
				if fact["ResultURL"] != server.URL+"/image" {
					t.Fatal("durable observer lost original result locator before download")
				}
			}
			if mode == "success" && (response.UsageKnown || len(response.Data) != 1 || response.Data[0].URL != server.URL+"/image") {
				t.Fatalf("unexpected response: %+v", response)
			}
		})
	}
}

func TestSynchronousAdapterMissingObserverCannotFallbackToOrdinaryEdit(t *testing.T) {
	var builds int
	client := NewClient(Config{Model: "gpt-image-2.5", SubmitURL: "https://example.com"})
	_, err := NewSynchronousProductImageAdapter(ProductImageAdapterConfig{Client: client, ImageModel: "gpt-image-2.5", RouteReference: "route", CredentialReference: "credential", ConfigurationVersion: "version", Build: func(ai.RouteBoundImageGenerator) (ProductImageProvider, error) {
		builds++
		return &grsaiProductImageProviderStub{}, nil
	}}, nil)
	if err == nil || builds != 0 {
		t.Fatalf("missing durable observer must fail closed: error=%v builds=%d", err, builds)
	}
}

func TestSynchronousProductEditRejectsUnsafeInputBeforeDispatch(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); http.Error(w, "unexpected", 500) }))
	defer server.Close()
	client := NewClient(Config{Model: "gpt-image-2.5", SubmitURL: server.URL, HTTPClient: server.Client()})
	for _, mode := range []string{"no_observer", "url", "no_bytes", "model", "mask", "n", "size"} {
		t.Run(mode, func(t *testing.T) {
			req := &ai.ImageEditRequest{Model: "gpt-image-2.5", Prompt: "white", Image: []byte("source"), ImageContentType: "image/png", N: 1}
			observer := GenerationObserver(func(context.Context, GenerationObservation) error { return nil })
			switch mode {
			case "no_observer":
				observer = nil
			case "url":
				req.ImageURL = "https://example.com/source.png"
			case "no_bytes":
				req.Image = nil
			case "model":
				req.Model = "nano-banana-fast"
			case "mask":
				req.Mask = []byte("mask")
			case "n":
				req.N = 2
			case "size":
				req.Size = "4096x4096"
			}
			_, err := client.EditImageOnce(context.Background(), req, observer)
			if err == nil {
				t.Fatal("expected rejection")
			}
			if strings.Contains(err.Error(), "example.com") {
				t.Fatal("error leaked source URL")
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("provider=%d", calls.Load())
	}
}
