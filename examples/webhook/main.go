package main

import (
	"log"
	"os"
	"strings"

	"github.com/TheKrainBow/gotification"
	"github.com/TheKrainBow/gotification/examples/internal/envutil"
	"github.com/TheKrainBow/gotification/providers/webhook"
)

func main() {
	_ = envutil.LoadFirstExisting(".env", "examples/webhook/.env")

	dispatcher, err := gotification.NewDispatcher()
	if err != nil {
		log.Fatal(err)
	}

	if err := dispatcher.AddWebhookProviderFromConfig("default", webhook.Config{}); err != nil {
		log.Fatal(err)
	}

	if err := dispatcher.SendWebhook(
		"default",
		requiredEnv("WEBHOOK_URL"),
		envOrDefault("WEBHOOK_MESSAGE", "Hello from gotification webhook example."),
	); err != nil {
		log.Fatalf("send webhook failed: %v (retryable=%v)", err, gotification.Retryable(err))
	}

	log.Printf("webhook sent")
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
