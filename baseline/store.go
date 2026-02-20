package baseline

import (
	"context"

	"github.com/xraph/sentinel/id"
)

// Store defines persistence operations for baselines.
type Store interface {
	SaveBaseline(ctx context.Context, b *Baseline) error
	GetBaseline(ctx context.Context, baselineID id.BaselineID) (*Baseline, error)
	GetLatestBaseline(ctx context.Context, suiteID id.SuiteID) (*Baseline, error)
	ListBaselines(ctx context.Context, suiteID id.SuiteID) ([]*Baseline, error)
	DeleteBaseline(ctx context.Context, baselineID id.BaselineID) error
}
