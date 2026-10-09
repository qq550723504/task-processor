package dataservicesapp

import (
	"crypto/tls"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	"task-processor/internal/dataservice"
	"testing"
)

func TestTransportRejectsAmbiguousBodyAndUnauthorizedProxy(t *testing.T) {
	for _, body := range []string{`{"maximumRows":1,"maximumRows":2}`, `{"Query":{}}`, `{"actorId":"other"}`, `{"query":{},"maximumRows":1,"maximumCostFen":5,"extra":true}`} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		var input createJob
		require.ErrorIs(t, readJSON(r, &input), dataservice.ErrInvalid)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "192.0.2.1:4000"
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("X-Forwarded-For", "198.51.100.2")
	_, err := externalPeer(r, nil)
	require.ErrorIs(t, err, dataservice.ErrForbidden)
	r.TLS = &tls.ConnectionState{}
	peer, err := externalPeer(r, nil)
	require.NoError(t, err)
	require.Equal(t, "192.0.2.1", peer)
	peer, err = externalPeer(r, []string{"192.0.2.0/24"})
	require.NoError(t, err)
	require.Equal(t, "198.51.100.2", peer)
	r.Header.Set("X-Forwarded-For", "198.51.100.2,198.51.100.3")
	_, err = externalPeer(r, []string{"192.0.2.0/24"})
	require.ErrorIs(t, err, dataservice.ErrForbidden)
}
