package currentapplication

import "testing"

func knowledgeRuntimeTestConfig() *Config {
	c := runtimeTestConfig()
	db := c.SourceAccountDatabase
	db.Database = "knowledge"
	db.User = "knowledge_runtime"
	db.MaxConnections = 4
	c.Knowledge = &KnowledgeConfig{Enabled: true, Database: db, ParserEndpoint: "http://knowledge-parser:9998", Storage: KnowledgeStorageConfig{Region: "local", Bucket: "knowledge", AccessKeyID: "isolated-access", SecretAccessKey: "isolated-secret", Mode: "aws"}}
	return c
}

func TestKnowledgeRequiresDedicatedNarrowPoolAndPrivateParser(t *testing.T) {
	makeConfig := knowledgeRuntimeTestConfig
	if err := makeConfig().validate(); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*Config){
		"wide role":                  func(c *Config) { c.Knowledge.Database.User = "knowledge_owner" },
		"other instance":             func(c *Config) { c.Knowledge.Database.Port++ },
		"wide pool":                  func(c *Config) { c.Knowledge.Database.MaxConnections = 5 },
		"foreign parser":             func(c *Config) { c.Knowledge.ParserEndpoint = "http://example.com" },
		"parser credentials":         func(c *Config) { c.Knowledge.ParserEndpoint = "http://user:secret@knowledge-parser" },
		"missing storage credential": func(c *Config) { c.Knowledge.Storage.SecretAccessKey = "" },
	} {
		t.Run(name, func(t *testing.T) {
			c := makeConfig()
			change(c)
			if c.validate() == nil {
				t.Fatal("unsafe configuration admitted")
			}
		})
	}
	c := runtimeTestConfig()
	c.Knowledge = &KnowledgeConfig{Enabled: false}
	if c.validate() != nil {
		t.Fatal("disabled feature acquired dependencies")
	}
}

func TestKnowledgeObjectEndpointProtectsDocumentsAndCredentials(t *testing.T) {
	for _, endpoint := range []string{
		"http://public-host", "http://objects.example.com:9000", "http://8.8.8.8:9000",
		"http://knowledge-objects.example.com", "http://169.254.169.254",
		"ftp://objects.example.com", "https://user:secret@objects.example.com", "https://objects.example.com?token=value",
	} {
		t.Run(endpoint, func(t *testing.T) {
			c := knowledgeRuntimeTestConfig()
			c.Knowledge.Storage.Endpoint = endpoint
			if c.validate() == nil {
				t.Fatal("unsafe object endpoint admitted")
			}
		})
	}
	for _, endpoint := range []string{
		"", "https://objects.example.com", "http://knowledge-objects:9000", "http://localhost:9000",
		"http://127.0.0.1:9000", "http://[::1]:9000", "http://10.2.3.4:9000", "http://[fd00::1]:9000",
	} {
		t.Run(endpoint, func(t *testing.T) {
			c := knowledgeRuntimeTestConfig()
			c.Knowledge.Storage.Endpoint = endpoint
			if err := c.validate(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
