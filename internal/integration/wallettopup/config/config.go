package config

import "github.com/spf13/viper"

// Secrets are file references, never inline YAML values or browser configuration.
type Config struct {
	MinMinor             int64    `json:"minMinor"`
	MaxMinor             int64    `json:"maxMinor"`
	QuickAmounts         []string `json:"quickAmounts"`
	PaymentWindowSeconds int      `json:"paymentWindowSeconds"`
	PayloadKeyFile       string   `json:"payloadKeyFile"`
	Alipay               Channel  `json:"alipay"`
	WeChat               Channel  `json:"wechat"`
}
type Channel struct {
	Enabled        bool   `json:"enabled"`
	NewPayments    bool   `json:"newPayments"`
	Environment    string `json:"environment"`
	ProfileVersion string `json:"profileVersion"`
	MerchantID     string `json:"merchantID"`
	AppID          string `json:"appID"`
	PrivateKeyFile string `json:"privateKeyFile"`
	PublicKeyFile  string `json:"publicKeyFile"`
	APIv3KeyFile   string `json:"apiV3KeyFile"`
	PublicKeyID    string `json:"publicKeyID"`
	SerialNumber   string `json:"serialNumber"`
	NotifyURL      string `json:"notifyURL"`
	ReturnURL      string `json:"returnURL"`
}

func Load(v *viper.Viper) Config {
	channel := func(name string) Channel {
		p := "walletTopUp." + name + "."
		return Channel{Enabled: v.GetBool(p + "enabled"), NewPayments: v.GetBool(p + "newPayments"), Environment: v.GetString(p + "environment"), ProfileVersion: v.GetString(p + "profileVersion"), MerchantID: v.GetString(p + "merchantID"), AppID: v.GetString(p + "appID"), PrivateKeyFile: v.GetString(p + "privateKeyFile"), PublicKeyFile: v.GetString(p + "publicKeyFile"), APIv3KeyFile: v.GetString(p + "apiV3KeyFile"), PublicKeyID: v.GetString(p + "publicKeyID"), SerialNumber: v.GetString(p + "serialNumber"), NotifyURL: v.GetString(p + "notifyURL"), ReturnURL: v.GetString(p + "returnURL")}
	}
	return Config{MinMinor: v.GetInt64("walletTopUp.minMinor"), MaxMinor: v.GetInt64("walletTopUp.maxMinor"), QuickAmounts: v.GetStringSlice("walletTopUp.quickAmounts"), PaymentWindowSeconds: v.GetInt("walletTopUp.paymentWindowSeconds"), PayloadKeyFile: v.GetString("walletTopUp.payloadKeyFile"), Alipay: channel("alipay"), WeChat: channel("wechat")}
}
