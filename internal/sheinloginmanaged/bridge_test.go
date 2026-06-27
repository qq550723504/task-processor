package sheinloginmanaged

import "testing"

func TestNewAccountProviderReturnsProvider(t *testing.T) {
	if NewAccountProviderWithStoreClientFactory(nil) == nil {
		t.Fatal("expected account provider")
	}
}

func TestNewStoreSyncClientFactoryReturnsFactory(t *testing.T) {
	if NewStoreSyncClientFactoryWithStoreAPI(nil) == nil {
		t.Fatal("expected store sync factory")
	}
}
