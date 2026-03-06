package gotification

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/TheKrainBow/gotification/slackmsg"
)

// SendMail sends one plain-text email notification using context.Background().
func (d *Dispatcher) SendMail(provider, to, subject, content string) error {
	return d.SendMailWithCtx(context.Background(), provider, to, subject, content)
}

// SendMailWithCtx sends one plain-text email notification.
func (d *Dispatcher) SendMailWithCtx(ctx context.Context, provider, to, subject, content string) error {
	n := Notification{
		Name: "mail",
		Content: Content{
			Subject: subject,
			Text:    content,
		},
	}
	dest := Destination{Channel: ChannelEmail, Kind: DestinationEmailAddress, ID: to, Provider: provider}
	return d.Send(ctx, n, []Destination{dest})
}

// SendSlackChannelMessage sends one message to a Slack channel ID on one workspace
// using context.Background().
// If workspace is empty, the default Slack provider is used.
func (d *Dispatcher) SendSlackChannelMessage(workspace, channelID, content string) error {
	return d.SendSlackChannelMessageWithCtx(context.Background(), workspace, channelID, content)
}

// SendSlackChannelMessageWithCtx sends one message to a Slack channel ID on one
// workspace.
// If workspace is empty, the default Slack provider is used.
func (d *Dispatcher) SendSlackChannelMessageWithCtx(ctx context.Context, workspace, channelID, content string) error {
	n := Notification{
		Name: "slack-channel",
		Content: Content{
			Text: content,
		},
	}
	dest := Destination{Channel: ChannelSlack, Kind: DestinationSlackChannel, ID: channelID, Provider: workspace}
	return d.Send(ctx, n, []Destination{dest})
}

// SendSlackChannelRichMessage sends one structured Slack message to a channel ID
// on one workspace using context.Background().
func (d *Dispatcher) SendSlackChannelRichMessage(workspace, channelID string, message slackmsg.Message) error {
	return d.SendSlackChannelRichMessageWithCtx(context.Background(), workspace, channelID, message)
}

// SendSlackChannelRichMessageWithCtx sends one structured Slack message to a
// channel ID on one workspace.
func (d *Dispatcher) SendSlackChannelRichMessageWithCtx(ctx context.Context, workspace, channelID string, message slackmsg.Message) error {
	n := Notification{
		Name:    "slack-channel",
		Content: Content{Text: message.Text},
		Slack:   &message,
	}
	dest := Destination{Channel: ChannelSlack, Kind: DestinationSlackChannel, ID: channelID, Provider: workspace}
	return d.Send(ctx, n, []Destination{dest})
}

// SendSlackThreadReply sends one structured Slack reply in an existing thread
// using context.Background().
func (d *Dispatcher) SendSlackThreadReply(workspace, channelID, threadTS string, message slackmsg.Message) error {
	return d.SendSlackThreadReplyWithCtx(context.Background(), workspace, channelID, threadTS, message)
}

// SendSlackThreadReplyWithCtx sends one structured Slack reply in an existing
// thread.
func (d *Dispatcher) SendSlackThreadReplyWithCtx(ctx context.Context, workspace, channelID, threadTS string, message slackmsg.Message) error {
	message.ThreadTS = strings.TrimSpace(threadTS)
	n := Notification{
		Name:    "slack-thread-reply",
		Content: Content{Text: message.Text},
		Slack:   &message,
	}
	dest := Destination{Channel: ChannelSlack, Kind: DestinationSlackChannel, ID: channelID, Provider: workspace}
	return d.Send(ctx, n, []Destination{dest})
}

// SendSlackChannelRawMessage sends one raw Slack chat.postMessage payload to a
// channel ID using context.Background(). The library injects the final channel.
func (d *Dispatcher) SendSlackChannelRawMessage(workspace, channelID string, payload json.RawMessage) error {
	return d.SendSlackChannelRawMessageWithCtx(context.Background(), workspace, channelID, payload)
}

// SendSlackChannelRawMessageWithCtx sends one raw Slack chat.postMessage
// payload to a channel ID. The library injects the final channel.
func (d *Dispatcher) SendSlackChannelRawMessageWithCtx(ctx context.Context, workspace, channelID string, payload json.RawMessage) error {
	providerKey, provider, err := d.slackProviderFor(workspace)
	if err != nil {
		return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelSlack, Provider: workspace, Cause: err}
	}

	rawProvider, ok := provider.(SlackRawProvider)
	if !ok {
		return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelSlack, Provider: providerKey, Cause: fmt.Errorf("slack provider %q does not support raw messages", providerKey)}
	}

	dest := Destination{Channel: ChannelSlack, Kind: DestinationSlackChannel, ID: channelID, Provider: providerKey}
	callErr := rawProvider.SendToChannelRawMessage(ctx, channelID, payload)
	if wrapped := wrapProviderError(callErr, ChannelSlack, providerKey, dest); wrapped != nil {
		return wrapped
	}
	return nil
}

// SendSlackUserMP resolves Slack users by name and sends a DM message to every
// match using context.Background().
// If workspace is empty, the default Slack provider is used.
func (d *Dispatcher) SendSlackUserMP(workspace, username, content string) error {
	return d.SendSlackUserMPWithCtx(context.Background(), workspace, username, content)
}

