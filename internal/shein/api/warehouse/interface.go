package warehouse

type WarehouseAPI interface {
	GetWarehouses() (*WarehouseResponse, error)
	ListStoreAddresses(addressType int) (*StoreAddressListInfo, error)
	AddStoreAddress(request *StoreAddressAddRequest) error
	CheckStoreAddress(request *StoreAddressCheckRequest) (*StoreAddressCheckInfo, error)
}

// Warehouse 仓库信息
type Warehouse struct {
	WarehouseName   string   `json:"warehouse_name"`
	WarehouseCode   string   `json:"warehouse_code"`
	SaleCountryList []string `json:"sale_country_list"`
	WarehouseType   int      `json:"warehouse_type"`
}

// WarehouseResponse 仓库信息响应
type WarehouseResponse struct {
	Data []Warehouse `json:"data"`
	Meta struct {
		Count     int `json:"count"`
		CustomObj any `json:"customObj"`
	} `json:"meta"`
}

type StoreAddressListRequest struct {
	AddressType int `json:"addressType"`
}

type StoreAddress struct {
	ID                    int64           `json:"id"`
	Country               string          `json:"country"`
	CountryID             int             `json:"countryId"`
	State                 string          `json:"state"`
	StateID               int             `json:"stateId"`
	City                  string          `json:"city"`
	CityID                int             `json:"cityId"`
	District              string          `json:"district"`
	DistrictID            int             `json:"districtId"`
	PostCode              string          `json:"postCode"`
	Address1              string          `json:"address1"`
	Address2              string          `json:"address2"`
	FirstName             string          `json:"firstName"`
	LastName              string          `json:"lastName"`
	FullName              string          `json:"fullName"`
	Phone                 string          `json:"phone"`
	FullAddress           string          `json:"fullAddress"`
	AddressType           int             `json:"addressType"`
	CollectionPatternType int             `json:"collectionPatternType"`
	SellerEmail           string          `json:"sellerEmail"`
	WarehouseName         string          `json:"warehouseName"`
	WarehouseCode         string          `json:"warehouseCode"`
	WarehouseType         int             `json:"warehouseType"`
	CollectionMark        int             `json:"collectionMark"`
	StoreSiteInfos        []StoreSiteInfo `json:"storeSiteInfos"`
}

type StoreSiteInfo struct {
	Site             string   `json:"site"`
	SiteStatus       int      `json:"siteStatus"`
	DefaultWarehouse int      `json:"defaultWarehouse"`
	SaleCountries    []string `json:"saleCountries"`
}

type StoreAddressListInfo struct {
	Addresses []StoreAddress `json:"addresses"`
}

type StoreAddressAddRequest struct {
	Address1              string                 `json:"address1"`
	AddressLeafID         string                 `json:"addressLeafId"`
	FirstName             string                 `json:"firstName"`
	LastName              string                 `json:"lastName"`
	Phone                 string                 `json:"phone"`
	AddressType           int                    `json:"addressType"`
	PostCode              string                 `json:"postCode"`
	CollectionPatternType int                    `json:"collectionPatternType"`
	BindSites             []string               `json:"bindSites"`
	SellerEmail           string                 `json:"sellerEmail"`
	Lat                   string                 `json:"lat"`
	Lng                   string                 `json:"lng"`
	WarehouseName         string                 `json:"warehouseName"`
	WarehouseType         int                    `json:"warehouseType"`
	IsRefundAddress       string                 `json:"isRefundAddress"`
	CollectionJudgeRecord *CollectionJudgeRecord `json:"collectionJudgeRecord,omitempty"`
	CollectionMark        int                    `json:"collectionMark"`
	ProviderInfoList      []ProviderInfo         `json:"providerInfoList,omitempty"`
	CheckResultUUID       string                 `json:"checkResultUUid"`
}

type StoreAddressCheckRequest struct {
	AddressLeafID      string `json:"addressLeafId"`
	Address1           string `json:"address1"`
	PostCode           string `json:"postCode"`
	QueryLatLngAddress int    `json:"queryLatLngAddress"`
}

type StoreAddressCheckInfo struct {
	ErrorMsg                 any                    `json:"errorMsg"`
	ErrorCode                any                    `json:"errorCode"`
	IsNeedInform             any                    `json:"isNeedInform"`
	CollectionList           []CollectionOption     `json:"collectionList"`
	CollectionPointResponses []any                  `json:"collectionPointResponses"`
	CollectionJudgeRecord    *CollectionJudgeRecord `json:"collectionJudgeRecord"`
	GoogleValidateResult     any                    `json:"googleValidateResult"`
	LatLng                   *LatLng                `json:"latLng"`
	MultipleFullAddress      []any                  `json:"multipleFullAddress"`
	CollectionMark           int                    `json:"collectionMark"`
	ProviderInfoList         []ProviderInfo         `json:"providerInfoList"`
	CheckResultUUID          string                 `json:"checkResultUUid"`
}

type CollectionOption struct {
	Collection     int    `json:"collection"`
	CollectionName string `json:"collectionName"`
}

type LatLng struct {
	Lat string `json:"lat"`
	Lng string `json:"lng"`
}

type CollectionJudgeRecord struct {
	TriggerReason               int    `json:"triggerReason"`
	InCollectionRange           int    `json:"inCollectionRange"`
	CollectionType              []int  `json:"collectionType,omitempty"`
	Operator                    string `json:"operator"`
	OperateTime                 string `json:"operateTime"`
	MaxAdo                      int    `json:"maxAdo"`
	PastDays                    any    `json:"pastDays,omitempty"`
	ForecastAvg                 any    `json:"forecastAvg,omitempty"`
	GroupMaxStore               any    `json:"groupMaxStore,omitempty"`
	GroupMaxAvg                 any    `json:"groupMaxAvg,omitempty"`
	GroupTotalAvg               any    `json:"groupTotalAvg,omitempty"`
	MultipleShopStore           int    `json:"multipleShopStore"`
	MultipleShopChange          any    `json:"multipleShopChange,omitempty"`
	CollectionPatternChangeTime any    `json:"collectionPatternChangeTime,omitempty"`
	ConfirmType                 any    `json:"confirmType,omitempty"`
	MultipleUUID                any    `json:"multipleUuid,omitempty"`
}

type ProviderInfo struct {
	ProviderID   int    `json:"providerId"`
	ProviderName string `json:"providerName"`
	IsMatchAdo   int    `json:"isMatchAdo"`
	AdoStandard  any    `json:"adoStandard,omitempty"`
}
