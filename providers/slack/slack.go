package slack

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/TheKrainBow/gotification/internal/httpx"
	"github.com/TheKrainBow/gotification/internal/notifyerr"
	"github.com/TheKrainBow/gotification/slackmsg"
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
	return p.SendToChannelMessage(ctx, channelID, slackmsg.Message{Text: message})
}

// SendToChannelMessage posts a structured message to an existing channel ID.
func (p *Provider) SendToChannelMessage(ctx context.Context, channelID string, message slackmsg.Message) error {
	if strings.TrimSpace(channelID) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack channel id is required")}
	}
	if !hasMessageContent(message) {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack message cannot be empty")}
	}
	return p.postMessage(ctx, channelID, message)
}

// SendToChannelRawMessage posts a raw chat.postMessage payload to an existing
// channel ID after injecting the final channel server-side.
func (p *Provider) SendToChannelRawMessage(ctx context.Context, channelID string, payload json.RawMessage) error {
	if strings.TrimSpace(channelID) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack channel id is required")}
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack raw payload is required")}
	}
	return p.postRawMessage(ctx, channelID, payload)
}

// SendToUser opens a DM channel then sends a message.
func (p *Provider) SendToUser(ctx context.Context, userID string, message string) error {
	return p.SendToUserMessage(ctx, userID, slackmsg.Message{Text: message})
}

// SendToUserMessage opens a DM channel then sends a structured message.
func (p *Provider) SendToUserMessage(ctx context.Context, userID string, message slackmsg.Message) error {
	if strings.TrimSpace(userID) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack user id is required")}
	}
	if !hasMessageContent(message) {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack message cannot be empty")}
	}
	channelID, err := p.openConversation(ctx, userID)
	if err != nil {
		return err
	}
	return p.postMessage(ctx, channelID, message)
}

// SendToUserRawMessage opens a DM channel then sends a raw chat.postMessage
// payload after injecting the resolved channel server-side.
func (p *Provider) SendToUserRawMessage(ctx context.Context, userID string, payload json.RawMessage) error {
	if strings.TrimSpace(userID) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack user id is required")}
	}
	if len(strings.TrimSpace(string(payload))) == 0 {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack raw payload is required")}
	}
	channelID, err := p.openConversation(ctx, userID)
	if err != nil {
		return err
	}
	return p.postRawMessage(ctx, channelID, payload)
}

// AddReaction adds one emoji reaction to an existing Slack message.
func (p *Provider) AddReaction(ctx context.Context, channelID, messageTS, emoji string) error {
	if strings.TrimSpace(channelID) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack channel id is required")}
	}
	if strings.TrimSpace(messageTS) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack message timestamp is required")}
	}
	emoji = normalizeEmojiName(emoji)
	if emoji == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack emoji is required")}
	}
	return p.addReaction(ctx, channelID, messageTS, emoji)
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

func hasMessageContent(message slackmsg.Message) bool {
	return strings.TrimSpace(message.Text) != "" || len(message.Blocks) > 0 || hasAttachmentContent(message.Attachments)
}

func normalizeEmojiName(emoji string) string {
	emoji = strings.TrimSpace(emoji)
	emoji = strings.TrimPrefix(emoji, ":")
	emoji = strings.TrimSuffix(emoji, ":")
	return strings.TrimSpace(emoji)
}

func hasAttachmentContent(attachments []slackmsg.Attachment) bool {
	for _, attachment := range attachments {
		if len(attachment.Blocks) > 0 {
			return true
		}
		if strings.TrimSpace(attachment.Color) != "" ||
			strings.TrimSpace(attachment.Pretext) != "" ||
			strings.TrimSpace(attachment.AuthorName) != "" ||
			strings.TrimSpace(attachment.AuthorLink) != "" ||
			strings.TrimSpace(attachment.AuthorIcon) != "" ||
			strings.TrimSpace(attachment.Title) != "" ||
			strings.TrimSpace(attachment.TitleLink) != "" ||
			strings.TrimSpace(attachment.Text) != "" ||
			len(attachment.Fields) > 0 ||
			strings.TrimSpace(attachment.Footer) != "" ||
			strings.TrimSpace(attachment.FooterIcon) != "" ||
			attachment.Timestamp != 0 ||
			len(attachment.MarkdownIn) > 0 {
			return true
		}
	}
	return false
}
