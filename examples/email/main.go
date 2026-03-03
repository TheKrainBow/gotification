package main

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/TheKrainBow/gotification"
	"github.com/TheKrainBow/gotification/examples/internal/envutil"
	"github.com/TheKrainBow/gotification/providers/email"
)

func main() {
	_ = envutil.LoadFirstExisting(".env", "examples/email/.env")

	dispatcher, err := gotification.NewDispatcher()
	if err != nil {
		log.Fatal(err)
	}

	tlsMode := email.TLSModeStartTLS
	if strings.EqualFold(envOrDefault("SMTP_TLS_MODE", "starttls"), "plain") {
		tlsMode = email.TLSModePlain
	}

	if err := dispatcher.AddEmailSMTP("default", email.SMTPConfig{
		Host:     requiredEnv("SMTP_HOST"),
		Port:     requiredEnvInt("SMTP_PORT"),
		Username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		Password: strings.TrimSpace(os.Getenv("SMTP_PASSWORD")),
		Hello:    strings.TrimSpace(os.Getenv("SMTP_HELO")),
		FromName: strings.TrimSpace(os.Getenv("SMTP_FROM_NAME")),
		From:     requiredEnv("SMTP_FROM"),
		TLSMode:  tlsMode,
	}); err != nil {
		log.Fatal(err)
	}

	if err := dispatcher.SendMail(
		"default",
		requiredEnv("MAIL_TO"),
		envOrDefault("MAIL_SUBJECT", "gotification email test"),
		envOrDefault("MAIL_BODY", "Hello from gotification email example."),
	); err != nil {
		log.Fatalf("send mail failed: %v (retryable=%v)", err, gotification.Retryable(err))
	}

	log.Printf("email sent")
}

func requiredEnv(key string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		log.Fatalf("missing required env var: %s", key)
	}
	return value
}

func requiredEnvInt(key string) int {
	value := requiredEnv(key)
	n, err := strconv.Atoi(value)
	if err != nil {
		log.Fatalf("invalid %s=%q: %v", key, value, err)
	}
	return n
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
