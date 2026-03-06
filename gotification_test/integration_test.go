package gotification_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheKrainBow/gotification"
	"github.com/TheKrainBow/gotification/slackmsg"
)

type fakeSlackProvider struct {
	users         []string
	channels      []string
	userMessages  []slackmsg.Message
	channelRiches []slackmsg.Message
	userRaws      []json.RawMessage
	channelRaws   []json.RawMessage
	reactions     []fakeSlackReaction
}

type fakeSlackReaction struct {
	channelID string
	messageTS string
	emoji     string
}

type fakeDiscordProvider struct {
	users    []string
	channels []string
}

type fakeEmailProvider struct {
	to      []string
	subject []string
	body    []string
	err     error
}

type fakeTelegramProvider struct {
	chats []string
	msgs  []string
}

type fakeWebhookProvider struct {
	endpoints []string
	msgs      []string
}

type fakeSlackLookupProvider struct {
	fakeSlackProvider
	ids []string
	err error
}

func (f *fakeDiscordProvider) SendToUser(_ context.Context, userID string, _ string) error {
	f.users = append(f.users, userID)
	return nil
}

func (f *fakeDiscordProvider) SendToChannel(_ context.Context, channelID string, _ string) error {
	f.channels = append(f.channels, channelID)
	return nil
}

func (f *fakeEmailProvider) Send(_ context.Context, to string, content gotification.Content) error {
	if f.err != nil {
		return f.err
	}
	f.to = append(f.to, to)
	f.subject = append(f.subject, content.Subject)
	f.body = append(f.body, content.Text)
	return nil
}

func (f *fakeTelegramProvider) SendToChat(_ context.Context, chatID string, message string) error {
	f.chats = append(f.chats, chatID)
	f.msgs = append(f.msgs, message)
	return nil
}

func (f *fakeWebhookProvider) Send(_ context.Context, endpoint, message string) error {
	f.endpoints = append(f.endpoints, endpoint)
	f.msgs = append(f.msgs, message)
	return nil
}

func (f *fakeSlackProvider) SendToUser(_ context.Context, userID string, _ string) error {
	f.users = append(f.users, userID)
	return nil
}

func (f *fakeSlackProvider) SendToChannel(_ context.Context, channelID string, _ string) error {
	f.channels = append(f.channels, channelID)
	return nil
}

func (f *fakeSlackProvider) SendToUserMessage(_ context.Context, userID string, message slackmsg.Message) error {
	f.users = append(f.users, userID)
	f.userMessages = append(f.userMessages, message)
	return nil
}

func (f *fakeSlackProvider) SendToChannelMessage(_ context.Context, channelID string, message slackmsg.Message) error {
	f.channels = append(f.channels, channelID)
	f.channelRiches = append(f.channelRiches, message)
	return nil
}

func (f *fakeSlackProvider) SendToUserRawMessage(_ context.Context, userID string, payload json.RawMessage) error {
	f.users = append(f.users, userID)
	f.userRaws = append(f.userRaws, append(json.RawMessage(nil), payload...))
	return nil
}

func (f *fakeSlackProvider) SendToChannelRawMessage(_ context.Context, channelID string, payload json.RawMessage) error {
	f.channels = append(f.channels, channelID)
	f.channelRaws = append(f.channelRaws, append(json.RawMessage(nil), payload...))
	return nil
}

func (f *fakeSlackProvider) AddReaction(_ context.Context, channelID, messageTS, emoji string) error {
	f.reactions = append(f.reactions, fakeSlackReaction{channelID: channelID, messageTS: messageTS, emoji: emoji})
	return nil
}

type legacyFakeSlackProvider struct {
	users    []string
	channels []string
}

func (f *legacyFakeSlackProvider) SendToUser(_ context.Context, userID string, _ string) error {
	f.users = append(f.users, userID)
	return nil
}

func (f *legacyFakeSlackProvider) SendToChannel(_ context.Context, channelID string, _ string) error {
	f.channels = append(f.channels, channelID)
	return nil
}

func (f *fakeSlackLookupProvider) FindUsersByName(_ context.Context, _ string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]string(nil), f.ids...), nil
}

