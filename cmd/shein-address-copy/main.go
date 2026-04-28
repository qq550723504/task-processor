package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/auth"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/shein/addresscopy"

	"github.com/sirupsen/logrus"
)

func main() {
	configPath := flag.String("config", "config/config-dev.yaml", "配置文件路径")
	sourceStoreID := flag.Int64("source-store-id", 0, "源店铺ID")
	targetStoreID := flag.Int64("target-store-id", 0, "目标店铺ID")
	dryRun := flag.Bool("dry-run", false, "仅预览，不实际创建地址")
	flag.Parse()

	if *sourceStoreID <= 0 || *targetStoreID <= 0 {
		log.Fatal("source-store-id and target-store-id are required")
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	cfg, err := config.LoadConfigWithFallback(*configPath, logger)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	tenantID := cfg.Management.TenantID
	if tenantID == "" {
		tenantID = "1"
	}
	authClient := auth.NewClientCredentialsAuthClient(
		cfg.Management.BaseURL,
		cfg.Management.ClientID,
		cfg.Management.ClientSecret,
		tenantID,
		logger,
	)
	accessToken, err := authClient.GetAccessToken()
	if err != nil {
		log.Fatalf("get management access token failed: %v", err)
	}

	managementClient := management.NewClientManager(&cfg.Management)
	managementClient.GetClient()
	managementClient.SetUserToken(accessToken, tenantID)
	service := addresscopy.NewService(managementClient, logger)

	result, err := service.Copy(context.Background(), addresscopy.CopyRequest{
		SourceStoreID: *sourceStoreID,
		TargetStoreID: *targetStoreID,
		DryRun:        *dryRun,
	})
	if err != nil {
		log.Fatalf("copy addresses failed: %v", err)
	}

	fmt.Printf("source_store_id=%d target_store_id=%d total=%d copied=%d skipped=%d failed=%d dry_run=%v\n",
		result.SourceStoreID,
		result.TargetStoreID,
		result.Total,
		result.Copied,
		result.Skipped,
		result.Failed,
		*dryRun,
	)
	for _, item := range result.Items {
		fmt.Printf("[%s] %s - %s\n", item.Action, item.WarehouseName, item.Reason)
	}
}
