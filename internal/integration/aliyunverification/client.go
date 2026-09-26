// Package aliyunverification implements personal verification with the official SDK.
package aliyunverification

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	sdk "github.com/alibabacloud-go/cloudauth-20190307/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	"github.com/alibabacloud-go/tea/dara"
	domain "task-processor/internal/subjectverification"
)

type Client struct {
	key, secret, returnURL string
	http                   *http.Client
}

func NewClient(key, secret, returnURL string) (*Client, error) {
	// The official SDK's DEBUG logging can include signed requests. Never enable
	// this capability in a process with that global debug switch configured.
	if strings.TrimSpace(key) == "" || strings.TrimSpace(secret) == "" || !domain.ValidPersonalURL(returnURL) || os.Getenv("DEBUG") != "" {
		return nil, domain.ErrUnavailable
	}
	return &Client{key: key, secret: secret, returnURL: returnURL, http: &http.Client{Timeout: 7 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

type contextHTTP struct {
	ctx    context.Context
	client *http.Client
}
type boundedBody struct {
	io.Reader
	io.Closer
}

func (h contextHTTP) Call(req *http.Request, _ *http.Transport) (*http.Response, error) {
	response, err := h.client.Do(req.WithContext(h.ctx))
	if err != nil {
		return nil, err
	}
	response.Body = boundedBody{io.LimitReader(response.Body, 65536), response.Body}
	return response, nil
}
func (c *Client) sdk(ctx context.Context) (*sdk.Client, error) {
	// One cheap SDK client per operation prevents concurrent requests from sharing
	// a mutable caller context. Connections are still pooled by the HTTP transport.
	return sdk.NewClient(&openapi.Config{AccessKeyId: dara.String(c.key), AccessKeySecret: dara.String(c.secret), Endpoint: dara.String("cloudauth.aliyuncs.com"), RegionId: dara.String("cn-shanghai"), Protocol: dara.String("https"), HttpClient: contextHTTP{ctx, c.http}})
}
func runtime() *dara.RuntimeOptions {
	return &dara.RuntimeOptions{Autoretry: dara.Bool(false), MaxAttempts: dara.Int(1), ConnectTimeout: dara.Int(7000), ReadTimeout: dara.Int(7000)}
}
func (c *Client) VerifyPhone(ctx context.Context, in domain.PersonalIdentity) (bool, error) {
	client, err := c.sdk(ctx)
	if err != nil {
		return false, domain.ErrUnavailable
	}
	r, err := client.Mobile3MetaSimpleVerifyWithOptions(&sdk.Mobile3MetaSimpleVerifyRequest{ParamType: dara.String("normal"), UserName: dara.String(in.Name), IdentifyNum: dara.String(in.IDNumber), Mobile: dara.String(in.Phone)}, runtime())
	if err != nil || r == nil || r.Body == nil || dara.StringValue(r.Body.Code) != "200" || r.Body.ResultObject == nil {
		return false, domain.ErrUnavailable
	}
	switch dara.StringValue(r.Body.ResultObject.BizCode) {
	case "1":
		return true, nil
	case "2", "3":
		return false, nil
	default:
		return false, domain.ErrUnavailable
	}
}
func (c *Client) CreateFace(ctx context.Context, id string, scene int64, in domain.PersonalIdentity, meta string) (domain.PersonalLink, error) {
	client, err := c.sdk(ctx)
	if err != nil {
		return domain.PersonalLink{}, domain.ErrUnavailable
	}
	request := &sdk.InitFaceVerifyRequest{SceneId: dara.Int64(scene), OuterOrderNo: dara.String(strings.ReplaceAll(id, "-", "")), ProductCode: dara.String("ID_PRO"), Model: dara.String("LIVENESS"), CertType: dara.String("IDENTITY_CARD"), CertName: dara.String(in.Name), CertNo: dara.String(in.IDNumber), ReturnUrl: dara.String(c.returnURL), MetaInfo: dara.String(meta), RarelyCharacters: dara.String("N"), ProcedurePriority: dara.String("url"), VideoEvidence: dara.String("false")}
	// The generated convenience method puts CertName/CertNo/MetaInfo in Query.
	// Use the official OpenAPI pipeline with POST form parameters instead, before
	// it signs the request. Never rewrite a signed request in the transport.
	raw, err := client.CallApi(&openapi.Params{Action: dara.String("InitFaceVerify"), Version: dara.String("2019-03-07"), Protocol: dara.String("HTTPS"), Pathname: dara.String("/"), Method: dara.String("POST"), AuthType: dara.String("AK"), Style: dara.String("RPC"), ReqBodyType: dara.String("formData"), BodyType: dara.String("json")}, &openapi.OpenApiRequest{Body: openapi.ParseToMap(request)}, runtime())
	if err != nil {
		return domain.PersonalLink{}, domain.ErrUnavailable
	}
	r := &sdk.InitFaceVerifyResponse{}
	if dara.Convert(raw, r) != nil || r.Body == nil || dara.StringValue(r.Body.Code) != "200" || r.Body.ResultObject == nil {
		return domain.PersonalLink{}, domain.ErrUnavailable
	}
	return domain.PersonalLink{CertifyID: dara.StringValue(r.Body.ResultObject.CertifyId), URL: dara.StringValue(r.Body.ResultObject.CertifyUrl)}, nil
}
func (c *Client) QueryFace(ctx context.Context, scene int64, id string) (domain.PersonalResult, error) {
	client, err := c.sdk(ctx)
	if err != nil {
		return domain.PersonalResult{}, domain.ErrUnavailable
	}
	r, err := client.DescribeFaceVerifyWithOptions(&sdk.DescribeFaceVerifyRequest{SceneId: dara.Int64(scene), CertifyId: dara.String(id)}, runtime())
	if err != nil || r == nil || r.Body == nil || dara.StringValue(r.Body.Code) != "200" || r.Body.ResultObject == nil {
		return domain.PersonalResult{}, domain.ErrUnavailable
	}
	return domain.PersonalResult{Passed: dara.StringValue(r.Body.ResultObject.Passed), SubCode: dara.StringValue(r.Body.ResultObject.SubCode)}, nil
}