func TestRuntimeMockModeWritesExpectedLines(t *testing.T) {
	logFolder := filepath.Join(t.TempDir(), "logs")
	d, err := gotification.NewDispatcher(
		gotification.WithEmailProvider("default", &fakeEmailProvider{}),
		gotification.WithWebhookProvider("default", &fakeWebhookProvider{}),
		gotification.WithTelegramProvider("default", &fakeTelegramProvider{}),
		gotification.WithLogFolder(logFolder),
		gotification.WithMockMode(gotification.ChannelEmail, "default", true),
		gotification.WithMockMode(gotification.ChannelWebhook, "default", true),
		gotification.WithMockMode(gotification.ChannelTelegram, "default", true),
	)
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	n := gotification.Notification{Content: gotification.Content{Subject: "Build result", Text: "Deployment complete"}}
	dests := []gotification.Destination{
		{Channel: gotification.ChannelEmail, Kind: gotification.DestinationEmailAddress, ID: "dev@example.com", Provider: "default"},
		{Channel: gotification.ChannelWebhook, Kind: gotification.DestinationWebhookURL, ID: "https://example.com/hook", Provider: "default"},
		{Channel: gotification.ChannelTelegram, Kind: gotification.DestinationTelegramChat, ID: "123456", Provider: "default"},
	}

	if err := d.Send(context.Background(), n, dests); err != nil {
		t.Fatalf("send: %v", err)
	}

	emailBody, err := os.ReadFile(filepath.Join(logFolder, "email", "default.log"))
	if err != nil {
		t.Fatalf("read email log: %v", err)
	}
	webhookBody, err := os.ReadFile(filepath.Join(logFolder, "webhook", "default.log"))
	if err != nil {
		t.Fatalf("read webhook log: %v", err)
	}
	telegramBody, err := os.ReadFile(filepath.Join(logFolder, "telegram", "default.log"))
	if err != nil {
		t.Fatalf("read telegram log: %v", err)
	}

	if !strings.Contains(string(emailBody), "[MOCKED]") || !strings.Contains(string(emailBody), "[EMAIL]") {
		t.Fatalf("unexpected email log line: %s", string(emailBody))
	}
	if !strings.Contains(string(webhookBody), "[MOCKED]") || !strings.Contains(string(webhookBody), "[WEBHOOK]") {
		t.Fatalf("unexpected webhook log line: %s", string(webhookBody))
	}
	if !strings.Contains(string(telegramBody), "[MOCKED]") || !strings.Contains(string(telegramBody), "[TELEGRAM]") {
		t.Fatalf("unexpected telegram log line: %s", string(telegramBody))
	}
}

func TestRetryableWithJoinedErrors(t *testing.T) {
	nonRetry := &gotification.NotifyError{Kind: gotification.ErrInvalidInput, Cause: errors.New("bad payload")}
	retry := &gotification.NotifyError{Kind: gotification.ErrTemporary, Cause: errors.New("timeout")}
	joined := errors.Join(nonRetry, retry)

	if !gotification.Retryable(retry) {
		t.Fatal("expected temporary error to be retryable")
	}
	if gotification.Retryable(nonRetry) {
		t.Fatal("expected invalid input error not to be retryable")
	}
	if !gotification.Retryable(joined) {
		t.Fatal("expected joined error containing temporary error to be retryable")
	}
}

func TestSlackProviderSelection(t *testing.T) {
	a := &fakeSlackProvider{}
	b := &fakeSlackProvider{}

	d, err := gotification.NewDispatcher(
		gotification.WithSlackProvider("workspace-a", a),
		gotification.WithSlackProvider("workspace-b", b),
	)
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	n := gotification.Notification{Content: gotification.Content{Text: "hello"}}
	dests := []gotification.Destination{
		{Channel: gotification.ChannelSlack, Kind: gotification.DestinationSlackChannel, ID: "C1", Provider: "workspace-a"},
		{Channel: gotification.ChannelSlack, Kind: gotification.DestinationSlackUser, ID: "U2", Provider: "workspace-b"},
	}

	if err := d.Send(context.Background(), n, dests); err != nil {
		t.Fatalf("send: %v", err)
	}

	if len(a.channels) != 1 || a.channels[0] != "C1" {
		t.Fatalf("workspace-a not used for channel destination: %#v", a.channels)
	}
	if len(b.users) != 1 || b.users[0] != "U2" {
		t.Fatalf("workspace-b not used for user destination: %#v", b.users)
	}
}

