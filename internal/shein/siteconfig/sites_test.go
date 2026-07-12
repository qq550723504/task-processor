package siteconfig

import (
	"reflect"
	"task-processor/internal/shein/api/product"
	"testing"
)

func TestNormalizeFiltersMergesAndPreservesOrder(t *testing.T) {
	got := Normalize([]product.SiteListGroup{
		{MainSite: " shein ", SubSiteList: []product.SiteListSubSite{{SiteAbbr: " shein-us ", SiteStatus: 1}, {SiteAbbr: "off", SiteStatus: 0}}},
		{MainSite: "shein", SubSiteList: []product.SiteListSubSite{{SiteAbbr: "shein-us", SiteStatus: 1}, {SiteAbbr: "shein-fr", SiteStatus: 1}}},
		{MainSite: " ", SubSiteList: []product.SiteListSubSite{{SiteAbbr: "bad", SiteStatus: 1}}},
	})
	want := []product.SiteInfo{{MainSite: "shein", SubSiteList: []string{"shein-us", "shein-fr"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
