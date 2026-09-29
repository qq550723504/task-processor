package currentapplication

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

type KnowledgeStorageConfig struct {
	Endpoint                             string `json:"endpoint"`
	Region                               string `json:"region"`
	Bucket                               string `json:"bucket"`
	AccessKeyID                          string `json:"accessKeyId"`
	SecretAccessKey                      string `json:"secretAccessKey"`
	Mode                                 string `json:"mode"`
	COSImmutableNonVersionedBucketPolicy bool   `json:"cosImmutableNonVersionedBucketPolicy,omitempty"`
}
type KnowledgeConfig struct {
	Enabled        bool                   `json:"enabled"`
	Database       DatabaseConfig         `json:"database"`
	Storage        KnowledgeStorageConfig `json:"storage"`
	ParserEndpoint string                 `json:"parserEndpoint"`
}

func (c *Config) validateKnowledge() error {
	if c.Knowledge == nil || !c.Knowledge.Enabled {
		return nil
	}
	k := c.Knowledge
	if err := k.Database.validate("knowledge.database"); err != nil {
		return err
	}
	if k.Database.Database != "knowledge" || k.Database.User != "knowledge_runtime" || k.Database.MaxConnections > 4 || k.Database.Host != c.SourceAccountDatabase.Host || k.Database.Port != c.SourceAccountDatabase.Port {
		return errors.New("knowledge requires its narrow database on the application PostgreSQL instance")
	}
	if k.Storage.Region == "" || k.Storage.Bucket == "" || k.Storage.AccessKeyID == "" || k.Storage.SecretAccessKey == "" || k.Storage.Mode != "aws" && k.Storage.Mode != "cos" || k.Storage.Mode == "cos" && !k.Storage.COSImmutableNonVersionedBucketPolicy {
		return errors.New("knowledge requires explicit private immutable object storage")
	}
	if k.Storage.Endpoint != "" {
		endpoint, err := url.Parse(k.Storage.Endpoint)
		if err != nil || endpoint.Hostname() == "" || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.Scheme != "https" && endpoint.Scheme != "http" {
			return errors.New("knowledge requires a valid private object endpoint")
		}
		ip := net.ParseIP(endpoint.Hostname())
		if endpoint.Scheme == "http" && !strings.EqualFold(endpoint.Hostname(), "knowledge-objects") && !strings.EqualFold(endpoint.Hostname(), "localhost") && (ip == nil || !ip.IsLoopback() && !ip.IsPrivate()) {
			return errors.New("knowledge external object endpoint requires HTTPS")
		}
	}
	u, err := url.Parse(k.ParserEndpoint)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" && u.Path != "/" {
		return errors.New("knowledge requires a private parser endpoint")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Hostname() != "knowledge-parser" && (ip == nil || !ip.IsLoopback() && !ip.IsPrivate()) {
		return errors.New("knowledge parser must use loopback or private network")
	}
	return nil
}
