package gotification

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	discordprovider "github.com/TheKrainBow/gotification/providers/discord"
	emailprovider "github.com/TheKrainBow/gotification/providers/email"
	slackprovider "github.com/TheKrainBow/gotification/providers/slack"
	telegramprovider "github.com/TheKrainBow/gotification/providers/telegram"
	webhookprovider "github.com/TheKrainBow/gotification/providers/webhook"
)

// Logger is an optional structured logger used by the dispatcher.
type Logger interface {
	Debug(msg string, kv ...any)
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Error(msg string, kv ...any)
}

type noopLogger struct{}

func (noopLogger) Debug(string, ...any) {}
func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}

// Dispatcher orchestrates delivery across channels/providers.
type Dispatcher struct {
	mu             sync.RWMutex
	logWriteMu     sync.Mutex
	emailProviders map[string]EmailProvider
	discord        map[string]DiscordProvider
	telegram       map[string]TelegramProvider
	webhook        map[string]WebhookProvider
	slackProviders map[string]SlackProvider
	mode           Mode
	logger         Logger
	idemTTL        time.Duration
	idemStore      map[string]time.Time
	mockFile       string
	mockTargets    map[string]bool
	logFolder      string
}

type smtpEmailAdapter struct {
	provider *emailprovider.SMTPProvider
}

func (a *smtpEmailAdapter) Send(ctx context.Context, to string, content Content) error {
	return a.provider.Send(ctx, to, content.Subject, content.Text, content.HTML)
}

// Option configures a Dispatcher.
type Option func(*Dispatcher) error

// NewDispatcher constructs a dispatcher from options.
func NewDispatcher(opts ...Option) (*Dispatcher, error) {
	d := &Dispatcher{
		emailProviders: make(map[string]EmailProvider),
		discord:        make(map[string]DiscordProvider),
		telegram:       make(map[string]TelegramProvider),
		webhook:        make(map[string]WebhookProvider),
		slackProviders: make(map[string]SlackProvider),
		mode:           SendBestEffort,
		logger:         noopLogger{},
		idemStore:      make(map[string]time.Time),
		mockTargets:    make(map[string]bool),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(d); err != nil {
			return nil, err
		}
	}
	return d, nil
}

// WithMode sets send behavior across multiple destinations.
func WithMode(mode Mode) Option {
	return func(d *Dispatcher) error {
		d.mode = mode
		return nil
	}
}

// WithIdempotencyTTL enables in-memory idempotency for Notification.IdempotencyKey.
// Duplicate successful sends are rejected for the configured TTL.
func WithIdempotencyTTL(ttl time.Duration) Option {
	return func(d *Dispatcher) error {
		return d.EnableIdempotency(ttl)
	}
}

// WithMockFile is a legacy helper kept for compatibility.
// It enables provider logs in the parent folder of the given path.
func WithMockFile(path string) Option {
	return func(d *Dispatcher) error {
		return d.SetMockFile(path)
	}
}

// WithMockMode enables/disables mock behavior for one channel/provider pair.
// Provider name is required.
func WithMockMode(channel Channel, provider string, enabled bool) Option {
	return func(d *Dispatcher) error {
		return d.SetMockMode(channel, provider, enabled)
	}
}

// WithLogFolder enables per-provider file logs in the provided folder.
func WithLogFolder(path string) Option {
	return func(d *Dispatcher) error {
		return d.SetLogFolder(path)
	}
}

// WithLogger configures a logger used by dispatcher internals.
func WithLogger(l Logger) Option {
	return func(d *Dispatcher) error {
		if l == nil {
			d.logger = noopLogger{}
			return nil
		}
		d.logger = l
		return nil
	}
}

// WithEmailProvider registers one named email provider.
func WithEmailProvider(name string, p EmailProvider) Option {
	return func(d *Dispatcher) error {
		return d.AddEmailProvider(name, p)
	}
}

// WithEmailSMTP builds and registers one named built-in SMTP email provider.
func WithEmailSMTP(name string, cfg emailprovider.SMTPConfig) Option {
	return func(d *Dispatcher) error {
		return d.AddEmailSMTP(name, cfg)
	}
}

// WithDiscordProvider registers one named discord provider.
func WithDiscordProvider(name string, p DiscordProvider) Option {
	return func(d *Dispatcher) error {
		return d.AddDiscordProvider(name, p)
	}
}

// WithDiscordConfig builds and registers one named built-in Discord provider.
func WithDiscordConfig(name string, cfg discordprovider.Config) Option {
	return func(d *Dispatcher) error {
		return d.AddDiscordProviderFromConfig(name, cfg)
	}
}

