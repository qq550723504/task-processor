package aicapability

import (
	"errors"
	"math/big"
	"strings"
)

// Model retail points are separate from provider currency cost and prepaid
// resource purchase prices. No zero-value tariff admits a model dispatch.
type ModelPointTariff struct {
	PriceVersion                 string `json:"priceVersion"`
	InputPointsPerMillionTokens  int64  `json:"inputPointsPerMillionTokens"`
	OutputPointsPerMillionTokens int64  `json:"outputPointsPerMillionTokens"`
}

func (p ModelPointTariff) Valid() bool {
	return p.PriceVersion != "" && strings.TrimSpace(p.PriceVersion) == p.PriceVersion && len(p.PriceVersion) <= 128 && !strings.ContainsAny(p.PriceVersion, "\x00\r\n") && p.InputPointsPerMillionTokens > 0 && p.OutputPointsPerMillionTokens > 0
}
func (p ModelPointTariff) Points(prompt, completion int64) (int64, error) {
	if !p.Valid() || prompt < 0 || completion < 0 {
		return 0, errors.New("invalid model point price or usage")
	}
	sum := new(big.Int).Mul(big.NewInt(prompt), big.NewInt(p.InputPointsPerMillionTokens))
	sum.Add(sum, new(big.Int).Mul(big.NewInt(completion), big.NewInt(p.OutputPointsPerMillionTokens)))
	sum.Add(sum, big.NewInt(999999))
	sum.Div(sum, big.NewInt(1000000))
	if !sum.IsInt64() {
		return 0, errors.New("model point quantity overflow")
	}
	return sum.Int64(), nil
}
