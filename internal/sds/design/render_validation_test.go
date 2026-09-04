package design

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task-processor/internal/integration/httpimage"
	sdsclient "task-processor/internal/sds/client"
)

type comparisonRoundTripFunc func(*http.Request) (*http.Response, error)

func (f comparisonRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestDownloadImageForComparisonRejectsPrivateURLBeforeRequest(t *testing.T) {
	t.Parallel()

	requests := 0
	client := &http.Client{Transport: comparisonRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		return comparisonImageResponse(request, http.StatusOK, "image/png", "not reached"), nil
	})}

	if _, ok := downloadImageForComparisonWithClient(context.Background(), "https://127.0.0.1/private.png", client); ok {
		t.Fatal("private image URL was accepted")
	}
	if requests != 0 {
		t.Fatalf("private image URL made %d requests, want 0", requests)
	}
}

func TestDownloadImageForComparisonRejectsPrivateRedirect(t *testing.T) {
	t.Parallel()

	requests := 0
	client := httpimage.NewPublicImageHTTPClient()
	client.Transport = comparisonRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		requests++
		response := comparisonImageResponse(request, http.StatusFound, "", "")
		response.Header.Set("Location", "https://169.254.169.254/latest/meta-data")
		return response, nil
	})

	if _, ok := downloadImageForComparisonWithClient(context.Background(), "https://example.com/render.png", client); ok {
		t.Fatal("private redirect target was accepted")
	}
	if requests != 1 {
		t.Fatalf("redirect path made %d requests, want only the public request", requests)
	}
}

func TestDownloadImageForComparisonRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	client := &http.Client{Transport: comparisonRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		response := comparisonImageResponse(request, http.StatusOK, "image/png", "body must not be read")
		response.ContentLength = httpimage.DefaultMaxBodyBytes + 1
		return response, nil
	})}

	if _, ok := downloadImageForComparisonWithClient(context.Background(), "https://example.com/render.png", client); ok {
		t.Fatal("oversized image body was accepted")
	}
}

func TestFetchFinishedProductImagesDoesNotReturnBlankCandidate(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/design_products" {
			t.Fatalf("request path = %q, want /design_products", request.URL.Path)
		}
		_, _ = response.Write([]byte(`{"items":[{"product_id":101,"buildFinish":true,"finish_time":1,"img_urls":["https://images.example.com/render.png"]}],"total_count":1}`))
	}))
	defer server.Close()

	config := sdsclient.DefaultConfig()
	config.Endpoints.DesignProductsPath = server.URL + "/design_products"
	config.AuthBootstrap = sdsclient.AuthBootstrapConfig{}
	client, err := sdsclient.New(config)
	if err != nil {
		t.Fatalf("new SDS client: %v", err)
	}
	blankPNG := solidComparisonPNG(t)
	service := NewService(client)
	service.imageHTTPClient = &http.Client{Transport: comparisonRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		response := comparisonImageResponse(request, http.StatusOK, "image/png", string(blankPNG))
		response.ContentLength = int64(len(blankPNG))
		return response, nil
	})}

	urls, _, _ := service.fetchFinishedProductImageURLsByProduct(
		context.Background(),
		PrepareSyncDesignInput{ParentProductID: 100, BlankDesignURL: "https://images.example.com/blank.png"},
		&PrepareSyncDesignResult{},
		100,
		[]int64{101},
	)
	if len(urls) != 0 {
		t.Fatalf("blank SDS candidate URLs = %#v, want none", urls)
	}
}

func comparisonImageResponse(request *http.Request, status int, contentType, body string) *http.Response {
	header := make(http.Header)
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	return &http.Response{
		StatusCode: status,
		Header:     header,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}

func solidComparisonPNG(t *testing.T) []byte {
	t.Helper()
	value := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			value.Set(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, value); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
	return encoded.Bytes()
}
