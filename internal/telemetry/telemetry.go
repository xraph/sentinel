// Package telemetry holds the span helpers sentinel's packages share, so
// every span records an error the same way.
package telemetry

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// MarkError sets an error status with a short, fixed description and adds
// an exception event that carries only the error's type. The message never
// goes on the span: a model provider's or a driver's error text can quote
// the prompt, the output or a connection string.
func MarkError(span trace.Span, description string, err error) {
	span.SetStatus(codes.Error, description)
	span.AddEvent("exception", trace.WithAttributes(attribute.String("exception.type", ErrorType(err))))
}

// ErrorType names an error for exception.type: the context sentinels by
// name, anything else by the Go type of the innermost error in its chain.
func ErrorType(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "context.Canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "context.DeadlineExceeded"
	}
	for {
		next := errors.Unwrap(err)
		if next == nil {
			return fmt.Sprintf("%T", err)
		}
		err = next
	}
}
