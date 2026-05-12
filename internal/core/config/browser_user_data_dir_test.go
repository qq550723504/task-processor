package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewViper_BindsBrowserUserDataDirEnvironmentVariable(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_BROWSER_USER_DATA_DIR", "./tmp/browser-profiles/env-1688")

	v := newViper()

	assert.Equal(t, "./tmp/browser-profiles/env-1688", v.GetString("browser.userDataDir"))
}

func TestLoadFromBytes_AppliesBrowserUserDataDirEnvironmentOverride(t *testing.T) {
	t.Setenv("TASK_PROCESSOR_BROWSER_USER_DATA_DIR", "./tmp/browser-profiles/from-env")

	cfg, err := LoadFromBytes([]byte(`
management:
  clientSecret: "test-secret"
  scopes: ["user.read"]
openai:
  apiKey: "test-key"
  model: "gemini-2.5-flash"
  baseURL: "https://api.example.test/v1"
  timeout: 30
browser:
  enabled: true
  browserPath: "./chrome/chrome.exe"
  userDataDir: "./tmp/browser-profiles/from-yaml"
  poolSize: 1
  viewportWidth: 1920
  viewportHeight: 1080
`))
	require.NoError(t, err)

	assert.Equal(t, "./tmp/browser-profiles/from-env", cfg.Browser.UserDataDir)
}
