package client

import (
	"context"
	"net/http"
	"testing"
)

func TestRetryableUploadFailure(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		err        error
		wantReason string
		wantOK     bool
	}{
		{
			name:       "response header timeout",
			err:        &Error{Kind: ErrorKindMultipartUpload, Message: "multipart upload failed", Err: context.DeadlineExceeded},
			wantReason: RetryableUploadReasonTimeout,
			wantOK:     true,
		},
		{
			name:       "temporary network failure",
			err:        &Error{Kind: ErrorKindMultipartUpload, Message: "multipart upload failed", Err: transientUploadNetworkError{}},
			wantReason: RetryableUploadReasonTransientNetwork,
			wantOK:     true,
		},
		{
			name:       "rate limited",
			err:        &Error{Kind: ErrorKindMultipartUpload, StatusCode: http.StatusTooManyRequests, Message: "too many requests"},
			wantReason: RetryableUploadReasonRateLimited,
			wantOK:     true,
		},
		{
			name:       "upstream server error",
			err:        &Error{Kind: ErrorKindMultipartUpload, StatusCode: http.StatusServiceUnavailable, Message: "unavailable"},
			wantReason: RetryableUploadReasonServerError,
			wantOK:     true,
		},
		{
			name:   "invalid signature is permanent",
			err:    &Error{Kind: ErrorKindMultipartUpload, StatusCode: http.StatusForbidden, Message: "SignatureDoesNotMatch"},
			wantOK: false,
		},
		{
			name:   "non SDS error is not classified",
			err:    context.DeadlineExceeded,
			wantOK: false,
		},
		{
			name:   "non upload SDS timeout is not classified",
			err:    &Error{Message: "request send failed", Err: context.DeadlineExceeded},
			wantOK: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gotReason, gotOK := RetryableUploadFailure(testCase.err)
			if gotOK != testCase.wantOK {
				t.Fatalf("RetryableUploadFailure() ok = %t, want %t", gotOK, testCase.wantOK)
			}
			if gotReason != testCase.wantReason {
				t.Fatalf("RetryableUploadFailure() reason = %q, want %q", gotReason, testCase.wantReason)
			}
		})
	}
}

type transientUploadNetworkError struct{}

func (transientUploadNetworkError) Error() string   { return "connection reset" }
func (transientUploadNetworkError) Timeout() bool   { return false }
func (transientUploadNetworkError) Temporary() bool { return true }
