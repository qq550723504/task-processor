package sds

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"os"
	"strings"
	"task-processor/internal/product/pod"
	"testing"
)

// Captured GETs from the retained controlled test, with account/image paths and
// unrelated fields removed. Field spellings and snowflake representation remain.
func TestCapturedFreeProtocolManifestAndFullFinishedReadback(t *testing.T) {
	raw, e := os.ReadFile("testdata/qualified-free.json")
	require.NoError(t, e)
	var f struct{ Manifest, Listed, Detail, Saved, Task, Children json.RawMessage }
	require.NoError(t, json.Unmarshal(raw, &f))
	credentials := readbackCredentials{Credentials{"binding", "revision", "123", "test", ""}}
	h := &http.Client{Transport: readbackRoundTripper(func(r *http.Request) (*http.Response, error) {
		require.Equal(t, "GET", r.Method)
		var body string
		switch r.URL.Path {
		case "/ps/design/products/95147":
			body = string(f.Manifest)
		case "/design_products":
			body = `{"total_count":1,"items":[` + string(f.Listed) + `]}`
		case "/design_products/962657110282047488":
			body = string(f.Detail)
		case "/ps/endproducts/962657110282047488/design":
			body = string(f.Saved)
		case "/ps/task/result":
			body = `{"total":1,"items":[` + string(f.Task) + `]}`
		case "/ps/task/962657107283120129/children":
			body = string(f.Children)
		default:
			t.Fatalf("unqualified path %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	templates, e := NewTemplateClient(h, credentials)
	require.NoError(t, e)
	m, e := templates.Manifest(context.Background(), pod.AccountBinding{"binding", "revision", "123", pod.ProtocolRevision}, "95146", "95147")
	require.NoError(t, e)
	require.Len(t, m.RenderFiles, 8)
	var saved savedDesignDTO
	require.NoError(t, json.Unmarshal(f.Saved, &saved))
	d := saved.record()
	i := pod.DesignIntent{OperationID: "12752596-6056-4316-9f2f-380c97df9675", MerchantID: "123", ParentID: d.ParentID, VariantID: d.VariantID, PrototypeID: d.PrototypeID, GroupID: d.GroupID, Materials: []pod.MaterialReference{{ID: "480953643", FileCode: "art.png"}}, Layers: d.Layers, RenderFileIDs: d.RenderFileIDs}
	observer, e := NewReadbackClient(h, credentials)
	require.NoError(t, e)
	q, e := observer.Observe(context.Background(), Binding{"binding", "revision", "123"}, i)
	require.NoError(t, e)
	require.Equal(t, "962657110282047488", q.Reference().ID)
	require.Len(t, q.Reference().RenderURLs, 8)
}
