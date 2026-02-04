package integration

import (
	"context"
	"fmt"
	"strings"
)

const emailChannel = "email"

// EmailMessage describes an outbound email.
type EmailMessage struct {
	To      string
	From    string
	Subject string
	Body    string
}

// EmailSender sends an email message.
type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

// EmailOption configures the email integration.
type EmailOption func(*EmailIntegration)

// EmailIntegration sends human requests via email.
type EmailIntegration struct {
	id        string
	sender    EmailSender
	from      string
	subject   string
	formatter func(Request) string
}

// NewEmail creates an email integration.
func NewEmail(sender EmailSender, opts ...EmailOption) (*EmailIntegration, error) {
	if sender == nil {
		return nil, fmt.Errorf("email sender required")
	}
	integration := &EmailIntegration{
		id:        "email",
		sender:    sender,
		subject:   "Meanwhile: human input requested",
		formatter: FormatPlainText,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(integration)
		}
	}
	if integration.id == "" {
		return nil, fmt.Errorf("email integration id required")
	}
	return integration, nil
}

// WithEmailID overrides the integration ID.
func WithEmailID(id string) EmailOption {
	return func(e *EmailIntegration) {
		e.id = id
	}
}

// WithEmailFrom sets the From header.
func WithEmailFrom(from string) EmailOption {
	return func(e *EmailIntegration) {
		e.from = from
	}
}

// WithEmailSubject sets the email subject.
func WithEmailSubject(subject string) EmailOption {
	return func(e *EmailIntegration) {
		e.subject = subject
	}
}

// WithEmailFormatter overrides the message formatter.
func WithEmailFormatter(formatter func(Request) string) EmailOption {
	return func(e *EmailIntegration) {
		if formatter != nil {
			e.formatter = formatter
		}
	}
}

// ID returns the integration ID.
func (e *EmailIntegration) ID() string { return e.id }

// Channel returns the channel identifier for email.
func (e *EmailIntegration) Channel() string { return emailChannel }

// Send delivers the human request over email.
func (e *EmailIntegration) Send(ctx context.Context, req Request) error {
	if e == nil || e.sender == nil {
		return fmt.Errorf("email sender required")
	}
	if req.Contact == "" {
		return fmt.Errorf("email contact required")
	}
	body := FormatPlainText(req)
	if e.formatter != nil {
		body = e.formatter(req)
	}
	subject := strings.TrimSpace(e.subject)
	if subject == "" {
		subject = "Meanwhile: human input requested"
	}
	msg := EmailMessage{
		To:      req.Contact,
		From:    e.from,
		Subject: subject,
		Body:    body,
	}
	return e.sender.Send(ctx, msg)
}
