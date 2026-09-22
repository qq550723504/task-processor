package batchcapture

import (
	"errors"
	"testing"
)

// The fragment parser is the only place the executor learns a capture's
// idempotency key, and the key decides which operation gets read back. A parser
// that is too permissive would let an unrelated tab be adopted as this item's
// handoff, so the rejections below are part of the safety contract rather than
// input hygiene.

const (
	testExtension = "blfjimkgomkcmkgoplpeaemcemaadcfm"
	testKey       = "3f6b1e2a-4c9d-4f1e-8a2b-5c6d7e8f9a0b"
	testHandoff   = "9a8b7c6d-5e4f-4a3b-9c8d-7e6f5a4b3c2d"
)

func TestCaptureRefFromHandoffFragment(t *testing.T) {
	raw := "https://acquisition.home.arpa/capture/1688#extensionId=" + testExtension +
		"&handoffId=" + testHandoff + "&idempotencyKey=" + testKey
	ref, err := captureRefFromURL(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := HandoffRef{ExtensionID: testExtension, HandoffID: testHandoff, IdempotencyKey: testKey}
	if ref != want {
		t.Fatalf("ref=%+v, want %+v", ref, want)
	}
}

// TestCaptureRefFromNormalisedFragment is the case that the first fixture run
// missed: the application rewrites the fragment to the recovery form as soon as it
// reads the payload, so a correct driver must accept both forms as the same
// capture.
func TestCaptureRefFromNormalisedFragment(t *testing.T) {
	ref, err := captureRefFromURL("https://acquisition.home.arpa/capture/1688#operationKey=" + testKey)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := HandoffRef{IdempotencyKey: testKey}
	if ref != want {
		t.Fatalf("ref=%+v, want %+v", ref, want)
	}
}

func TestCaptureRefRejections(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"no fragment", "https://acquisition.home.arpa/capture/1688"},
		{"empty fragment", "https://acquisition.home.arpa/capture/1688#"},
		{"unrelated fragment", "https://acquisition.home.arpa/capture/1688#other=1"},
		{"handoff missing key", "https://acquisition.home.arpa/capture/1688#extensionId=" + testExtension + "&handoffId=" + testHandoff},
		{"handoff extra parameter", "https://acquisition.home.arpa/capture/1688#extensionId=" + testExtension + "&handoffId=" + testHandoff + "&idempotencyKey=" + testKey + "&extra=1"},
		{"handoff key not a uuid", "https://acquisition.home.arpa/capture/1688#extensionId=" + testExtension + "&handoffId=" + testHandoff + "&idempotencyKey=nope"},
		{"handoff id not a uuid", "https://acquisition.home.arpa/capture/1688#extensionId=" + testExtension + "&handoffId=nope&idempotencyKey=" + testKey},
		{"handoff with query", "https://acquisition.home.arpa/capture/1688?redirect=evil#extensionId=" + testExtension + "&handoffId=" + testHandoff + "&idempotencyKey=" + testKey},
		{"handoff with credentials", "https://user:pass@acquisition.home.arpa/capture/1688#extensionId=" + testExtension + "&handoffId=" + testHandoff + "&idempotencyKey=" + testKey},
		{"recovery key not a uuid", "https://acquisition.home.arpa/capture/1688#operationKey=nope"},
		{"recovery extra parameter", "https://acquisition.home.arpa/capture/1688#operationKey=" + testKey + "&extensionId=" + testExtension},
		{"recovery with query", "https://acquisition.home.arpa/capture/1688?x=1#operationKey=" + testKey},
		{"recovery with credentials", "https://user:pass@acquisition.home.arpa/capture/1688#operationKey=" + testKey},
		{"not a url", "://::"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ref, err := captureRefFromURL(testCase.raw)
			if !errors.Is(err, ErrHandoffRef) {
				t.Fatalf("err=%v, want ErrHandoffRef", err)
			}
			if !ref.IsZero() {
				t.Fatalf("rejected url still produced %+v", ref)
			}
		})
	}
}

// TestCaptureRefIsCaseSensitiveAboutTheParameterNames pins that the parser does not
// accept lookalike parameters which would let a crafted URL name a different key.
func TestCaptureRefIsCaseSensitiveAboutTheParameterNames(t *testing.T) {
	_, err := captureRefFromURL("https://acquisition.home.arpa/capture/1688#OperationKey=" + testKey)
	if !errors.Is(err, ErrHandoffRef) {
		t.Fatalf("err=%v, want ErrHandoffRef", err)
	}
}
