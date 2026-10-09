package servicepayments

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	wechat "github.com/go-pay/gopay/wechat/v3"
	"strconv"
	"testing"
	"time"
)

func TestServiceUnsplitRequiresSignedPresentIntegerAndOriginalTransaction(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p, o, _ := wechatServiceFixture()
	p.publicKey = &key.PublicKey
	now := time.Now().UTC()
	p.now = func() time.Time { return now }
	for _, body := range []string{`{"transaction_id":"transaction","unsplit_amount":0}`, `{"transaction_id":"transaction"}`, `{"transaction_id":"foreign","unsplit_amount":0}`, `{"transaction_id":"transaction","unsplit_amount":null}`, `{"transaction_id":"transaction","unsplit_amount":0.5}`, `{"transaction_id":"transaction","unsplit_amount":-1}`} {
		timestamp := strconv.FormatInt(now.Unix(), 10)
		hash := sha256.Sum256([]byte(timestamp + "\nnonce\n" + body + "\n"))
		signature, _ := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
		sign := &wechat.SignInfo{HeaderTimestamp: timestamp, HeaderNonce: "nonce", HeaderSerial: "PUB_KEY_ID_fixture", HeaderSignature: base64.StdEncoding.EncodeToString(signature), SignBody: body}
		got, e := p.unsplitResult(o, sign)
		if body == `{"transaction_id":"transaction","unsplit_amount":0}` {
			if e != nil || !got.Matches(o) || got.UnsplitMinor != 0 {
				t.Fatalf("signed original zero not accepted: %+v %v", got, e)
			}
			sign.HeaderSignature = "invalid"
			if _, e = p.unsplitResult(o, sign); e == nil {
				t.Fatal("unverified response accepted")
			}
		} else if e == nil {
			t.Fatalf("invalid original amount accepted: %s", body)
		}
	}
}
