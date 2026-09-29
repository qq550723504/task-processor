package currentapplication

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/integration/shein"
	"task-processor/internal/storecenter"
)

type OfficialStoreConnectionConfig struct {
	AppID             string `json:"appId"`
	Version           string `json:"version"`
	APIOrigin         string `json:"apiOrigin"`
	CallbackURL       string `json:"callbackURL"`
	AppSecretFile     string `json:"appSecretFile"`
	CredentialKeyFile string `json:"credentialKeyFile"`
	CredentialKeyID   string `json:"credentialKeyId"`
}

func (c *OfficialStoreConnectionConfig) validate() error {
	if c == nil {
		return nil
	}
	if !filepath.IsAbs(c.AppSecretFile) || !filepath.IsAbs(c.CredentialKeyFile) || c.AppSecretFile == c.CredentialKeyFile || c.CredentialKeyID == "" || len(c.CredentialKeyID) > 128 {
		return errors.New("official Store connection requires separate private credential files and key identity")
	}
	// Validate protocol/configuration without network calls or secret-file reads.
	if _, err := shein.NewOfficialClient(shein.OfficialClientOptions{Application: storecenter.OfficialApplication{AppID: c.AppID, Version: c.Version, CallbackURL: c.CallbackURL}, AppSecret: strings.Repeat("0", 32), APIOrigin: c.APIOrigin}); err != nil {
		return errors.New("official Store application configuration invalid")
	}
	return nil
}
func (c *OfficialStoreConnectionConfig) prepare(ctx context.Context) (storecenter.OfficialConnectionProvider, storecenter.OfficialCredentialProtection, error) {
	if c == nil {
		return nil, nil, nil
	}
	secret, err := readOfficialPrivateFile(ctx, c.AppSecretFile)
	if err != nil {
		return nil, nil, err
	}
	defer clear(secret)
	keyText, err := readOfficialPrivateFile(ctx, c.CredentialKeyFile)
	if err != nil {
		return nil, nil, err
	}
	defer clear(keyText)
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(keyText)))
	if err != nil {
		return nil, nil, errors.New("official Store credential key must be Base64 encoded")
	}
	defer clear(key)
	protection, err := shein.NewCredentialProtection(c.CredentialKeyID, key)
	if err != nil {
		return nil, nil, err
	}
	provider, err := shein.NewOfficialClient(shein.OfficialClientOptions{Application: storecenter.OfficialApplication{AppID: c.AppID, Version: c.Version, CallbackURL: c.CallbackURL}, AppSecret: strings.TrimSpace(string(secret)), APIOrigin: c.APIOrigin})
	if err != nil {
		return nil, nil, errors.New("official Store application configuration unavailable")
	}
	return provider, protection, nil
}
func readOfficialPrivateFile(ctx context.Context, path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1024 || info.Size() < 1 {
		return nil, errors.New("official Store credential file unavailable")
	}
	if runtime.GOOS == "windows" {
		if err := coreconfig.VerifyPrivateFiles(ctx, []string{path}); err != nil {
			return nil, errors.New("official Store credential file must be private")
		}
	} else if info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("official Store credential file must be private")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("official Store credential file unavailable")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 1025))
	if err != nil || len(data) > 1024 {
		return nil, errors.New("official Store credential file unavailable")
	}
	return data, nil
}
