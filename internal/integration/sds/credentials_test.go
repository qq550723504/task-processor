package sds

import (
	"context"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"strings"
	"testing"
)

type credentialLoaderFunc func(context.Context) (Credentials, error)

func (f credentialLoaderFunc) Current(c context.Context) (Credentials, error) { return f(c) }

type credentialTransportFunc func(*http.Request) (*http.Response, error)

func (f credentialTransportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVerifiedCredentialSourceAttestsActualMerchant(t *testing.T) {
	c := Credentials{BindingID: "platform-sds", Revision: "revision-a", MerchantID: "36811", AccessToken: "private-fixture"}
	for _, body := range []string{`{"id":36811}`, `{"id":36812}`, `{"id":36811,"id":36812}`, `{"id":"36811"}`} {
		source, err := NewVerifiedCredentialSource(&http.Client{Transport: credentialTransportFunc(func(r *http.Request) (*http.Response, error) {
			require.Equal(t, "https://mapi.sdspod.com/merchants/36811/setMeals", r.URL.String())
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, c.AccessToken, r.Header.Get("Access-Token"))
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})}, credentialLoaderFunc(func(context.Context) (Credentials, error) { return c, nil }))
		require.NoError(t, err)
		got, err := source.Current(context.Background())
		if body == `{"id":36811}` || body == `{"id":"36811"}` {
			require.NoError(t, err)
			require.Equal(t, c, got)
		} else {
			require.Error(t, err)
			require.Empty(t, got.AccessToken)
		}
	}
}
