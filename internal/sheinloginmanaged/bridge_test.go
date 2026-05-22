package sheinloginmanaged

import "testing"

func TestNewAccountProviderReturnsProvider(t *testing.T) {
	if NewAccountProvider(nil) == nil {
		t.Fatal("expected account provider")
	}
}

func TestNewStoreSyncClientFactoryReturnsFactory(t *testing.T) {
	if NewStoreSyncClientFactory(nil) == nil {
		t.Fatal("expected store sync factory")
	}
}
