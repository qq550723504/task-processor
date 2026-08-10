package tests

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

const listingKitSharedSecret = "listingkit-workbench-secret"

type workloadManifest struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name         string `yaml:"name"`
		GenerateName string `yaml:"generateName"`
	} `yaml:"metadata"`
	Spec struct {
		Template podTemplate `yaml:"template"`
	} `yaml:"spec"`
}

type podTemplate struct {
	Spec struct {
		Containers []workloadContainer `yaml:"containers"`
	} `yaml:"spec"`
}

type workloadContainer struct {
	Name    string `yaml:"name"`
	EnvFrom []struct {
		SecretRef *struct {
			Name     string `yaml:"name"`
			Optional *bool  `yaml:"optional"`
		} `yaml:"secretRef"`
	} `yaml:"envFrom"`
	Env []struct {
		Name      string `yaml:"name"`
		ValueFrom *struct {
			SecretKeyRef *struct {
				Name     string `yaml:"name"`
				Key      string `yaml:"key"`
				Optional *bool  `yaml:"optional"`
			} `yaml:"secretKeyRef"`
		} `yaml:"valueFrom"`
	} `yaml:"env"`
}

func TestListingKitWorkloadsUseLeastPrivilegeSharedSecretKeys(t *testing.T) {
	want := map[string]map[string]string{
		"base/listingkit-ui-deployment.yaml": {
			"AUTH_SECRET":           "AUTH_SECRET",
			"ZITADEL_ISSUER_URL":    "ZITADEL_ISSUER_URL",
			"ZITADEL_CLIENT_ID":     "ZITADEL_CLIENT_ID",
			"ZITADEL_CLIENT_SECRET": "ZITADEL_CLIENT_SECRET",
			"TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS": "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS",
			"TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USER_IDS":   "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USER_IDS",
			"TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES":      "TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES",
			"LISTINGKIT_DEMO_WEBHOOK_URL":                          "LISTINGKIT_DEMO_WEBHOOK_URL",
		},
		"base/shein-login-worker-deployment.yaml": {
			"TASK_PROCESSOR_DATABASE_HOST":               "TASK_PROCESSOR_DATABASE_HOST",
			"TASK_PROCESSOR_DATABASE_PORT":               "TASK_PROCESSOR_DATABASE_PORT",
			"TASK_PROCESSOR_DATABASE_USER":               "TASK_PROCESSOR_DATABASE_USER",
			"TASK_PROCESSOR_DATABASE_PASSWORD":           "TASK_PROCESSOR_DATABASE_PASSWORD",
			"TASK_PROCESSOR_DATABASE_NAME":               "TASK_PROCESSOR_DATABASE_NAME",
			"TASK_PROCESSOR_SHEIN_COOKIE_REDIS_HOST":     "TASK_PROCESSOR_SHEIN_COOKIE_REDIS_HOST",
			"TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PORT":     "TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PORT",
			"TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PASSWORD": "TASK_PROCESSOR_SHEIN_COOKIE_REDIS_PASSWORD",
			"TASK_PROCESSOR_SHEIN_COOKIE_REDIS_DB":       "TASK_PROCESSOR_SHEIN_COOKIE_REDIS_DB",
		},
		"base/imgproxy-deployment.yaml": {
			"IMGPROXY_KEY":          "IMGPROXY_KEY",
			"IMGPROXY_SALT":         "IMGPROXY_SALT",
			"AWS_ACCESS_KEY_ID":     "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_ACCESSKEYID",
			"AWS_SECRET_ACCESS_KEY": "TASK_PROCESSOR_PRODUCTIMAGE_PUBLISHER_S3_SECRETACCESSKEY",
		},
		"jobs/product-listing-api-schema-migrate-job.yaml": databaseSecretKeys(),
		"jobs/listingkit-schema-migrate-job.yaml":          databaseSecretKeys(),
	}

	for relativePath, expected := range want {
		t.Run(relativePath, func(t *testing.T) {
			container := loadOnlyContainer(t, relativePath)
			for _, source := range container.EnvFrom {
				if source.SecretRef != nil {
					t.Fatalf("%s must not import Secret %s with envFrom", relativePath, source.SecretRef.Name)
				}
			}

			actual := make(map[string]string)
			forbidden := map[string]bool{
				"TASK_PROCESSOR_LISTINGKIT_ZITADEL_TENANT_DIRECTORY_TOKEN":  true,
				"TASK_PROCESSOR_LISTINGKIT_ZITADEL_MEMBER_INVITATION_TOKEN": true,
			}
			for _, variable := range container.Env {
				if variable.ValueFrom == nil || variable.ValueFrom.SecretKeyRef == nil {
					continue
				}
				ref := variable.ValueFrom.SecretKeyRef
				if forbidden[ref.Key] {
					t.Fatalf("%s must not receive forbidden key %s", relativePath, ref.Key)
				}
				if ref.Name != listingKitSharedSecret {
					t.Fatalf("%s references unexpected Secret %s", relativePath, ref.Name)
				}
				actual[variable.Name] = ref.Key
			}
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("%s shared Secret allowlist = %#v, want %#v", relativePath, actual, expected)
			}
		})
	}
}

