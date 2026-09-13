package config

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ReferralsConfig enables only the bounded registration and personal referral
// seam. Secrets are loaded from private files by the runtime, never from env.
type ReferralsConfig struct {
	Enabled               bool              `json:"enabled"`
	Issuer                string            `json:"issuer"`
	InstanceID            string            `json:"instanceID"`
	SignupOrganizationID  string            `json:"signupOrganizationID"`
	ProviderOrigin        string            `json:"providerOrigin"`
	OfficialLoginOrigin   string            `json:"officialLoginOrigin"`
	PublicAppOrigin       string            `json:"publicAppOrigin"`
	CredentialFile        string            `json:"credentialFile"`
	ServiceCredentialFile string            `json:"serviceCredentialFile"`
	LookupKeyFile         string            `json:"lookupKeyFile"`
	KeyID                 string            `json:"keyID"`
	ProofKeyFiles         map[string]string `json:"proofKeyFiles"`
	EncryptionKeyFiles    map[string]string `json:"encryptionKeyFiles"`
	ProviderCAFile        string            `json:"providerCAFile,omitempty"`
	Prepared              *ReferralSecrets  `json:"-" yaml:"-"`
}

// ReferralSecrets is runtime-only. The runtime owns the client lifecycle.
type ReferralSecrets struct {
	ProviderToken, ServiceCredential string
	Lookup                           []byte
	Proof, Encryption                map[string][]byte
	HTTPClient                       *http.Client
}

func (cfg ReferralsConfig) Prepare(ctx context.Context) (*ReferralSecrets, error) {
	invalid := errors.New("referrals configuration or private credentials invalid")
	if !cfg.Enabled {
		return nil, nil
	}
	if ctx == nil || ctx.Err() != nil {
		return nil, invalid
	}
	for _, s := range []string{cfg.Issuer, cfg.InstanceID, cfg.SignupOrganizationID, cfg.KeyID} {
		if s == "" || len(s) > 200 || strings.TrimSpace(s) != s || strings.ContainsAny(s, "\r\n\x00") {
			return nil, invalid
		}
	}
	for _, raw := range []string{cfg.ProviderOrigin, cfg.OfficialLoginOrigin, cfg.PublicAppOrigin} {
		u, err := url.Parse(raw)
		if err != nil || len(raw) > 2048 || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Opaque != "" || u.RawPath != "" || (u.Path != "" && u.Path != "/") || strings.ContainsAny(raw, "?#\r\n") {
			return nil, invalid
		}
	}
	if runtime.GOOS == "windows" {
		paths := []string{cfg.CredentialFile, cfg.ServiceCredentialFile, cfg.LookupKeyFile}
		for _, m := range []map[string]string{cfg.ProofKeyFiles, cfg.EncryptionKeyFiles} {
			for _, p := range m {
				paths = append(paths, p)
			}
		}
		if cfg.ProviderCAFile != "" {
			paths = append(paths, cfg.ProviderCAFile)
		}
		for _, p := range paths {
			if !filepath.IsAbs(p) {
				return nil, invalid
			}
		}
		data, _ := json.Marshal(paths)
		check, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		// Use the native ACL API through a fixed script. Paths are JSON stdin,
		// never shell code. Only this account, SYSTEM and Administrators may
		// have allow ACEs; inherited public/group access also fails closed.
		command := exec.CommandContext(check, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", `$ErrorActionPreference='Stop'; $paths=([Console]::In.ReadToEnd() | ConvertFrom-Json); $allowed=@([System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value,'S-1-5-18','S-1-5-32-544'); foreach($p in $paths) { $acl=[System.IO.File]::GetAccessControl($p); foreach($ace in $acl.GetAccessRules($true,$true,[System.Security.Principal.SecurityIdentifier])) { if($ace.AccessControlType -eq 'Allow' -and $allowed -notcontains $ace.IdentityReference.Value) { exit 1 } } }; exit 0`)
		command.Stdin = bytes.NewReader(data)
		if command.Run() != nil {
			return nil, invalid
		}
	}
	read := func(path string) ([]byte, error) {
		if ctx.Err() != nil {
			return nil, invalid
		}
		if !filepath.IsAbs(path) {
			return nil, invalid
		}
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
			return nil, invalid
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, invalid
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, 65537))
		if err != nil || len(b) > 65536 {
			return nil, invalid
		}
		return b, nil
	}
	provider, err := read(cfg.CredentialFile)
	if err != nil {
		return nil, invalid
	}
	token := strings.TrimSpace(string(provider))
	if len(token) < 32 || len(token) > 4096 || strings.ContainsAny(token, " \t\r\n\x00") {
		return nil, invalid
	}
	service, err := read(cfg.ServiceCredentialFile)
	if err != nil {
		return nil, invalid
	}
	credential := strings.TrimSpace(string(service))
	decoded, err := hex.DecodeString(credential)
	if err != nil || len(decoded) != 32 || credential == token {
		return nil, invalid
	}
	out := &ReferralSecrets{ProviderToken: token, ServiceCredential: credential, Proof: map[string][]byte{}, Encryption: map[string][]byte{}}
	seen := map[[32]byte]bool{sha256.Sum256(decoded): true, sha256.Sum256([]byte(token)): true}
	if tokenBytes, e := hex.DecodeString(token); e == nil {
		seen[sha256.Sum256(tokenBytes)] = true
	}
	key := func(path string) ([]byte, error) {
		b, err := read(path)
		if err != nil {
			return nil, invalid
		}
		value, err := hex.DecodeString(strings.TrimSpace(string(b)))
		if err != nil || len(value) != 32 {
			return nil, invalid
		}
		digest := sha256.Sum256(value)
		if seen[digest] {
			return nil, invalid
		}
		seen[digest] = true
		return value, nil
	}
	if out.Lookup, err = key(cfg.LookupKeyFile); err != nil {
		return nil, invalid
	}
	for _, pair := range []struct {
		paths  map[string]string
		values map[string][]byte
	}{{cfg.ProofKeyFiles, out.Proof}, {cfg.EncryptionKeyFiles, out.Encryption}} {
		if len(pair.paths) == 0 || len(pair.paths) > 8 || pair.paths[cfg.KeyID] == "" {
			return nil, invalid
		}
		for id, path := range pair.paths {
			if id == "" || len(id) > 64 {
				return nil, invalid
			}
			if pair.values[id], err = key(path); err != nil {
				return nil, invalid
			}
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.MaxConnsPerHost = 8
	if cfg.ProviderCAFile != "" {
		pem, err := read(cfg.ProviderCAFile)
		if err != nil {
			return nil, invalid
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			return nil, invalid
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, invalid
		}
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}
	}
	out.HTTPClient = &http.Client{Transport: transport}
	return out, nil
}