// WithTelegramProvider registers one named telegram provider.
func WithTelegramProvider(name string, p TelegramProvider) Option {
	return func(d *Dispatcher) error {
		return d.AddTelegramProvider(name, p)
	}
}

// WithTelegramConfig builds and registers one named built-in Telegram provider.
func WithTelegramConfig(name string, cfg telegramprovider.Config) Option {
	return func(d *Dispatcher) error {
		return d.AddTelegramProviderFromConfig(name, cfg)
	}
}

// WithSlackProvider registers one named Slack provider.
func WithSlackProvider(name string, p SlackProvider) Option {
	return func(d *Dispatcher) error {
		return d.AddSlackProvider(name, p)
	}
}

// WithSlackConfig builds and registers one named Slack provider.
func WithSlackConfig(name string, cfg slackprovider.Config) Option {
	return func(d *Dispatcher) error {
		return d.AddSlackProviderFromConfig(name, cfg)
	}
}

// WithWebhookProvider registers one named webhook provider.
func WithWebhookProvider(name string, p WebhookProvider) Option {
	return func(d *Dispatcher) error {
		return d.AddWebhookProvider(name, p)
	}
}

// WithWebhookConfig builds and registers one named built-in webhook provider.
func WithWebhookConfig(name string, cfg webhookprovider.Config) Option {
	return func(d *Dispatcher) error {
		return d.AddWebhookProviderFromConfig(name, cfg)
	}
}

func (d *Dispatcher) slackProviderFor(name string) (string, SlackProvider, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if name == "" {
		return "", nil, errors.New("slack destination requires provider name")
	}
	p, ok := d.slackProviders[name]
	if !ok {
		return "", nil, fmt.Errorf("slack provider %q not found", name)
	}
	return name, p, nil
}

func (d *Dispatcher) discordProviderFor(name string) (string, DiscordProvider, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if name == "" {
		return "", nil, errors.New("discord destination requires provider name")
	}
	p, ok := d.discord[name]
	if !ok {
		return "", nil, fmt.Errorf("discord provider %q not found", name)
	}
	return name, p, nil
}

func (d *Dispatcher) telegramProviderFor(name string) (string, TelegramProvider, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if name == "" {
		return "", nil, errors.New("telegram destination requires provider name")
	}
	p, ok := d.telegram[name]
	if !ok {
		return "", nil, fmt.Errorf("telegram provider %q not found", name)
	}
	return name, p, nil
}

func (d *Dispatcher) webhookProviderFor(name string) (string, WebhookProvider, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if name == "" {
		return "", nil, errors.New("webhook destination requires provider name")
	}
	p, ok := d.webhook[name]
	if !ok {
		return "", nil, fmt.Errorf("webhook provider %q not found", name)
	}
	return name, p, nil
}

func (d *Dispatcher) emailProviderFor(name string) (string, EmailProvider, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if name == "" {
		return "", nil, errors.New("email destination requires provider name")
	}
	p, ok := d.emailProviders[name]
	if !ok {
		return "", nil, fmt.Errorf("email provider %q not found", name)
	}
	return name, p, nil
}

// AddEmailProvider registers or replaces one named email provider.
func (d *Dispatcher) AddEmailProvider(name string, p EmailProvider) error {
	if name == "" {
		return errors.New("email provider name is required")
	}
	if p == nil {
		return errors.New("email provider is nil")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.emailProviders[name] = p
	return nil
}

// AddEmailSMTP builds and registers one named built-in SMTP email provider.
func (d *Dispatcher) AddEmailSMTP(name string, cfg emailprovider.SMTPConfig) error {
	p, err := emailprovider.NewSMTPProvider(cfg)
	if err != nil {
		return err
	}
	return d.AddEmailProvider(name, &smtpEmailAdapter{provider: p})
}

// RemoveEmailProvider removes one named email provider.
func (d *Dispatcher) RemoveEmailProvider(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.emailProviders, name)
}

// AddDiscordProvider registers or replaces one named discord provider.
func (d *Dispatcher) AddDiscordProvider(name string, p DiscordProvider) error {
	if name == "" {
		return errors.New("discord provider name is required")
	}
	if p == nil {
		return errors.New("discord provider is nil")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.discord[name] = p
	return nil
}

// AddDiscordProviderFromConfig builds and registers one named built-in Discord provider.
func (d *Dispatcher) AddDiscordProviderFromConfig(name string, cfg discordprovider.Config) error {
	p, err := discordprovider.NewProvider(cfg)
	if err != nil {
		return err
	}
	return d.AddDiscordProvider(name, p)
}

// RemoveDiscordProvider removes one named discord provider.
func (d *Dispatcher) RemoveDiscordProvider(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.discord, name)
}

