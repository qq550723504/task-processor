package listingkit

import (
	"errors"
	"testing"

	submissiondomain "task-processor/internal/listing/submission"
)

func TestClassifyRetryableTaskFailure_OpenAIInsufficientCredits(t *testing.T) {
	t.Parallel()

	block, ok := classifyRetryableTaskFailure(errors.New("OpenAI API error: insufficient credits in account balance"))
	if !ok {
		t.Fatal("classifyRetryableTaskFailure() ok = false, want true")
	}
	if block == nil {
		t.Fatal("classifyRetryableTaskFailure() block = nil, want retryable block")
	}
	if block.ReasonCode != submissiondomain.RetryableReasonCodeOpenAIInsufficientCredits {
		t.Fatalf("ReasonCode = %q, want %q", block.ReasonCode, submissiondomain.RetryableReasonCodeOpenAIInsufficientCredits)
	}
	if block.ReasonMessage == "" {
		t.Fatal("ReasonMessage = empty, want preserved detail")
	}
}

func TestClassifyRetryableTaskFailure_RateLimited(t *testing.T) {
	t.Parallel()

	block, ok := classifyRetryableTaskFailure(errors.New("OpenAI upstream rate limited with status code: 429"))
	if !ok || block == nil {
		t.Fatalf("classifyRetryableTaskFailure() = (%+v, %t), want retryable block", block, ok)
	}
	if block.ReasonCode != submissiondomain.RetryableReasonCodeOpenAIRateLimited {
		t.Fatalf("ReasonCode = %q, want %q", block.ReasonCode, submissiondomain.RetryableReasonCodeOpenAIRateLimited)
	}
}

func TestClassifyRetryableTaskFailure_UpstreamTimeout(t *testing.T) {
	t.Parallel()

	block, ok := classifyRetryableTaskFailure(errors.New("upstream request failed: context deadline exceeded"))
	if !ok || block == nil {
		t.Fatalf("classifyRetryableTaskFailure() = (%+v, %t), want retryable block", block, ok)
	}
	if block.ReasonCode != submissiondomain.RetryableReasonCodeUpstreamTimeout {
		t.Fatalf("ReasonCode = %q, want %q", block.ReasonCode, submissiondomain.RetryableReasonCodeUpstreamTimeout)
	}
}

func TestClassifyRetryableTaskFailure_TransientUnavailable(t *testing.T) {
	t.Parallel()

	block, ok := classifyRetryableTaskFailure(errors.New("dial tcp 10.0.0.8:443: connect: connection refused"))
	if !ok || block == nil {
		t.Fatalf("classifyRetryableTaskFailure() = (%+v, %t), want retryable block", block, ok)
	}
	if block.ReasonCode != submissiondomain.RetryableReasonCodeUpstreamTransientUnavailable {
		t.Fatalf("ReasonCode = %q, want %q", block.ReasonCode, submissiondomain.RetryableReasonCodeUpstreamTransientUnavailable)
	}
}

func TestClassifyRetryableTaskFailure_WorkerQueueBackpressure(t *testing.T) {
	t.Parallel()

	block, ok := classifyRetryableTaskFailure(errors.New("工作队列已满"))
	if !ok || block == nil {
		t.Fatalf("classifyRetryableTaskFailure() = (%+v, %t), want retryable block", block, ok)
	}
	if block.ReasonCode != submissiondomain.RetryableReasonCodeWorkerQueueBackpressure {
		t.Fatalf("ReasonCode = %q, want %q", block.ReasonCode, submissiondomain.RetryableReasonCodeWorkerQueueBackpressure)
	}
}

func TestClassifyRetryableTaskFailure_NonRetryablePermanentError(t *testing.T) {
	t.Parallel()

	block, ok := classifyRetryableTaskFailure(errors.New("validation failed: missing required category_id"))
	if ok || block != nil {
		t.Fatalf("classifyRetryableTaskFailure() = (%+v, %t), want nil,false", block, ok)
	}
}

func TestClassifyRetryableTaskFailure_DoesNotMatchAmbiguousTimeoutOrTransientWords(t *testing.T) {
	t.Parallel()

	cases := []error{
		errors.New("user configured session timeout policy is invalid"),
		errors.New("transient style field is required"),
		errors.New("network name is invalid"),
	}

	for _, err := range cases {
		block, ok := classifyRetryableTaskFailure(err)
		if ok || block != nil {
			t.Fatalf("classifyRetryableTaskFailure(%q) = (%+v, %t), want nil,false", err.Error(), block, ok)
		}
	}
}

func TestClassifyRetryableTaskFailure_DoesNotMatchBusinessEOFMessage(t *testing.T) {
	t.Parallel()

	block, ok := classifyRetryableTaskFailure(errors.New("product description ended with EOF marker in source data"))
	if ok || block != nil {
		t.Fatalf("classifyRetryableTaskFailure() = (%+v, %t), want nil,false", block, ok)
	}
}

func TestClassifyRetryableTaskFailure_TransportEOF(t *testing.T) {
	t.Parallel()

	cases := []error{
		errors.New("EOF"),
		errors.New(`Post "https://api.openai.com/v1/responses": EOF`),
	}

	for _, err := range cases {
		block, ok := classifyRetryableTaskFailure(err)
		if !ok || block == nil {
			t.Fatalf("classifyRetryableTaskFailure(%q) = (%+v, %t), want retryable block", err.Error(), block, ok)
		}
		if block.ReasonCode != submissiondomain.RetryableReasonCodeUpstreamTransientUnavailable {
			t.Fatalf("ReasonCode = %q, want %q", block.ReasonCode, submissiondomain.RetryableReasonCodeUpstreamTransientUnavailable)
		}
	}
}
