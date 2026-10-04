package contract

import (
	"errors"

	"github.com/xraph/forge"
	dashcontract "github.com/xraph/forge/extensions/dashboard/contract"

	"github.com/xraph/sentinel"
)

// mapError translates an engine or store error into a contract error. A
// *dashcontract.Error passes through unchanged. Anything unrecognised
// becomes INTERNAL with a generic message: an unknown error's text can carry
// a connection string, so it never reaches the client.
func mapError(err error) error {
	if err == nil {
		return nil
	}
	var ce *dashcontract.Error
	switch {
	case errors.As(err, &ce):
		return ce
	case errors.Is(err, sentinel.ErrSuiteNotFound):
		return notFound("suite not found")
	case errors.Is(err, sentinel.ErrCaseNotFound):
		return notFound("case not found")
	case errors.Is(err, sentinel.ErrRunNotFound):
		return notFound("run not found")
	case errors.Is(err, sentinel.ErrBaselineNotFound):
		return notFound("baseline not found")
	case errors.Is(err, sentinel.ErrPromptVersionNotFound):
		return notFound("prompt version not found")
	case errors.Is(err, sentinel.ErrSuiteAlreadyExists):
		return conflict("a suite with this name already exists")
	case errors.Is(err, sentinel.ErrPromptVersionExists):
		return conflict("another prompt version was created at the same moment; try again")
	case errors.Is(err, sentinel.ErrInvalidState):
		return conflict(err.Error())
	// These describe the caller's own input, in engine-written text that
	// carries no stored data, so the message goes back as written.
	case errors.Is(err, sentinel.ErrUnknownTarget),
		errors.Is(err, sentinel.ErrUnknownScorer),
		errors.Is(err, sentinel.ErrNoScorers),
		errors.Is(err, sentinel.ErrEmptyInput),
		errors.Is(err, sentinel.ErrInvalidInput),
		errors.Is(err, sentinel.ErrUnsupportedFormat):
		return badRequest(err.Error())
	case errors.Is(err, sentinel.ErrNoStore):
		return &dashcontract.Error{Code: dashcontract.CodeUnavailable, Message: "sentinel has no store configured"}
	default:
		return &dashcontract.Error{Code: dashcontract.CodeInternal, Message: "an internal error occurred"}
	}
}

// fail maps err and, when the result is INTERNAL, logs the underlying
// error with the intent, which is the only case an operator cannot
// diagnose from what the client sees.
func (d Deps) fail(intent string, err error) error {
	mapped := mapError(err)
	var ce *dashcontract.Error
	if d.Logger != nil && errors.As(mapped, &ce) && ce.Code == dashcontract.CodeInternal {
		d.Logger.Error("sentinel/contract: internal error answering intent", forge.F("intent", intent), forge.F("error", err))
	}
	return mapped
}

func badRequest(msg string) error {
	return &dashcontract.Error{Code: dashcontract.CodeBadRequest, Message: msg}
}

func notFound(msg string) error {
	return &dashcontract.Error{Code: dashcontract.CodeNotFound, Message: msg}
}

func conflict(msg string) error {
	return &dashcontract.Error{Code: dashcontract.CodeConflict, Message: msg}
}

func permissionDenied(msg string) error {
	return &dashcontract.Error{Code: dashcontract.CodePermissionDenied, Message: msg}
}
