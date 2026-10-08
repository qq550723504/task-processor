package servicepayments

import (
	"encoding/json"
	"github.com/go-pay/gopay"
	"github.com/google/uuid"
	"strings"
	e "task-processor/internal/ecoservices"
	"testing"
)

func TestMerchantChannelContractUsesApprovedIdentityEncryptedFieldsAndOriginalMedia(t *testing.T) {
	ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	a := e.MerchantAttempt{Intent: e.MerchantIntent{ID: uuid.NewString(), OutRequestNo: "immutable-original", FileIDs: ids}, CompanyName: "approved-company", RegistrationNumber: "approved-registration", Dispatched: true, MediaIDs: map[string]string{ids[0]: "license-media", ids[1]: "legal-front-media", ids[2]: "legal-back-media", ids[3]: "ubo-media"}}
	d := e.MerchantDetails{LicenseFileID: ids[0], Legal: e.IdentityDocument{Type: "IDENTIFICATION_TYPE_MAINLAND_IDCARD", Name: "private-legal", Number: "private-id", FrontFileID: ids[1], BackFileID: ids[2], ValidFrom: "2020-01-01", ValidUntil: "长期"}, Beneficiaries: []e.IdentityDocument{{Type: "IDENTIFICATION_TYPE_OVERSEA_PASSPORT", Name: "private-ubo", Number: "private-ubo-id", Address: "private-address", FrontFileID: ids[3], ValidFrom: "2020-01-01", ValidUntil: "长期"}}, AccountBank: "工商银行", AccountNumber: "private-bank", ContactMobile: "private-mobile", MerchantShortName: "服务商", StoreName: "企业网站", StoreURL: "https://provider.example/"}
	body, err := merchantBody(a, d, func(v string) (string, error) { return "encrypted(" + v + ")", nil })
	if err != nil {
		t.Fatal(err)
	}
	if body["out_request_no"] != a.Intent.OutRequestNo || body["organization_type"] != "2" {
		t.Fatal("original enterprise identity missing")
	}
	license := body["business_license_info"].(gopay.BodyMap)
	if license["legal_person"] != d.Legal.Name || license["merchant_name"] != a.CompanyName || license["business_license_number"] != a.RegistrationNumber || license["business_license_copy"] != "license-media" {
		t.Fatal("approval identity replaced")
	}
	account := body["account_info"].(gopay.BodyMap)
	if account["bank_account_type"] != "74" || account["account_name"] != "encrypted(approved-company)" || account["account_number"] != "encrypted(private-bank)" {
		t.Fatal("public enterprise bank not encrypted")
	}
	data, _ := json.Marshal(body)
	for _, secret := range []string{"private-id", "private-ubo", "private-ubo-id", "private-address", "private-bank", "private-mobile"} {
		if strings.Contains(string(data), `"`+secret+`"`) {
			t.Fatal("plaintext sent to channel")
		}
	}
	if body["sales_scene_info"].(gopay.BodyMap)["store_url"] != d.StoreURL {
		t.Fatal("real store evidence omitted")
	}
}
