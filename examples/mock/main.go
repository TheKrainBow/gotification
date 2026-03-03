package main

import (
	"log"
	"os"
	"strings"

	"github.com/TheKrainBow/gotification"
	"github.com/TheKrainBow/gotification/examples/internal/envutil"
	"github.com/TheKrainBow/gotification/providers/telegram"
	"github.com/TheKrainBow/gotification/providers/webhook"
)

func main() {
	_ = envutil.LoadFirstExisting(".env", "examples/mock/.env")

	logFolder := envOrDefault("LOG_FOLDER", "./logs")
	dispatcher, err := gotification.NewDispatcher(
		gotification.WithLogFolder(logFolder),
	)
	if err != nil {
		log.Fatal(err)
	}

	// Configure providers exactly like production.
	if err := dispatcher.AddWebhookProviderFromConfig("default", webhook.Config{}); err != nil {
		log.Fatal(err)
	}
	if err := dispatcher.AddTelegramProviderFromConfig("default", telegram.Config{BotToken: requiredEnv("TELEGRAM_BOT_TOKEN")}); err != nil {
		log.Fatal(err)
	}

	// Toggle mock mode per provider type.
	// Here: webhook is mocked, telegram stays real.
	if err := dispatcher.SetMockMode(gotification.ChannelWebhook, "default", true); err != nil {
		log.Fatal(err)
	}
	if err := dispatcher.SetMockMode(gotification.ChannelTelegram, "default", false); err != nil {
		log.Fatal(err)
	}

	if err := dispatcher.SendWebhook(
		"default",
		envOrDefault("WEBHOOK_URL", "https://example.com/hooks/mock"),
		envOrDefault("WEBHOOK_MESSAGE", "This webhook send is MOCKED."),
	); err != nil {
		log.Fatalf("send webhook failed: %v", err)
	}

	if err := dispatcher.SendTelegramMessage(
		"default",
		requiredEnv("TELEGRAM_CHAT_ID"),
		envOrDefault("TELEGRAM_MESSAGE", "This telegram send is REAL."),
	); err != nil {
		log.Fatalf("send telegram failed: %v", err)
	}

	log.Printf("done; check stdout and logs in %s", logFolder)
}

func requiredEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return value
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
