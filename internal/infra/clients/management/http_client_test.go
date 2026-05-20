package management

import "testing"

func TestNewHTTPClientDefaultsToStrictTLS(t *testing.T) {
	client := NewHTTPClient(5, false)

	tlsConfig := client.GetClient().GetTLSClientConfig()
	if tlsConfig == nil {
		t.Fatal("expected tls config")
	}
	if tlsConfig.InsecureSkipVerify {
		t.Fatal("expected strict tls verification by default")
	}
}

func TestNewHTTPClientCanEnableInsecureSkipVerify(t *testing.T) {
	client := NewHTTPClient(5, true)

	tlsConfig := client.GetClient().GetTLSClientConfig()
	if tlsConfig == nil {
		t.Fatal("expected tls config")
	}
	if !tlsConfig.InsecureSkipVerify {
		t.Fatal("expected insecure skip verify to be enabled")
	}
}
