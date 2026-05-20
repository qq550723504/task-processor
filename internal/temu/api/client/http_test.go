package client

import (
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewHTTPManagerDefaultsToStrictTLS(t *testing.T) {
	manager := NewHTTPManager("", logrus.New().WithField("test", true), nil)
	client := manager.CreateClient()

	tlsConfig := client.GetTLSClientConfig()
	if tlsConfig == nil {
		t.Fatal("expected tls config")
	}
	if tlsConfig.InsecureSkipVerify {
		t.Fatal("expected strict tls verification by default")
	}
}

func TestNewHTTPManagerUsesExplicitTLSSetting(t *testing.T) {
	cfg := DefaultConfig()
	cfg.InsecureSkipVerify = true

	manager := NewHTTPManager("", logrus.New().WithField("test", true), cfg)
	client := manager.CreateClient()

	tlsConfig := client.GetTLSClientConfig()
	if tlsConfig == nil {
		t.Fatal("expected tls config")
	}
	if !tlsConfig.InsecureSkipVerify {
		t.Fatal("expected insecure skip verify to be enabled")
	}
}
