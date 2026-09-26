package sourcing

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// public_browser is the second anonymous public provider admitted for
// src2b-public-browser-v1. It must be accepted alongside public_http and
// produce the same publication identity for the same evidence.
func TestMapAcquisitionEvidenceAdmitsPublicBrowserChannel(t *testing.T) {
	source, err := Canonical1688Source("981645030344")
	require.NoError(t, err)
	e := acquisitionEvidenceFixture()
	for _, channel := range []string{"public_http", "public_browser", "browser_capture"} {
		envelope, err := MapAcquisitionEvidence(source, e, channel, "operation-browser")
		require.NoError(t, err, channel)
		require.Equal(t, channel, envelope.RawReference.Metadata["channel"], channel)
		key, _, err := PublicationIdentity(envelope)
		require.NoError(t, err, channel)
		require.Equal(t, "crawler:1688:981645030344", key, channel)
		require.Equal(t, AcquisitionContractVersion, envelope.RawReference.Metadata["contract_version"])
		require.Empty(t, envelope.RawReference.Metadata["capture_sha256"], "public_browser keeps CaptureSHA256 empty like public_http")
	}
}

// A public_browser envelope must be byte-identical to the public_http envelope
// for the same evidence: only the channel metadata may differ, and channel is
// not part of the snapshot, so publication identity is unchanged.
func TestMapAcquisitionEvidencePublicBrowserMatchesPublicHTTP(t *testing.T) {
	source, err := Canonical1688Source("981645030344")
	require.NoError(t, err)
	e := acquisitionEvidenceFixture()
	httpEnvelope, err := MapAcquisitionEvidence(source, e, "public_http", "operation-browser")
	require.NoError(t, err)
	browserEnvelope, err := MapAcquisitionEvidence(source, e, "public_browser", "operation-browser")
	require.NoError(t, err)
	require.Equal(t, httpEnvelope.RawReference.Metadata["parser_version"], browserEnvelope.RawReference.Metadata["parser_version"])
	require.NotEqual(t, httpEnvelope.RawReference.Metadata["channel"], browserEnvelope.RawReference.Metadata["channel"])
	keyHTTP, pubHTTP, err := PublicationIdentity(httpEnvelope)
	require.NoError(t, err)
	keyBrowser, pubBrowser, err := PublicationIdentity(browserEnvelope)
	require.NoError(t, err)
	require.Equal(t, keyHTTP, keyBrowser)
	require.Equal(t, pubHTTP, pubBrowser)
}

// An unknown channel, including a typo of the browser channel, must still be
// rejected: the allow-list is exactly the three admitted values.
func TestMapAcquisitionEvidenceRejectsUnknownChannel(t *testing.T) {
	source, err := Canonical1688Source("981645030344")
	require.NoError(t, err)
	e := acquisitionEvidenceFixture()
	for _, channel := range []string{"", "public", "public_browser_v2", "browser", "unknown", "public_http ", "browser_captures"} {
		_, err := MapAcquisitionEvidence(source, e, channel, "operation-browser")
		require.ErrorIs(t, err, ErrInvalidAcquisition, strings.TrimSpace(channel))
	}
}
