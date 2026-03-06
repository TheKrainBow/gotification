package gotification

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/TheKrainBow/gotification/internal/notifyerr"
	"github.com/TheKrainBow/gotification/internal/validate"
	"github.com/TheKrainBow/gotification/slackmsg"
)

// Send dispatches one notification to every destination.
func (d *Dispatcher) Send(ctx context.Context, n Notification, destinations []Destination) (err error) {
	if err := validateNotification(n); err != nil {
		return err
	}
	if len(destinations) == 0 {
		return &NotifyError{Kind: ErrInvalidInput, Cause: errors.New("at least one destination is required")}
	}
	if err := d.beginIdempotency(n.IdempotencyKey); err != nil {
		return err
	}
	defer func() {
		d.finishIdempotency(n.IdempotencyKey, err == nil)
	}()

	var errs []error
	for _, dest := range destinations {
		sendErr := d.sendOne(ctx, n, dest)
		if sendErr == nil {
			continue
		}
		errs = append(errs, sendErr)
		if d.mode == SendFailFast {
			break
		}
	}

	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return errors.Join(errs...)
	}
}

func (d *Dispatcher) sendOne(ctx context.Context, n Notification, dest Destination) error {
	if err := validateDestination(dest); err != nil {
		return err
	}

	switch dest.Channel {
	case ChannelEmail:
		providerKey, emailProvider, err := d.emailProviderFor(dest.Provider)
		if err != nil {
			return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelEmail, Provider: dest.Provider, Dest: dest, Cause: err}
		}
		if d.shouldMock(ChannelEmail, providerKey) {
			d.logSend(n, dest, providerKey, true)
			return nil
		}
		err = emailProvider.Send(ctx, dest.ID, n.Content)
		if wrapped := wrapProviderError(err, ChannelEmail, providerKey, dest); wrapped != nil {
			return wrapped
		}
		d.logSend(n, dest, providerKey, false)
		return nil
	case ChannelSlack:
		providerKey, provider, err := d.slackProviderFor(dest.Provider)
		if err != nil {
			return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelSlack, Provider: dest.Provider, Dest: dest, Cause: err}
		}
		if d.shouldMock(ChannelSlack, providerKey) {
			d.logSend(n, dest, providerKey, true)
			return nil
		}
		var callErr error
		message := slackMessageFromNotification(n)
		switch dest.Kind {
		case DestinationSlackUser:
			if richProvider, ok := provider.(SlackRichProvider); ok {
				callErr = richProvider.SendToUserMessage(ctx, dest.ID, message)
			} else if requiresSlackRichMessage(message) {
				callErr = &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack provider does not support structured slack messages")}
			} else {
				callErr = provider.SendToUser(ctx, dest.ID, message.Text)
			}
		case DestinationSlackChannel:
			if richProvider, ok := provider.(SlackRichProvider); ok {
				callErr = richProvider.SendToChannelMessage(ctx, dest.ID, message)
			} else if requiresSlackRichMessage(message) {
				callErr = &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("slack provider does not support structured slack messages")}
			} else {
				callErr = provider.SendToChannel(ctx, dest.ID, message.Text)
			}
		default:
			callErr = &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: fmt.Errorf("unsupported slack destination kind %q", dest.Kind)}
		}
		if wrapped := wrapProviderError(callErr, ChannelSlack, providerKey, dest); wrapped != nil {
			return wrapped
		}
		d.logSend(n, dest, providerKey, false)
		return nil
	case ChannelDiscord:
		providerKey, provider, err := d.discordProviderFor(dest.Provider)
		if err != nil {
			return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelDiscord, Provider: dest.Provider, Dest: dest, Cause: err}
		}
		if d.shouldMock(ChannelDiscord, providerKey) {
			d.logSend(n, dest, providerKey, true)
			return nil
		}
		var callErr error
		switch dest.Kind {
		case DestinationDiscordUser:
			callErr = provider.SendToUser(ctx, dest.ID, n.Content.Text)
		case DestinationDiscordChannel:
			callErr = provider.SendToChannel(ctx, dest.ID, n.Content.Text)
		default:
			callErr = &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: fmt.Errorf("unsupported discord destination kind %q", dest.Kind)}
		}
		if wrapped := wrapProviderError(callErr, ChannelDiscord, providerKey, dest); wrapped != nil {
			return wrapped
		}
		d.logSend(n, dest, providerKey, false)
		return nil
	case ChannelWebhook:
		providerKey, provider, err := d.webhookProviderFor(dest.Provider)
		if err != nil {
			return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelWebhook, Provider: dest.Provider, Dest: dest, Cause: err}
		}
		if d.shouldMock(ChannelWebhook, providerKey) {
			d.logSend(n, dest, providerKey, true)
			return nil
		}
		if dest.Kind != DestinationWebhookURL {
			return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelWebhook, Dest: dest, Cause: errors.New("invalid webhook destination kind")}
		}
		callErr := provider.Send(ctx, dest.ID, n.Content.Text)
		if wrapped := wrapProviderError(callErr, ChannelWebhook, providerKey, dest); wrapped != nil {
			return wrapped
		}
		d.logSend(n, dest, providerKey, false)
		return nil
	case ChannelTelegram:
		providerKey, provider, err := d.telegramProviderFor(dest.Provider)
		if err != nil {
			return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelTelegram, Provider: dest.Provider, Dest: dest, Cause: err}
		}
		if d.shouldMock(ChannelTelegram, providerKey) {
			d.logSend(n, dest, providerKey, true)
			return nil
		}
		if dest.Kind != DestinationTelegramChat {
			return &NotifyError{Kind: ErrInvalidInput, Channel: ChannelTelegram, Dest: dest, Cause: errors.New("invalid telegram destination kind")}
		}
		callErr := provider.SendToChat(ctx, dest.ID, n.Content.Text)
		if wrapped := wrapProviderError(callErr, ChannelTelegram, providerKey, dest); wrapped != nil {
			return wrapped
		}
		d.logSend(n, dest, providerKey, false)
		return nil
	default:
		return &NotifyError{Kind: ErrInvalidInput, Channel: dest.Channel, Dest: dest, Cause: fmt.Errorf("unsupported channel %q", dest.Channel)}
	}
}

