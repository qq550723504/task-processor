package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	coreconfig "task-processor/internal/core/config"
	persistence "task-processor/internal/integration/persistence/knowledge"
	platformdatabase "task-processor/internal/platform/database"
	"time"
)

func main() {
	path := flag.String("config", "", "absolute private knowledge schema-owner JSON config")
	flag.Parse()
	if err := run(*path); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("knowledge schema installed and narrow runtime grants verified")
}
func run(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := loadOwnerConfig(path, "knowledge_owner")
	if err != nil {
		return err
	}
	if cfg.Database != "knowledge" {
		return errors.New("knowledge requires dedicated knowledge database")
	}
	db, err := platformdatabase.OpenExistingWritableContext(ctx, cfg)
	if err != nil {
		return errors.New("open knowledge owner database failed")
	}
	defer platformdatabase.Close(db)
	if err = migrateKnowledge(ctx, db); err != nil {
		return errors.New("install knowledge schema failed")
	}
	if err = persistence.GrantRuntime(ctx, db); err != nil {
		return errors.New("grant knowledge runtime access failed")
	}
	return nil
}
func loadOwnerConfig(path, role string) (*platformdatabase.Config, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("schema-owner config must be an absolute private file")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16384 {
		return nil, errors.New("schema-owner config must be a bounded regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("schema-owner config must be private")
	}
	if runtime.GOOS == "windows" && coreconfig.VerifyPrivateFiles(context.Background(), []string{path}) != nil {
		return nil, errors.New("schema-owner config must be private")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var config struct {
		Host               string `json:"host"`
		Port               int    `json:"port"`
		User               string `json:"user"`
		Password           string `json:"password"`
		Database           string `json:"database"`
		MaxConnections     int    `json:"maxConnections"`
		MaxIdleConnections int    `json:"maxIdleConnections"`
	}
	decoder := json.NewDecoder(io.LimitReader(f, 16385))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, errors.New("invalid schema-owner JSON")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("schema-owner config must contain one object")
	}
	if config.Host != "127.0.0.1" || config.Port < 1 || config.Port > 65535 || config.User != role || config.Password == "" || len(config.Password) > 1024 || strings.ContainsAny(config.Password, " =\\'\t\r\n\v\f\x00") || !regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]{0,62}$`).MatchString(config.Database) || config.MaxConnections < 1 || config.MaxConnections > 2 || config.MaxIdleConnections < 0 || config.MaxIdleConnections > config.MaxConnections {
		return nil, errors.New("schema-owner requires explicit loopback target, owner role and bounded credentials")
	}
	return &platformdatabase.Config{Host: config.Host, Port: config.Port, User: config.User, Password: config.Password, Database: config.Database, MaxConnections: config.MaxConnections, MaxIdleConnections: config.MaxIdleConnections}, nil
}
