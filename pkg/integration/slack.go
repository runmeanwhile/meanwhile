package integration

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/slack-go/slack"
)

const slackChannel = "slack"

// SlackClient defines the subset of slack.Client used by the integration.
type SlackClient interface {
	PostMessageContext(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error)
}

// SlackOption configures the Slack integration.
type SlackOption func(*SlackIntegration)

// SlackIntegration sends human requests to Slack.
type SlackIntegration struct {
	id        string
	client    SlackClient
	formatter func(Request) string
}

// NewSlack creates a Slack integration using a slack-go client.
func NewSlack(client SlackClient, opts ...SlackOption) (*SlackIntegration, error) {
	if client == nil {
		return nil, fmt.Errorf("slack client required")
	}
	integration := &SlackIntegration{
		id:        "slack",
		client:    client,
		formatter: FormatMarkdown,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(integration)
		}
	}
	if integration.id == "" {
		return nil, fmt.Errorf("slack integration id required")
	}
	return integration, nil
}

// NewSlackClient constructs a slack-go client with a default timeout.
func NewSlackClient(token string) (*slack.Client, error) {
	if token == "" {
		return nil, fmt.Errorf("slack token required")
	}
	httpClient := &http.Client{Timeout: 10 * time.Second}
	return slack.New(token, slack.OptionHTTPClient(httpClient)), nil
}

// WithSlackID overrides the integration ID.
func WithSlackID(id string) SlackOption {
	return func(s *SlackIntegration) {
		s.id = id
	}
}

// WithSlackFormatter overrides the message formatter.
func WithSlackFormatter(formatter func(Request) string) SlackOption {
	return func(s *SlackIntegration) {
		if formatter != nil {
			s.formatter = formatter
		}
	}
}

// ID returns the integration ID.
func (s *SlackIntegration) ID() string { return s.id }

// Channel returns the channel identifier for Slack.
func (s *SlackIntegration) Channel() string { return slackChannel }

// Send posts the human request to Slack.
func (s *SlackIntegration) Send(ctx context.Context, req Request) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("slack client required")
	}
	if req.Contact == "" {
		return fmt.Errorf("slack contact required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	body := FormatMarkdown(req)
	if s.formatter != nil {
		body = s.formatter(req)
	}
	_, _, err := s.client.PostMessageContext(ctx, req.Contact, slack.MsgOptionText(body, false))
	if err != nil {
		return fmt.Errorf("slack post message: %w", err)
	}
	return nil
}
