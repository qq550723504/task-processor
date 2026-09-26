// Package tencentesign adapts the official enterprise self-built application API.
package tencentesign

import (
	"context"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	ess "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ess/v20201111"
	domain "task-processor/internal/subjectverification"
	"time"
)

type sdk interface {
	CreateOrganizationAuthUrlWithContext(context.Context, *ess.CreateOrganizationAuthUrlRequest) (*ess.CreateOrganizationAuthUrlResponse, error)
}
type Client struct {
	sdk      sdk
	operator string
}

func NewClient(secretID, secretKey, operator string) (*Client, error) {
	p := profile.NewClientProfile()
	p.Debug = false
	p.DisableRegionBreaker = true
	p.NetworkFailureMaxRetries = 0
	p.RateLimitExceededMaxRetries = 0
	p.UnsafeRetryOnConnectionFailure = false
	p.HttpProfile.ReqTimeout = 8
	client, err := ess.NewClient(common.NewCredential(secretID, secretKey), "", p)
	if err != nil {
		return nil, domain.ErrUnavailable
	}
	return &Client{sdk: client, operator: operator}, nil
}
func (c *Client) Create(ctx context.Context, in domain.CreateRequest) (domain.Link, error) {
	r := ess.NewCreateOrganizationAuthUrlRequest()
	r.Operator = &ess.UserInfo{UserId: &c.operator}
	r.OrganizationName = &in.CompanyName
	r.UniformSocialCreditCode = &in.CreditCode
	r.AdminMobile = &in.Phone
	r.UserData = &in.Correlation
	r.OrganizationNameSame = common.BoolPtr(true)
	r.UniformSocialCreditCodeSame = common.BoolPtr(true)
	r.AdminMobileSame = common.BoolPtr(true)
	r.AuthorizationTypes = []*uint64{common.Uint64Ptr(2), common.Uint64Ptr(5)}
	r.Endpoint = common.StringPtr("PC")
	r.OrganizationIdCardType = common.StringPtr("USCC")
	r.OrganizationIdCardTypeSame = common.BoolPtr(true)
	if in.LegalName != "" {
		r.LegalName = &in.LegalName
		r.LegalNameSame = common.BoolPtr(true)
	}
	response, err := c.sdk.CreateOrganizationAuthUrlWithContext(ctx, r)
	if err != nil || response == nil || response.Response == nil || response.Response.AuthUrl == nil || response.Response.ExpiredTime == nil {
		return domain.Link{}, domain.ErrUnavailable
	}
	return domain.Link{URL: *response.Response.AuthUrl, ExpiresAt: time.Unix(*response.Response.ExpiredTime, 0).UTC()}, nil
}
