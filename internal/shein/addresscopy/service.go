package addresscopy

import (
	"context"
	"fmt"

	"task-processor/internal/infra/clients/management"
	"task-processor/internal/shein/api/warehouse"
	sheinclient "task-processor/internal/shein/client"
	sheinmanagedclient "task-processor/internal/shein/managedclient"

	"github.com/sirupsen/logrus"
)

type Service struct {
	managementClient *management.ClientManager
	logger           *logrus.Entry
}

func NewService(managementClient *management.ClientManager, logger *logrus.Logger) *Service {
	entry := logrus.NewEntry(logger)
	if logger == nil {
		entry = logrus.NewEntry(logrus.New())
	}
	return &Service{
		managementClient: managementClient,
		logger:           entry.WithField("component", "shein-address-copy"),
	}
}

func (s *Service) Copy(_ context.Context, req CopyRequest) (*CopyResult, error) {
	if req.SourceStoreID <= 0 || req.TargetStoreID <= 0 {
		return nil, fmt.Errorf("source_store_id and target_store_id are required")
	}
	if req.SourceStoreID == req.TargetStoreID {
		return nil, fmt.Errorf("source_store_id and target_store_id must be different")
	}

	sourceClient, err := s.newWarehouseClient(req.SourceStoreID)
	if err != nil {
		return nil, fmt.Errorf("init source store client: %w", err)
	}
	targetClient, err := s.newWarehouseClient(req.TargetStoreID)
	if err != nil {
		return nil, fmt.Errorf("init target store client: %w", err)
	}

	sourceAddresses, err := sourceClient.ListStoreAddresses(2)
	if err != nil {
		return nil, fmt.Errorf("load source addresses: %w", err)
	}
	targetAddresses, err := targetClient.ListStoreAddresses(2)
	if err != nil {
		return nil, fmt.Errorf("load target addresses: %w", err)
	}

	result := &CopyResult{
		SourceStoreID: req.SourceStoreID,
		TargetStoreID: req.TargetStoreID,
		Total:         len(sourceAddresses.Addresses),
		Items:         make([]CopyItemResult, 0, len(sourceAddresses.Addresses)),
	}

	existing := make(map[string]struct{}, len(targetAddresses.Addresses))
	for i := range targetAddresses.Addresses {
		key := duplicateKeyForStoreAddress(targetAddresses.Addresses[i])
		if key != "" {
			existing[key] = struct{}{}
		}
	}

	for i := range sourceAddresses.Addresses {
		address := &sourceAddresses.Addresses[i]
		item := CopyItemResult{
			Address:       address,
			WarehouseName: address.WarehouseName,
		}

		key := duplicateKey(address)
		if _, ok := existing[key]; ok {
			item.Action = "skipped"
			item.Reason = "already exists"
			result.Skipped++
			result.Items = append(result.Items, item)
			continue
		}

		checkInfo, err := targetClient.CheckStoreAddress(&warehouse.StoreAddressCheckRequest{
			AddressLeafID:      addressLeafID(address),
			Address1:           address.Address1,
			PostCode:           address.PostCode,
			QueryLatLngAddress: 2,
		})
		if err != nil {
			item.Action = "failed"
			item.Reason = err.Error()
			result.Failed++
			result.Items = append(result.Items, item)
			continue
		}

		addReq, err := buildAddRequest(address, checkInfo)
		if err != nil {
			item.Action = "failed"
			item.Reason = err.Error()
			result.Failed++
			result.Items = append(result.Items, item)
			continue
		}

		if req.DryRun {
			item.Action = "dry_run"
			item.Reason = fmt.Sprintf("ready to copy to sites=%v", addReq.BindSites)
			result.Copied++
			existing[key] = struct{}{}
			result.Items = append(result.Items, item)
			continue
		}

		if err := targetClient.AddStoreAddress(addReq); err != nil {
			item.Action = "failed"
			item.Reason = err.Error()
			result.Failed++
			result.Items = append(result.Items, item)
			continue
		}

		item.Action = "copied"
		item.Reason = fmt.Sprintf("copied to sites=%v", addReq.BindSites)
		result.Copied++
		existing[key] = struct{}{}
		result.Items = append(result.Items, item)
	}

	return result, nil
}

func (s *Service) newWarehouseClient(storeID int64) (*warehouse.Client, error) {
	if s.managementClient == nil {
		return nil, fmt.Errorf("management client is nil")
	}

	apiClient := sheinmanagedclient.NewAPIClient(storeID, s.managementClient)
	if !apiClient.HasCookies() {
		return nil, fmt.Errorf("store %d has no shein cookies", storeID)
	}

	baseClient := sheinclient.NewBaseAPIClient(
		apiClient.GetBaseURL(),
		apiClient.GetTenantID(),
		storeID,
		apiClient.GetHTTPClient(),
	)
	return warehouse.NewClient(baseClient), nil
}
