package testcase

import (
	"context"

	"github.com/xraph/sentinel/id"
)

// Store defines persistence operations for test cases.
type Store interface {
	CreateCase(ctx context.Context, tc *Case) error
	CreateCaseBatch(ctx context.Context, cases []*Case) error
	GetCase(ctx context.Context, caseID id.CaseID) (*Case, error)
	UpdateCase(ctx context.Context, tc *Case) error
	DeleteCase(ctx context.Context, caseID id.CaseID) error
	ListCases(ctx context.Context, suiteID id.SuiteID) ([]*Case, error)
	CountCases(ctx context.Context, suiteID id.SuiteID) (int64, error)
	// ImportCases is unused. Every backend implemented it as a stub, and
	// engine.ImportCases now parses and writes cases itself. It stays on the
	// interface so custom stores keep compiling.
	ImportCases(ctx context.Context, suiteID id.SuiteID, format string, data []byte) (int64, error)
}
