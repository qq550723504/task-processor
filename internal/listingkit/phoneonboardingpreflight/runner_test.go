package phoneonboardingpreflight

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunnerStartCreatesIsolatedTechnicalUserAndSMSChallenge(t *testing.T) {
	t.Parallel()

	fake := &fakeRunnerClient{}
	var output bytes.Buffer
	runner, err := NewRunner(fake, bytes.NewReader(bytes.Repeat([]byte{0x42}, 10)), func() time.Time {
		return time.Date(2026, 8, 25, 1, 2, 3, 0, time.UTC)
	}, &output)
	require.NoError(t, err)

	attempt, err := runner.Start(context.Background(), " +8613712345678 ")
	require.NoError(t, err)
	require.Equal(t, []string{"CreateOrganization", "CreateTechnicalUser", "AddOTPSMS", "CreateSMSChallenge"}, fake.calls)
	require.Regexp(t, `^lk-phone-preflight-[0-9A-HJKMNP-TV-Z]{26}$`, fake.organizationName)
	require.Regexp(t, `^lkp-[0-9A-HJKMNP-TV-Z]{26}$`, fake.user.Username)
	require.Regexp(t, `^u-[0-9A-HJKMNP-TV-Z]{26}@phone\.invalid$`, fake.user.TechnicalEmail)
	require.Equal(t, "+8613712345678", fake.user.Phone)
	require.NotContains(t, fake.user.Username+fake.user.TechnicalEmail, "13712345678")
	require.Equal(t, "org-1", attempt.OrganizationID)
	require.Equal(t, "user-1", attempt.UserID)
	require.Equal(t, "session-1", attempt.SessionID)
	require.Regexp(t, `^status=challenge_sent attempt=[0-9A-HJKMNP-TV-Z]{26} organization_id=org-1 user_id=user-1 session_id=session-1\n$`, output.String())
}

func TestRunnerVerifyReplacesTokenChecksFactorsAndDeletesSession(t *testing.T) {
	t.Parallel()

	fake := &fakeRunnerClient{proof: SessionProof{
		UserID: "user-1", OrganizationID: "org-1", UserVerifiedAt: time.Now(), OTPSMSVerifiedAt: time.Now(),
	}}
	var output bytes.Buffer
	runner, err := NewRunner(fake, bytes.NewReader(bytes.Repeat([]byte{0x42}, 10)), time.Now, &output)
	require.NoError(t, err)
	attempt := &Attempt{OrganizationID: "org-1", UserID: "user-1", SessionID: "session-1", sessionToken: "created-token", id: "01JTEST"}

	proof, err := runner.Verify(context.Background(), attempt, "654321")
	require.NoError(t, err)
	require.Equal(t, fake.proof, proof)
	require.Equal(t, []string{"VerifySMS", "GetSession", "DeleteSession"}, fake.calls)
	require.Equal(t, "verified-token", fake.getSessionToken)
	require.Equal(t, "", attempt.sessionToken)
	require.Equal(t, "status=otp_verified attempt=01JTEST user_factor=true otp_sms_factor=true\n", output.String())
}

func TestRunnerVerifyDeletesSessionAfterFactorMismatchWithoutLeakingCode(t *testing.T) {
	t.Parallel()

	fake := &fakeRunnerClient{proof: SessionProof{
		UserID: "other-user", OrganizationID: "org-1", UserVerifiedAt: time.Now(), OTPSMSVerifiedAt: time.Now(),
	}}
	var output bytes.Buffer
	runner, err := NewRunner(fake, bytes.NewReader(bytes.Repeat([]byte{0x42}, 10)), time.Now, &output)
	require.NoError(t, err)
	attempt := &Attempt{OrganizationID: "org-1", UserID: "user-1", SessionID: "session-1", sessionToken: "created-token", id: "01JTEST"}

	_, err = runner.Verify(context.Background(), attempt, "654321")
	require.Error(t, err)
	require.Equal(t, []string{"VerifySMS", "GetSession", "DeleteSession"}, fake.calls)
	require.Equal(t, "", attempt.sessionToken)
	require.Equal(t, "status=failed attempt=01JTEST step=session_read\n", output.String())
	require.NotContains(t, output.String(), "654321")
	require.NotContains(t, err.Error(), "654321")
}

