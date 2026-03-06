package gotification

import (
	"context"

	"github.com/TheKrainBow/gotification/slackmsg"
)

// Channel identifies a notification transport.
type Channel string

const (
	ChannelEmail    Channel = "email"
	ChannelSlack    Channel = "slack"
	ChannelDiscord  Channel = "discord"
	ChannelWebhook  Channel = "webhook"
	ChannelTelegram Channel = "telegram"
)

// DestinationKind refines destination semantics per channel.
type DestinationKind string

const (
	DestinationEmailAddress   DestinationKind = "email_address"
	DestinationSlackUser      DestinationKind = "slack_user"
	DestinationSlackChannel   DestinationKind = "slack_channel"
	DestinationDiscordUser    DestinationKind = "discord_user"
	DestinationDiscordChannel DestinationKind = "discord_channel"
	DestinationWebhookURL     DestinationKind = "webhook_url"
	DestinationTelegramChat   DestinationKind = "telegram_chat"
)

// Destination selects where and how a notification is sent.
type Destination struct {
	Channel  Channel
	Kind     DestinationKind
	ID       string
	Provider string
	Meta     map[string]string
}

// Notification is the payload sent to providers.
type Notification struct {
	Name    string
	Content Content
	Slack   *slackmsg.Message
	Data    map[string]any
	TraceID string
	// IdempotencyKey is optional. When dispatcher idempotency is enabled,
	// duplicate successful sends with the same key are rejected during TTL.
	IdempotencyKey string
}

// Content contains textual and HTML representations.
type Content struct {
	Subject string
	Text    string
	HTML    string
}

// Mode controls dispatch behavior when multiple destinations are passed.
type Mode int

const (
	SendBestEffort Mode = iota
	SendFailFast
)

// EmailProvider sends notification content to an email address.
type EmailProvider interface {
	Send(ctx context.Context, to string, content Content) error
}

// SlackProvider sends Slack DMs and channel messages.
type SlackProvider interface {
	SendToUser(ctx context.Context, userID string, message string) error
	SendToChannel(ctx context.Context, channelID string, message string) error
}

// SlackRichProvider is an optional capability for Slack providers that can send
// structured Slack payloads, including attachments.
type SlackRichProvider interface {
	SendToUserMessage(ctx context.Context, userID string, message slackmsg.Message) error
	SendToChannelMessage(ctx context.Context, channelID string, message slackmsg.Message) error
}

// SlackReactionProvider is an optional capability for Slack providers that can
// add emoji reactions to existing messages.
type SlackReactionProvider interface {
	AddReaction(ctx context.Context, channelID, messageTS, emoji string) error
}

// SlackUserLookupProvider is an optional capability for Slack providers that can
// resolve user IDs by a username/display-name query.
type SlackUserLookupProvider interface {
	FindUsersByName(ctx context.Context, query string) ([]string, error)
}

// DiscordProvider sends Discord DMs and channel messages.
type DiscordProvider interface {
	SendToUser(ctx context.Context, userID string, message string) error
	SendToChannel(ctx context.Context, channelID string, message string) error
}

// WebhookProvider sends JSON payloads to an HTTP endpoint.
type WebhookProvider interface {
	Send(ctx context.Context, endpoint string, message string) error
}

// TelegramProvider sends Bot API messages to one chat.
type TelegramProvider interface {
	SendToChat(ctx context.Context, chatID string, message string) error
}