// AddTelegramProvider registers or replaces one named telegram provider.
func (d *Dispatcher) AddTelegramProvider(name string, p TelegramProvider) error {
	if name == "" {
		return errors.New("telegram provider name is required")
	}
	if p == nil {
		return errors.New("telegram provider is nil")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.telegram[name] = p
	return nil
}

// AddTelegramProviderFromConfig builds and registers one named built-in Telegram provider.
func (d *Dispatcher) AddTelegramProviderFromConfig(name string, cfg telegramprovider.Config) error {
	p, err := telegramprovider.NewProvider(cfg)
	if err != nil {
		return err
	}
	return d.AddTelegramProvider(name, p)
}

// RemoveTelegramProvider removes one named telegram provider.
func (d *Dispatcher) RemoveTelegramProvider(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.telegram, name)
}

// AddSlackProvider registers or replaces one named Slack provider.
func (d *Dispatcher) AddSlackProvider(name string, p SlackProvider) error {
	if name == "" {
		return errors.New("slack provider name is required")
	}
	if p == nil {
		return fmt.Errorf("slack provider %q is nil", name)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.slackProviders[name] = p
	return nil
}

// AddSlackProviderFromConfig builds and registers one named Slack provider.
func (d *Dispatcher) AddSlackProviderFromConfig(name string, cfg slackprovider.Config) error {
	p, err := slackprovider.NewProvider(cfg)
	if err != nil {
		return err
	}
	return d.AddSlackProvider(name, p)
}

// RemoveSlackProvider removes one named Slack provider.
func (d *Dispatcher) RemoveSlackProvider(name string) error {
	if name == "" {
		return errors.New("slack provider name is required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.slackProviders, name)
	return nil
}

// EnableIdempotency enables in-memory idempotency for Notification.IdempotencyKey.
func (d *Dispatcher) EnableIdempotency(ttl time.Duration) error {
	if ttl <= 0 {
		return errors.New("idempotency ttl must be > 0")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.idemTTL = ttl
	if d.idemStore == nil {
		d.idemStore = make(map[string]time.Time)
	}
	return nil
}

// DisableIdempotency disables in-memory idempotency and clears in-memory keys.
func (d *Dispatcher) DisableIdempotency() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.idemTTL = 0
	d.idemStore = make(map[string]time.Time)
}

// SetMockFile is a legacy helper kept for compatibility.
// It enables provider logs in the parent folder of the given path.
func (d *Dispatcher) SetMockFile(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("mock file path is required")
	}
	if err := d.SetLogFolder(filepath.Dir(path)); err != nil {
		return err
	}
	d.mu.Lock()
	d.mockFile = path
	d.mu.Unlock()
	return nil
}

// SetMockMode enables/disables mock behavior for one channel/provider pair.
// Provider name is required.
func (d *Dispatcher) SetMockMode(channel Channel, provider string, enabled bool) error {
	if strings.TrimSpace(string(channel)) == "" {
		return errors.New("mock mode channel is required")
	}
	if strings.TrimSpace(provider) == "" {
		return errors.New("mock mode provider name is required")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.mockTargets == nil {
		d.mockTargets = make(map[string]bool)
	}
	d.mockTargets[mockTargetKey(channel, provider)] = enabled
	return nil
}

// SetLogFolder enables per-provider file logs in the provided folder.
func (d *Dispatcher) SetLogFolder(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("log folder path is required")
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.logFolder = path
	return nil
}

// AddWebhookProvider registers or replaces one named webhook provider.
func (d *Dispatcher) AddWebhookProvider(name string, p WebhookProvider) error {
	if name == "" {
		return errors.New("webhook provider name is required")
	}
	if p == nil {
		return errors.New("webhook provider is nil")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.webhook[name] = p
	return nil
}

// AddWebhookProviderFromConfig builds and registers one named built-in webhook provider.
func (d *Dispatcher) AddWebhookProviderFromConfig(name string, cfg webhookprovider.Config) error {
	p, err := webhookprovider.NewProvider(cfg)
	if err != nil {
		return err
	}
	return d.AddWebhookProvider(name, p)
}

// RemoveWebhookProvider removes one named webhook provider.
func (d *Dispatcher) RemoveWebhookProvider(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.webhook, name)
}
