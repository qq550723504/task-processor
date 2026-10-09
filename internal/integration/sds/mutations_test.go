package sds

import (
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/pod"
	"testing"
	"time"
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
	e = c.Sync(context.Background(), p, []byte(`{"product_id":95147}`), permit)
	require.ErrorIs(t, e, pod.ErrUnknown)
	require.Equal(t, 1, calls)
	_, e = c.Upload(context.Background(), p, nil, nil)
	require.ErrorIs(t, e, pod.ErrInvalid)
	require.Equal(t, 1, calls)
}
