package store_test

import (
	"testing"

	"task-processor/internal/shein/store"
)

func TestGetSiteListByRegion(t *testing.T) {
	tests := []struct {
		region       string
		wantMainSite string
		wantSubSite  string
	}{
		{"US", "shein", "shein-us"},
		{"FR", "shein", "shein-fr"},
		{"DE", "shein", "shein-de"},
		{"IT", "shein", "shein-it"},
		{"ES", "shein", "shein-es"},
		{"UK", "shein", "shein-uk"},
		{"AU", "shein", "shein-au"},
		{"JP", "shein", "shein-jp"},
		{"MX", "shein", "shein-mx"},
		{"SA", "shein", "shein-sa"},
		{"AE", "shein", "shein-ae"},
		{"CA", "shein", "shein-ca"},
		// 未知区域默认返回 US
		{"UNKNOWN", "shein", "shein-us"},
		{"", "shein", "shein-us"},
	}

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			sites := store.GetSiteListByRegion(tt.region)

			if len(sites) == 0 {
				t.Fatalf("GetSiteListByRegion(%q) returned empty slice", tt.region)
			}
			if sites[0].MainSite != tt.wantMainSite {
				t.Errorf("MainSite = %q, want %q", sites[0].MainSite, tt.wantMainSite)
			}
			if len(sites[0].SubSiteList) == 0 {
				t.Fatalf("SubSiteList is empty for region %q", tt.region)
			}
			if sites[0].SubSiteList[0] != tt.wantSubSite {
				t.Errorf("SubSiteList[0] = %q, want %q", sites[0].SubSiteList[0], tt.wantSubSite)
			}
		})
	}
}

func TestGetCurrencyByRegion(t *testing.T) {
	tests := []struct {
		region string
		want   string
	}{
		{"US", "USD"},
		{"FR", "EUR"},
		{"DE", "EUR"},
		{"IT", "EUR"},
		{"ES", "EUR"},
		{"UK", "GBP"},
		{"AU", "AUD"},
		{"JP", "JPY"},
		{"CA", "CAD"},
		{"MX", "MXN"},
		{"SA", "SAR"},
		{"AE", "AED"},
		// 未知区域默认 USD
		{"UNKNOWN", "USD"},
		{"", "USD"},
	}

	for _, tt := range tests {
		t.Run(tt.region, func(t *testing.T) {
			got := store.GetCurrencyByRegion(tt.region)
			if got != tt.want {
				t.Errorf("GetCurrencyByRegion(%q) = %q, want %q", tt.region, got, tt.want)
			}
		})
	}
}
