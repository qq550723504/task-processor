package currentapplication

import "testing"

func TestKnowledgeRequiresDedicatedNarrowPoolAndPrivateParser(t *testing.T) {
	makeConfig := func() *Config {
		c := runtimeTestConfig()
		db := c.SourceAccountDatabase
		db.Database = "knowledge"
		db.User = "knowledge_runtime"
		db.MaxConnections = 4
		c.Knowledge = &KnowledgeConfig{Enabled: true, Database: db, ParserEndpoint: "http://knowledge-parser:9998", Storage: KnowledgeStorageConfig{Region: "local", Bucket: "knowledge", AccessKeyID: "isolated-access", SecretAccessKey: "isolated-secret", Mode: "aws"}}
		return c
	}
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
