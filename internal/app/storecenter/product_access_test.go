package storecenterapp

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/stretchr/testify/require"
	"task-processor/internal/integration/shein"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/storecenter"
	"testing"
	"time"
)

type productMaterialReader struct {
	material storecenter.ProductExecutionMaterial
	err      error
	reads    int
}

func (r *productMaterialReader) ReadProductExecution(context.Context, storecenter.ProductExecutionSubject, string, storecenter.ProductExecutionAuthorizer, time.Time) (storecenter.ProductExecutionMaterial, error) {
	r.reads++
	return r.material, r.err
}

type productLiveAccess struct{}

func (productLiveAccess) AuthorizeProductExecution(context.Context, storecenter.ProductExecutionSubject) (storecenter.ProductExecutionAuthorization, error) {
	return storecenter.ProductExecutionAuthorization{}, nil
}

type productSecretProtection struct {
	storecenter.OfficialCredentialProtection
}

func (productSecretProtection) Open(storecenter.OfficialConnectionAttempt, string, string) (storecenter.OfficialMerchantCredential, error) {
	return storecenter.OfficialMerchantCredential{AppID: "app-a", SupplierID: "123", OpenKeyID: "private-open-key", SecretKey: "private-merchant-secret"}, nil
}

type productProvider struct {
	OfficialGoodsProvider
	storecenter.OfficialConnectionProvider
	calls      int
	appVersion string
	appID      string
	deadline   time.Time
}

func (p *productProvider) Application() storecenter.OfficialApplication {
	id := p.appID
	if id == "" {
		id = "app-a"
	}
	return storecenter.OfficialApplication{AppID: id, Version: p.appVersion}
}
func TestProductExecutionUsesOnlyOriginalApplicationProtectionAndMode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	now := time.Now().UTC()
	providers := []*productProvider{}
	protections := []*shein.CredentialProtection{}
	entries := []OfficialApplicationRegistration{}
	modes := []storecenter.OfficialApplicationType{storecenter.ApplicationSelfOperated, storecenter.ApplicationSemiManaged, storecenter.ApplicationFullyManaged}
	for index, mode := range modes {
		p := &productProvider{appID: string(mode), appVersion: BoundOfficialRevision("v1", mode)}
		providers = append(providers, p)
		key := make([]byte, 32)
		key[0] = byte(index + 1)
		protection, err := shein.NewCredentialProtection(string(mode), key)
		require.NoError(t, err)
		protections = append(protections, protection)
		entries = append(entries, OfficialApplicationRegistration{Provider: p, Protection: protection, Type: mode})
	}
	registry, err := NewOfficialApplicationRegistry(entries)
	require.NoError(t, err)
	subject := storecenter.ProductExecutionSubject{OrganizationID: "org-a", ActorID: "actor-a", MemberID: "member-a", Purpose: storecenter.ProductPurposePublish}
	for index, mode := range modes {
		attempt := storecenter.OfficialConnectionAttempt{OrganizationID: "org-a", StoreID: "store-a", AttemptID: "connection-a", ActorID: "actor-a", MemberID: "member-a", AppID: string(mode), AppVersion: providers[index].appVersion, ConnectionVersion: 1, State: "verified"}
		attempt.KeyID, attempt.Ciphertext, err = protections[index].Seal(attempt, storecenter.OfficialMerchantCredential{AppID: string(mode), OpenKeyID: "merchant-key", SecretKey: "synthetic-private", SupplierID: "123"})
		require.NoError(t, err)
		reader := &productMaterialReader{material: storecenter.ProductExecutionMaterial{StoreVersion: 1, Platform: "shein", ServiceExpiresAt: now.Add(time.Hour), Connection: storecenter.OfficialConnectionView{AttemptID: "connection-a", Version: 1, Status: "connected", State: "verified"}, Attempt: attempt}}
		access, err := NewOfficialProductAccess(reader, productLiveAccess{}, registry)
		require.NoError(t, err)
		handle, err := access.Authorize(ctx, subject, "store-a", nil)
		require.NoError(t, err)
		require.Equal(t, mode, handle.Binding().ApplicationType)
		require.Equal(t, string(mode), handle.Binding().ApplicationID)
		other := (index + 1) % len(modes)
		reader.material.Attempt.AppID = string(modes[other])
		reader.material.Attempt.AppVersion = providers[other].appVersion
		_, err = handle.Publish(ctx, model.PublishProduct{})
		require.ErrorIs(t, err, ErrProductAccessChanged)
		_, err = access.Authorize(ctx, subject, "store-a", nil)
		require.ErrorIs(t, err, ErrProductAccessChanged, "ciphertext sealed for original AppID cannot be used with another app/key")
		reader.material.Attempt = attempt
		handle, err = access.Authorize(ctx, subject, "store-a", nil)
		require.NoError(t, err)
		_, err = handle.Publish(ctx, model.PublishProduct{})
		require.NoError(t, err)
		require.Equal(t, 1, providers[index].calls)
	}
}
func (p *productProvider) PublishProduct(ctx context.Context, c storecenter.OfficialMerchantCredential, input model.PublishProduct) (model.PublishResult, error) {
	p.calls++
	p.deadline, _ = ctx.Deadline()
	return model.PublishResult{SPUName: "spu-a"}, nil
}

