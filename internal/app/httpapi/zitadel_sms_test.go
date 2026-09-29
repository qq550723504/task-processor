package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
	"task-processor/internal/authz"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/integration/zitadelsms"
	kernelmodule "task-processor/internal/kernel/module"
)

func TestCurrentApplicationSMSDefaultWiring(t *testing.T) {
	t.Setenv("ZITADEL_SMS_CONFIG_FILE", "")
	for _, tc := range []struct {
		name, config string
		status       int
		startupError bool
	}{
		{name: "unconfigured", status: http.StatusServiceUnavailable},
		{name: "configured-unsigned", config: validSMSPrivateConfig, status: http.StatusUnauthorized},
		{name: "invalid-fails-startup", config: "{}", startupError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := ""
			if tc.config != "" {
				path = writeSMSPrivateConfig(t, tc.config)
			}
			t.Setenv("ZITADEL_SMS_CONFIG_FILE", path)
			server, err := buildSMSApplication(t)
			if tc.startupError {
				if err == nil || server != nil {
					t.Fatal("invalid SMS configuration admitted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			server.Handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/identity/notifications/sms", strings.NewReader("{}")))
			if response.Code != tc.status {
				t.Fatalf("SMS response got %d want %d (missing route is 404)", response.Code, tc.status)
			}
			legacy := httptest.NewRecorder()
			server.Handler.ServeHTTP(legacy, httptest.NewRequest(http.MethodPost, "/api/v1/listing-kits/integrations/zitadel/sms", strings.NewReader("{}")))
			if legacy.Code != http.StatusNotFound {
				t.Fatal("current application revived old SMS route")
			}
		})
	}
}

func buildSMSApplication(t *testing.T) (*http.Server, error) {
	t.Helper()
	deps := newRouteAuthDependencies()
	factories := currentApplicationFactories{
		buildWorkbench: func(*config.Config, *logrus.Logger) (workbenchContextBuildResult, error) {
			return workbenchContextBuildResult{module: currentApplicationTestModule{name: "base", routes: currentWorkbenchApplicationRoutes}, authDependencies: &deps}, nil
		},
		buildSourceAccount: func(*gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
		buildCommercial:    func(*gorm.DB, *gorm.DB, *authz.ListingKitAuthorizer) (kernelmodule.Module, error) { return nil, nil },
	}
	return buildCurrentApplication(context.Background(), &gorm.DB{}, currentApplicationTestConfig(), logrus.New(), factories)
}

const validSMSPrivateConfig = `{"SigningKey":"test-signing-key","TencentSecretID":"fake-id","TencentSecretKey":"fake-key","TencentAppID":"fake-app","TencentSignName":"test-sign","TencentTemplateID":"test-template","PhoneVerificationExpiryMinutes":5}`

func writeSMSPrivateConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sms.json")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; $p=[Console]::In.ReadToEnd(); $acl=[System.IO.File]::GetAccessControl($p); $acl.SetAccessRuleProtection($true,$false); $sid=[System.Security.Principal.WindowsIdentity]::GetCurrent().User; $rule=[System.Security.AccessControl.FileSystemAccessRule]::new($sid,'FullControl','Allow'); $acl.SetAccessRule($rule); [System.IO.File]::SetAccessControl($p,$acl)`)
		command.Stdin = strings.NewReader(path)
		if out, err := command.CombinedOutput(); err != nil {
			t.Fatalf("protect test fixture: %v: %s", err, out)
		}
	}
	return path
}

func TestZitadelSMSPrivateConfig(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{
		{"valid", validSMSPrivateConfig, true},
		{"no-template-expiry", strings.Replace(validSMSPrivateConfig, `,"PhoneVerificationExpiryMinutes":5`, "", 1), true},
		{"unknown", strings.Replace(validSMSPrivateConfig, `"SigningKey"`, `"Unknown"`, 1), false},
		{"trailing", validSMSPrivateConfig + " {}", false},
		{"bom", "\xef\xbb\xbf" + validSMSPrivateConfig, false},
		{"oversize", validSMSPrivateConfig + strings.Repeat(" ", 8193), false},
		{"invalid-expiry", strings.Replace(validSMSPrivateConfig, ":5}", ":61}", 1), false},
		{"missing-signing-key", strings.Replace(validSMSPrivateConfig, "test-signing-key", "", 1), false},
		{"missing-credential", strings.Replace(validSMSPrivateConfig, "fake-key", "", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ZITADEL_SMS_CONFIG_FILE", writeSMSPrivateConfig(t, tc.body))
			m, err := buildZitadelSMSModule(context.Background())
			if tc.valid {
				if err != nil || m == nil {
					t.Fatalf("valid private config rejected: %v", err)
				}
				return
			}
			if m != nil || err == nil || err.Error() != "ZITADEL SMS private configuration invalid" {
				t.Fatal("invalid config accepted or error exposed configuration")
			}
		})
	}
	t.Run("relative", func(t *testing.T) {
		t.Setenv("ZITADEL_SMS_CONFIG_FILE", "sms.json")
		if _, err := buildZitadelSMSModule(context.Background()); err == nil {
			t.Fatal("relative path admitted")
		}
	})
	t.Run("cancelled", func(t *testing.T) {
		t.Setenv("ZITADEL_SMS_CONFIG_FILE", writeSMSPrivateConfig(t, validSMSPrivateConfig))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := buildZitadelSMSModule(ctx); err == nil {
			t.Fatal("cancelled startup admitted")
		}
	})
	t.Run("public-read", func(t *testing.T) {
		path := writeSMSPrivateConfig(t, validSMSPrivateConfig)
		if runtime.GOOS == "windows" {
			if err := exec.Command("icacls", path, "/grant", "*S-1-1-0:(R)").Run(); err != nil {
				t.Fatal(err)
			}
		} else if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("ZITADEL_SMS_CONFIG_FILE", path)
		if _, err := buildZitadelSMSModule(context.Background()); err == nil {
			t.Fatal("public config admitted")
		}
	})
}

