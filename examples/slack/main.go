package main

import (
	"log"
	"os"
	"strings"

	"github.com/TheKrainBow/gotification"
	"github.com/TheKrainBow/gotification/examples/internal/envutil"
	"github.com/TheKrainBow/gotification/providers/slack"
)

func main() {
	_ = envutil.LoadFirstExisting(".env", "examples/slack/.env")

	dispatcher, err := gotification.NewDispatcher()
	if err != nil {
		log.Fatal(err)
	}

	workspace := envOrDefault("SLACK_PROVIDER", "default")
	if err := dispatcher.AddSlackProviderFromConfig(workspace, slack.Config{BotToken: requiredEnv("SLACK_BOT_TOKEN")}); err != nil {
		log.Fatal(err)
	}

	if err := dispatcher.SendSlackChannelMessage(
		workspace,
		requiredEnv("SLACK_CHANNEL_ID"),
		envOrDefault("SLACK_CHANNEL_MESSAGE", "Hello from gotification Slack channel example."),
	); err != nil {
		log.Fatalf("send slack channel failed: %v (retryable=%v)", err, gotification.Retryable(err))
	}

	if username := strings.TrimSpace(os.Getenv("SLACK_USERNAME")); username != "" {
		if err := dispatcher.SendSlackUserMP(
			workspace,
			username,
			envOrDefault("SLACK_USER_MESSAGE", "Hello from gotification Slack user example."),
		); err != nil {
			log.Fatalf("send slack user mp failed: %v (retryable=%v)", err, gotification.Retryable(err))
		}
	}

	log.Printf("slack messages sent")
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
