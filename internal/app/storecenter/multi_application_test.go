package storecenterapp_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	storeapp "task-processor/internal/app/storecenter"
	"task-processor/internal/integration/shein"
	"task-processor/internal/storecenter"
)

type multiApplicationProvider struct {
	officialProvider
	application storecenter.OfficialApplication
}

func (p *multiApplicationProvider) Application() storecenter.OfficialApplication {
	return p.application
}
func (p *multiApplicationProvider) Exchange(_ context.Context, _, _ string) (storecenter.OfficialMerchantCredential, error) {
	p.exchanges++
	return storecenter.OfficialMerchantCredential{AppID: p.application.AppID, OpenKeyID: "fixture-merchant", SecretKey: "fixture-secret", SupplierID: "123"}, nil
}
func (p *multiApplicationProvider) QueryStore(_ context.Context, c storecenter.OfficialMerchantCredential) (storecenter.OfficialStoreInformation, error) {
	if c.AppID != p.application.AppID {
		return storecenter.OfficialStoreInformation{}, storecenter.ErrOfficialConnectionUnavailable
	}
	p.queries++
	return storecenter.OfficialStoreInformation{}, nil
}
func TestConnectionCallbackAndRecoveryAlwaysUseTheOriginalApplication(t *testing.T) {
	modes := []storecenter.OfficialApplicationType{storecenter.ApplicationSelfOperated, storecenter.ApplicationSemiManaged, storecenter.ApplicationFullyManaged}
	for selected, mode := range modes {
		t.Run(string(mode), func(t *testing.T) {
			_, repo, _, stored := serviceFixture(t)
			providers := []*multiApplicationProvider{}
			entries := []storeapp.OfficialApplicationRegistration{}
			for index, kind := range modes {
				provider := &multiApplicationProvider{application: storecenter.OfficialApplication{AppID: string(kind), Version: storeapp.BoundOfficialRevision("v1", kind), CallbackURL: "https://localhost/callback"}}
				providers = append(providers, provider)
				key := make([]byte, 32)
				key[0] = byte(index + 1)
				protection, err := shein.NewCredentialProtection(string(kind), key)
				require.NoError(t, err)
				entries = append(entries, storeapp.OfficialApplicationRegistration{Provider: provider, Protection: protection, Type: kind})
			}
			registry, err := storeapp.NewOfficialApplicationRegistry(entries)
			require.NoError(t, err)
			app, err := storeapp.NewOfficialConnections(repo, registry)
			require.NoError(t, err)
			command := storecenter.OfficialConnectionCommand{OrganizationID: "org-a", StoreID: stored.ID(), AttemptID: uuid.NewString(), ExpectedStoreVersion: stored.Version(), ApplicationID: string(mode)}
			begin, err := app.Begin(context.Background(), command)
			require.NoError(t, err)
			binding, err := repo.ReadOfficialAttemptBinding(context.Background(), "org-a", stored.ID(), command.AttemptID)
			require.NoError(t, err)
			require.Equal(t, string(mode), binding.AppID)
			request := storecenter.CompleteOfficialConnection{OrganizationID: "org-a", StoreID: stored.ID(), AttemptID: command.AttemptID, AppID: string(modes[(selected+1)%3]), State: authorizationState(t, begin.AuthorizationURL), TempToken: "fixture-token"}
			_, err = app.Complete(context.Background(), request)
			require.ErrorIs(t, err, storecenter.ErrNotFound)
			request.AppID = string(mode)
			view, err := app.Complete(context.Background(), request)
			require.NoError(t, err)
			require.Equal(t, string(mode), view.AppID)
			_, err = app.Complete(context.Background(), request)
			require.NoError(t, err)
			_, err = app.ResumeQuery(context.Background(), "org-a", stored.ID(), command.AttemptID)
			require.NoError(t, err)
			for index, provider := range providers {
				if index == selected {
					require.Equal(t, 1, provider.exchanges)
					require.Equal(t, 2, provider.queries)
				} else {
					require.Zero(t, provider.exchanges)
					require.Zero(t, provider.queries)
				}
			}
			// Removing the original configuration cannot route a saved credential to
			// one of the remaining applications or exchange the temporary token again.
			remaining := append([]storeapp.OfficialApplicationRegistration(nil), entries[:selected]...)
			remaining = append(remaining, entries[selected+1:]...)
			changed, err := storeapp.NewOfficialApplicationRegistry(remaining)
			require.NoError(t, err)
			restarted, err := storeapp.NewOfficialConnections(repo, changed)
			require.NoError(t, err)
			_, err = restarted.Complete(context.Background(), request)
			require.ErrorIs(t, err, storecenter.ErrOfficialConnectionUnavailable)
			_, err = restarted.ResumeQuery(context.Background(), "org-a", stored.ID(), command.AttemptID)
			require.ErrorIs(t, err, storecenter.ErrOfficialConnectionUnavailable)
			_, err = repo.ReadOfficialAttemptBinding(context.Background(), "foreign-org", stored.ID(), command.AttemptID)
			require.ErrorIs(t, err, storecenter.ErrNotFound)
		})
	}
}