func TestListingKitAPIAndIdentityPreflightKeepRequiredSharedSecret(t *testing.T) {
	container := loadOnlyContainer(t, "base/product-listing-api-deployment.yaml")
	for _, source := range container.EnvFrom {
		if source.SecretRef == nil || source.SecretRef.Name != listingKitSharedSecret {
			continue
		}
		if source.SecretRef.Optional != nil && *source.SecretRef.Optional {
			t.Fatalf("product API must require %s", listingKitSharedSecret)
		}
		return
	}
	t.Fatalf("product API must import required shared Secret %s", listingKitSharedSecret)
}

func TestListingKitIdentityPreflightUsesExactSharedSecretKeys(t *testing.T) {
	container := loadOnlyContainer(t, "jobs/listingkit-identity-preflight-job.yaml")
	for _, source := range container.EnvFrom {
		if source.SecretRef != nil && source.SecretRef.Name == listingKitSharedSecret {
			t.Fatalf("identity preflight must not import shared Secret %s with envFrom", listingKitSharedSecret)
		}
	}

	actual := make(map[string]string)
	for _, variable := range container.Env {
		if variable.ValueFrom == nil || variable.ValueFrom.SecretKeyRef == nil {
			continue
		}
		ref := variable.ValueFrom.SecretKeyRef
		if ref.Name == listingKitSharedSecret {
			if ref.Optional == nil || *ref.Optional {
				t.Fatalf("identity preflight Secret key %s must be required", ref.Key)
			}
			actual[variable.Name] = ref.Key
		}
	}
	want := databaseSecretKeys()
	want["ZITADEL_ISSUER_URL"] = "ZITADEL_ISSUER_URL"
	want["TASK_PROCESSOR_LISTINGKIT_ZITADEL_TENANT_DIRECTORY_TOKEN"] = "TASK_PROCESSOR_LISTINGKIT_ZITADEL_TENANT_DIRECTORY_TOKEN"
	if !reflect.DeepEqual(actual, want) {
		t.Fatalf("identity preflight Secret allowlist = %#v, want %#v", actual, want)
	}
}

func TestListingKitSecretExamplesDefineCanonicalUIAllowlists(t *testing.T) {
	for _, relativePath := range []string{
		"listingkit-workbench/base/secret.example.yaml",
		"zitadel/local/listingkit-workbench-zitadel-secret.example.yaml",
	} {
		t.Run(relativePath, func(t *testing.T) {
			path := filepath.Join("..", "deployments", "kubernetes", filepath.FromSlash(relativePath))
			contents, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", relativePath, err)
			}
			var secret struct {
				StringData map[string]string `yaml:"stringData"`
			}
			if err := yaml.Unmarshal(contents, &secret); err != nil {
				t.Fatalf("parse %s: %v", relativePath, err)
			}
			for _, key := range []string{
				"TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_TENANT_IDS",
				"TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_USER_IDS",
				"TASK_PROCESSOR_LISTINGKIT_ZITADEL_ALLOWED_ROLES",
			} {
				if _, ok := secret.StringData[key]; !ok {
					t.Errorf("%s must define required canonical UI allowlist key %s", relativePath, key)
				}
			}
			if _, ok := secret.StringData["LISTINGKIT_ZITADEL_ALLOWED_ROLES"]; ok {
				t.Errorf("%s must not define deprecated UI allowlist key LISTINGKIT_ZITADEL_ALLOWED_ROLES", relativePath)
			}
		})
	}
}

func databaseSecretKeys() map[string]string {
	return map[string]string{
		"TASK_PROCESSOR_DATABASE_HOST":     "TASK_PROCESSOR_DATABASE_HOST",
		"TASK_PROCESSOR_DATABASE_PORT":     "TASK_PROCESSOR_DATABASE_PORT",
		"TASK_PROCESSOR_DATABASE_USER":     "TASK_PROCESSOR_DATABASE_USER",
		"TASK_PROCESSOR_DATABASE_PASSWORD": "TASK_PROCESSOR_DATABASE_PASSWORD",
		"TASK_PROCESSOR_DATABASE_NAME":     "TASK_PROCESSOR_DATABASE_NAME",
	}
}

func loadOnlyContainer(t *testing.T, relativePath string) workloadContainer {
	t.Helper()
	path := filepath.Join("..", "deployments", "kubernetes", "listingkit-workbench", filepath.FromSlash(relativePath))
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var manifest workloadManifest
	if err := yaml.Unmarshal(contents, &manifest); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(manifest.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("%s has %d containers, want exactly one", path, len(manifest.Spec.Template.Spec.Containers))
	}
	return manifest.Spec.Template.Spec.Containers[0]
}
