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
	"runtime"
	"strings"
	"time"

	"task-processor/internal/app/httpapi"
	"task-processor/internal/app/localtrial"
	"task-processor/internal/authidentity"
	coreconfig "task-processor/internal/core/config"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	platformdatabase "task-processor/internal/platform/database"
)

type ownerConfig struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	User               string `json:"user"`
	Password           string `json:"password"`
	Database           string `json:"database"`
	MaxConnections     int    `json:"maxConnections"`
	MaxIdleConnections int    `json:"maxIdleConnections"`
}

func loadOwner(path string) (*platformdatabase.Config, error) {
	if !filepath.IsAbs(path) {
		return nil, errors.New("local trial owner config must be absolute")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16*1024 {
		return nil, errors.New("local trial owner config must be a bounded regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 || runtime.GOOS == "windows" && coreconfig.VerifyPrivateFiles(context.Background(), []string{path}) != nil {
		return nil, errors.New("local trial owner config must be private")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 16*1024+1))
	decoder.DisallowUnknownFields()
	var cfg ownerConfig
	if err := decoder.Decode(&cfg); err != nil {
		return nil, errors.New("invalid local trial owner config")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, errors.New("local trial owner config must contain one object")
	}
	if cfg.Host != "127.0.0.1" || cfg.Port != 5433 || cfg.User != "store_center_owner" || cfg.Database != "store_center" ||
		cfg.Password == "" || len(cfg.Password) > 1024 || strings.ContainsAny(cfg.Password, " =\\'\t\r\n\v\f\x00") ||
		cfg.MaxConnections < 1 || cfg.MaxConnections > 2 || cfg.MaxIdleConnections < 0 || cfg.MaxIdleConnections > cfg.MaxConnections {
		return nil, errors.New("local trial requires the isolated Store Center schema owner")
	}
	return &platformdatabase.Config{Host: cfg.Host, Port: cfg.Port, User: cfg.User, Password: cfg.Password, Database: cfg.Database,
		MaxConnections: cfg.MaxConnections, MaxIdleConnections: cfg.MaxIdleConnections}, nil
}

func execute(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("issue36-local-trial-init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	mode := flags.String("mode", "", "schema or seed")
	path := flags.String("config", "", "absolute private Store Center owner config")
	organizationID := flags.String("organization-id", "", "synthetic trial organization from local ZITADEL")
	actorID := flags.String("actor-id", "", "synthetic trial user from local ZITADEL")
	sampleOutput := flags.String("sample-output", "", "absolute private JSON path for the synthetic sample IDs")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (*mode != "schema" && *mode != "seed") || output == nil {
		return errors.New("invalid local trial init arguments")
	}
	if *mode == "seed" && (!authidentity.IsBoundedIdentifier(*organizationID) || !authidentity.IsBoundedIdentifier(*actorID) || !filepath.IsAbs(*sampleOutput)) ||
		*mode == "schema" && (*organizationID != "" || *actorID != "" || *sampleOutput != "") {
		return errors.New("invalid local trial sample identity")
	}
	cfg, err := loadOwner(*path)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := platformdatabase.OpenExistingWritableContext(ctx, cfg)
	if err != nil {
		return errors.New("local trial Store Center owner database unavailable")
	}
	defer platformdatabase.Close(db)
	if *mode == "schema" {
		if err := httpapi.InstallProductReviewSchema(db); err != nil {
			return fmt.Errorf("install trial Review and Catalog schema: %w", err)
		}
		if err := assetstore.AutoMigrate(db); err != nil {
			return fmt.Errorf("install trial ApprovedAsset schema: %w", err)
		}
		return nil
	}
	sample, err := localtrial.PrepareSample(ctx, db, *organizationID, *actorID)
	if err != nil {
		return fmt.Errorf("prepare local trial sample: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(*sampleOutput), ".issue36-sample-*.tmp")
	if err != nil {
		return errors.New("create private trial sample record failed")
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if err := json.NewEncoder(file).Encode(sample); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), *sampleOutput); err != nil {
		return errors.New("publish private trial sample record failed")
	}
	_, err = fmt.Fprintln(output, "local trial sample prepared")
	return err
}

func main() {
	if err := execute(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