func TestFindSlackUsersByName(t *testing.T) {
	lookup := &fakeSlackLookupProvider{ids: []string{"U1", "U2"}}
	d, err := gotification.NewDispatcher(gotification.WithSlackProvider("workspace-a", lookup))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	ids, err := d.FindSlackUsersByName(context.Background(), "workspace-a", "heinz")
	if err != nil {
		t.Fatalf("FindSlackUsersByName failed: %v", err)
	}
	if len(ids) != 2 || ids[0] != "U1" || ids[1] != "U2" {
		t.Fatalf("unexpected ids: %#v", ids)
	}
}

func TestConvenienceSendMail(t *testing.T) {
	emailProvider := &fakeEmailProvider{}
	d, err := gotification.NewDispatcher(gotification.WithEmailProvider("default", emailProvider))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	if err := d.SendMail("default", "dev@example.com", "subject-a", "body-a"); err != nil {
		t.Fatalf("SendMail failed: %v", err)
	}
	if len(emailProvider.to) != 1 || emailProvider.to[0] != "dev@example.com" {
		t.Fatalf("unexpected recipients: %#v", emailProvider.to)
	}
}

func TestConvenienceSendSlackChannelMessage(t *testing.T) {
	slackProvider := &fakeSlackProvider{}
	d, err := gotification.NewDispatcher(gotification.WithSlackProvider("workspace-a", slackProvider))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	if err := d.SendSlackChannelMessage("workspace-a", "C123", "hello"); err != nil {
		t.Fatalf("SendSlackChannelMessage failed: %v", err)
	}
	if len(slackProvider.channels) != 1 || slackProvider.channels[0] != "C123" {
		t.Fatalf("unexpected channels: %#v", slackProvider.channels)
	}
}

func TestConvenienceSendSlackChannelRichMessage(t *testing.T) {
	slackProvider := &fakeSlackProvider{}
	d, err := gotification.NewDispatcher(gotification.WithSlackProvider("workspace-a", slackProvider))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	message := slackmsg.Message{
		Text: "USB receiver unplugged",
		Attachments: []slackmsg.Attachment{{
			Color: "#7B2CBF",
			Fields: []slackmsg.AttachmentField{
				{Title: "Host", Value: "maagosti", Short: true},
			},
		}},
	}
	if err := d.SendSlackChannelRichMessage("workspace-a", "C123", message); err != nil {
		t.Fatalf("SendSlackChannelRichMessage failed: %v", err)
	}
	if len(slackProvider.channelRiches) != 1 {
		t.Fatalf("expected one rich message, got %#v", slackProvider.channelRiches)
	}
	if slackProvider.channelRiches[0].Attachments[0].Color != "#7B2CBF" {
		t.Fatalf("unexpected rich message payload: %#v", slackProvider.channelRiches[0])
	}
}

func TestConvenienceSendSlackChannelRichMessageWithAttachmentBlocks(t *testing.T) {
	slackProvider := &fakeSlackProvider{}
	d, err := gotification.NewDispatcher(gotification.WithSlackProvider("workspace-a", slackProvider))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	message := slackmsg.Message{
		Text: "📁 Un dossier est en attente de validation",
		Attachments: []slackmsg.Attachment{{
			Color: "#ffcc00",
			Blocks: []slackmsg.Block{
				{
					"type": "header",
					"text": map[string]any{
						"type": "plain_text",
						"text": "📁 Un dossier est en attente de validation",
					},
				},
				{
					"type": "section",
					"text": map[string]any{
						"type": "mrkdwn",
						"text": "*maagosti*\\nmaagosti a envoyé tous ses documents pour l'année *2025-2026*.",
					},
					"accessory": map[string]any{
						"type":      "image",
						"image_url": "https://cdn.intra.42.fr/users/medium_maagosti.jpg",
						"alt_text":  "avatar",
					},
				},
			},
		}},
	}
	if err := d.SendSlackChannelRichMessage("workspace-a", "C123", message); err != nil {
		t.Fatalf("SendSlackChannelRichMessage failed: %v", err)
	}
	if len(slackProvider.channelRiches) != 1 {
		t.Fatalf("expected one rich message, got %#v", slackProvider.channelRiches)
	}
	if got := slackProvider.channelRiches[0].Attachments[0].Blocks[0]["type"]; got != "header" {
		t.Fatalf("unexpected attachment blocks payload: %#v", slackProvider.channelRiches[0])
	}
}

