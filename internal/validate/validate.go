package validate

import (
	"net/mail"
	"net/url"
	"regexp"
	"strings"
)

var discordSnowflakeRe = regexp.MustCompile(`^[0-9]+$`)
var telegramNumericChatIDRe = regexp.MustCompile(`^-?[0-9]+$`)
var telegramUsernameChatIDRe = regexp.MustCompile(`^@[A-Za-z0-9_]+$`)

func EmailAddress(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	a, err := mail.ParseAddress(v)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(a.Address), v)
}

func SlackID(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	return !strings.ContainsAny(v, " \t\n\r")
}

func DiscordID(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	return discordSnowflakeRe.MatchString(v)
}

func HTTPURL(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	u, err := url.Parse(v)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return u.Host != ""
}

func TelegramChatID(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" {
		return false
	}
	return telegramNumericChatIDRe.MatchString(v) || telegramUsernameChatIDRe.MatchString(v)
}
