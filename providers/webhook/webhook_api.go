package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/TheKrainBow/gotification/internal/httpx"
	"github.com/TheKrainBow/gotification/internal/notifyerr"
)

type apiHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type payload struct {
	Text string `json:"text"`
}

func (p *Provider) post(ctx context.Context, endpoint, message string) error {
	headers := map[string]string{}
	for k, v := range p.headers {
		headers[k] = v
	}
	req, err := httpx.JSONRequest(ctx, http.MethodPost, endpoint, payload{Text: message}, headers)
	if err != nil {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: err}
	}

	resp, err := p.client.Do(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: err}
		}
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: err}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: err}
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return &notifyerr.Error{Kind: notifyerr.KindRateLimited, RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")), Cause: errors.New("webhook rate limited")}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return &notifyerr.Error{Kind: notifyerr.KindAuth, Cause: errors.New(string(body))}
	}
	if resp.StatusCode == http.StatusNotFound {
		return &notifyerr.Error{Kind: notifyerr.KindNotFound, Cause: errors.New(string(body))}
	}
	if resp.StatusCode >= 500 {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: errors.New(string(body))}
	}

	var decoded map[string]any
	if json.Unmarshal(body, &decoded) == nil {
		if msg, ok := decoded["error"].(string); ok {
			return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New(msg)}
		}
	}
	return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New(string(body))}
}

func parseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	secs, err := strconv.Atoi(value)
	if err != nil || secs < 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}