func TestRunnerVerifyReportsSessionDeleteWhenCleanupFails(t *testing.T) {
	t.Parallel()

	fake := &fakeRunnerClient{proof: SessionProof{
		UserID: "other-user", OrganizationID: "org-1", UserVerifiedAt: time.Now(), OTPSMSVerifiedAt: time.Now(),
	}, deleteErr: errors.New("provider error")}
	var output bytes.Buffer
	runner, err := NewRunner(fake, bytes.NewReader(bytes.Repeat([]byte{0x42}, 10)), time.Now, &output)
	require.NoError(t, err)

	_, err = runner.Verify(context.Background(), &Attempt{OrganizationID: "org-1", UserID: "user-1", SessionID: "session-1", sessionToken: "created-token", id: "01JTEST"}, "654321")
	require.Error(t, err)
	require.Equal(t, []string{"VerifySMS", "GetSession", "DeleteSession"}, fake.calls)
	require.Equal(t, "status=failed attempt=01JTEST step=session_delete\n", output.String())
}

func TestRunnerRejectsInvalidPhoneBeforeProviderCalls(t *testing.T) {
	t.Parallel()

	fake := &fakeRunnerClient{}
	runner, err := NewRunner(fake, strings.NewReader("random"), time.Now, &bytes.Buffer{})
	require.NoError(t, err)

	_, err = runner.Start(context.Background(), "+86 13712345678")
	require.Error(t, err)
	require.Empty(t, fake.calls)
}

type fakeRunnerClient struct {
	calls            []string
	organizationName string
	user             TechnicalUserInput
	proof            SessionProof
	getSessionToken  string
	deleteErr        error
}

func (f *fakeRunnerClient) CreateOrganization(_ context.Context, name string) (string, error) {
	f.calls = append(f.calls, "CreateOrganization")
	f.organizationName = name
	return "org-1", nil
}

func (f *fakeRunnerClient) CreateTechnicalUser(_ context.Context, user TechnicalUserInput) (string, error) {
	f.calls = append(f.calls, "CreateTechnicalUser")
	f.user = user
	return "user-1", nil
}

func (f *fakeRunnerClient) AddOTPSMS(_ context.Context, _ string) error {
	f.calls = append(f.calls, "AddOTPSMS")
	return nil
}

func (f *fakeRunnerClient) CreateSMSChallenge(_ context.Context, _ string, lifetime time.Duration) (SessionMaterial, error) {
	f.calls = append(f.calls, "CreateSMSChallenge")
	if lifetime != 5*time.Minute {
		return SessionMaterial{}, errors.New("wrong lifetime")
	}
	return SessionMaterial{ID: "session-1", Token: "created-token"}, nil
}

func (f *fakeRunnerClient) VerifySMS(_ context.Context, sessionID, code string) (string, error) {
	f.calls = append(f.calls, "VerifySMS")
	if sessionID != "session-1" || code != "654321" {
		return "", errors.New("wrong verification input")
	}
	return "verified-token", nil
}

func (f *fakeRunnerClient) GetSession(_ context.Context, sessionID, token string) (SessionProof, error) {
	f.calls = append(f.calls, "GetSession")
	f.getSessionToken = token
	if sessionID != "session-1" {
		return SessionProof{}, errors.New("wrong session")
	}
	return f.proof, nil
}

func (f *fakeRunnerClient) DeleteSession(_ context.Context, sessionID string) error {
	f.calls = append(f.calls, "DeleteSession")
	if sessionID != "session-1" {
		return errors.New("wrong session")
	}
	return f.deleteErr
}
