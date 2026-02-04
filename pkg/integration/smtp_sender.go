package integration

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTPConfig configures SMTP delivery.
type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	Timeout  time.Duration
	TLS      *tls.Config
}

// SMTPSender sends email via SMTP.
type SMTPSender struct {
	cfg SMTPConfig
}

// NewSMTPSender creates an SMTP sender.
func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, fmt.Errorf("smtp host required")
	}
	if cfg.Port == 0 {
		cfg.Port = 587
	}
	if strings.TrimSpace(cfg.From) == "" {
		return nil, fmt.Errorf("smtp from address required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.TLS == nil {
		cfg.TLS = &tls.Config{
			ServerName: cfg.Host,
			MinVersion: tls.VersionTLS12,
		}
	}
	return &SMTPSender{cfg: cfg}, nil
}

// Send sends an email via SMTP with STARTTLS when available.
func (s *SMTPSender) Send(ctx context.Context, msg EmailMessage) error {
	if s == nil {
		return fmt.Errorf("smtp sender required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateEmailMessage(msg); err != nil {
		return err
	}

	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	dialer := net.Dialer{Timeout: s.cfg.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	client, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() {
		_ = client.Close()
	}()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(s.cfg.TLS); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if s.cfg.Username != "" {
		auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(s.cfg.From); err != nil {
		return fmt.Errorf("smtp from: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := writer.Write([]byte(formatEmailMessage(s.cfg.From, msg))); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

func validateEmailMessage(msg EmailMessage) error {
	if strings.TrimSpace(msg.To) == "" {
		return fmt.Errorf("email to required")
	}
	if strings.TrimSpace(msg.Subject) == "" {
		return fmt.Errorf("email subject required")
	}
	if strings.TrimSpace(msg.Body) == "" {
		return fmt.Errorf("email body required")
	}
	if containsNewline(msg.To) || containsNewline(msg.From) || containsNewline(msg.Subject) {
		return fmt.Errorf("email headers must not contain newlines")
	}
	return nil
}

func containsNewline(value string) bool {
	return strings.Contains(value, "\n") || strings.Contains(value, "\r")
}

func formatEmailMessage(from string, msg EmailMessage) string {
	subject := msg.Subject
	if subject == "" {
		subject = "Meanwhile: human input requested"
	}
	lines := []string{
		fmt.Sprintf("To: %s", msg.To),
		fmt.Sprintf("From: %s", from),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		msg.Body,
	}
	return strings.Join(lines, "\r\n")
}