func TestConvenienceSendSlackThreadReply(t *testing.T) {
	slackProvider := &fakeSlackProvider{}
	d, err := gotification.NewDispatcher(gotification.WithSlackProvider("workspace-a", slackProvider))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	err = d.SendSlackThreadReply("workspace-a", "C123", "1741256640.123456", slackmsg.Message{
		Text: "thread reply",
	})
	if err != nil {
		t.Fatalf("SendSlackThreadReply failed: %v", err)
	}
	if len(slackProvider.channelRiches) != 1 {
		t.Fatalf("expected one rich message, got %#v", slackProvider.channelRiches)
	}
	if slackProvider.channelRiches[0].ThreadTS != "1741256640.123456" {
		t.Fatalf("unexpected thread ts: %#v", slackProvider.channelRiches[0])
	}
}

func TestConvenienceSendSlackChannelRawMessage(t *testing.T) {
	slackProvider := &fakeSlackProvider{}
	d, err := gotification.NewDispatcher(gotification.WithSlackProvider("workspace-a", slackProvider))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	payload := json.RawMessage(`{"text":"📁 Un dossier est en attente de validation","attachments":[{"color":"#ffcc00"}]}`)
	if err := d.SendSlackChannelRawMessage("workspace-a", "C123", payload); err != nil {
		t.Fatalf("SendSlackChannelRawMessage failed: %v", err)
	}
	if len(slackProvider.channelRaws) != 1 {
		t.Fatalf("expected one raw message, got %#v", slackProvider.channelRaws)
	}
	if string(slackProvider.channelRaws[0]) != string(payload) {
		t.Fatalf("unexpected raw payload: %s", string(slackProvider.channelRaws[0]))
	}
}

func TestConvenienceSendSlackUserMP(t *testing.T) {
	lookup := &fakeSlackLookupProvider{ids: []string{"U1", "U2"}}
	d, err := gotification.NewDispatcher(gotification.WithSlackProvider("workspace-a", lookup))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	if err := d.SendSlackUserMP("workspace-a", "heinz", "hello"); err != nil {
		t.Fatalf("SendSlackUserMP failed: %v", err)
	}
	if len(lookup.users) != 2 {
		t.Fatalf("unexpected user IDs: %#v", lookup.users)
	}
}

func TestConvenienceSendSlackUserMPRaw(t *testing.T) {
	lookup := &fakeSlackLookupProvider{ids: []string{"U1", "U2"}}
	d, err := gotification.NewDispatcher(gotification.WithSlackProvider("workspace-a", lookup))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	payload := json.RawMessage(`{"text":"hello","blocks":[{"type":"divider"}]}`)
	if err := d.SendSlackUserMPRaw("workspace-a", "heinz", payload); err != nil {
		t.Fatalf("SendSlackUserMPRaw failed: %v", err)
	}
	if len(lookup.userRaws) != 2 {
		t.Fatalf("unexpected raw payload count: %#v", lookup.userRaws)
	}
	if len(lookup.users) != 2 {
		t.Fatalf("unexpected user IDs: %#v", lookup.users)
	}
}

