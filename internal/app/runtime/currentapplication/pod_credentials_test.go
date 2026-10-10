package currentapplication

import (
	"context"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestPODPrivateCredentialLoader(t *testing.T) {
	path := writeManifest(t, `{"format":"sds-platform-session-v1","bindingId":"platform-sds","revision":"67da694b-bd79-4aea-bc4a-ecf93462a723","merchantId":"36811","accessToken":"private-fixture","capturedAt":"2026-10-10","source":{},"verification":{}}`)
	c, e := (podCredentialFile{path}).Current(context.Background())
	require.NoError(t, e)
	require.Equal(t, "36811", c.MerchantID)
	require.NoError(t, os.WriteFile(path, []byte(`{"format":"sds-platform-session-v1","bindingId":"platform-sds","revision":"revision","merchantId":"36811","accessToken":"fixture","accessToken":"second"}`), 0600))
	c, e = (podCredentialFile{path}).Current(context.Background())
	require.Error(t, e)
	require.Empty(t, c.AccessToken)
	_, e = (podCredentialFile{"relative.json"}).Current(context.Background())
	require.Error(t, e)
}
