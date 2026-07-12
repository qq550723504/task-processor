package studiobatch

type Selection struct {
	ProductID          int64
	VariantID          int64
	ParentProductID    int64
	PrototypeGroupID   int64
	LayerID            string
	DesignType         string
	PrintableWidth     int
	PrintableHeight    int
	TemplateImageURL   string
	MaskImageURL       string
	ProductSize        string
	PackagingSpec      string
	VariantLabel       string
	ProductName        string
	SelectedVariantIDs []int64
	Variants           []VariantSurface
}

type VariantSurface struct {
	VariantID        int64
	PrototypeGroupID int64
	LayerID          string
	TemplateImageURL string
	MaskImageURL     string
}

type GroupedSelection struct {
	SelectionID string
	StoreID     int64
	Selection   Selection
}

type Item struct {
	ID               string
	TargetGroupKey   string
	TargetGroupLabel string
	GroupMode        string
	SelectionIDs     []string
}

type Design struct {
	ID               string
	BatchID          string
	ItemID           string
	TargetGroupKey   string
	TargetGroupLabel string
	Approved         bool
	ImageURL         string
}

type GateInput struct {
	BatchID        string
	BatchGroupMode string
	Candidate      Candidate
	Designs        []Design
	SelectionByID  map[string]GroupedSelection
	ItemSelections []GroupedSelection
}

type GateResult struct {
	Eligible   bool
	ReasonCode string
	Message    string
}

type EvaluationInput struct {
	TenantID                   string
	BatchID                    string
	BatchGroupMode             string
	BatchStoreID               int64
	BatchSelection             Selection
	Item                       Item
	Design                     Design
	ResolvedSelections         []GroupedSelection
	ExplicitSelectionOwnership bool
	FallbackSelection          GroupedSelection
}

type Candidate struct {
	Design                   Design
	Item                     Item
	Selection                GroupedSelection
	SelectionSnapshot        Selection
	SelectionID              string
	CompatibilityFingerprint string
	CandidateKey             string
	StoreID                  int64
	StyleID                  string
	Title                    string
}

type Rejection struct {
	DesignID    string
	ItemID      string
	SelectionID string
	ReasonCode  string
	Message     string
}

type EvaluationResult struct {
	Candidates []Candidate
	Rejections []Rejection
}
