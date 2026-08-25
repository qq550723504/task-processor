package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"task-processor/internal/listingkit/phoneonboardingpreflight"

	"github.com/stretchr/testify/require"
)

func TestRunUsesOnlyPreflightEnvironmentAndRedactsSecrets(t *testing.T) {
	const (
		phone             = "+8613712345678"
		code              = "654321"
		provisioningToken = "provisioning-token-secret"
		loginToken        = "login-token-secret"
	)
	values := map[string]string{
		"ZITADEL_ISSUER_URL":                "https://zitadel.example.test",
		"ZITADEL_PREFLIGHT_PROVISION_TOKEN": provisioningToken,
		"ZITADEL_PREFLIGHT_LOGIN_TOKEN":     loginToken,
		"DATABASE_URL":                      "must-not-be-read",
		"TENCENT_SECRET_ID":                 "must-not-be-read",
	}
	var requested []string
	getenv := func(key string) string {
		requested = append(requested, key)
		return values[key]
	}
	var gotConfig phoneonboardingpreflight.ClientConfig
	fake := &cliFakeClient{proof: validCLIProof()}
	var stdout, stderr bytes.Buffer

	err := run(context.Background(), getenv, func(_ string, _ *os.File, _ io.Writer) (string, error) {
		if fake.secretReads == 0 {
			fake.secretReads++
			return phone, nil
		}
		fake.secretReads++
		return code, nil
	}, func(cfg phoneonboardingpreflight.ClientConfig) (phoneonboardingpreflight.Client, error) {
		gotConfig = cfg
		return fake, nil
	}, os.Stdin, &stdout, &stderr, []string{"--non-production"}, preflightTimeout)

	require.NoError(t, err)
	require.Equal(t, []string{"ZITADEL_ISSUER_URL", "ZITADEL_PREFLIGHT_PROVISION_TOKEN", "ZITADEL_PREFLIGHT_LOGIN_TOKEN"}, requested)
	require.Equal(t, values["ZITADEL_ISSUER_URL"], gotConfig.IssuerURL)
	require.Equal(t, provisioningToken, gotConfig.ProvisioningToken)
	require.Equal(t, loginToken, gotConfig.SessionToken)
	require.Equal(t, 2, fake.secretReads)
	require.Empty(t, stderr.String())
	for _, sensitive := range []string{phone, code, provisioningToken, loginToken, "@phone.invalid"} {
		require.NotContains(t, stdout.String(), sensitive)
		require.NotContains(t, stderr.String(), sensitive)
	}
	require.True(t, fake.deadlineSet)
	require.WithinDuration(t, time.Now().Add(5*time.Minute), fake.deadline, 2*time.Second)
}

func TestReadSecretReadsSuccessiveLinesFromPowerShellLikeStdin(t *testing.T) {
	input, err := os.CreateTemp(t.TempDir(), "stdin")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, input.Close()) })
	_, err = input.WriteString("+8613712345678\n654321\n")
	require.NoError(t, err)
	_, err = input.Seek(0, io.SeekStart)
	require.NoError(t, err)
	var output bytes.Buffer

	phone, err := readSecret("phone: ", input, &output)
	require.NoError(t, err)
	code, err := readSecret("verification code: ", input, &output)
	require.NoError(t, err)

	require.Equal(t, "+8613712345678", phone)
	require.Equal(t, "654321", code)
}

func TestReadSecretReadsSuccessiveCRLFConsoleLines(t *testing.T) {
	input, err := os.CreateTemp(t.TempDir(), "console-stdin")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, input.Close()) })
	_, err = input.WriteString("+8613712345678\r\n654321\r\n")
	require.NoError(t, err)
	_, err = input.Seek(0, io.SeekStart)
	require.NoError(t, err)
	var output bytes.Buffer

	phone, err := readSecret("phone: ", input, &output)
	require.NoError(t, err)
	code, err := readSecret("verification code: ", input, &output)
	require.NoError(t, err)

	require.Equal(t, "+8613712345678", phone)
	require.Equal(t, "654321", code)
}

func TestRunReturnsGenericRedactedErrorOnFirstFailure(t *testing.T) {
	values := map[string]string{
		"ZITADEL_ISSUER_URL":                "https://zitadel.example.test",
		"ZITADEL_PREFLIGHT_PROVISION_TOKEN": "provisioning-token-secret",
		"ZITADEL_PREFLIGHT_LOGIN_TOKEN":     "login-token-secret",
	}
	fake := &cliFakeClient{organizationErr: errors.New("provider leaked +8613712345678")}
	var stdout, stderr bytes.Buffer

	err := run(context.Background(), func(key string) string { return values[key] }, func(_ string, _ *os.File, _ io.Writer) (string, error) {
		return "+8613712345678", nil
	}, func(phoneonboardingpreflight.ClientConfig) (phoneonboardingpreflight.Client, error) {
		return fake, nil
	}, os.Stdin, &stdout, &stderr, []string{"--non-production"}, preflightTimeout)

	require.Error(t, err)
	require.Empty(t, stderr.String())
	require.Regexp(t, `^status=failed attempt=[0-9A-HJKMNP-TV-Z]{26} step=organization_create\n$`, stdout.String())
	require.NotContains(t, stdout.String()+stderr.String()+err.Error(), "+8613712345678")
	require.Equal(t, []string{"CreateOrganization"}, fake.calls)
}

