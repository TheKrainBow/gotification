package slack

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/TheKrainBow/gotification/internal/httpx"
	"github.com/TheKrainBow/gotification/internal/notifyerr"
)

// Config configures Slack Web API access.
type Config struct {
	BotToken   string
	BaseURL    string
	HTTPClient apiHTTPClient
}

// Provider sends messages through Slack Web API.
type Provider struct {
	token   string
	baseURL string
	client  apiHTTPClient
}

// NewProvider creates a Slack provider.
func NewProvider(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.BotToken) == "" {
		return nil, errors.New("slack bot token is required")
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://slack.com/api"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = httpx.NewClient(10 * time.Second)
	}
	return &Provider{token: cfg.BotToken, baseURL: baseURL, client: client}, nil
}

// SendToChannel posts a message to an existing channel ID.
func (p *Provider) SendToChannel(ctx context.Context, channelID string, message string) error {
	if strings.TrimSpace(channelID) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack channel id is required")}
	}
	if strings.TrimSpace(message) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack message cannot be empty")}
	}
	return p.postMessage(ctx, channelID, message)
}

// SendToUser opens a DM channel then sends a message.
func (p *Provider) SendToUser(ctx context.Context, userID string, message string) error {
	if strings.TrimSpace(userID) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack user id is required")}
	}
	if strings.TrimSpace(message) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack message cannot be empty")}
	}
	channelID, err := p.openConversation(ctx, userID)
	if err != nil {
		return err
	}
	return p.postMessage(ctx, channelID, message)
}

// FindUsersByName resolves Slack user IDs matching query against username,
// real name, or display name (case-insensitive).
func (p *Provider) FindUsersByName(ctx context.Context, query string) ([]string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack user query is required")}
	}
	return p.findUsersByName(ctx, query)
}
