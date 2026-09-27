package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/zitadelsms"
	kernelmodule "task-processor/internal/kernel/module"
)

const zitadelSMSPath = "/api/v1/identity/notifications/sms"

type zitadelSMSModule struct{ handler zitadelSMSHandler }

func (zitadelSMSModule) Name() string { return "zitadel-sms" }
func (zitadelSMSModule) Enabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Workbench.Enabled
}
func (m zitadelSMSModule) Register(reg *kernelmodule.Registry) error {
	reg.AddRoutes(httproute.Descriptor{
		Method: http.MethodPost, Path: zitadelSMSPath, Module: m.Name(),
		AuthPolicy: httproute.AuthPolicyPublic, OrganizationAccessPolicy: httproute.OrganizationAccessPolicyNone,
		RequestTimeout: 30 * time.Second, Handler: m.handler.Deliver,
	})
	return nil
}

// The provider authenticates its bounded raw request with HMAC. No bearer
// identity, organization, code store or application retry loop is involved.
func buildZitadelSMSModule(ctx context.Context) (kernelmodule.Module, error) {
	m := zitadelSMSModule{}
	path := strings.TrimSpace(os.Getenv("ZITADEL_SMS_CONFIG_FILE"))
	if path == "" {
		return m, nil
	}
	invalid := errors.New("ZITADEL SMS private configuration invalid")
	if config.VerifyPrivateFiles(ctx, []string{path}) != nil {
		return nil, invalid
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, invalid
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 8193))
	if err != nil || len(raw) > 8192 {
		return nil, invalid
	}
	var values struct {
		SigningKey, TencentSecretID, TencentSecretKey, TencentAppID, TencentSignName, TencentTemplateID string
		PhoneVerificationExpiryMinutes                                                                  int
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	var extra any
	if d.Decode(&values) != nil || d.Decode(&extra) != io.EOF {
		return nil, invalid
	}
	sender, err := zitadelsms.NewTencentSender(values.TencentSecretID, values.TencentSecretKey)
	if err != nil {
		return nil, invalid
	}
	m.handler.Service, err = zitadelsms.NewService(zitadelsms.Config{
		SigningKey: values.SigningKey, AppID: values.TencentAppID, SignName: values.TencentSignName,
		TemplateID: values.TencentTemplateID, PhoneVerificationExpiryMinutes: values.PhoneVerificationExpiryMinutes,
	}, sender)
	if err != nil {
		return nil, invalid
	}
	return m, nil
}