func TestRunRequiresNonProductionFlagBeforeRemoteMutation(t *testing.T) {
	fake := &cliFakeClient{}
	var stdout, stderr bytes.Buffer

	err := run(context.Background(), func(string) string { return "" }, func(_ string, _ *os.File, _ io.Writer) (string, error) {
		return "", errors.New("hidden input must not be read")
	}, func(phoneonboardingpreflight.ClientConfig) (phoneonboardingpreflight.Client, error) {
		return fake, nil
	}, os.Stdin, &stdout, &stderr, nil, preflightTimeout)

	require.ErrorIs(t, err, errNonProductionConfirmation)
	require.Empty(t, fake.calls)
	require.Empty(t, stdout.String())
	require.Empty(t, stderr.String())
}

func TestRunTimeoutDuringHiddenCodeReadDeletesSessionWithSeparateCleanupContext(t *testing.T) {
	values := map[string]string{
		"ZITADEL_ISSUER_URL":                "https://zitadel.example.test",
		"ZITADEL_PREFLIGHT_PROVISION_TOKEN": "provisioning-token-secret",
		"ZITADEL_PREFLIGHT_LOGIN_TOKEN":     "login-token-secret",
	}
	fake := &cliFakeClient{proof: validCLIProof()}
	blocked := make(chan struct{})
	defer close(blocked)
	var stdout, stderr bytes.Buffer

	err := run(context.Background(), func(key string) string { return values[key] }, func(_ string, _ *os.File, _ io.Writer) (string, error) {
		if fake.secretReads == 0 {
			fake.secretReads++
			return "+8613712345678", nil
		}
		fake.secretReads++
		<-blocked
		return "", errors.New("terminal read released")
	}, func(phoneonboardingpreflight.ClientConfig) (phoneonboardingpreflight.Client, error) {
		return fake, nil
	}, os.Stdin, &stdout, &stderr, []string{"--non-production"}, time.Millisecond)

	require.Error(t, err)
	require.Equal(t, []string{"CreateOrganization", "CreateTechnicalUser", "AddOTPSMS", "CreateSMSChallenge", "DeleteSession"}, fake.calls)
	require.True(t, fake.deleteDeadlineSet)
	require.True(t, fake.deleteDeadline.Before(time.Now().Add(30*time.Second)))
	require.Empty(t, fake.deleteContextErr)
	require.Empty(t, stderr.String())
	require.Regexp(t, `^status=challenge_sent attempt=[0-9A-HJKMNP-TV-Z]{26} organization_id=org-1 user_id=user-1 session_id=session-1\nstatus=failed attempt=[0-9A-HJKMNP-TV-Z]{26} step=code_verify\n$`, stdout.String())
}

func validCLIProof() phoneonboardingpreflight.SessionProof {
	return phoneonboardingpreflight.SessionProof{
		UserID: "user-1", OrganizationID: "org-1", UserVerifiedAt: time.Now(), OTPSMSVerifiedAt: time.Now(),
	}
}

type cliFakeClient struct {
	calls             []string
	proof             phoneonboardingpreflight.SessionProof
	organizationErr   error
	secretReads       int
	deadlineSet       bool
	deadline          time.Time
	deleteDeadlineSet bool
	deleteDeadline    time.Time
	deleteContextErr  error
}

func (f *cliFakeClient) CreateOrganization(ctx context.Context, _ string) (string, error) {
	f.calls = append(f.calls, "CreateOrganization")
	f.deadline, f.deadlineSet = ctx.Deadline()
	if f.organizationErr != nil {
		return "", f.organizationErr
	}
	return "org-1", nil
}

func (f *cliFakeClient) CreateTechnicalUser(_ context.Context, _ phoneonboardingpreflight.TechnicalUserInput) (string, error) {
	f.calls = append(f.calls, "CreateTechnicalUser")
	return "user-1", nil
}

func (f *cliFakeClient) AddOTPSMS(_ context.Context, _ string) error {
	f.calls = append(f.calls, "AddOTPSMS")
	return nil
}

func (f *cliFakeClient) CreateSMSChallenge(_ context.Context, _ string, _ time.Duration) (phoneonboardingpreflight.SessionMaterial, error) {
	f.calls = append(f.calls, "CreateSMSChallenge")
	return phoneonboardingpreflight.SessionMaterial{ID: "session-1", Token: "created-token"}, nil
}

func (f *cliFakeClient) VerifySMS(_ context.Context, _, _ string) (string, error) {
	f.calls = append(f.calls, "VerifySMS")
	return "verified-token", nil
}

func (f *cliFakeClient) GetSession(_ context.Context, _, _ string) (phoneonboardingpreflight.SessionProof, error) {
	f.calls = append(f.calls, "GetSession")
	return f.proof, nil
}

func (f *cliFakeClient) DeleteSession(ctx context.Context, _ string) error {
	f.calls = append(f.calls, "DeleteSession")
	f.deleteDeadline, f.deleteDeadlineSet = ctx.Deadline()
	f.deleteContextErr = ctx.Err()
	return nil
}
