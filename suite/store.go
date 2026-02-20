package suite

import (
	"context"

	"github.com/xraph/sentinel/id"
)

// Store defines persistence operations for evaluation suites.
type Store interface {
	CreateSuite(ctx context.Context, s *Suite) error
	GetSuite(ctx context.Context, suiteID id.SuiteID) (*Suite, error)
	GetSuiteByName(ctx context.Context, appID, name string) (*Suite, error)
	UpdateSuite(ctx context.Context, s *Suite) error
	DeleteSuite(ctx context.Context, suiteID id.SuiteID) error
	ListSuites(ctx context.Context, filter *ListFilter) ([]*Suite, error)
}
