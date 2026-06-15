package sheinsync

type SheinCostPriceSource string

const (
	SheinCostPriceSourceNone   SheinCostPriceSource = "none"
	SheinCostPriceSourceAuto   SheinCostPriceSource = "auto"
	SheinCostPriceSourceManual SheinCostPriceSource = "manual"
)

func sheinFloat64Ptr(v float64) *float64 {
	return &v
}
