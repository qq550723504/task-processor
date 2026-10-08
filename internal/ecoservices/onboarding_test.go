package ecoservices

import (
	"github.com/google/uuid"
	"testing"
	"time"
)

func validMerchantFixture() MerchantDetails {
	return MerchantDetails{LicenseFileID: uuid.NewString(), Legal: IdentityDocument{Type: "IDENTIFICATION_TYPE_MAINLAND_IDCARD", Name: "法人", Number: "110101199001011234", FrontFileID: uuid.NewString(), BackFileID: uuid.NewString(), ValidFrom: "2020-01-01", ValidUntil: "长期"}, SoleLegalBeneficiary: true, ContactMobile: "13800138000", AccountBank: "工商银行", AccountNumber: "622200000000000001", MerchantShortName: "企业服务", StoreName: "企业服务网站", StoreURL: "https://provider.example/"}
}

func TestMerchantDocumentValidityUsesChinaCalendarDay(t *testing.T) {
	for _, beneficiary := range []bool{false, true} {
		for _, hour := range []int{0, 1, 7, 8, 23} {
			now := time.Date(2026, 10, 8, hour, 0, 0, 0, time.FixedZone("China", 8*60*60))
			d := validMerchantFixture().Legal
			d.Address = "上海市试用地址"
			d.ValidFrom = "2026-10-07"
			if !validDocumentAt(d, beneficiary, now.UTC()) {
				t.Fatalf("past start date rejected: beneficiary=%v hour=%d", beneficiary, hour)
			}
			d.ValidFrom = "2026-10-08"
			if validDocumentAt(d, beneficiary, now.UTC()) {
				t.Fatalf("channel-forbidden current start date accepted: beneficiary=%v hour=%d", beneficiary, hour)
			}
			d.ValidFrom = "2026-10-09"
			if validDocumentAt(d, beneficiary, now) {
				t.Fatalf("future start date accepted: beneficiary=%v hour=%d", beneficiary, hour)
			}
			d.ValidFrom, d.ValidUntil = "2020-01-01", "2026-10-08"
			if !validDocumentAt(d, beneficiary, now) {
				t.Fatalf("current expiry date rejected: beneficiary=%v hour=%d", beneficiary, hour)
			}
			d.ValidUntil = "2026-10-07"
			if validDocumentAt(d, beneficiary, now) {
				t.Fatalf("expired date accepted: beneficiary=%v hour=%d", beneficiary, hour)
			}
		}
	}
}
func TestMerchantDocumentsIncludeLegalAmongBeneficiariesWithoutDuplicateUpload(t *testing.T) {
	d := validMerchantFixture()
	d.SoleLegalBeneficiary = false
	b := d.Legal
	b.Address = "上海市原登记地址"
	d.Beneficiaries = []IdentityDocument{b}
	ids, err := ValidateMerchantDetails(d)
	if err != nil || len(ids) != 3 {
		t.Fatal("legal beneficiary requires duplicate proof upload", err)
	}
}
func TestMerchantNonPassportRequiresBothDocumentSides(t *testing.T) {
	d := validMerchantFixture()
	d.Legal.Type = "IDENTIFICATION_TYPE_HONGKONG"
	d.Legal.BackFileID = ""
	if _, err := ValidateMerchantDetails(d); err == nil {
		t.Fatal("channel-mandatory back side omitted")
	}
}
