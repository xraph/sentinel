// Package store defines the composite store interface for Sentinel.
// It embeds all subsystem store interfaces and adds lifecycle methods
// for migration, health checking, and shutdown.
package store

import (
	"context"

	"github.com/xraph/sentinel/baseline"
	"github.com/xraph/sentinel/evalrun"
	"github.com/xraph/sentinel/promptversion"
	"github.com/xraph/sentinel/suite"
	"github.com/xraph/sentinel/testcase"
)

// Store is the composite persistence interface for Sentinel.
// Implementations (Postgres, SQLite, Memory) satisfy all subsystem stores
// through a single concrete type.
type Store interface {
	suite.Store
	testcase.Store
	evalrun.Store
	baseline.Store
	promptversion.Store

	// Migrate runs database migrations. For in-memory stores this is a no-op.
	Migrate(ctx context.Context) error

	// Ping verifies the store connection is alive.
	Ping(ctx context.Context) error

	// Close releases store resources.
	Close() error
}
