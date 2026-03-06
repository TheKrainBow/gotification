package slack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/TheKrainBow/gotification/internal/httpx"
	"github.com/TheKrainBow/gotification/internal/notifyerr"
	"github.com/TheKrainBow/gotification/slackmsg"
)

type apiHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type openConversationResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	Channel struct {
		ID string `json:"id"`
	} `json:"channel"`
}

type postMessageResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

type usersListResponse struct {
	OK      bool   `json:"ok"`
	Error   string `json:"error"`
	Members []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		RealName string `json:"real_name"`
		Deleted  bool   `json:"deleted"`
		IsBot    bool   `json:"is_bot"`
		Profile  struct {
			DisplayName string `json:"display_name"`
		} `json:"profile"`
	} `json:"members"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

func (p *Provider) openConversation(ctx context.Context, userID string) (string, error) {
	var resp openConversationResponse
	err := p.call(ctx, "/conversations.open", map[string]string{"users": userID}, &resp)
	if err != nil {
		return "", err
	}
	if !resp.OK {
		return "", classifySlackAPIError(http.StatusOK, resp.Error, 0)
	}
	if strings.TrimSpace(resp.Channel.ID) == "" {
		return "", &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: errors.New("slack API returned empty dm channel id")}
	}
	return resp.Channel.ID, nil
}

func (p *Provider) postMessage(ctx context.Context, channelID string, message slackmsg.Message) error {
	var resp postMessageResponse
	err := p.call(ctx, "/chat.postMessage", struct {
		Channel        string                `json:"channel"`
		Text           string                `json:"text,omitempty"`
		Blocks         []slackmsg.Block      `json:"blocks,omitempty"`
		ThreadTS       string                `json:"thread_ts,omitempty"`
		ReplyBroadcast bool                  `json:"reply_broadcast,omitempty"`
		Attachments    []slackmsg.Attachment `json:"attachments,omitempty"`
	}{
		Channel:        channelID,
		Text:           message.Text,
		Blocks:         message.Blocks,
		ThreadTS:       message.ThreadTS,
		ReplyBroadcast: message.ReplyBroadcast,
		Attachments:    message.Attachments,
	}, &resp)
	if err != nil {
		return err
	}
	if !resp.OK {
		return classifySlackAPIError(http.StatusOK, resp.Error, 0)
	}
	return nil
}

func (p *Provider) postRawMessage(ctx context.Context, channelID string, payload json.RawMessage) error {
	body, err := injectRawChannel(payload, channelID)
	if err != nil {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: err}
	}

	var resp postMessageResponse
	if err := p.callRaw(ctx, "/chat.postMessage", body, &resp); err != nil {
		return err
	}
	if !resp.OK {
		return classifySlackAPIError(http.StatusOK, resp.Error, 0)
	}
	return nil
}

func (p *Provider) addReaction(ctx context.Context, channelID, messageTS, emoji string) error {
	var resp postMessageResponse
	err := p.call(ctx, "/reactions.add", struct {
		Channel   string `json:"channel"`
		Timestamp string `json:"timestamp"`
		Name      string `json:"name"`
	}{
		Channel:   channelID,
		Timestamp: messageTS,
		Name:      emoji,
	}, &resp)
	if err != nil {
		return err
	}
	if !resp.OK {
		return classifySlackAPIError(http.StatusOK, resp.Error, 0)
	}
	return nil
}

func (p *Provider) findUsersByName(ctx context.Context, query string) ([]string, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	cursor := ""
	seen := make(map[string]struct{})
	var out []string

	for {
		params := url.Values{}
		params.Set("limit", "200")
		if cursor != "" {
			params.Set("cursor", cursor)
		}
		var resp usersListResponse
		if err := p.callGET(ctx, "/users.list", params, &resp); err != nil {
			return nil, err
		}
		if !resp.OK {
			return nil, classifySlackAPIError(http.StatusOK, resp.Error, 0)
		}

		for _, member := range resp.Members {
			if member.Deleted || member.IsBot {
				continue
			}
			if !matchesSlackUserQuery(q, member.Name, member.RealName, member.Profile.DisplayName) {
				continue
			}
			if _, ok := seen[member.ID]; ok {
				continue
			}
			seen[member.ID] = struct{}{}
			out = append(out, member.ID)
		}

		cursor = strings.TrimSpace(resp.ResponseMetadata.NextCursor)
		if cursor == "" {
			break
		}
	}
	return out, nil
}

func matchesSlackUserQuery(query string, values ...string) bool {
	for _, v := range values {
		if strings.Contains(strings.ToLower(v), query) {
			return true
		}
	}
	return false
}

func injectRawChannel(payload json.RawMessage, channelID string) ([]byte, error) {
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		return nil, fmt.Errorf("invalid slack JSON payload: %w", err)
	}
	if body == nil {
		return nil, errors.New("slack raw payload must be a JSON object")
	}
	if existing, ok := body["channel"]; ok {
		if value, ok := existing.(string); ok && strings.TrimSpace(value) != "" && value != channelID {
			return nil, errors.New("slack raw payload already contains a different channel")
		}
	}
	body["channel"] = channelID
	return json.Marshal(body)
}

func (p *Provider) call(ctx context.Context, path string, body any, out any) error {
	req, err := httpx.JSONRequest(ctx, http.MethodPost, p.baseURL+path, body, map[string]string{
		"Authorization": "Bearer " + p.token,
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

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return classifySlackAPIError(resp.StatusCode, "rate_limited", retryAfter)
	}
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusOK {
		return classifySlackAPIError(resp.StatusCode, string(bodyBytes), 0)
	}

	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: fmt.Errorf("decode slack response: %w", err)}
	}
	return nil
}

func (p *Provider) callRaw(ctx context.Context, path string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, strings.NewReader(string(body)))
	if err != nil {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: err}
	}
	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Content-Type", "application/json")

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

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return classifySlackAPIError(resp.StatusCode, "rate_limited", retryAfter)
	}
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusOK {
		return classifySlackAPIError(resp.StatusCode, string(bodyBytes), 0)
	}

	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: fmt.Errorf("decode slack response: %w", err)}
	}
	return nil
}

func (p *Provider) callGET(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := p.baseURL + path
	if query != nil && len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: err}
	}
	req.Header.Set("Authorization", "Bearer "+p.token)

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

	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return classifySlackAPIError(resp.StatusCode, "rate_limited", retryAfter)
	}
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusOK {
		return classifySlackAPIError(resp.StatusCode, string(bodyBytes), 0)
	}
	if err := json.Unmarshal(bodyBytes, out); err != nil {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: fmt.Errorf("decode slack response: %w", err)}
	}
	return nil
}

func classifySlackAPIError(status int, apiError string, retryAfter time.Duration) error {
	errText := strings.ToLower(apiError)
	if status == http.StatusTooManyRequests {
		return &notifyerr.Error{Kind: notifyerr.KindRateLimited, RetryAfter: retryAfter, Cause: errors.New("slack rate limited")}
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden || strings.Contains(errText, "invalid_auth") || strings.Contains(errText, "not_authed") {
		return &notifyerr.Error{Kind: notifyerr.KindAuth, Cause: errors.New(apiError)}
	}
	if strings.Contains(errText, "channel_not_found") || strings.Contains(errText, "user_not_found") || status == http.StatusNotFound {
		return &notifyerr.Error{Kind: notifyerr.KindNotFound, Cause: errors.New(apiError)}
	}
	if status >= 500 {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: errors.New(apiError)}
	}
	if status >= 400 {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New(apiError)}
	}
	return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: errors.New(apiError)}
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
