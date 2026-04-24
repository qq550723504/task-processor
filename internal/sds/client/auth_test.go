package client

import "testing"

func TestSetAuthState(t *testing.T) {
	t.Parallel()

	c, err := New(nil)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}

	c.SetAuthState(&AuthState{
		AccessToken: "token-1",
		OutToken:    "token-2",
		MerchantID:  42,
	})

	state := c.AuthState()
	if state == nil {
		t.Fatal("expected auth state")
	}
	if state.AccessToken != "token-1" || state.OutToken != "token-2" {
		t.Fatalf("unexpected auth state: %+v", state)
	}
}
