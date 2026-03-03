package telegram

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/TheKrainBow/gotification/internal/httpx"
	"github.com/TheKrainBow/gotification/internal/notifyerr"
	"github.com/TheKrainBow/gotification/internal/validate"
)

// Config configures Telegram Bot API access.
type Config struct {
	BotToken   string
	BaseURL    string
	HTTPClient apiHTTPClient
	Timeout    time.Duration
}

// Provider sends messages through Telegram Bot API.
type Provider struct {
	baseURL string
	client  apiHTTPClient
}

// NewProvider creates a Telegram provider.
func NewProvider(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.BotToken) == "" {
		return nil, errors.New("telegram bot token is required")
	}
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.telegram.org"
	}
	client := cfg.HTTPClient
	if client == nil {
		client = httpx.NewClient(cfg.Timeout)
	}
	return &Provider{baseURL: baseURL + "/bot" + cfg.BotToken, client: client}, nil
}

// SendToChat sends a message to one Telegram chat.
func (p *Provider) SendToChat(ctx context.Context, chatID string, message string) error {
	if !validate.TelegramChatID(chatID) {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("invalid telegram chat id")}
	}
	if strings.TrimSpace(message) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("telegram message cannot be empty")}
	}
	return p.sendMessage(ctx, chatID, message)
}
