package httpapi

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"task-processor/internal/core/config"
)

func TestBuildImageAgentCapabilitiesFailsClosedWithoutRealProvidersAndPublisher(t *testing.T) {
	_, err := BuildImageAgentCapabilities(RuntimeBuildInput{Logger: logrus.New(), Config: &config.Config{}, ImageWorkDir: t.TempDir()})
	require.ErrorContains(t, err, "governed tenant credential routing")
}

func TestValidateImageAgentWorkerCapabilitiesRequiresGovernedDurableRuntime(t *testing.T) {
	validConfig := &config.Config{
		AICapability: config.AICapabilityConfig{ProductImageSceneEnabled: true, ProductImageSceneAllowedTenantIDs: []string{"tenant-a"}},
		ProductImage: config.ProductImageConfig{Publisher: config.ProductImagePublisherConfig{Enabled: true, Provider: "s3", S3: config.ProductImagePublisherS3Config{Bucket: "durable-bucket", Region: "us-east-1"}}},
	}
	valid := imageAgentWorkerCapabilityPolicyInput{config: validConfig, hasCredentialResolver: true, hasResolverBackedManager: true, hasInvocationRecorder: true}

	tests := []struct {
		name      string
		mutate    func(imageAgentWorkerCapabilityPolicyInput) imageAgentWorkerCapabilityPolicyInput
		wantError string
	}{
		{name: "governance disabled", mutate: func(in imageAgentWorkerCapabilityPolicyInput) imageAgentWorkerCapabilityPolicyInput {
			in.config = cloneImageAgentCapabilityConfig(validConfig)
			in.config.AICapability.ProductImageSceneEnabled = false
			return in
		}, wantError: "governed tenant credential routing"},
		{name: "allowlist missing", mutate: func(in imageAgentWorkerCapabilityPolicyInput) imageAgentWorkerCapabilityPolicyInput {
			in.config = cloneImageAgentCapabilityConfig(validConfig)
			in.config.AICapability.ProductImageSceneAllowedTenantIDs = nil
			return in
		}, wantError: "tenant allowlist"},
		{name: "credential resolver missing", mutate: func(in imageAgentWorkerCapabilityPolicyInput) imageAgentWorkerCapabilityPolicyInput {
			in.hasCredentialResolver = false
			return in
		}, wantError: "credential resolver"},
		{name: "static manager", mutate: func(in imageAgentWorkerCapabilityPolicyInput) imageAgentWorkerCapabilityPolicyInput {
			in.hasResolverBackedManager = false
			return in
		}, wantError: "resolver-backed"},
		{name: "invocation recorder missing", mutate: func(in imageAgentWorkerCapabilityPolicyInput) imageAgentWorkerCapabilityPolicyInput {
			in.hasInvocationRecorder = false
			return in
		}, wantError: "invocation recorder"},
		{name: "publisher disabled", mutate: func(in imageAgentWorkerCapabilityPolicyInput) imageAgentWorkerCapabilityPolicyInput {
			in.config = cloneImageAgentCapabilityConfig(validConfig)
			in.config.ProductImage.Publisher.Enabled = false
			return in
		}, wantError: "durable s3 publisher"},
		{name: "local publisher", mutate: func(in imageAgentWorkerCapabilityPolicyInput) imageAgentWorkerCapabilityPolicyInput {
			in.config = cloneImageAgentCapabilityConfig(validConfig)
			in.config.ProductImage.Publisher.Provider = "local"
			return in
		}, wantError: "durable s3 publisher"},
		{name: "hybrid local publisher", mutate: func(in imageAgentWorkerCapabilityPolicyInput) imageAgentWorkerCapabilityPolicyInput {
			in.config = cloneImageAgentCapabilityConfig(validConfig)
			in.config.ProductImage.Publisher.Provider = "hybrid-local"
			return in
		}, wantError: "durable s3 publisher"},
		{name: "s3 bucket missing", mutate: func(in imageAgentWorkerCapabilityPolicyInput) imageAgentWorkerCapabilityPolicyInput {
			in.config = cloneImageAgentCapabilityConfig(validConfig)
			in.config.ProductImage.Publisher.S3.Bucket = ""
			return in
		}, wantError: "s3 bucket"},
		{name: "valid governed durable config", mutate: func(in imageAgentWorkerCapabilityPolicyInput) imageAgentWorkerCapabilityPolicyInput { return in }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateImageAgentWorkerCapabilityPolicy(test.mutate(valid))
			if test.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantError)
		})
	}
}

func cloneImageAgentCapabilityConfig(input *config.Config) *config.Config {
	cloned := *input
	cloned.AICapability.ProductImageSceneAllowedTenantIDs = append([]string(nil), input.AICapability.ProductImageSceneAllowedTenantIDs...)
	return &cloned
}
