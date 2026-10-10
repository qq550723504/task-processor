package sds

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"strings"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSyncEmptyResponseRemainsUnknownAndDoesNotRetry(t *testing.T) {
	calls := 0
	c, e := NewMutationClient(&http.Client{Transport: readbackRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/ps/design/syncDesign", r.URL.Path)
		require.Empty(t, r.Header.Get("Idempotency-Key"))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	})}, readbackCredentials{Credentials{"binding", "revision", "36811", "test", ""}}, []string{"test.oss-cn-shenzhen.aliyuncs.com"})
	require.NoError(t, e)
	p := pod.Plan{Binding: pod.AccountBinding{"binding", "revision", "36811", pod.ProtocolRevision}, OperationID: "12752596-6056-4316-9f2f-380c97df9675"}
	permit := &submission.SendPermit{AttemptID: "attempt", ClaimToken: "test", LeaseExpiresAt: time.Now().Add(time.Minute)}
	e = c.Sync(context.Background(), p, []byte(`{"product_id":95147}`), pod.TransportPermit(permit))
	require.ErrorIs(t, e, pod.ErrUnknown)
	require.Equal(t, 1, calls)
	_, e = c.Upload(context.Background(), p, nil, nil)
	require.ErrorIs(t, e, pod.ErrInvalid)
	require.Equal(t, 1, calls)
}

func TestCreateMaterialQualifiedShapeAndExactOriginalIdentity(t *testing.T) {
	raw, err := os.ReadFile("testdata/qualified-material.json")
	require.NoError(t, err)
	var thumbnail bytes.Buffer
	require.NoError(t, png.Encode(&thumbnail, image.NewRGBA(image.Rect(0, 0, 1200, 1799))))
	for _, tc := range []struct {
		name   string
		change func(map[string]any, map[string]any)
		ok     bool
	}{
		{"qualified current shape", func(_, _ map[string]any) {}, true},
		{"different merchant", func(m, _ map[string]any) { m["merchant_id"] = 124 }, false},
		{"another operation material", func(m, _ map[string]any) { m["name"] = "sds-pod-another-operation" }, false},
		{"different file", func(m, _ map[string]any) { m["file_code"] = "another.png" }, false},
		{"different format", func(m, _ map[string]any) { m["file_format"] = "jpg" }, false},
		{"unconfirmed name", func(_, m map[string]any) { m["name"] = "another-operation" }, false},
		{"unconfirmed dimensions", func(_, m map[string]any) { m["height"] = 1999 }, false},
		{"unconfirmed file", func(_, m map[string]any) {
			m["imgUrl"] = "https://cdn.sdspod.com/imagesThumbs/test-account/another.png"
		}, false},
		{"untrusted origin", func(_, m map[string]any) { m["imgUrl"] = "https://example.com/imagesThumbs/test-account/test-file.png" }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var captured struct {
				Create struct {
					Ret  int              `json:"ret"`
					Msg  string           `json:"msg"`
					Data []map[string]any `json:"data"`
				} `json:"create"`
				FindByIDs []map[string]any `json:"findByIds"`
			}
			require.NoError(t, json.Unmarshal(raw, &captured))
			tc.change(captured.Create.Data[0], captured.FindByIDs[0])
			posts, reads, images := 0, 0, 0
			client, err := NewMutationClient(&http.Client{Transport: readbackRoundTripper(func(r *http.Request) (*http.Response, error) {
				var body []byte
				switch r.URL.Host + r.URL.Path {
				case "mapi.sdspod.com/materials/one":
					posts++
					require.Equal(t, http.MethodPost, r.Method)
					var payload map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
					require.Equal(t, true, payload["repeatReturnId"])
					require.Equal(t, "sds-pod-6464cb0d-9ebe-4e3d-b306-e0ed5e889a24", payload["name"])
					body, err = json.Marshal(captured.Create)
				case "mapi.sdspod.com/materials/findByIds":
					reads++
					require.Equal(t, http.MethodGet, r.Method)
					require.Equal(t, "480968536", r.URL.Query().Get("ids"))
					body, err = json.Marshal(captured.FindByIDs)
				case "cdn.sdspod.com/images1000Thumbs/test-account/test-file.png":
					images++
					require.Equal(t, http.MethodGet, r.Method)
					require.Empty(t, r.Header.Get("Access-Token"))
					require.Equal(t, "480968536", r.URL.Query().Get("material_id"))
					body = thumbnail.Bytes()
				default:
					t.Fatalf("unexpected request: %s %s", r.Method, r.URL)
				}
				require.NoError(t, err)
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}, nil
			})}, readbackCredentials{Credentials{"binding", "revision", "123", "test", ""}}, []string{"test.oss-cn-shenzhen.aliyuncs.com"})
			require.NoError(t, err)
			plan := pod.Plan{Binding: pod.AccountBinding{"binding", "revision", "123", pod.ProtocolRevision}, OperationID: "6464cb0d-9ebe-4e3d-b306-e0ed5e889a24", Artwork: pod.ArtworkReference{Hash: collection.Digest("approved-artwork"), Bytes: 62279, Width: 1334, Height: 2000, MediaType: "image/png"}}
			permit := pod.TransportPermit(&submission.SendPermit{AttemptID: "attempt", ClaimToken: "test", LeaseExpiresAt: time.Now().Add(time.Minute)})
			material, err := client.CreateMaterial(context.Background(), plan, pod.ObjectReceipt{FileCode: "test-file.png", Hash: plan.Artwork.Hash}, permit)
			if tc.ok {
				require.NoError(t, err)
				require.Equal(t, "480968536", material.ID)
				require.Equal(t, plan.Artwork.Hash, material.Hash)
				require.Equal(t, 1200, material.Width)
				require.Equal(t, 1799, material.Height)
				require.Equal(t, 1, reads)
				require.Equal(t, 1, images)
			} else {
				require.ErrorIs(t, err, pod.ErrUnknown)
				require.Empty(t, material.ID)
				require.Zero(t, images)
			}
			require.Equal(t, 1, posts)
		})
	}
}
