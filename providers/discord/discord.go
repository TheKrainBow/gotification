package discord

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/TheKrainBow/gotification/internal/httpx"
	"github.com/TheKrainBow/gotification/internal/notifyerr"
)

// Config configures Discord Bot API access.
type Config struct {
	BotToken   string
	BaseURL    string
	HTTPClient apiHTTPClient
}

// Provider sends messages through Discord Bot API.
type Provider struct {
	token   string
	baseURL string
	client  apiHTTPClient
}

// NewProvider creates a Discord provider.
func NewProvider(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.BotToken) == "" {
		return nil, errors.New("discord bot token is required")
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://discord.com/api/v10"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = httpx.NewClient(10 * time.Second)
	}
	return &Provider{token: cfg.BotToken, baseURL: baseURL, client: client}, nil
}

// SendToChannel sends a message directly to a channel ID.
func (p *Provider) SendToChannel(ctx context.Context, channelID string, message string) error {
	if strings.TrimSpace(channelID) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("discord channel id is required")}
	}
	if strings.TrimSpace(message) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("discord message cannot be empty")}
	}
	return p.sendMessage(ctx, channelID, message)
}

// SendToUser opens a DM channel then sends a message.
func (p *Provider) SendToUser(ctx context.Context, userID string, message string) error {
	if strings.TrimSpace(userID) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("discord user id is required")}
	}
	if strings.TrimSpace(message) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("discord message cannot be empty")}
	}
	channelID, err := p.createDMChannel(ctx, userID)
	if err != nil {
		return err
	}
	return p.sendMessage(ctx, channelID, message)
}