func TestIncrementalSlackProviderManagement(t *testing.T) {
	d, err := gotification.NewDispatcher()
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	provider := &fakeSlackProvider{}
	if err := d.AddSlackProvider("workspace-a", provider); err != nil {
		t.Fatalf("AddSlackProvider failed: %v", err)
	}

	if err := d.SendSlackChannelMessage("workspace-a", "C42", "hello"); err != nil {
		t.Fatalf("SendSlackChannelMessage failed: %v", err)
	}
	if len(provider.channels) != 1 || provider.channels[0] != "C42" {
		t.Fatalf("unexpected channels: %#v", provider.channels)
	}

	if err := d.RemoveSlackProvider("workspace-a"); err != nil {
		t.Fatalf("RemoveSlackProvider failed: %v", err)
	}
	err = d.SendSlackChannelMessage("workspace-a", "C43", "hello")
	if err == nil {
		t.Fatal("expected error after removing slack provider")
	}
}

func TestSlackAttachmentsRequireRichProvider(t *testing.T) {
	slackProvider := &legacyFakeSlackProvider{}
	d, err := gotification.NewDispatcher(gotification.WithSlackProvider("workspace-a", slackProvider))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	err = d.Send(context.Background(), gotification.Notification{
		Slack: &slackmsg.Message{
			Text: "hello",
			Attachments: []slackmsg.Attachment{{
				Color: "#ff0000",
			}},
		},
	}, []gotification.Destination{{
		Channel:  gotification.ChannelSlack,
		Kind:     gotification.DestinationSlackChannel,
		ID:       "C123",
		Provider: "workspace-a",
	}})
	if err == nil {
		t.Fatal("expected error for legacy slack provider with attachments")
	}
	if !strings.Contains(err.Error(), "does not support structured slack messages") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSlackBlocksRequireRichProvider(t *testing.T) {
	slackProvider := &legacyFakeSlackProvider{}
	d, err := gotification.NewDispatcher(gotification.WithSlackProvider("workspace-a", slackProvider))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	err = d.Send(context.Background(), gotification.Notification{
		Slack: &slackmsg.Message{
			Text: "hello",
			Blocks: []slackmsg.Block{
				{"type": "divider"},
			},
		},
	}, []gotification.Destination{{
		Channel:  gotification.ChannelSlack,
		Kind:     gotification.DestinationSlackChannel,
		ID:       "C123",
		Provider: "workspace-a",
	}})
	if err == nil {
		t.Fatal("expected error for legacy slack provider with blocks")
	}
	if !strings.Contains(err.Error(), "does not support structured slack messages") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddSlackReaction(t *testing.T) {
	slackProvider := &fakeSlackProvider{}
	d, err := gotification.NewDispatcher(gotification.WithSlackProvider("workspace-a", slackProvider))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	if err := d.AddSlackReaction("workspace-a", "C123", "1741256640.123456", ":rotating_light:"); err != nil {
		t.Fatalf("AddSlackReaction failed: %v", err)
	}
	if len(slackProvider.reactions) != 1 {
		t.Fatalf("unexpected reactions: %#v", slackProvider.reactions)
	}
	if slackProvider.reactions[0].emoji != "rotating_light" {
		t.Fatalf("unexpected emoji normalization: %#v", slackProvider.reactions[0])
	}
}

func TestAddSlackReactionRequiresCapableProvider(t *testing.T) {
	slackProvider := &legacyFakeSlackProvider{}
	d, err := gotification.NewDispatcher(gotification.WithSlackProvider("workspace-a", slackProvider))
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	err = d.AddSlackReaction("workspace-a", "C123", "1741256640.123456", "eyes")
	if err == nil {
		t.Fatal("expected error for legacy slack provider without reactions support")
	}
	if !strings.Contains(err.Error(), "does not support reactions") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConvenienceSendTelegramMessage(t *testing.T) {
	tg := &fakeTelegramProvider{}
	d, err := gotification.NewDispatcher()
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}
	if err := d.AddTelegramProvider("default", tg); err != nil {
		t.Fatalf("AddTelegramProvider failed: %v", err)
	}

	if err := d.SendTelegramMessage("default", "123456", "hello telegram"); err != nil {
		t.Fatalf("SendTelegramMessage failed: %v", err)
	}
	if len(tg.chats) != 1 || tg.chats[0] != "123456" {
		t.Fatalf("unexpected chats: %#v", tg.chats)
	}
}

func TestConvenienceSendWebhook(t *testing.T) {
	wh := &fakeWebhookProvider{}
	d, err := gotification.NewDispatcher()
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}
	if err := d.AddWebhookProvider("default", wh); err != nil {
		t.Fatalf("AddWebhookProvider failed: %v", err)
	}

	endpoint := "https://example.com/hooks/alerts"
	if err := d.SendWebhook("default", endpoint, "hello webhook"); err != nil {
		t.Fatalf("SendWebhook failed: %v", err)
	}
	if len(wh.endpoints) != 1 || wh.endpoints[0] != endpoint {
		t.Fatalf("unexpected endpoints: %#v", wh.endpoints)
	}
}

func TestIdempotencyBlocksDuplicateSuccessfulSend(t *testing.T) {
	emailProvider := &fakeEmailProvider{}
	d, err := gotification.NewDispatcher(
		gotification.WithEmailProvider("default", emailProvider),
		gotification.WithIdempotencyTTL(time.Minute),
	)
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	n := gotification.Notification{IdempotencyKey: "mail-1", Content: gotification.Content{Subject: "sub", Text: "body"}}
	dest := []gotification.Destination{{Channel: gotification.ChannelEmail, Kind: gotification.DestinationEmailAddress, ID: "dev@example.com", Provider: "default"}}

	if err := d.Send(context.Background(), n, dest); err != nil {
		t.Fatalf("first send failed: %v", err)
	}
	err = d.Send(context.Background(), n, dest)
	if err == nil {
		t.Fatal("expected duplicate idempotency key error")
	}
}

func TestIdempotencyAllowsRetryAfterFailure(t *testing.T) {
	emailProvider := &fakeEmailProvider{err: errors.New("temporary send failure")}
	d, err := gotification.NewDispatcher(
		gotification.WithEmailProvider("default", emailProvider),
		gotification.WithIdempotencyTTL(time.Minute),
	)
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}

	n := gotification.Notification{IdempotencyKey: "mail-2", Content: gotification.Content{Subject: "sub", Text: "body"}}
	dest := []gotification.Destination{{Channel: gotification.ChannelEmail, Kind: gotification.DestinationEmailAddress, ID: "dev@example.com", Provider: "default"}}

	if err := d.Send(context.Background(), n, dest); err == nil {
		t.Fatal("expected first send to fail")
	}
	emailProvider.err = nil
	if err := d.Send(context.Background(), n, dest); err != nil {
		t.Fatalf("retry send should succeed, got: %v", err)
	}
}

