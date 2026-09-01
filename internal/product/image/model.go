package image

const (
	MaxInlineArtifactBytes          = 32 << 20
	MaxInlineArtifactAggregateBytes = 64 << 20
)

type Role string

const (
	RoleSource          Role = "source"
	RoleSubject         Role = "subject"
	RoleWhiteBackground Role = "white_background"
	RoleScene           Role = "scene"
)

type Asset struct {
	URL           string
	Bytes         []byte
	MediaType     string
	SourceURL     string
	SourceAssetID string
	Role          Role
	Width         int
	Height        int
	Operations    []string
}

type ProductContext struct {
	ProductKey  string
	Title       string
	ProductType string
	Attributes  map[string]string
}

type GenerationMetadata struct {
	Capability      string
	ModelFamily     string
	InvocationID    string
	PromptReference string
	PromptVersion   string
	Values          map[string]string
}

type Candidate struct {
	Asset    Asset
	Metadata GenerationMetadata
}

type ExtractRequest struct {
	Source  Asset
	Product ProductContext
}

type RenderRequest struct {
	Source  Asset
	Product ProductContext
}

type SceneRequest struct {
	Source          Asset
	Product         ProductContext
	Options         SceneOptions
	ProfileName     string
	StyleReferences []Asset
	MaximumOutputs  int
}

type ReviewRequest struct {
	Product    ProductContext
	Sources    []Asset
	Candidates []Candidate
}

type Review struct {
	Score            float64
	NeedsHumanReview bool
	Reasons          []string
}

type UsageQuoteRequest struct {
	Operation        string
	InputFingerprint string
	MaximumOutputs   int64
}

type UsageQuote struct {
	Operation            string
	RouteReference       string
	Model                string
	CredentialReference  string
	ConfigurationVersion string
	PricingVersion       string
	Fingerprint          string
	MaximumOutputs       int64
	MaximumModelCalls    int64
	MaximumCostMicros    int64
	CostUpperBoundKnown  bool
}

type ImageAudit struct {
	ImageURL       string
	HasOverlayText bool
	HasPromoBadge  bool
	HasLogo        bool
	PrimaryObject  string
	Issues         []string
}

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type IPRiskAssessment struct {
	Level   RiskLevel
	Score   float64
	Reasons []string
}
