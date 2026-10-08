package paymentsecurity

import (
	"bytes"
	"testing"
)

func TestPayloadCannotMoveAcrossOrdersNamespacesOrKeys(t *testing.T) {
	key := bytes.Repeat([]byte{9}, 32)
	eco, err := NewPayloadProtection(key, "ecoservices-payment:v1:")
	if err != nil {
		t.Fatal(err)
	}
	wallet, _ := NewPayloadProtection(key, "wallet-topup:v1:")
	otherKey, _ := NewPayloadProtection(bytes.Repeat([]byte{8}, 32), "ecoservices-payment:v1:")
	binding := "service-checkout:original-immutable-order"
	sealed, err := eco.Seal(binding, "weixin://wxpay/bizpayurl?pr=private-order")
	if err != nil {
		t.Fatal(err)
	}
	if plain, err := eco.Open(binding, sealed); err != nil || plain != "weixin://wxpay/bizpayurl?pr=private-order" {
		t.Fatal("original payload cannot decrypt")
	}
	if _, err := eco.Open("service-checkout:different-order", sealed); err == nil {
		t.Fatal("another order decrypted private checkout")
	}
	for _, reader := range []*PayloadProtection{wallet, otherKey} {
		if _, err := reader.Open(binding, sealed); err == nil {
			t.Fatal("another consumer/key decrypted private checkout")
		}
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := eco.Open(binding, sealed); err == nil {
		t.Fatal("changed payload authenticated")
	}
}