func TestProductHandleReauthorizesBeforeSendAndCannotSerializeOrRepeatMutation(t *testing.T) {
	now := time.Now().UTC()
	reader := &productMaterialReader{material: storecenter.ProductExecutionMaterial{StoreVersion: 3, Platform: "shein", ServiceExpiresAt: now.Add(time.Hour), Connection: storecenter.OfficialConnectionView{AttemptID: "connection-a", Version: 2, Status: "connected", State: "verified"}, Attempt: storecenter.OfficialConnectionAttempt{AppID: "app-a", AppVersion: BoundOfficialRevision("v1", storecenter.ApplicationSelfOperated), State: "verified", KeyID: "key-a", Ciphertext: "sealed", AttemptID: "connection-a", OrganizationID: "org-a", StoreID: "store-a"}}}
	provider := &productProvider{appVersion: BoundOfficialRevision("v1", storecenter.ApplicationSelfOperated)}
	registry, err := NewOfficialApplicationRegistry([]OfficialApplicationRegistration{{Provider: provider, Protection: productSecretProtection{}, Type: storecenter.ApplicationSelfOperated}})
	require.NoError(t, err)
	access, err := NewOfficialProductAccess(reader, productLiveAccess{}, registry)
	require.NoError(t, err)
	access.now = func() time.Time { return now }
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	subject := storecenter.ProductExecutionSubject{OrganizationID: "org-a", ActorID: "actor-a", MemberID: "member-a", Purpose: storecenter.ProductPurposePublish}
	handle, err := access.Authorize(ctx, subject, "store-a", nil)
	require.NoError(t, err)
	encoded, err := json.Marshal(handle)
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(encoded))
	encoded, err = json.Marshal(handle.Binding())
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-")
	result, err := handle.Publish(ctx, model.PublishProduct{})
	require.NoError(t, err)
	require.Equal(t, "spu-a", result.SPUName)
	require.False(t, provider.deadline.After(handle.expiresAt), "provider call cannot outlive merchant handle authority")
	_, err = handle.Publish(ctx, model.PublishProduct{})
	require.ErrorIs(t, err, ErrProductAccessChanged)
	require.Equal(t, 1, provider.calls)
	handle, err = access.Authorize(ctx, subject, "store-a", nil)
	require.NoError(t, err)
	reader.err = errors.New("private IAM failure")
	_, err = handle.Publish(ctx, model.PublishProduct{})
	require.ErrorIs(t, err, ErrProductAccessChanged)
	require.NotContains(t, err.Error(), "private IAM")
	require.Equal(t, 1, provider.calls)
	reader.err = nil
	handle, err = access.Authorize(ctx, subject, "store-a", nil)
	require.NoError(t, err)
	reader.material.Connection.Version++
	_, err = handle.Publish(ctx, model.PublishProduct{})
	require.ErrorIs(t, err, ErrProductAccessChanged)
	require.Equal(t, 1, provider.calls)
	reader.material.Connection.Version--
	handle, err = access.Authorize(ctx, subject, "store-a", nil)
	require.NoError(t, err)
	provider.appVersion = "v2"
	_, err = handle.Publish(ctx, model.PublishProduct{})
	require.ErrorIs(t, err, ErrProductAccessChanged)
	provider.appVersion = BoundOfficialRevision("v1", storecenter.ApplicationSelfOperated)
	handle, err = access.Authorize(ctx, subject, "store-a", nil)
	require.NoError(t, err)
	now = now.Add(11 * time.Second)
	_, err = handle.Publish(ctx, model.PublishProduct{})
	require.ErrorIs(t, err, ErrProductAccessChanged)
	require.Equal(t, 1, provider.calls)
	var zero MerchantHandle
	_, err = zero.Publish(ctx, model.PublishProduct{})
	require.ErrorIs(t, err, ErrProductAccessChanged)
}