func validateNotification(n Notification) error {
	if strings.TrimSpace(n.Content.Text) == "" && strings.TrimSpace(n.Content.HTML) == "" && !hasSlackMessageContent(n.Slack) {
		return &NotifyError{Kind: ErrInvalidInput, Cause: errors.New("content.text or content.html is required")}
	}
	return nil
}

func slackMessageFromNotification(n Notification) slackmsg.Message {
	if n.Slack == nil {
		return slackmsg.Message{Text: n.Content.Text}
	}

	message := *n.Slack
	if strings.TrimSpace(message.Text) == "" {
		message.Text = n.Content.Text
	}
	return message
}

func hasSlackMessageContent(message *slackmsg.Message) bool {
	if message == nil {
		return false
	}
	return strings.TrimSpace(message.Text) != "" || requiresSlackRichMessage(*message)
}

func requiresSlackRichMessage(message slackmsg.Message) bool {
	if len(message.Blocks) > 0 || len(message.Attachments) > 0 {
		return true
	}
	return false
}

func validateDestination(d Destination) error {
	if strings.TrimSpace(d.ID) == "" {
		return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: errors.New("destination id is required")}
	}
	if strings.TrimSpace(d.Provider) == "" {
		return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: errors.New("destination provider is required")}
	}

	switch d.Channel {
	case ChannelEmail:
		if d.Kind != DestinationEmailAddress {
			return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: fmt.Errorf("email channel requires kind %q", DestinationEmailAddress)}
		}
		if !validate.EmailAddress(d.ID) {
			return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: errors.New("invalid email address")}
		}
	case ChannelSlack:
		if d.Kind != DestinationSlackUser && d.Kind != DestinationSlackChannel {
			return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: errors.New("invalid slack destination kind")}
		}
		if !validate.SlackID(d.ID) {
			return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: errors.New("invalid slack id")}
		}
	case ChannelDiscord:
		if d.Kind != DestinationDiscordUser && d.Kind != DestinationDiscordChannel {
			return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: errors.New("invalid discord destination kind")}
		}
		if !validate.DiscordID(d.ID) {
			return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: errors.New("invalid discord id")}
		}
	case ChannelWebhook:
		if d.Kind != DestinationWebhookURL {
			return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: errors.New("invalid webhook destination kind")}
		}
		if !validate.HTTPURL(d.ID) {
			return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: errors.New("invalid webhook url")}
		}
	case ChannelTelegram:
		if d.Kind != DestinationTelegramChat {
			return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: errors.New("invalid telegram destination kind")}
		}
		if !validate.TelegramChatID(d.ID) {
			return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: errors.New("invalid telegram chat id")}
		}
	default:
		return &NotifyError{Kind: ErrInvalidInput, Channel: d.Channel, Provider: d.Provider, Dest: d, Cause: errors.New("unsupported channel")}
	}
	return nil
}
