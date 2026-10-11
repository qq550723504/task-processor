package currentapplication

import (
	"encoding/base64"
	"errors"
	"gorm.io/gorm"
	"net"
	"net/url"
	"strings"
	b "task-processor/internal/commercial/billing"
	e "task-processor/internal/ecoservices"
	"task-processor/internal/integration/servicepayments"
)

type EcoservicesPaymentsConfig struct {
	NewMerchantApplications bool                     `json:"newMerchantApplications"`
	Profile                 b.ServiceMerchantProfile `json:"profile"`
	NewPayments             bool                     `json:"newPayments"`
	ProductQualified        bool                     `json:"productQualified"`
	PlatformPaysFees        bool                     `json:"platformPaysFees"`
	PrivateKey              string                   `json:"privateKey"`
	SerialNumber            string                   `json:"serialNumber"`
	APIv3Key                string                   `json:"apiV3Key"`
	PublicKeyID             string                   `json:"publicKeyId"`
	PublicKey               string                   `json:"publicKey"`
	NotifyURL               string                   `json:"notifyUrl"`
}

func (p EcoservicesPaymentsConfig) ChannelConfig() servicepayments.WeChatConfig {
	return servicepayments.WeChatConfig{NewMerchantApplications: p.NewMerchantApplications, Profile: p.Profile, NewPayments: p.NewPayments, ProductQualified: p.ProductQualified, PlatformPaysFees: p.PlatformPaysFees, PrivateKey: p.PrivateKey, SerialNumber: p.SerialNumber, APIv3Key: p.APIv3Key, PublicKeyID: p.PublicKeyID, PublicKey: p.PublicKey, NotifyURL: p.NotifyURL}
}

type EcoservicesConfig struct {
	Enabled        bool                      `json:"enabled"`
	NonPaymentOnly bool                      `json:"nonPaymentOnly"`
	Database       DatabaseConfig            `json:"database"`
	Storage        KnowledgeStorageConfig    `json:"storage"`
	Payments       EcoservicesPaymentsConfig `json:"payments"`
	PayloadKey     string                    `json:"payloadKey"`
}
type EcoservicesRuntime struct {
	NonPaymentOnly     bool
	DB                 *gorm.DB
	Objects            e.PrivateObjectStore
	Channel            b.ServicePurchaseProvider
	Protection         b.ServicePayloadProtection
	MerchantProtection e.MerchantProtection
}

func (c *Config) validateEcoservices() error {
	if c.Ecoservices == nil || !c.Ecoservices.Enabled {
		return nil
	}
	v := c.Ecoservices
	if err := v.Database.validate("ecoservices.database"); err != nil {
		return err
	}
	if v.Database.Database != "ecoservices" || v.Database.User != "ecoservices_runtime" || v.Database.MaxConnections > 4 || v.Database.Host != c.SourceAccountDatabase.Host || v.Database.Port != c.SourceAccountDatabase.Port {
		return errors.New("ecoservices requires a dedicated owner database and canonical financial owners")
	}
	p := v.Payments
	if v.NonPaymentOnly {
		if p != (EcoservicesPaymentsConfig{}) || v.PayloadKey != "" {
			return errors.New("ecoservices qualification mode cannot consume retained financial configuration")
		}
	} else {
		if c.CommercialOwnerDatabase == nil || c.MoneyOwnerDatabase == nil {
			return errors.New("ecoservices requires canonical financial owners")
		}
		if p.Profile.Validate() != nil || p.Profile.Environment != "PRODUCTION" || len(p.APIv3Key) != 32 || p.PrivateKey == "" || p.PublicKey == "" || p.SerialNumber == "" || !strings.HasPrefix(p.PublicKeyID, "PUB_KEY_ID_") {
			return errors.New("ecoservices requires its original explicit WeChat merchant profile")
		}
		notify, err := url.Parse(p.NotifyURL)
		if err != nil || notify.Scheme != "https" || notify.Host == "" || notify.User != nil || notify.RawQuery != "" || notify.Fragment != "" || notify.Path != "/api/v1/payments/ecoservices/wechat/notify" {
			return errors.New("ecoservices requires its fixed HTTPS payment notification ingress")
		}
		if p.NewPayments && (!p.ProductQualified || !p.PlatformPaysFees || c.Identity.TenantDirectoryToken == "") {
			return errors.New("ecoservices new payments require qualified platform fee product and live purchase authorization")
		}
		if p.NewMerchantApplications && (!p.ProductQualified || c.Identity.TenantDirectoryToken == "") {
			return errors.New("ecoservices merchant applications require qualified product and live application authorization")
		}
		key, err := base64.StdEncoding.DecodeString(v.PayloadKey)
		if err != nil || len(key) != 32 {
			return errors.New("ecoservices private payload encryption key is required")
		}
	}
	s := v.Storage
	if s.Region == "" || s.Bucket == "" || s.AccessKeyID == "" || s.SecretAccessKey == "" || s.Mode != "aws" && s.Mode != "cos" || s.Mode == "cos" && !s.COSImmutableNonVersionedBucketPolicy {
		return errors.New("ecoservices requires private immutable object storage")
	}
	if s.Endpoint != "" {
		u, err := url.Parse(s.Endpoint)
		if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Scheme != "https" && u.Scheme != "http" {
			return errors.New("ecoservices private object endpoint is invalid")
		}
		ip := net.ParseIP(u.Hostname())
		if u.Scheme == "http" && u.Hostname() != "ecoservices-objects" && u.Hostname() != "localhost" && (ip == nil || !ip.IsPrivate() && !ip.IsLoopback()) {
			return errors.New("ecoservices external object endpoint requires HTTPS")
		}
	}
	return nil
}
