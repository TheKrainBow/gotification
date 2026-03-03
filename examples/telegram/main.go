package main

import (
	"log"
	"os"
	"strings"

	"github.com/TheKrainBow/gotification"
	"github.com/TheKrainBow/gotification/examples/internal/envutil"
	"github.com/TheKrainBow/gotification/providers/telegram"
)

func main() {
	_ = envutil.LoadFirstExisting(".env", "examples/telegram/.env")

	dispatcher, err := gotification.NewDispatcher()
	if err != nil {
		log.Fatal(err)
	}

	if err := dispatcher.AddTelegramProviderFromConfig("default", telegram.Config{BotToken: requiredEnv("TELEGRAM_BOT_TOKEN")}); err != nil {
		log.Fatal(err)
	}

	if err := dispatcher.SendTelegramMessage(
		"default",
		requiredEnv("TELEGRAM_CHAT_ID"),
		envOrDefault("TELEGRAM_MESSAGE", "Hello from gotification Telegram example."),
	); err != nil {
		log.Fatalf("send telegram message failed: %v (retryable=%v)", err, gotification.Retryable(err))
	}

	log.Printf("telegram message sent")
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
