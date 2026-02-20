package promptversion

import (
	"context"

	"github.com/xraph/sentinel/id"
)

// Store defines persistence operations for prompt versions.
type Store interface {
	CreatePromptVersion(ctx context.Context, pv *PromptVersion) error
	GetPromptVersion(ctx context.Context, pvID id.PromptVersionID) (*PromptVersion, error)
	ListPromptVersions(ctx context.Context, suiteID id.SuiteID) ([]*PromptVersion, error)
	GetCurrentPromptVersion(ctx context.Context, suiteID id.SuiteID) (*PromptVersion, error)
	SetCurrentPromptVersion(ctx context.Context, suiteID id.SuiteID, pvID id.PromptVersionID) error
}
