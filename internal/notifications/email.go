package notifications

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/Astraxx04/pr-reviewer/pkg/logger"
)

// SMTPSettings holds the connection details for an SMTP server.
type SMTPSettings struct {
	Host     string
	Port     int
	Username string
	Password string
}

func (s SMTPSettings) configured() bool { return s.Host != "" && s.Port != 0 }

// ResolveEmail returns the SMTP settings and from address to send with for a
// channel. SMTP is configured per channel (DB-only); the stored password is
// decrypted here.
func ResolveEmail(ec EmailChannelConfig) (SMTPSettings, string) {
	return SMTPSettings{
		Host:     ec.SMTPHost,
		Port:     ec.SMTPPort,
		Username: ec.SMTPUsername,
		Password: DecryptSecret(ec.SMTPPassword),
	}, ec.From
}

// SendEmail delivers an HTML email over SMTP. Port 465 uses implicit TLS; other
// ports attempt STARTTLS when the server advertises it. Authentication is only
// performed when a username is set (internal relays often need none). It returns
// a descriptive error on failure so callers — including the Test button — can
// surface exactly what went wrong.
func SendEmail(ctx context.Context, s SMTPSettings, from string, to []string, subject, htmlBody string) error {
	if !s.configured() {
		return fmt.Errorf("SMTP not configured (set host and port)")
	}
	if from == "" {
		return fmt.Errorf("from address not configured")
	}
	if len(to) == 0 {
		return nil
	}

	msg := buildMessage(from, to, subject, htmlBody)
	addr := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
	start := time.Now()
	err := sendSMTP(ctx, s, addr, from, to, msg)
	logger.ExternalCall(ctx, "smtp", "send", start, err, "host", s.Host, "recipients", len(to))
	return err
}

// buildMessage assembles a minimal RFC 5322 HTML message.
func buildMessage(from string, to []string, subject, htmlBody string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(htmlBody)
	return []byte(b.String())
}

func sendSMTP(ctx context.Context, s SMTPSettings, addr, from string, to []string, msg []byte) error {
	dialer := &net.Dialer{Timeout: 15 * time.Second}

	var client *smtp.Client
	if s.Port == 465 {
		// Implicit TLS (SMTPS).
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.Host})
		if err != nil {
			return fmt.Errorf("smtp tls dial: %w", err)
		}
		client, err = smtp.NewClient(conn, s.Host)
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("smtp client: %w", err)
		}
	} else {
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
		client, err = smtp.NewClient(conn, s.Host)
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("smtp client: %w", err)
		}
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: s.Host}); err != nil {
				_ = client.Close()
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}
	defer func() { _ = client.Close() }()

	if s.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", s.Username, s.Password, s.Host)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(envelopeAddr(from)); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(envelopeAddr(rcpt)); err != nil {
			return fmt.Errorf("smtp rcpt %s: %w", rcpt, err)
		}
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := wc.Write(msg); err != nil {
		_ = wc.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return client.Quit()
}

// envelopeAddr extracts the bare address from a possibly display-formatted
// header value (e.g. "PR Reviewer <pr@x.com>" -> "pr@x.com") for use in the
// SMTP envelope (MAIL FROM / RCPT TO), which must not include a display name.
func envelopeAddr(addr string) string {
	if a, err := mail.ParseAddress(addr); err == nil {
		return a.Address
	}
	return addr
}
