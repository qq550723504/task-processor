package listingruntime

type ImportTask struct {
	ID              int64
	TenantID        int64
	StoreID         int64
	Platform        string
	Region          string
	CategoryID      int64
	ProductID       string
	Status          int16
	ErrorMessage    string
	RetryCount      int
	MaxRetryCount   int
	Priority        int
	CreateTime      int64
	PublishedTime   int64
	Creator         string
	StatusKey       string
	CanonicalStatus string
}

type StoreInfo struct {
	ID                       int64
	TenantID                 int64
	StoreID                  string
	Username                 string
	Platform                 string
	Name                     string
	Region                   string
	ShopType                 string
	LoginURL                 string
	Proxy                    string
	PriceType                string
	DailyLimit               *int
	DailyLimitType           string
	EnableDraft              *bool
	EnableAutoListing        *bool
	FixedStockCount          *int
	SkuGenerateStrategy      string
	Prefix                   string
	Suffix                   string
	EnableBrandAuthorization *bool
	AuthorizedBrandCode      string
	AuthorizedBrandName      string
}

type ScheduledTaskConfig struct {
	TenantID        int64
	StoreID         int64
	Platform        string
	TaskType        string
	Enabled         bool
	IntervalSeconds int
}

type StorePauseStatusDetail struct {
	Paused     bool
	PauseType  string
	Reason     string
	TTLSeconds int64
}

type TaskStatusUpdate struct {
	ID                    int64
	Status                int16
	ErrorMessage          string
	ReasonCode            string
	Stage                 string
	ExpectedCurrentStatus *int16
	RetryCount            *int
	Priority              *int
}

type ProductImportMapping struct {
	ID                      int64
	ImportTaskID            int64
	StoreID                 int64
	Platform                string
	Region                  string
	ProductID               string
	ParentProductID         *string
	SKU                     *string
	PlatformProductID       *string
	PlatformParentProductID *string
	CostPrice               float64
	FilterRuleID            int64
	FilterRuleRange         *string
	ProfitRuleID            int64
	SalePriceMultiplier     *float64
	DiscountPriceMultiplier *float64
	Status                  int16
	Remark                  *string
	TenantID                int64
}

type ProductImportMappingUpsert struct {
	ID                      *int64
	TenantID                int64
	ImportTaskID            int64
	StoreID                 int64
	Platform                string
	Region                  string
	ProductID               string
	ParentProductID         *string
	SKU                     *string
	PlatformProductID       *string
	PlatformParentProductID *string
	CostPrice               *float64
	FilterRuleID            *int64
	FilterRuleRange         *string
	ProfitRuleID            *int64
	SalePriceMultiplier     *float64
	DiscountPriceMultiplier *float64
	Status                  *int16
	Remark                  *string
}

type StoreService interface {
	GetStore(storeID int64) (*StoreInfo, error)
	GetStorePauseStatus(storeID int64) (bool, error)
	GetStorePauseStatusDetail(storeID int64) (*StorePauseStatusDetail, error)
	SetStorePauseStatus(storeID int64, pause bool, pauseType string) (bool, error)
}
