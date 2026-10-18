package evalrun

import (
	"fmt"
	"math"

	"github.com/xraph/sentinel"
)

// Quality says whether a usage value came from an authoritative receipt.
type Quality string

const (
	// Unknown includes historical records without accounting provenance.
	Unknown Quality = "unknown"
	// Known identifies an authoritative measured value, including zero.
	Known Quality = "known"
	// Estimated identifies a value inferred from a rate or token estimate.
	Estimated Quality = "estimated"
)

// Usage keeps unknown values distinct from measured zero.
type Usage struct {
	Tokens        int64   `json:"tokens"`
	TokensQuality Quality `json:"tokens_quality,omitempty"`
	Cost          float64 `json:"cost"`
	CostQuality   Quality `json:"cost_quality,omitempty"`
	Currency      string  `json:"currency,omitempty"`
}

// CostKnown reports whether cost is measured rather than estimated or absent.
func (u Usage) CostKnown() bool { return u.CostQuality == Known }

func validQuality(q Quality) bool { return q == "" || q == Unknown || q == Known || q == Estimated }

// Validate checks numeric values and accounting provenance.
func (u Usage) Validate() error {
	if !validQuality(u.TokensQuality) || !validQuality(u.CostQuality) || u.Tokens < 0 || math.IsNaN(u.Cost) || math.IsInf(u.Cost, 0) || u.Cost < 0 {
		return fmt.Errorf("%w: invalid captured usage", sentinel.ErrInvalidInput)
	}
	if (u.CostQuality == Known || u.CostQuality == Estimated) && u.Currency == "" {
		return fmt.Errorf("%w: captured cost requires currency", sentinel.ErrInvalidInput)
	}
	return nil
}

// AggregateUsage combines receipts without turning missing accounting into zero.
// Different currencies cannot form one monetary total.
func AggregateUsage(samples []Usage) (Usage, error) {
	if len(samples) == 0 {
		return Usage{}, nil
	}
	result := Usage{TokensQuality: Known, CostQuality: Known}
	for _, sample := range samples {
		if err := sample.Validate(); err != nil {
			return Usage{}, err
		}
		if result.Tokens > math.MaxInt64-sample.Tokens {
			return Usage{}, fmt.Errorf("%w: token sum overflows", sentinel.ErrInvalidInput)
		}
		result.Tokens += sample.Tokens
		if sample.TokensQuality == "" || sample.TokensQuality == Unknown {
			result.TokensQuality = Unknown
		} else if sample.TokensQuality == Estimated && result.TokensQuality == Known {
			result.TokensQuality = Estimated
		}
		if sample.CostQuality == "" || sample.CostQuality == Unknown {
			result.CostQuality = Unknown
		} else if sample.CostQuality == Estimated && result.CostQuality == Known {
			result.CostQuality = Estimated
		}
		if sample.Currency != "" {
			if result.Currency != "" && result.Currency != sample.Currency {
				return Usage{}, fmt.Errorf("%w: judge currencies differ", sentinel.ErrInvalidInput)
			}
			result.Currency = sample.Currency
		}
		result.Cost += sample.Cost
	}
	if err := result.Validate(); err != nil {
		return Usage{}, err
	}
	return result, nil
}
