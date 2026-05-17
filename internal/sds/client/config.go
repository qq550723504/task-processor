package client

import (
	"strings"
	"time"

	coreconfig "task-processor/internal/core/config"
)

// EndpointSet 定义 SDS 相关接口路径。
// 这些值先保持可配置，避免在没有抓包前把路径写死到代码里。
type EndpointSet struct {
	TemplateListPath         string
	TemplateGroupsPath       string
	TemplateDetailPath       string
	TemplateCyclePath        string
	TemplateRecommend        string
	LoginPath                string
	UploadSignPath           string
	MaterialCreatePath       string
	MaterialFindByIDs        string
	DesignProductPath        string
	PrototypeGroupPath       string
	ResultGroupPath          string
	CutFileContentPath       string
	SyncDesignPath           string
	AddAndDesignPath         string
	DesignProductsPath       string
	DesignProductsUpdatePath string
	DesignUploadPath         string
	PreviewCreatePath        string
	ProductDraftPath         string
}

// Config SDS 请求客户端配置。
type Config struct {
	BaseURL       string
	Timeout       time.Duration
	RetryCount    int
	RetryInterval time.Duration
	ProxyURL      string
	UserAgent     string
	Referer       string
	CookieFile    string
	AuthFile      string
	Management    *coreconfig.ManagementConfig
	AuthBootstrap AuthBootstrapConfig
	Endpoints     EndpointSet
}

// AuthBootstrapConfig 定义 SDS 登录态自动引导来源。
type AuthBootstrapConfig struct {
	StaticAccessToken string
	StaticOutToken    string
	StaticMerchantID  int64
	StaticCookie      string

	LoginServiceBaseURL    string
	LoginServiceSharedKey  string
	LoginServiceTenantID   string
	LoginServiceIdentifier string

	LoginUsername           string
	LoginPassword           string
	LoginMerchantName       string
	LoginDomainName         string
	LoginVerifyCaptchaParam string
	LoginExtraInfo          string

	ManagementStoreID int64
}

func (c AuthBootstrapConfig) HasSource() bool {
	hasLoginService := strings.TrimSpace(c.LoginServiceTenantID) != "" &&
		strings.TrimSpace(c.LoginServiceIdentifier) != "" &&
		(strings.TrimSpace(c.LoginServiceBaseURL) != "" || loadLocalLoginProvider() != nil)
	return strings.TrimSpace(c.StaticAccessToken) != "" ||
		strings.TrimSpace(c.StaticCookie) != "" ||
		hasLoginService ||
		strings.TrimSpace(c.LoginUsername) != "" ||
		strings.TrimSpace(c.LoginPassword) != "" ||
		c.ManagementStoreID > 0
}

// DefaultConfig 返回默认配置。
func DefaultConfig() *Config {
	return &Config{
		BaseURL:       "https://mapi.sdspod.com",
		Timeout:       45 * time.Second,
		RetryCount:    2,
		RetryInterval: 1500 * time.Millisecond,
		UserAgent:     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36",
		Referer:       "https://www.sdsdiy.com/portal/search?sideActiveId=overseas&isOverseas=overseas",
		CookieFile:    "data/sds/session_cookies.json",
		AuthFile:      "data/sds/auth_state.json",
		AuthBootstrap: AuthBootstrapConfig{},
		Endpoints: EndpointSet{
			TemplateListPath:         "/products/page",
			TemplateGroupsPath:       "/products/pageOptionGroup",
			TemplateDetailPath:       "/products/%s",
			TemplateCyclePath:        "/products/%s/cycle",
			TemplateRecommend:        "/products/%s/recommend",
			LoginPath:                "/login",
			UploadSignPath:           "/ps/image/get_post_signature_to_image_for_oss_upload",
			MaterialCreatePath:       "/materials/one",
			MaterialFindByIDs:        "/materials/findByIds",
			DesignProductPath:        "/ps/design/products/%d",
			PrototypeGroupPath:       "/merchant_product_parents/%d/prototypeGroup",
			ResultGroupPath:          "/merchant/product/resultGroup/select",
			CutFileContentPath:       "/cut/filecode/content",
			SyncDesignPath:           "/ps/design/syncDesign",
			AddAndDesignPath:         "/ps/design/add_and_design",
			DesignProductsPath:       "https://mapi2.sdspod.com/design_products",
			DesignProductsUpdatePath: "https://mapi.sdspod.com/design_products",
			DesignUploadPath:         "",
		},
	}
}
