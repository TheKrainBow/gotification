package discord

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

type createDMResponse struct {
	ID string `json:"id"`
}

type discordErrorResponse struct {
	Message string `json:"message"`
}

func (p *Provider) createDMChannel(ctx context.Context, userID string) (string, error) {
	var resp createDMResponse
	err := p.call(ctx, "/users/@me/channels", map[string]string{"recipient_id": userID}, &resp)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(resp.ID) == "" {
		return "", &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: errors.New("discord API returned empty dm channel id")}
	}
	return resp.ID, nil
}

func (p *Provider) sendMessage(ctx context.Context, channelID, message string) error {
	var ignored map[string]any
	return p.call(ctx, "/channels/"+channelID+"/messages", map[string]string{"content": message}, &ignored)
}

func (p *Provider) call(ctx context.Context, path string, body any, out any) error {
	req, err := httpx.JSONRequest(ctx, http.MethodPost, p.baseURL+path, body, map[string]string{
		"Authorization": "Bot " + p.token,
	})
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

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: err}
	}

	if resp.StatusCode >= 400 {
		var derr discordErrorResponse
		_ = json.Unmarshal(bodyBytes, &derr)
		msg := derr.Message
		if msg == "" {
			msg = string(bodyBytes)
		}
		return classifyDiscordAPIError(resp.StatusCode, msg, parseDiscordRetryAfter(resp.Header))
	}

	if len(bodyBytes) == 0 || out == nil {
		return nil
	}
	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: fmt.Errorf("decode discord response: %w", err)}
	}
	return nil
}

func classifyDiscordAPIError(status int, message string, retryAfter time.Duration) error {
	switch {
	case status == http.StatusTooManyRequests:
		return &notifyerr.Error{Kind: notifyerr.KindRateLimited, RetryAfter: retryAfter, Cause: errors.New(message)}
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return &notifyerr.Error{Kind: notifyerr.KindAuth, Cause: errors.New(message)}
	case status == http.StatusNotFound:
		return &notifyerr.Error{Kind: notifyerr.KindNotFound, Cause: errors.New(message)}
	case status >= 500:
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: errors.New(message)}
	default:
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New(message)}
	}
}

func parseDiscordRetryAfter(header http.Header) time.Duration {
	if v := strings.TrimSpace(header.Get("Retry-After")); v != "" {
		if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	if v := strings.TrimSpace(header.Get("X-RateLimit-Reset-After")); v != "" {
		if secFloat, err := strconv.ParseFloat(v, 64); err == nil && secFloat >= 0 {
			return time.Duration(secFloat * float64(time.Second))
		}
	}
	return 0
}
