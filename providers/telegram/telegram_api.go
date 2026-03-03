package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/TheKrainBow/gotification/internal/httpx"
	"github.com/TheKrainBow/gotification/internal/notifyerr"
)

type apiHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type sendMessageRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	ErrorCode   int    `json:"error_code"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

func (p *Provider) sendMessage(ctx context.Context, chatID, text string) error {
	var resp apiResponse
	err := p.call(ctx, "/sendMessage", sendMessageRequest{ChatID: chatID, Text: text}, &resp)
	if err != nil {
		return err
	}
	if !resp.OK {
		return classifyTelegramError(resp.ErrorCode, resp.Description, time.Duration(resp.Parameters.RetryAfter)*time.Second)
	}
	return nil
}

func (p *Provider) call(ctx context.Context, path string, body any, out any) error {
	req, err := httpx.JSONRequest(ctx, http.MethodPost, p.baseURL+path, body, nil)
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
		var payload apiResponse
		if json.Unmarshal(bodyBytes, &payload) == nil {
			if payload.Description != "" {
				return classifyTelegramError(payload.ErrorCode, payload.Description, time.Duration(payload.Parameters.RetryAfter)*time.Second)
			}
		}
		return classifyTelegramError(resp.StatusCode, string(bodyBytes), 0)
	}

	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: fmt.Errorf("decode telegram response: %w", err)}
	}
	return nil
}

func classifyTelegramError(code int, description string, retryAfter time.Duration) error {
	description = strings.TrimSpace(description)
	if description == "" {
		description = "telegram error"
	}
	switch {
	case code == http.StatusTooManyRequests:
		return &notifyerr.Error{Kind: notifyerr.KindRateLimited, RetryAfter: retryAfter, Cause: errors.New(description)}
	case code == http.StatusUnauthorized || code == http.StatusForbidden:
		return &notifyerr.Error{Kind: notifyerr.KindAuth, Cause: errors.New(description)}
	case code == http.StatusNotFound:
		return &notifyerr.Error{Kind: notifyerr.KindNotFound, Cause: errors.New(description)}
	case code >= 500:
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: errors.New(description)}
	default:
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New(description)}
	}
}
