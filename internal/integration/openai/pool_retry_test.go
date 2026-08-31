package openai

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	goopenai "github.com/sashabaranov/go-openai"
)

func TestShouldRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil error",
			err:  nil,
			want: false,
		},
		{
			name: "context canceled",
			err:  context.Canceled,
			want: false,
		},
		{
			name: "api rate limited",
			err: &goopenai.APIError{
				HTTPStatusCode: http.StatusTooManyRequests,
				HTTPStatus:     "429 Too Many Requests",
				Message:        "rate limit exceeded",
			},
			want: true,
		},
		{
			name: "api server overloaded reported as 400",
			err: &goopenai.APIError{
				HTTPStatusCode: http.StatusBadRequest,
				HTTPStatus:     "400 Bad Request",
				Message:        "The model load is too high, please try again later",
			},
			want: true,
		},
		{
			name: "api invalid request",
			err: &goopenai.APIError{
				HTTPStatusCode: http.StatusBadRequest,
				HTTPStatus:     "400 Bad Request",
				Message:        "Invalid parameter: response_format",
			},
			want: false,
		},
		{
			name: "generic error defaults retryable",
			err:  errors.New("temporary network issue"),
			want: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := shouldRetry(tt.err); got != tt.want {
				t.Fatalf("shouldRetry(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestShouldRetryWithContext(t *testing.T) {
	t.Parallel()

	t.Run("request timeout from inner attempt is retryable while parent context is still active", func(t *testing.T) {
		t.Parallel()

		err := &url.Error{
			Op:  "Post",
			URL: "https://example.com/v1/chat/completions",
			Err: context.DeadlineExceeded,
		}

		if got := shouldRetryWithContext(context.Background(), err); !got {
			t.Fatalf("shouldRetryWithContext(active parent, %v) = %v, want true", err, got)
		}
	})

	t.Run("parent context deadline exceeded is not retryable", func(t *testing.T) {
		t.Parallel()

		parentCtx, cancel := context.WithCancel(context.Background())
		cancel()

		if got := shouldRetryWithContext(parentCtx, context.Canceled); got {
			t.Fatalf("shouldRetryWithContext(canceled parent, context.Canceled) = %v, want false", got)
		}
	})

	t.Run("parent context timeout is not retryable", func(t *testing.T) {
		t.Parallel()

		parentCtx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
		defer cancel()
		<-parentCtx.Done()

		if got := shouldRetryWithContext(parentCtx, parentCtx.Err()); got {
			t.Fatalf("shouldRetryWithContext(expired parent, %v) = %v, want false", parentCtx.Err(), got)
		}
	})
}
