package client

import (
	"context"
	"errors"
	"net"
	"net/http"
)

const (
	RetryableUploadReasonTimeout          = "sds_oss_upload_timeout"
	RetryableUploadReasonTransientNetwork = "sds_oss_upload_transient_network"
	RetryableUploadReasonRateLimited      = "sds_oss_upload_rate_limited"
	RetryableUploadReasonServerError      = "sds_oss_upload_server_error"
)

// RetryableUploadFailure reports whether an SDS multipart-upload error can be
// retried after a durable backoff.
func RetryableUploadFailure(err error) (reasonCode string, ok bool) {
	var uploadErr *Error
	if !errors.As(err, &uploadErr) || uploadErr.Kind != ErrorKindMultipartUpload {
		return "", false
	}

	switch {
	case errors.Is(uploadErr, context.DeadlineExceeded):
		return RetryableUploadReasonTimeout, true
	case uploadErr.StatusCode == http.StatusTooManyRequests:
		return RetryableUploadReasonRateLimited, true
	case uploadErr.StatusCode >= http.StatusInternalServerError && uploadErr.StatusCode < 600:
		return RetryableUploadReasonServerError, true
	}

	var networkErr net.Error
	if errors.As(uploadErr, &networkErr) && networkErr.Timeout() {
		return RetryableUploadReasonTimeout, true
	}
	var temporaryErr interface{ Temporary() bool }
	if errors.As(uploadErr, &temporaryErr) && temporaryErr.Temporary() {
		return RetryableUploadReasonTransientNetwork, true
	}
	return "", false
}
