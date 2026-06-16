package openai

import (
	"context"
	"errors"
	"net/http"
	"testing"

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
