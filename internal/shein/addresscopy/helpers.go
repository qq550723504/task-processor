package addresscopy

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"task-processor/internal/shein/api/warehouse"
)

var nonDigitPattern = regexp.MustCompile(`\D+`)

func addressLeafID(address *warehouse.StoreAddress) string {
	if address == nil {
		return ""
	}
	if address.DistrictID > 0 {
		return fmt.Sprint(address.DistrictID)
	}
	if address.CityID > 0 {
		return fmt.Sprint(address.CityID)
	}
	if address.StateID > 0 {
		return fmt.Sprint(address.StateID)
	}
	return ""
}

func bindSites(address *warehouse.StoreAddress) []string {
	if address == nil || len(address.StoreSiteInfos) == 0 {
		return nil
	}

	sites := make([]string, 0, len(address.StoreSiteInfos))
	seen := make(map[string]struct{}, len(address.StoreSiteInfos))
	for _, item := range address.StoreSiteInfos {
		site := strings.TrimSpace(item.Site)
		if site == "" {
			continue
		}
		if _, ok := seen[site]; ok {
			continue
		}
		seen[site] = struct{}{}
		sites = append(sites, site)
	}

	slices.Sort(sites)
	return sites
}

func duplicateKey(address *warehouse.StoreAddress) string {
	if address == nil {
		return ""
	}
	return duplicateKeyParts(address.Address1, address.PostCode, address.WarehouseName, bindSites(address))
}

func duplicateKeyParts(address1, postCode, warehouseName string, sites []string) string {
	return strings.ToLower(strings.Join([]string{
		strings.TrimSpace(address1),
		strings.TrimSpace(postCode),
		strings.TrimSpace(warehouseName),
		strings.Join(sites, ","),
	}, "|"))
}

func duplicateKeyForStoreAddress(address warehouse.StoreAddress) string {
	return duplicateKeyParts(address.Address1, address.PostCode, address.WarehouseName, bindSites(&address))
}

func normalizePhone(address *warehouse.StoreAddress) string {
	if address == nil {
		return ""
	}

	digits := nonDigitPattern.ReplaceAllString(address.Phone, "")
	if digits == "" {
		return ""
	}

	if address.CountryID == 226 && len(digits) == 11 && strings.HasPrefix(digits, "1") {
		return digits[1:]
	}
	return digits
}

func selectCollectionPatternType(source int, checkInfo *warehouse.StoreAddressCheckInfo) (int, error) {
	if source > 0 && checkInfo != nil {
		for _, item := range checkInfo.CollectionList {
			if item.Collection == source {
				return source, nil
			}
		}
	}

	if checkInfo != nil && len(checkInfo.CollectionList) == 1 {
		return checkInfo.CollectionList[0].Collection, nil
	}

	if source > 0 && checkInfo != nil && len(checkInfo.CollectionList) == 0 {
		return source, nil
	}

	return 0, fmt.Errorf("cannot resolve collection pattern type")
}

func buildAddRequest(address *warehouse.StoreAddress, checkInfo *warehouse.StoreAddressCheckInfo) (*warehouse.StoreAddressAddRequest, error) {
	if address == nil {
		return nil, fmt.Errorf("source address is nil")
	}
	if checkInfo == nil {
		return nil, fmt.Errorf("check address response is nil")
	}

	leafID := addressLeafID(address)
	if leafID == "" {
		return nil, fmt.Errorf("address %q has no address leaf id", address.WarehouseName)
	}

	collectionPatternType, err := selectCollectionPatternType(address.CollectionPatternType, checkInfo)
	if err != nil {
		return nil, fmt.Errorf("address %q: %w", address.WarehouseName, err)
	}

	if checkInfo.LatLng == nil {
		return nil, fmt.Errorf("address %q check response missing lat/lng", address.WarehouseName)
	}

	req := &warehouse.StoreAddressAddRequest{
		Address1:              address.Address1,
		AddressLeafID:         leafID,
		FirstName:             address.FirstName,
		LastName:              address.LastName,
		Phone:                 normalizePhone(address),
		AddressType:           address.AddressType,
		PostCode:              address.PostCode,
		CollectionPatternType: collectionPatternType,
		BindSites:             bindSites(address),
		SellerEmail:           address.SellerEmail,
		Lat:                   checkInfo.LatLng.Lat,
		Lng:                   checkInfo.LatLng.Lng,
		WarehouseName:         address.WarehouseName,
		WarehouseType:         address.WarehouseType,
		IsRefundAddress:       "2",
		CollectionJudgeRecord: checkInfo.CollectionJudgeRecord,
		CollectionMark:        checkInfo.CollectionMark,
		ProviderInfoList:      checkInfo.ProviderInfoList,
		CheckResultUUID:       checkInfo.CheckResultUUID,
	}

	if req.CollectionMark == 0 {
		req.CollectionMark = address.CollectionMark
	}
	if len(req.BindSites) == 0 {
		return nil, fmt.Errorf("address %q has no bind sites", address.WarehouseName)
	}
	if req.Phone == "" {
		return nil, fmt.Errorf("address %q has empty phone", address.WarehouseName)
	}

	return req, nil
}
