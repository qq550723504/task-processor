package sds

import (
	"context"
	"io"
	"net/http"
	"strings"
	"task-processor/internal/product/pod"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManifestRejectsUnsupportedLayersAndCrossVariant(t *testing.T) {
	raw := `{"product":{"id":95147,"parent_id":95146,"prototypeId":"730897612975054849","prototypeType":"FREE"},"prototypeGroup":{"id":15261,"productId":95146},"layers":[{"id":"730897620537384960","prototypeId":"730897612975054849","type":1,"width":567,"height":850,"print_width":1334,"print_height":2000}],"psds":[{"id":"753068348962332672","prototypeId":"730897612975054849","thumbnail_url":"http://e.sdspod.com/builds?content=test"}]}`
	current := raw
	calls := 0
	c, _ := NewTemplateClient(&http.Client{Transport: readbackRoundTripper(func(r *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "GET", r.Method)
		require.True(t, strings.HasPrefix(r.URL.Path, "/ps/design/products/9514"))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(current)), Header: http.Header{}}, nil
	})}, readbackCredentials{Credentials{"binding", "revision", "36811", "test", ""}})
	binding := pod.AccountBinding{"binding", "revision", "36811", pod.ProtocolRevision}
	m, e := c.Manifest(context.Background(), binding, "95146", "95147")
	require.NoError(t, e)
	require.Len(t, m.Layers, 1)
	current = strings.Replace(raw, `"type":1`, `"type":2`, 1)
	_, e = c.Manifest(context.Background(), binding, "95146", "95147")
	require.ErrorIs(t, e, pod.ErrUnavailable)
	current = raw
	_, e = c.Manifest(context.Background(), binding, "95146", "95148")
	require.Error(t, e)
	require.Equal(t, 3, calls)
}
