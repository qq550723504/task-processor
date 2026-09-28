package shein

import (
	"strings"
	"task-processor/internal/storecenter"
	"testing"
)

func TestMerchantCredentialCiphertextBindsOriginalScopeAndKey(t *testing.T) {
	p, err := NewCredentialProtection("key-1", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	a := storecenter.OfficialConnectionAttempt{OrganizationID: "org-a", StoreID: "store-a", AttemptID: "attempt-a", AppID: "app-a", AppVersion: "version-a", ActorID: "actor-a", MemberID: "member-a", ConnectionVersion: 1}
	credential := storecenter.OfficialMerchantCredential{AppID: a.AppID, OpenKeyID: "open-key", SecretKey: "synthetic-secret", SupplierID: "123"}
	key, encrypted, err := p.Seal(a, credential)
	if err != nil || strings.Contains(encrypted, credential.SecretKey) {
		t.Fatalf("seal: %v", err)
	}
	if actual, err := p.Open(a, key, encrypted); err != nil || actual != credential {
		t.Fatalf("open: %v", err)
	}
	for _, alter := range []func(*storecenter.OfficialConnectionAttempt){func(a *storecenter.OfficialConnectionAttempt) { a.OrganizationID = "org-b" }, func(a *storecenter.OfficialConnectionAttempt) { a.StoreID = "store-b" }, func(a *storecenter.OfficialConnectionAttempt) { a.MemberID = "rejoined" }, func(a *storecenter.OfficialConnectionAttempt) { a.AttemptID = "attempt-b" }, func(a *storecenter.OfficialConnectionAttempt) { a.AppVersion = "version-b" }, func(a *storecenter.OfficialConnectionAttempt) { a.ConnectionVersion++ }} {
		changed := a
		alter(&changed)
		if _, err := p.Open(changed, key, encrypted); err == nil {
			t.Fatal("cipher accepted different original scope")
		}
	}
	if _, err := p.Open(a, "key-2", encrypted); err == nil {
		t.Fatal("unknown key accepted")
	}
	bytes := []byte(encrypted)
	bytes[len(bytes)/2] = '!'
	if _, err := p.Open(a, key, string(bytes)); err == nil {
		t.Fatal("altered cipher accepted")
	}
	if _, err := NewCredentialProtection("key", make([]byte, 16)); err == nil {
		t.Fatal("wrong deployment key length accepted")
	}
}
