package webhook

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/TheKrainBow/gotification/internal/httpx"
	"github.com/TheKrainBow/gotification/internal/notifyerr"
	"github.com/TheKrainBow/gotification/internal/validate"
)

// Config configures the generic webhook provider.
type Config struct {
	HTTPClient apiHTTPClient
	Timeout    time.Duration
	Headers    map[string]string
}

// Provider sends JSON payloads to arbitrary webhook endpoints.
type Provider struct {
	client  apiHTTPClient
	headers map[string]string
}

// NewProvider creates a webhook provider.
func NewProvider(cfg Config) (*Provider, error) {
	client := cfg.HTTPClient
	if client == nil {
		client = httpx.NewClient(cfg.Timeout)
	}
	headers := make(map[string]string, len(cfg.Headers))
	for k, v := range cfg.Headers {
		headers[k] = v
	}
	return &Provider{client: client, headers: headers}, nil
}

// Send posts a text payload to endpoint.
func (p *Provider) Send(ctx context.Context, endpoint string, message string) error {
	if !validate.HTTPURL(endpoint) {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("invalid webhook endpoint URL")}
	}
	if strings.TrimSpace(message) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("webhook message cannot be empty")}
	}
	return p.post(ctx, endpoint, message)
}
