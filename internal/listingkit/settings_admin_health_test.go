package listingkit

import (
	"context"
	"testing"
)

func TestSettingsAdminServiceReturnsConfiguredHealthProbes(t *testing.T) {
	t.Parallel()

	svc := newSettingsAdminService(settingsAdminServiceConfig{
		settingsHealthProbes: SettingsHealthProbes{
			SheinIntegration: SettingsHealthProbe{Configured: true},
			SDSLogin:         SettingsHealthProbe{Missing: []string{"sds.loginService.identifier 缺失"}},
			ObjectStorage:    SettingsHealthProbe{Configured: true},
		},
	})

	probes := svc.GetSettingsHealthProbes(context.Background())
	if !probes.SheinIntegration.Configured {
		t.Fatalf("shein integration probe = %+v, want configured", probes.SheinIntegration)
	}
	if len(probes.SDSLogin.Missing) != 1 || probes.SDSLogin.Missing[0] != "sds.loginService.identifier 缺失" {
		t.Fatalf("sds probe = %+v", probes.SDSLogin)
	}
	if !probes.ObjectStorage.Configured {
		t.Fatalf("object storage probe = %+v, want configured", probes.ObjectStorage)
	}
}

func TestServiceReturnsConfiguredHealthProbes(t *testing.T) {
	t.Parallel()

	want := SettingsHealthProbes{
		SheinIntegration: SettingsHealthProbe{Configured: true},
		SDSLogin:         SettingsHealthProbe{Missing: []string{"sds.loginService.identifier 缺失"}},
		ObjectStorage:    SettingsHealthProbe{Configured: true},
	}
	svc := newServiceBase(&ServiceConfig{Health: want}, SheinSettings{})

	got := svc.GetSettingsHealthProbes(context.Background())
	if !got.SheinIntegration.Configured {
		t.Fatalf("shein integration probe = %+v, want configured", got.SheinIntegration)
	}
	if len(got.SDSLogin.Missing) != 1 || got.SDSLogin.Missing[0] != "sds.loginService.identifier 缺失" {
		t.Fatalf("sds probe = %+v", got.SDSLogin)
	}
	if !got.ObjectStorage.Configured {
		t.Fatalf("object storage probe = %+v, want configured", got.ObjectStorage)
	}
}
