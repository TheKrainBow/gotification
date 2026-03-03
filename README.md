# gotification

`gotification` is a reusable Go package that sends notifications through:

- Email (SMTP)
- Slack DMs
- Slack channel messages
- Discord DMs
- Discord channel messages
- Telegram bot messages
- Generic webhook messages

It is designed for queue workers and background jobs. The library does **not** implement retries. Instead, it returns typed errors so callers can decide whether an error is retryable.

## Installation

```bash
go get github.com/TheKrainBow/gotification
```

## Quick Start

```go
package main

import (
    "log"

    "github.com/TheKrainBow/gotification"
    "github.com/TheKrainBow/gotification/providers/discord"
    "github.com/TheKrainBow/gotification/providers/email"
    "github.com/TheKrainBow/gotification/providers/slack"
    "github.com/TheKrainBow/gotification/providers/telegram"
    "github.com/TheKrainBow/gotification/providers/webhook"
)

func main() {
    d, err := gotification.NewDispatcher()
    if err != nil {
        log.Fatal(err)
    }
    _ = d.AddEmailSMTP("default", email.SMTPConfig{
        Host: "smtp.internal.local",
        Port: 587,
        From: "noreply@example.com",
        TLSMode: email.TLSModeStartTLS,
    })
    _ = d.AddSlackProviderFromConfig("workspace-a", slack.Config{BotToken: "xoxb-a"})
    _ = d.AddDiscordProviderFromConfig("default", discord.Config{BotToken: "discord-token"})
    _ = d.AddTelegramProviderFromConfig("default", telegram.Config{BotToken: "telegram-token"})
    _ = d.AddWebhookProviderFromConfig("default", webhook.Config{})

    _ = d.SendSlackChannelMessage("workspace-a", "C123", "Release 1.2.3 completed.")
    _ = d.SendSlackUserMP("workspace-a", "heinz", "Release 1.2.3 completed.")
    _ = d.SendMail("default", "ops@example.com", "Deploy done", "Release 1.2.3 completed.")
    _ = d.SendTelegramMessage("default", "123456789", "Release 1.2.3 completed.")
    err = d.SendWebhook("default", "https://example.com/hooks/deploy", "Release 1.2.3 completed.")

    if err != nil {
        log.Printf("send failed: %v", err)
        log.Printf("retryable: %v", gotification.Retryable(err))
    }
}
```

## Destination Model

Destination fields:

- `Channel`: `email`, `slack`, `discord`, `telegram`, `webhook`
- `Kind`:
  - `email_address`
  - `slack_user`
  - `slack_channel`
  - `discord_user`
  - `discord_channel`
  - `telegram_chat`
  - `webhook_url`
- `ID`: email address, Slack ID, or Discord numeric ID
- `Provider`: selects provider instance (mainly Slack workspaces)
- `Meta`: optional extensibility map

Expected IDs:

- Email: address like `user@example.com`
- Slack user: ID like `U...`
- Slack channel: ID like `C...`
- Discord user/channel: numeric snowflake IDs
- Telegram chat: numeric chat id (for example `123456789` or `-100123...`) or `@channel_username`
- Webhook: full `http://` or `https://` URL

## Multiple Slack Providers

Register multiple Slack providers with names:

```go
gotification.WithSlackProvider("workspace-a", slackA)
gotification.WithSlackProvider("workspace-b", slackB)
```

Route per destination using `Destination.Provider` (required).

## Error Handling

`Send` returns `error` and may return `errors.Join(...)` for multi-destination sends.
The core typed error is `NotifyError`:

- `Kind`: `invalid_input`, `auth`, `not_found`, `rate_limited`, `temporary`
- `Channel`
- `Provider`
- `Dest`
- `RetryAfter` (for rate limits)
- `Cause`

Use `gotification.Retryable(err)` to detect whether retry makes sense.
Only `rate_limited` and `temporary` are retryable.

## Retry Policy

This library intentionally does not retry or sleep.
Caller code (worker/queue) is responsible for retry strategy.

## Optional Idempotency (In-Memory)

You can enable a process-local idempotency safety net:

```go
d, _ := gotification.NewDispatcher(gotification.WithIdempotencyTTL(10 * time.Minute))
```

Then set `Notification.IdempotencyKey`. A successful send with the same key is
rejected during TTL. Failed sends do not lock the key, so caller retries still work.

This is an in-memory safeguard only; strong idempotency should still be handled
by caller infrastructure (DB/Redis/queue dedupe).

## Runtime Mock Mode (Per Provider)

You can keep the same dispatcher setup and toggle mock behavior per channel/provider:

```go
d, _ := gotification.NewDispatcher(
    gotification.WithLogFolder("./notifications-logs"),
)
_ = d.SetMockMode(gotification.ChannelSlack, "workspace-a", true) // mock only Slack workspace-a
_ = d.SetMockMode(gotification.ChannelDiscord, "default", false)  // keep Discord real
```

Every send is logged to stdout. When `WithLogFolder` is configured, one file per
provider is also appended in a channel subfolder in that folder:

```text
<log-folder>/
  slack/
    workspace-a.log
  discord/
    default.log
  email/
    default.log
```

Provider name is mandatory. Use:
- Slack: your workspace key (for example `workspace-a`)
- Email/Discord/Telegram/Webhook: `default`

When mock mode is enabled for a target, gotification skips the remote API call and
prints `[MOCKED]` in the stdout/file log line.

## Examples

Each provider has a dedicated runnable example with its own `.env.example`:

- `examples/email`
- `examples/slack`
- `examples/discord`
- `examples/telegram`
- `examples/webhook`
- `examples/mock`

To run one:

```bash
cp examples/slack/.env.example examples/slack/.env
go run ./examples/slack
```

## Mocking Behavior

Use dispatcher mock mode flags (`SetMockMode` / `WithMockMode`) to skip real API
calls per channel/provider while keeping the same send code paths.

## License

MIT
