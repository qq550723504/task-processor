package alibaba1688

import (
	"errors"
	"testing"
)

func TestIsChallengeErrorRecognizesTypedAndKnownChallengeMessages(t *testing.T) {
	if !isChallengeError(NewPublicAccessError(PublicAccessFailureChallenge, errors.New("blocked"))) {
		t.Fatal("typed challenge was not recognized")
	}
	if !isChallengeError(errors.New("验证码处理失败")) {
		t.Fatal("captcha message was not recognized")
	}
	if isChallengeError(errors.New("network timeout")) {
		t.Fatal("transport message was classified as challenge")
	}
}