type smsApplicationSender struct {
	calls int
	err   error
}

func (s *smsApplicationSender) Send(context.Context, zitadelsms.Message) error {
	s.calls++
	return s.err
}

func TestZitadelSMSCurrentHTTPDelivery(t *testing.T) {
	const body = `{"contextInfo":{"recipientPhoneNumber":"+8613800138000","eventType":"user.human.phone.code.added"},"args":{"code":"123456"}}`
	for _, tc := range []struct {
		name, body    string
		signed        bool
		providerErr   error
		status, calls int
	}{
		{"signed", body, true, nil, 204, 1},
		{"unsigned", body, false, nil, 401, 0},
		{"unsupported-event", strings.Replace(body, "user.human.phone.code.added", "session.otp.sms.checked", 1), true, nil, 400, 0},
		{"oversize", strings.Repeat("x", 64*1024+1), true, nil, 413, 0},
		{"provider-failure", body, true, fmt.Errorf("private provider response"), 502, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sender := &smsApplicationSender{err: tc.providerErr}
			service, err := zitadelsms.NewService(zitadelsms.Config{SigningKey: "test-key", TemplateID: "template", SignName: "sign", AppID: "app"}, sender)
			if err != nil {
				t.Fatal(err)
			}
			registry := kernelmodule.NewRegistry()
			if err := (zitadelSMSModule{handler: zitadelSMSHandler{Service: service}}).Register(registry); err != nil {
				t.Fatal(err)
			}
			server := buildCurrentApplicationHTTPServer(registry.Routes(), newRouteAuthDependencies())
			req := httptest.NewRequest(http.MethodPost, zitadelSMSPath, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer irrelevant")
			if tc.signed {
				timestamp := time.Now().Unix()
				mac := hmac.New(sha256.New, []byte("test-key"))
				fmt.Fprintf(mac, "%d.%s", timestamp, tc.body)
				req.Header.Set("ZITADEL-Signature", fmt.Sprintf("t=%d,v1=%x", timestamp, mac.Sum(nil)))
			}
			response := httptest.NewRecorder()
			server.Handler.ServeHTTP(response, req)
			if response.Code != tc.status || sender.calls != tc.calls {
				t.Fatalf("got HTTP %d sends %d, want %d sends %d", response.Code, sender.calls, tc.status, tc.calls)
			}
			if response.Body.Len() != 0 {
				t.Fatal("SMS response exposed payload or provider detail")
			}
		})
	}
}

func TestZitadelSMSRouteAdmission(t *testing.T) {
	registry := kernelmodule.NewRegistry()
	if err := (zitadelSMSModule{}).Register(registry); err != nil {
		t.Fatal(err)
	}
	base := []httproute.Descriptor{}
	for _, r := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	routes := append(base, registry.Routes()...)
	validate := func(rs []httproute.Descriptor, enabled bool) error {
		return validateCurrentApplicationRoutesInternal(rs, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{ZitadelSMS: enabled})
	}
	if err := validate(routes, true); err != nil {
		t.Fatal(err)
	}
	if validate(routes, false) == nil || validate(base, true) == nil {
		t.Fatal("missing or unrequested SMS route admitted")
	}
	for _, mutate := range []func(*httproute.Descriptor){
		func(r *httproute.Descriptor) { r.Module = "wrong" },
		func(r *httproute.Descriptor) { r.Method = http.MethodGet },
		func(r *httproute.Descriptor) { r.AuthPolicy = httproute.AuthPolicyCurrentIdentity },
		func(r *httproute.Descriptor) {
			r.OrganizationAccessPolicy = httproute.OrganizationAccessPolicyLiveWrite
		},
		func(r *httproute.Descriptor) { r.Permission = "wrong" },
		func(r *httproute.Descriptor) { r.OrganizationTargetResolver = accountOrganizationTarget },
		func(r *httproute.Descriptor) { r.RequestTimeout = 0 },
		func(r *httproute.Descriptor) { r.Handler = nil },
	} {
		changed := append([]httproute.Descriptor(nil), routes...)
		mutate(&changed[len(changed)-1])
		if validate(changed, true) == nil {
			t.Fatal("SMS descriptor drift admitted")
		}
	}
}
