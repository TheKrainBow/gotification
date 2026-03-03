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
	// Configure once at server startup.
	dispatcher, err := gotification.NewDispatcher()
	if err != nil {
		log.Fatal(err)
	}
	if err := dispatcher.AddEmailSMTP("default", email.SMTPConfig{
		Host:    "smtp.internal.local",
		Port:    587,
		From:    "noreply@example.com",
		TLSMode: email.TLSModeStartTLS,
	}); err != nil {
		log.Fatal(err)
	}
	if err := dispatcher.AddSlackProviderFromConfig("workspace-a", slack.Config{BotToken: "xoxb-workspace-a"}); err != nil {
		log.Fatal(err)
	}
	if err := dispatcher.AddSlackProviderFromConfig("workspace-b", slack.Config{BotToken: "xoxb-workspace-b"}); err != nil {
		log.Fatal(err)
	}
	if err := dispatcher.AddDiscordProviderFromConfig("default", discord.Config{BotToken: "discord-bot-token"}); err != nil {
		log.Fatal(err)
	}
	if err := dispatcher.AddTelegramProviderFromConfig("default", telegram.Config{BotToken: "telegram-bot-token"}); err != nil {
		log.Fatal(err)
	}
	if err := dispatcher.AddWebhookProviderFromConfig("default", webhook.Config{}); err != nil {
		log.Fatal(err)
	}

	// Easy runtime calls.
	if err := dispatcher.SendSlackChannelMessage("workspace-a", "C012345", "Deploy done"); err != nil {
		log.Printf("slack channel send error: %v", err)
	}
	if err := dispatcher.SendSlackUserMP("workspace-b", "heinz", "Hello from gotification"); err != nil {
		log.Printf("slack user MP send error: %v", err)
	}
	if err := dispatcher.SendMail("default", "dev@example.com", "Deployment complete", "Version v1.4.2 deployed successfully."); err != nil {
		log.Printf("mail send error: %v", err)
	}
	if err := dispatcher.SendTelegramMessage("default", "123456789", "Hello from Telegram"); err != nil {
		log.Printf("telegram send error: %v", err)
	}
	if err := dispatcher.SendWebhook("default", "https://example.com/hooks/deploy", "Deploy done"); err != nil {
		log.Printf("webhook send error: %v", err)
	}
}
