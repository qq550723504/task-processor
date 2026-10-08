package servicepayments

import (
	"context"
	"github.com/go-pay/gopay"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	e "task-processor/internal/ecoservices"
)

func (p *WeChat) MerchantProfile() e.MerchantProfile {
	return e.MerchantProfile{Version: p.Profile().Version, PlatformMerchantID: p.Profile().PlatformMerchantID}
}
func (p *WeChat) NewMerchantApplicationsEnabled() bool {
	return p.config.NewMerchantApplications && p.config.ProductQualified
}
func (p *WeChat) UploadMerchantImage(ctx context.Context, f e.File, data []byte) (string, error) {
	if !p.NewMerchantApplicationsEnabled() || f.State != "CONFIRMED" || f.SizeBytes > 2<<20 || f.SizeBytes != int64(len(data)) || e.FileDigest(data) != f.SHA256 || f.ContentType != "image/jpeg" && f.ContentType != "image/png" {
		return "", e.ErrInvalid
	}
	r, err := p.client.V3MediaUploadImage(ctx, f.Filename, f.SHA256, &gopay.File{Name: f.Filename, Content: data})
	if err != nil || r == nil || r.Code != 0 || r.Response == nil || r.Response.MediaId == "" || len(r.Response.MediaId) > 256 {
		return "", e.ErrUnavailable
	}
	return r.Response.MediaId, nil
}
func merchantBody(a e.MerchantAttempt, d e.MerchantDetails, encrypt func(string) (string, error)) (gopay.BodyMap, error) {
	if !a.Dispatched || encrypt == nil || a.Intent.OutRequestNo == "" || a.CompanyName == "" || a.RegistrationNumber == "" {
		return nil, e.ErrConflict
	}
	for _, id := range a.Intent.FileIDs {
		if a.MediaIDs[id] == "" {
			return nil, e.ErrConflict
		}
	}
	secrets := map[string]string{}
	for _, v := range []string{d.Legal.Name, d.Legal.Number, a.CompanyName, d.AccountNumber, d.ContactMobile} {
		secret, err := encrypt(v)
		if err != nil {
			return nil, e.ErrUnavailable
		}
		secrets[v] = secret
	}
	body := gopay.BodyMap{"out_request_no": a.Intent.OutRequestNo, "organization_type": "2", "business_license_info": gopay.BodyMap{"business_license_copy": a.MediaIDs[d.LicenseFileID], "business_license_number": a.RegistrationNumber, "merchant_name": a.CompanyName, "legal_person": d.Legal.Name}, "id_doc_type": d.Legal.Type, "merchant_shortname": d.MerchantShortName,
		"account_info":     gopay.BodyMap{"bank_account_type": "74", "account_bank": d.AccountBank, "account_name": secrets[a.CompanyName], "account_number": secrets[d.AccountNumber]},
		"contact_info":     gopay.BodyMap{"contact_type": "65", "contact_name": secrets[d.Legal.Name], "contact_id_card_number": secrets[d.Legal.Number], "mobile_phone": secrets[d.ContactMobile]},
		"sales_scene_info": gopay.BodyMap{"store_name": d.StoreName, "store_url": d.StoreURL}}
	if d.BankBranchName != "" {
		body["account_info"].(gopay.BodyMap).Set("bank_name", d.BankBranchName)
	}
	if d.Legal.Type == "IDENTIFICATION_TYPE_MAINLAND_IDCARD" {
		body["id_card_info"] = gopay.BodyMap{"id_card_copy": a.MediaIDs[d.Legal.FrontFileID], "id_card_national": a.MediaIDs[d.Legal.BackFileID], "id_card_name": secrets[d.Legal.Name], "id_card_number": secrets[d.Legal.Number], "id_card_valid_time_begin": d.Legal.ValidFrom, "id_card_valid_time": d.Legal.ValidUntil}
	} else {
		body["id_doc_info"] = gopay.BodyMap{"id_doc_copy": a.MediaIDs[d.Legal.FrontFileID], "id_doc_name": secrets[d.Legal.Name], "id_doc_number": secrets[d.Legal.Number], "doc_period_begin": d.Legal.ValidFrom, "doc_period_end": d.Legal.ValidUntil}
		if d.Legal.BackFileID != "" {
			body["id_doc_info"].(gopay.BodyMap).Set("id_doc_copy_back", a.MediaIDs[d.Legal.BackFileID])
		}
	}
	if len(d.Beneficiaries) > 0 {
		list := []gopay.BodyMap{}
		for _, b := range d.Beneficiaries {
			name, err := encrypt(b.Name)
			if err != nil {
				return nil, e.ErrUnavailable
			}
			number, err := encrypt(b.Number)
			if err != nil {
				return nil, e.ErrUnavailable
			}
			address, err := encrypt(b.Address)
			if err != nil {
				return nil, e.ErrUnavailable
			}
			v := gopay.BodyMap{"ubo_id_doc_type": b.Type, "ubo_id_doc_copy": a.MediaIDs[b.FrontFileID], "ubo_id_doc_name": name, "ubo_id_doc_number": number, "ubo_id_doc_address": address, "ubo_id_doc_period_begin": b.ValidFrom, "ubo_id_doc_period_end": b.ValidUntil}
			if b.BackFileID != "" {
				v.Set("ubo_id_doc_copy_back", a.MediaIDs[b.BackFileID])
			}
			list = append(list, v)
		}
		body["ubo_info_list"] = list
	}
	return body, nil
}
func (p *WeChat) SubmitMerchant(ctx context.Context, a e.MerchantAttempt, d e.MerchantDetails) error {
	if !p.NewMerchantApplicationsEnabled() || a.Intent.Profile != p.MerchantProfile() {
		return e.ErrUnavailable
	}
	body, err := merchantBody(a, d, p.client.V3EncryptText)
	if err != nil {
		return err
	}
	r, err := p.client.V3EcommerceApply(ctx, body)
	if err != nil || r == nil || r.Code != 0 || r.Response == nil || r.Response.OutRequestNo != a.Intent.OutRequestNo || r.Response.ApplymentId < 1 {
		return e.ErrUnavailable
	}
	return nil
}
func controlledMerchantURL(raw string) bool {
	if raw == "" {
		return true
	}
	u, err := url.Parse(raw)
	return err == nil && len(raw) <= 2048 && u.Scheme == "https" && u.Host == "pay.weixin.qq.com" && u.User == nil && u.Fragment == "" && strings.HasPrefix(u.Path, "/public/")
}
func (p *WeChat) QueryMerchant(ctx context.Context, a e.MerchantAttempt) (e.MerchantObservation, error) {
	empty := e.MerchantObservation{}
	if a.Intent.Profile != p.MerchantProfile() || a.Intent.OutRequestNo == "" || !a.Dispatched {
		return empty, e.ErrConflict
	}
	r, err := p.client.V3EcommerceApplyStatus(ctx, 0, a.Intent.OutRequestNo)
	if err == nil && r != nil && r.Code == http.StatusNotFound && r.ErrResponse.Code == "RESOURCE_NOT_EXISTS" && p.verifiedError(r.SignInfo, "RESOURCE_NOT_EXISTS") {
		return e.MerchantObservation{Profile: a.Intent.Profile, OutRequestNo: a.Intent.OutRequestNo, State: "NOT_FOUND", VerificationVersion: "wechat-v3:" + p.config.PublicKeyID}, nil
	}
	if err != nil || r == nil || r.Code != 0 || r.Response == nil {
		return empty, e.ErrUnavailable
	}
	v := r.Response
	if v.OutRequestNo != a.Intent.OutRequestNo || v.ApplymentId < 1 || !controlledMerchantURL(v.SignUrl) || !controlledMerchantURL(v.LegalValidationUrl) {
		return empty, e.ErrConflict
	}
	o := e.MerchantObservation{Profile: a.Intent.Profile, OutRequestNo: v.OutRequestNo, ChannelApplicationID: strconv.FormatInt(v.ApplymentId, 10), State: v.ApplymentState, SignState: v.SignState, MerchantID: v.SubMchid, SignURL: v.SignUrl, LegalValidationURL: v.LegalValidationUrl, VerificationVersion: "wechat-v3:" + p.config.PublicKeyID}
	switch o.State {
	case "CHECKING", "ACCOUNT_NEED_VERIFY", "AUDITING", "REJECTED", "NEED_SIGN", "FINISH", "FROZEN", "CANCELED":
	default:
		return empty, e.ErrConflict
	}
	reasons := []string{}
	for _, reason := range v.AuditDetail {
		if reason != nil && len(reason.RejectReason) > 0 && len(reason.RejectReason) <= 1000 && len(reasons) < 20 {
			reasons = append(reasons, reason.RejectReason)
		}
	}
	o.Reason = strings.Join(reasons, "；")
	if o.State == "ACCOUNT_NEED_VERIFY" && v.AccountValidation.AccountName != "" {
		bank := v.AccountValidation
		name, err := p.client.V3DecryptText(bank.AccountName)
		if err != nil {
			return empty, e.ErrConflict
		}
		number := ""
		if bank.AccountNo != "" {
			number, err = p.client.V3DecryptText(bank.AccountNo)
			if err != nil {
				return empty, e.ErrConflict
			}
		}
		if bank.PayAmount < 1 || len(name) > 256 || len(number) > 128 || len(bank.DestinationAccountNumber) > 128 || len(bank.DestinationAccountName) > 256 || len(bank.DestinationAccountBank) > 256 || len(bank.Remark) > 512 || len(bank.Deadline) > 128 || len(bank.City) > 256 {
			return empty, e.ErrConflict
		}
		o.Bank = &e.BankValidation{AccountName: name, AccountNumber: number, PayAmountMinor: int64(bank.PayAmount), DestinationNumber: bank.DestinationAccountNumber, DestinationName: bank.DestinationAccountName, DestinationBank: bank.DestinationAccountBank, City: bank.City, Remark: bank.Remark, Deadline: bank.Deadline}
	}
	return o, nil
}
