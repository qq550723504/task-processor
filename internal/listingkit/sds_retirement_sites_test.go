package listingkit

import (
	"encoding/json"
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildSDSRetirementShelfRequestUsesSelectedOffSubSites(t *testing.T) {
	item := SDSRetirementItemRecord{
		SPUName:       "SPU-1",
		SKCName:       "SKC-1",
		SiteSelection: `[{"site_abbr":"US","store_type":1}]`,
	}
	req, err := buildSDSRetirementShelfRequest(item, 2)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if req.SpuName != "SPU-1" || len(req.SkcSiteInfos) != 1 {
		t.Fatalf("request = %+v", req)
	}
	info := req.SkcSiteInfos[0]
	if info.BusinessModel != 2 || info.SkcName != "SKC-1" {
		t.Fatalf("skc site info = %+v", info)
	}
	if len(info.OffSubSites) != 1 || info.OffSubSites[0].SiteAbbr != "US" || info.OffSubSites[0].StoreType != 1 {
		t.Fatalf("off sites = %+v", info.OffSubSites)
	}
}

func TestSDSRetirementShelfRequestRejectsMissingSiteSelection(t *testing.T) {
	_, err := buildSDSRetirementShelfRequest(SDSRetirementItemRecord{SPUName: "SPU", SKCName: "SKC"}, 1)
	if err == nil {
		t.Fatal("expected missing site selection error")
	}
}

func TestSDSRetirementSiteSelectionRoundTrip(t *testing.T) {
	raw := []sheinproduct.SubSite{{SiteAbbr: "US", StoreType: 1}}
	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	sites, err := parseSDSRetirementSiteSelection(string(encoded))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(sites) != 1 || sites[0].SiteAbbr != "US" {
		t.Fatalf("sites = %+v", sites)
	}
}
