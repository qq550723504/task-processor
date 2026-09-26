package httproute

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type blockingRequestBody struct {
	closed chan struct{}
	once   sync.Once
}

func (body *blockingRequestBody) Read([]byte) (int, error) {
	<-body.closed
	return 0, io.ErrClosedPipe
}

func (body *blockingRequestBody) Close() error {
	body.once.Do(func() { close(body.closed) })
	return nil
}

func TestWithRequestBodyReadTimeoutClosesBlockedHandlerBody(t *testing.T) {
	body := &blockingRequestBody{closed: make(chan struct{})}
	request := httptest.NewRequest(http.MethodPost, "/", body)
	writer := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(writer)
	context.Request = request

	done := make(chan error, 1)
	handler := WithRequestBodyReadTimeout(5*time.Millisecond, func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		done <- err
	})
	go handler(context)

	select {
	case err := <-done:
		require.ErrorIs(t, err, io.ErrClosedPipe)
	case <-time.After(250 * time.Millisecond):
		_ = body.Close()
		t.Fatal("slow request body was not interrupted")
	}
}
