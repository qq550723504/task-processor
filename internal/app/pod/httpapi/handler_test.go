package httpapi

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"strings"
	podapp "task-processor/internal/app/pod"
	"testing"
)

func TestPODWriteRejectsDuplicateAndCredentialsBeforeService(t *testing.T) {
	for _, body := range []string{`{"name":"first","name":"other"}`, `{"credentials":"secret"}`, `{"organizationId":"org-other"}`} {
		r := httptest.NewRequest("POST", BasePath+"/designs", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Idempotency-Key", uuid.NewString())
		var in podapp.DesignRequest
		_, e := readBody(r, &in)
		require.Error(t, e)
	}
	r := httptest.NewRequest("POST", BasePath+"/designs", strings.NewReader(`{}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Add("Idempotency-Key", uuid.NewString())
	r.Header.Add("Idempotency-Key", uuid.NewString())
	var in podapp.DesignRequest
	_, e := readBody(r, &in)
	require.Error(t, e)
}
