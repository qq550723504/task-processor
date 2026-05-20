package httpclient

import (
	"crypto/tls"
	"time"

	"github.com/imroc/req/v3"
)

// ClientConfig 定义共享 HTTP/TLS client 的基础配置。
type ClientConfig struct {
	Timeout            time.Duration
	RetryCount         int
	RetryDelay         time.Duration
	RetryInterval      req.GetRetryIntervalFunc
	RetryCondition     req.RetryConditionFunc
	RetryHook          req.RetryHookFunc
	ProxyURL           string
	InsecureSkipVerify bool
	Headers            map[string]string
}

// Build 使用统一的 TLS 和 req client 基础配置创建客户端。
func Build(cfg ClientConfig) *req.Client {
	client := req.C().
		SetTLSFingerprintChrome().
		SetTLSClientConfig(buildTLSConfig(cfg.InsecureSkipVerify))

	if cfg.Timeout > 0 {
		client = client.SetTimeout(cfg.Timeout)
	}
	if len(cfg.Headers) > 0 {
		client = client.SetCommonHeaders(cfg.Headers)
	}
	if cfg.RetryCount > 0 {
		client = client.SetCommonRetryCount(cfg.RetryCount)
	}
	if cfg.RetryInterval != nil {
		client = client.SetCommonRetryInterval(cfg.RetryInterval)
	} else if cfg.RetryDelay > 0 {
		client = client.SetCommonRetryFixedInterval(cfg.RetryDelay)
	}
	if cfg.RetryCondition != nil {
		client = client.SetCommonRetryCondition(cfg.RetryCondition)
	}
	if cfg.RetryHook != nil {
		client = client.SetCommonRetryHook(cfg.RetryHook)
	}
	if cfg.ProxyURL != "" {
		client = client.SetProxyURL(cfg.ProxyURL)
	}

	return client
}

func buildTLSConfig(insecureSkipVerify bool) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: insecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_AES_128_GCM_SHA256,
			tls.TLS_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
	}
}
