package sentinel

import "errors"

var (
	// Store errors.
	ErrNoStore         = errors.New("sentinel: no store configured")
	ErrStoreClosed     = errors.New("sentinel: store closed")
	ErrMigrationFailed = errors.New("sentinel: migration failed")

	// Not found errors.
	ErrSuiteNotFound         = errors.New("sentinel: suite not found")
	ErrCaseNotFound          = errors.New("sentinel: case not found")
	ErrRunNotFound           = errors.New("sentinel: run not found")
	ErrBaselineNotFound      = errors.New("sentinel: baseline not found")
	ErrPromptVersionNotFound = errors.New("sentinel: prompt version not found")

	// Conflict errors.
	ErrSuiteAlreadyExists  = errors.New("sentinel: suite already exists")
	ErrPromptVersionExists = errors.New("sentinel: prompt version already exists")

	// State errors.
	ErrInvalidState = errors.New("sentinel: invalid state transition")
	ErrRunCancelled = errors.New("sentinel: run cancelled")
	ErrEmptyInput   = errors.New("sentinel: empty input")

	// Evaluation errors.
	ErrNoTarget  = errors.New("sentinel: no target configured")
	ErrNoScorers = errors.New("sentinel: no scorers configured")

	ErrUnknownTarget     = errors.New("sentinel: unknown target")
	ErrUnknownScorer     = errors.New("sentinel: unknown scorer")
	ErrInvalidInput      = errors.New("sentinel: invalid input")
	ErrUnsupportedFormat = errors.New("sentinel: unsupported format")
)
