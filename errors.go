package gotification

import (
	"errors"
	"fmt"
	"time"

	"github.com/TheKrainBow/gotification/internal/notifyerr"
)

// ErrKind classifies dispatch errors for caller-side retry decisions.
type ErrKind = notifyerr.Kind

const (
	ErrInvalidInput ErrKind = notifyerr.KindInvalidInput
	ErrAuth         ErrKind = notifyerr.KindAuth
	ErrNotFound     ErrKind = notifyerr.KindNotFound
	ErrRateLimited  ErrKind = notifyerr.KindRateLimited
	ErrTemporary    ErrKind = notifyerr.KindTemporary
)

// NotifyError describes a channel/provider aware error returned by Send.
type NotifyError struct {
	Kind       ErrKind
	Channel    Channel
	Provider   string
	Dest       Destination
	RetryAfter time.Duration
	Cause      error
}

func (e *NotifyError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Cause == nil {
		return fmt.Sprintf("notify error kind=%s channel=%s provider=%s dest=%s", e.Kind, e.Channel, e.Provider, e.Dest.ID)
	}
	return fmt.Sprintf("notify error kind=%s channel=%s provider=%s dest=%s: %v", e.Kind, e.Channel, e.Provider, e.Dest.ID, e.Cause)
}

func (e *NotifyError) Unwrap() error { return e.Cause }

func wrapProviderError(err error, ch Channel, provider string, dest Destination) error {
	if err == nil {
		return nil
	}
	var perr *notifyerr.Error
	if errors.As(err, &perr) {
		return &NotifyError{
			Kind:       perr.Kind,
			Channel:    ch,
			Provider:   provider,
			Dest:       dest,
			RetryAfter: perr.RetryAfter,
			Cause:      perr.Cause,
		}
	}
	return &NotifyError{
		Kind:     ErrTemporary,
		Channel:  ch,
		Provider: provider,
		Dest:     dest,
		Cause:    err,
	}
}

// Retryable reports whether err contains at least one retryable notification error.
func Retryable(err error) bool {
	if err == nil {
		return false
	}

	type multi interface{ Unwrap() []error }
	if m, ok := err.(multi); ok {
		for _, inner := range m.Unwrap() {
			if Retryable(inner) {
				return true
			}
		}
		return false
	}
	var ne *NotifyError
	if errors.As(err, &ne) {
		return ne.Kind == ErrRateLimited || ne.Kind == ErrTemporary
	}
	if inner := errors.Unwrap(err); inner != nil {
		return Retryable(inner)
	}
	return false
}
