package browsercollector

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"strings"
)

// CredentialHeader is the header carrying the service-to-service credential.
// It is a service credential between current-application and the collector; it
// is NOT a user or tenant credential and must never be reused from any database,
// session, or user credential (design A1 / D7).
const CredentialHeader = "X-Collector-Credential"

// ErrAdmissionDenied is returned by an admission check that rejected the caller.
var ErrAdmissionDenied = errors.New("browser collector caller admission denied")

// SharedSecretAdmission builds the service-identity admission check resolved in
// design A1: both sides hold one random service credential, the collector
// compares it in constant time, and an absent or wrong credential is denied.
//
// This is deliberately the only shipped admission mechanism. The alternative in
// the design (network-namespace-only binding) was rejected because any workload
// on the same network could otherwise drive Chromium and spend the shared
// browser/IP budget.
func SharedSecretAdmission(secret string) AdmitFunc {
	expected := []byte(secret)
	return func(r *http.Request) error {
		if len(expected) == 0 {
			// Fail closed: an unconfigured collector must never accept a caller.
			return ErrAdmissionDenied
		}
		presented := []byte(r.Header.Get(CredentialHeader))
		// subtle.ConstantTimeCompare returns 0 for differing lengths, and the
		// length itself is not secret (a fixed-size random token).
		if subtle.ConstantTimeCompare(presented, expected) != 1 {
			return ErrAdmissionDenied
		}
		return nil
	}
}

// AttachSharedSecret returns the client-side counterpart: it sets the credential
// header on an outbound RPC request, and returns an error when no secret is
// configured so the caller fails closed instead of sending an unauthenticated
// request that would only waste a browser slot.
func AttachSharedSecret(secret string) func(*http.Request) error {
	value := strings.TrimSpace(secret)
	return func(r *http.Request) error {
		if value == "" {
			return ErrAdmissionDenied
		}
		r.Header.Set(CredentialHeader, value)
		return nil
	}
}

// SharedSecretFromEnv reads the service credential from the environment. It is a
// process-level secret supplied by the deployment, never from a database or
// user store.
func SharedSecretFromEnv(env func(string) string, key string) (string, error) {
	if env == nil {
		env = os.Getenv
	}
	secret := strings.TrimSpace(env(key))
	if secret == "" {
		return "", ErrAdmissionDenied
	}
	return secret, nil
}
