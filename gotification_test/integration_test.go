package gotification_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/TheKrainBow/gotification"
)

type fakeSlackProvider struct {
	users    []string
	channels []string
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