// SendSlackUserMPWithCtx resolves Slack users by name and sends a DM message to
// every match.
// If workspace is empty, the default Slack provider is used.
func (d *Dispatcher) SendSlackUserMPWithCtx(ctx context.Context, workspace, username, content string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelSlack, Provider: workspace, Cause: fmt.Errorf("username is required")}
	}

	ids, err := d.FindSlackUsersByName(ctx, workspace, username)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return &NotifyError{
			Kind:     ErrNotFound,
			Channel:  ChannelSlack,
			Provider: workspace,
			Dest: Destination{
				Channel:  ChannelSlack,
				Kind:     DestinationSlackUser,
				ID:       username,
				Provider: workspace,
			},
			Cause: fmt.Errorf("no slack users matched %q", username),
		}
	}

	dests := make([]Destination, 0, len(ids))
	for _, id := range ids {
		dests = append(dests, Destination{
			Channel:  ChannelSlack,
			Kind:     DestinationSlackUser,
			ID:       id,
			Provider: workspace,
		})
	}

	n := Notification{
		Name: "slack-user-mp",
		Content: Content{
			Text: content,
		},
	}
	return d.Send(ctx, n, dests)
}

// SendSlackUserMPRaw resolves Slack users by name and sends one raw Slack
// chat.postMessage payload as a DM to every match using context.Background().
func (d *Dispatcher) SendSlackUserMPRaw(workspace, username string, payload json.RawMessage) error {
	return d.SendSlackUserMPRawWithCtx(context.Background(), workspace, username, payload)
}

// SendSlackUserMPRawWithCtx resolves Slack users by name and sends one raw
// Slack chat.postMessage payload as a DM to every match.
func (d *Dispatcher) SendSlackUserMPRawWithCtx(ctx context.Context, workspace, username string, payload json.RawMessage) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelSlack, Provider: workspace, Cause: fmt.Errorf("username is required")}
	}

	providerKey, provider, err := d.slackProviderFor(workspace)
	if err != nil {
		return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelSlack, Provider: workspace, Cause: err}
	}

	rawProvider, ok := provider.(SlackRawProvider)
	if !ok {
		return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelSlack, Provider: providerKey, Cause: fmt.Errorf("slack provider %q does not support raw messages", providerKey)}
	}

	ids, err := d.FindSlackUsersByName(ctx, workspace, username)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return &NotifyError{
			Kind:     ErrNotFound,
			Channel:  ChannelSlack,
			Provider: providerKey,
			Dest: Destination{
				Channel:  ChannelSlack,
				Kind:     DestinationSlackUser,
				ID:       username,
				Provider: providerKey,
			},
			Cause: fmt.Errorf("no slack users matched %q", username),
		}
	}

	for _, id := range ids {
		dest := Destination{Channel: ChannelSlack, Kind: DestinationSlackUser, ID: id, Provider: providerKey}
		callErr := rawProvider.SendToUserRawMessage(ctx, id, payload)
		if wrapped := wrapProviderError(callErr, ChannelSlack, providerKey, dest); wrapped != nil {
			return wrapped
		}
	}
	return nil
}

// AddSlackReaction adds one emoji reaction to an existing Slack message using
// context.Background().
func (d *Dispatcher) AddSlackReaction(workspace, channelID, messageTS, emoji string) error {
	return d.AddSlackReactionWithCtx(context.Background(), workspace, channelID, messageTS, emoji)
}

// AddSlackReactionWithCtx adds one emoji reaction to an existing Slack message.
func (d *Dispatcher) AddSlackReactionWithCtx(ctx context.Context, workspace, channelID, messageTS, emoji string) error {
	providerKey, provider, err := d.slackProviderFor(workspace)
	if err != nil {
		return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelSlack, Provider: workspace, Cause: err}
	}

	reactionProvider, ok := provider.(SlackReactionProvider)
	if !ok {
		return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelSlack, Provider: providerKey, Cause: fmt.Errorf("slack provider %q does not support reactions", providerKey)}
	}

	dest := Destination{Channel: ChannelSlack, Kind: DestinationSlackChannel, ID: channelID, Provider: providerKey}
	callErr := reactionProvider.AddReaction(ctx, channelID, messageTS, normalizeSlackEmoji(emoji))
	if wrapped := wrapProviderError(callErr, ChannelSlack, providerKey, dest); wrapped != nil {
		return wrapped
	}
	return nil
}

func normalizeSlackEmoji(emoji string) string {
	emoji = strings.TrimSpace(emoji)
	emoji = strings.TrimPrefix(emoji, ":")
	emoji = strings.TrimSuffix(emoji, ":")
	return strings.TrimSpace(emoji)
}

// SendTelegramMessage sends one message to a Telegram chat using
// context.Background().
func (d *Dispatcher) SendTelegramMessage(provider, chatID, content string) error {
	return d.SendTelegramMessageWithCtx(context.Background(), provider, chatID, content)
}

// SendTelegramMessageWithCtx sends one message to a Telegram chat.
func (d *Dispatcher) SendTelegramMessageWithCtx(ctx context.Context, provider, chatID, content string) error {
	n := Notification{
		Name: "telegram",
		Content: Content{
			Text: content,
		},
	}
	dest := Destination{Channel: ChannelTelegram, Kind: DestinationTelegramChat, ID: chatID, Provider: provider}
	return d.Send(ctx, n, []Destination{dest})
}

// SendWebhook sends one message to a webhook endpoint using
// context.Background().
func (d *Dispatcher) SendWebhook(provider, endpoint, content string) error {
	return d.SendWebhookWithCtx(context.Background(), provider, endpoint, content)
}

// SendWebhookWithCtx sends one message to a webhook endpoint.
func (d *Dispatcher) SendWebhookWithCtx(ctx context.Context, provider, endpoint, content string) error {
	n := Notification{
		Name: "webhook",
		Content: Content{
			Text: content,
		},
	}
	dest := Destination{Channel: ChannelWebhook, Kind: DestinationWebhookURL, ID: endpoint, Provider: provider}
	return d.Send(ctx, n, []Destination{dest})
}
