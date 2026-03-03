package notifyerr

import (
	"fmt"
	"time"
)

// Kind classifies provider errors so caller-side code can decide retry behavior.
type Kind string

const (
	KindInvalidInput Kind = "invalid_input"
	KindAuth         Kind = "auth"
	KindNotFound     Kind = "not_found"
	KindRateLimited  Kind = "rate_limited"
	KindTemporary    Kind = "temporary"
)

// Error is returned by provider implementations.
type Error struct {
	Kind       Kind
	RetryAfter time.Duration
	Cause      error
}

func (e *Error) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause == nil {
		return fmt.Sprintf("provider error (%s)", e.Kind)
	}
	return fmt.Sprintf("provider error (%s): %v", e.Kind, e.Cause)
}

func (e *Error) Unwrap() error { return e.Cause }