func TestPerProviderMockMode(t *testing.T) {
	logFolder := filepath.Join(t.TempDir(), "runtime-logs")
	slackA := &fakeSlackProvider{}
	slackB := &fakeSlackProvider{}
	discordProvider := &fakeDiscordProvider{}

	d, err := gotification.NewDispatcher(
		gotification.WithSlackProvider("workspace-a", slackA),
		gotification.WithSlackProvider("workspace-b", slackB),
		gotification.WithDiscordProvider("default", discordProvider),
		gotification.WithLogFolder(logFolder),
	)
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}
	if err := d.SetMockMode(gotification.ChannelSlack, "workspace-a", true); err != nil {
		t.Fatalf("SetMockMode failed: %v", err)
	}

	n := gotification.Notification{Content: gotification.Content{Text: "hello"}}
	err = d.Send(context.Background(), n, []gotification.Destination{
		{Channel: gotification.ChannelSlack, Kind: gotification.DestinationSlackChannel, ID: "C1", Provider: "workspace-a"},
		{Channel: gotification.ChannelSlack, Kind: gotification.DestinationSlackChannel, ID: "C2", Provider: "workspace-b"},
		{Channel: gotification.ChannelDiscord, Kind: gotification.DestinationDiscordChannel, ID: "123456", Provider: "default"},
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}

	if len(slackA.channels) != 0 {
		t.Fatalf("workspace-a should be mocked, got sends: %#v", slackA.channels)
	}
	if len(slackB.channels) != 1 || slackB.channels[0] != "C2" {
		t.Fatalf("workspace-b should be real, got sends: %#v", slackB.channels)
	}
	if len(discordProvider.channels) != 1 || discordProvider.channels[0] != "123456" {
		t.Fatalf("discord should be real, got sends: %#v", discordProvider.channels)
	}
}
