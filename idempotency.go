package gotification

import (
	"errors"
	"strings"
	"time"
)

func (d *Dispatcher) beginIdempotency(key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.idemTTL <= 0 {
		return nil
	}

	now := time.Now()
	for k, until := range d.idemStore {
		if !until.IsZero() && !until.After(now) {
			delete(d.idemStore, k)
		}
	}

	if until, exists := d.idemStore[key]; exists {
		if until.IsZero() || until.After(now) {
			return &NotifyError{Kind: ErrInvalidInput, Cause: errors.New("duplicate idempotency key")}
		}
	}

	// In-flight marker; finalized in finishIdempotency.
	d.idemStore[key] = time.Time{}
	return nil
}

func (d *Dispatcher) finishIdempotency(key string, success bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.idemTTL <= 0 {
		delete(d.idemStore, key)
		return
	}
	if success {
		d.idemStore[key] = time.Now().Add(d.idemTTL)
		return
	}
	// Failed sends should not block retries.
	delete(d.idemStore, key)
}
