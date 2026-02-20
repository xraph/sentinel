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
	ErrSuiteAlreadyExists = errors.New("sentinel: suite already exists")

	// State errors.
	ErrInvalidState = errors.New("sentinel: invalid state transition")
	ErrEmptyInput   = errors.New("sentinel: empty input")

	// Evaluation errors.
	ErrNoTarget  = errors.New("sentinel: no target configured")
	ErrNoScorers = errors.New("sentinel: no scorers configured")
)
