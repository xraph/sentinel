package api

import (
	"errors"

	"github.com/xraph/forge"

	"github.com/xraph/sentinel"
)

// mapStoreError translates domain errors to Forge HTTP errors.
func mapStoreError(err error) error {
	if isNotFound(err) {
		return forge.NotFound(err.Error())
	}
	if errors.Is(err, sentinel.ErrSuiteAlreadyExists) {
		return forge.BadRequest(err.Error())
	}
	if errors.Is(err, sentinel.ErrEmptyInput) || errors.Is(err, sentinel.ErrNoTarget) || errors.Is(err, sentinel.ErrNoScorers) {
		return forge.BadRequest(err.Error())
	}
	if errors.Is(err, sentinel.ErrInvalidState) {
		return forge.BadRequest(err.Error())
	}
	return err
}

// isNotFound returns true if the error is a not-found sentinel error.
func isNotFound(err error) bool {
	return errors.Is(err, sentinel.ErrSuiteNotFound) ||
		errors.Is(err, sentinel.ErrCaseNotFound) ||
		errors.Is(err, sentinel.ErrRunNotFound) ||
		errors.Is(err, sentinel.ErrBaselineNotFound) ||
		errors.Is(err, sentinel.ErrPromptVersionNotFound)
}
