package gotification

import (
	"context"
	"fmt"
	"strings"
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
