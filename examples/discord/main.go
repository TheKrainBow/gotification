package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/TheKrainBow/gotification"
	"github.com/TheKrainBow/gotification/examples/internal/envutil"
	"github.com/TheKrainBow/gotification/providers/discord"
)

func main() {
	_ = envutil.LoadFirstExisting(".env", "examples/discord/.env")

	dispatcher, err := gotification.NewDispatcher()
	if err != nil {
		log.Fatal(err)
	}

	if err := dispatcher.AddDiscordProviderFromConfig("default", discord.Config{BotToken: requiredEnv("DISCORD_BOT_TOKEN")}); err != nil {
		log.Fatal(err)
	}

	n := gotification.Notification{
		Name: "discord-channel-example",
		Content: gotification.Content{
			Text: envOrDefault("DISCORD_MESSAGE", "Hello from gotification Discord example."),
		},
	}
	dest := gotification.Destination{
		Channel:  gotification.ChannelDiscord,
		Kind:     gotification.DestinationDiscordChannel,
		ID:       requiredEnv("DISCORD_CHANNEL_ID"),
		Provider: "default",
	}

	if err := dispatcher.Send(context.Background(), n, []gotification.Destination{dest}); err != nil {
		log.Fatalf("send discord message failed: %v (retryable=%v)", err, gotification.Retryable(err))
	}

	log.Printf("discord message sent")
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
