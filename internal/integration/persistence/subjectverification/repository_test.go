package subjectverification

import (
	"bytes"
	"testing"
)

func TestProtectionRejectsTamperingAndOtherApplication(t *testing.T) {
	p, err := NewProtection(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := p.Seal("scope/org/actor/app", "https://qian.tencent.com/private")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Open("scope/other/actor/app", ciphertext); err == nil {
		t.Fatal("cross-organization ciphertext accepted")
	}
	ciphertext[len(ciphertext)-1] ^= 1
	if _, err = p.Open("scope/org/actor/app", ciphertext); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	if p.Digest("phone", "13800000001") == p.Digest("input", "13800000001") {
		t.Fatal("digest domains not separated")
	}
}
