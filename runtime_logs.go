package gotification

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var unsafeFileChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func (d *Dispatcher) logSend(n Notification, dest Destination, providerName string, mocked bool) {
	line := formatSendLogLine(time.Now(), n, dest, providerName, mocked)
	fmt.Println(line)

	folder := d.logFolderPath()
	if folder == "" {
		return
	}
	if err := d.appendProviderLog(folder, dest, providerName, line); err != nil {
		fmt.Printf("[gotification] log write error: %v\n", err)
	}
}

func (d *Dispatcher) logFolderPath() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return strings.TrimSpace(d.logFolder)
}

func formatSendLogLine(ts time.Time, n Notification, dest Destination, providerName string, mocked bool) string {
	stamp := ts.Format("02/01/2006 15:04:05")
	mockTag := ""
	if mocked {
		mockTag = " [MOCKED]"
	}
	providerName = strings.TrimSpace(providerName)
	if providerName == "" {
		providerName = "default"
	}

	content := n.Content.Text
	if strings.TrimSpace(content) == "" {
		content = n.Content.HTML
	}

	typeTag := strings.ToUpper(string(dest.Channel))
	detail := ""
	switch dest.Channel {
	case ChannelEmail:
		detail = fmt.Sprintf("[to:%s] [Object: %s] [Content: %s]", dest.ID, n.Content.Subject, content)
	case ChannelSlack:
		if dest.Kind == DestinationSlackChannel {
			detail = fmt.Sprintf("[to:#%s] [Message: %s]", dest.ID, n.Content.Text)
		} else {
			detail = fmt.Sprintf("[to:%s] [Message: %s]", dest.ID, n.Content.Text)
		}
	case ChannelDiscord:
		if dest.Kind == DestinationDiscordChannel {
			detail = fmt.Sprintf("[to:#%s] [Message: %s]", dest.ID, n.Content.Text)
		} else {
			detail = fmt.Sprintf("[to:%s] [Message: %s]", dest.ID, n.Content.Text)
		}
	case ChannelWebhook, ChannelTelegram:
		detail = fmt.Sprintf("[to:%s] [Message: %s]", dest.ID, n.Content.Text)
	default:
		detail = fmt.Sprintf("[to:%s]", dest.ID)
	}

	return fmt.Sprintf("[%s]%s [%s] [provider:%s] %s", stamp, mockTag, typeTag, providerName, detail)
}

func (d *Dispatcher) appendProviderLog(folder string, dest Destination, providerName string, line string) error {
	providerName = strings.TrimSpace(providerName)
	if providerName == "" {
		providerName = "default"
	}
	channelDir := filepath.Join(folder, sanitizeFileToken(strings.ToLower(string(dest.Channel))))
	if err := os.MkdirAll(channelDir, 0o755); err != nil {
		return err
	}
	fileName := fmt.Sprintf("%s.log", sanitizeFileToken(providerName))
	path := filepath.Join(channelDir, fileName)

	d.logWriteMu.Lock()
	defer d.logWriteMu.Unlock()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(line + "\n")
	return err
}

func sanitizeFileToken(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "default"
	}
	return unsafeFileChars.ReplaceAllString(v, "_")
}
