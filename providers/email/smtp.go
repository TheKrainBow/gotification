package email

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/TheKrainBow/gotification/internal/notifyerr"
	"github.com/TheKrainBow/gotification/internal/validate"
)

// TLSMode controls SMTP transport security.
type TLSMode string

const (
	TLSModeStartTLS TLSMode = "starttls"
	TLSModePlain    TLSMode = "plain"
)

// SMTPConfig configures the built-in SMTP provider.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	Hello    string
	FromName string
	From     string
	TLSMode  TLSMode
	Timeout  time.Duration
}

// SMTPProvider sends emails through an SMTP server.
type SMTPProvider struct {
	cfg SMTPConfig
}

// NewSMTPProvider builds an SMTP provider from config.
func NewSMTPProvider(cfg SMTPConfig) (*SMTPProvider, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, errors.New("smtp host is required")
	}
	if cfg.Port <= 0 {
		return nil, errors.New("smtp port must be > 0")
	}
	if !validate.EmailAddress(cfg.From) {
		return nil, errors.New("smtp from must be a valid email address")
	}
	if cfg.TLSMode == "" {
		cfg.TLSMode = TLSModeStartTLS
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	return &SMTPProvider{cfg: cfg}, nil
}

// Send sends one email.
func (p *SMTPProvider) Send(ctx context.Context, to, subject, textBody, htmlBody string) error {
	if !validate.EmailAddress(to) {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("invalid recipient email address")}
	}
	if strings.TrimSpace(textBody) == "" && strings.TrimSpace(htmlBody) == "" {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: errors.New("email content text/html is empty")}
	}

	headerFrom := p.cfg.From
	if name := strings.TrimSpace(p.cfg.FromName); name != "" {
		headerFrom = (&mail.Address{Name: name, Address: p.cfg.From}).String()
	}

	msg, err := buildMIMEMessage(headerFrom, to, subject, textBody, htmlBody)
	if err != nil {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: err}
	}

	addr := net.JoinHostPort(p.cfg.Host, fmt.Sprintf("%d", p.cfg.Port))
	dialer := &net.Dialer{Timeout: p.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return classifySMTPError(err)
	}
	defer conn.Close()

	if deadline, ok := smtpDeadline(ctx, p.cfg.Timeout); ok {
		_ = conn.SetDeadline(deadline)
	}

	client, err := smtp.NewClient(conn, p.cfg.Host)
	if err != nil {
		return classifySMTPError(err)
	}
	defer client.Close()

	if hello := strings.TrimSpace(p.cfg.Hello); hello != "" {
		if err := client.Hello(hello); err != nil {
			return classifySMTPError(err)
		}
	}

	if p.cfg.TLSMode == TLSModeStartTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: errors.New("smtp server does not support STARTTLS")}
		}
		if err := client.StartTLS(&tls.Config{ServerName: p.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return classifySMTPError(err)
		}
	}

	if strings.TrimSpace(p.cfg.Username) != "" {
		auth := smtp.PlainAuth("", p.cfg.Username, p.cfg.Password, p.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return &notifyerr.Error{Kind: notifyerr.KindAuth, Cause: err}
		}
	}

	if err := client.Mail(p.cfg.From); err != nil {
		return classifySMTPError(err)
	}
	if err := client.Rcpt(to); err != nil {
		return classifySMTPError(err)
	}

	wc, err := client.Data()
	if err != nil {
		return classifySMTPError(err)
	}
	if _, err := wc.Write(msg); err != nil {
		_ = wc.Close()
		return classifySMTPError(err)
	}
	if err := wc.Close(); err != nil {
		return classifySMTPError(err)
	}
	if err := client.Quit(); err != nil {
		return classifySMTPError(err)
	}
	return nil
}

func smtpDeadline(ctx context.Context, timeout time.Duration) (time.Time, bool) {
	timeoutDeadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok {
		if ctxDeadline.Before(timeoutDeadline) {
			return ctxDeadline, true
		}
	}
	return timeoutDeadline, true
}

func classifySMTPError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: err}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: err}
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "auth") || strings.Contains(msg, "535") || strings.Contains(msg, "534") {
		return &notifyerr.Error{Kind: notifyerr.KindAuth, Cause: err}
	}
	if strings.Contains(msg, "invalid") || strings.Contains(msg, "bad address") || strings.Contains(msg, "mailbox") {
		return &notifyerr.Error{Kind: notifyerr.KindInvalidInput, Cause: err}
	}
	return &notifyerr.Error{Kind: notifyerr.KindTemporary, Cause: err}
}

func buildMIMEMessage(from, to, subject, textBody, htmlBody string) ([]byte, error) {
	var out bytes.Buffer
	w := bufio.NewWriter(&out)

	headers := []string{
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("To: %s", to),
		fmt.Sprintf("Subject: %s", sanitizeHeader(subject)),
		"MIME-Version: 1.0",
	}

	if strings.TrimSpace(htmlBody) == "" {
		headers = append(headers, "Content-Type: text/plain; charset=UTF-8")
		for _, h := range headers {
			fmt.Fprintf(w, "%s\r\n", h)
		}
		fmt.Fprint(w, "\r\n")
		fmt.Fprint(w, textBody)
		_ = w.Flush()
		return out.Bytes(), nil
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.SetBoundary("gotification-boundary")

	textPart, err := mw.CreatePart(mapToHeader(map[string]string{
		"Content-Type":              "text/plain; charset=UTF-8",
		"Content-Transfer-Encoding": "8bit",
	}))
	if err != nil {
		return nil, err
	}
	if _, err := textPart.Write([]byte(textBody)); err != nil {
		return nil, err
	}

	htmlPart, err := mw.CreatePart(mapToHeader(map[string]string{
		"Content-Type":              "text/html; charset=UTF-8",
		"Content-Transfer-Encoding": "8bit",
	}))
	if err != nil {
		return nil, err
	}
	if _, err := htmlPart.Write([]byte(htmlBody)); err != nil {
		return nil, err
	}
	if err := mw.Close(); err != nil {
		return nil, err
	}

	headers = append(headers, fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q", mw.Boundary()))
	for _, h := range headers {
		fmt.Fprintf(w, "%s\r\n", h)
	}
	fmt.Fprint(w, "\r\n")
	if _, err := body.WriteTo(w); err != nil {
		return nil, err
	}
	if err := w.Flush(); err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

func mapToHeader(values map[string]string) map[string][]string {
	h := make(map[string][]string, len(values))
	for k, v := range values {
		h[k] = []string{v}
	}
	return h
}

func sanitizeHeader(v string) string {
	v = strings.ReplaceAll(v, "\r", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	return strings.TrimSpace(v)
}
