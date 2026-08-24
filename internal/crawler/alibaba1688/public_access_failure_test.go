package alibaba1688

import (
	"errors"
	"testing"
)

func TestIsAccountFallbackEligibleOnlyForRecoverablePublicFailures(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "challenge", err: NewPublicAccessError(PublicAccessFailureChallenge, errors.New("captcha")), want: true},
		{name: "missing fields", err: NewPublicAccessError(PublicAccessFailureMissingFields, errors.New("title missing")), want: true},
		{name: "invalid url", err: NewPublicAccessError(PublicAccessFailureInvalidURL, errors.New("bad url")), want: false},
		{name: "transport", err: NewPublicAccessError(PublicAccessFailureTransport, errors.New("timeout")), want: false},
		{name: "validation", err: NewPublicAccessError(PublicAccessFailureValidation, errors.New("invalid image")), want: false},
		{name: "unknown", err: errors.New("unexpected"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAccountFallbackEligible(tt.err); got != tt.want {
				t.Fatalf("IsAccountFallbackEligible() = %t, want %t", got, tt.want)
			}
		})
	}
}
