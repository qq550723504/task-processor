package ecoservices

import (
	"github.com/google/uuid"
	"testing"
)

func validMerchantFixture() MerchantDetails {
	return MerchantDetails{LicenseFileID: uuid.NewString(), Legal: IdentityDocument{Type: "IDENTIFICATION_TYPE_MAINLAND_IDCARD", Name: "法人", Number: "110101199001011234", FrontFileID: uuid.NewString(), BackFileID: uuid.NewString(), ValidFrom: "2020-01-01", ValidUntil: "长期"}, SoleLegalBeneficiary: true, ContactMobile: "13800138000", AccountBank: "工商银行", AccountNumber: "622200000000000001", MerchantShortName: "企业服务", StoreName: "企业服务网站", StoreURL: "https://provider.example/"}
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
