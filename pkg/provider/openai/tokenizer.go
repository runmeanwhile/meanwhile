package openai

import (
	"context"
	"fmt"

	"github.com/runmeanwhile/meanwhile/pkg/agent"
	tiktoken "github.com/weaviate/tiktoken-go"
)

const openAIImageTokenEstimate = 256

// EstimateMessageTokens provides a best-effort token estimate for OpenAI models.
func (c *Client) EstimateMessageTokens(ctx context.Context, model string, msg agent.Message) (int, error) {
	_ = ctx
	if model == "" {
		return 0, fmt.Errorf("model required for token estimation")
	}
	enc, err := c.encodingForModel(model)
	if err != nil {
		return 0, err
	}

	text := agent.TextFromParts(msg.Parts)
	if text == "" {
		text = msg.Text()
	}
	tokens := 0
	if text != "" {
		tokens = len(enc.Encode(text, nil, nil))
	}
	if msg.ImageCount() > 0 {
		tokens += msg.ImageCount() * openAIImageTokenEstimate
	}
	return tokens, nil
}

func (c *Client) encodingForModel(model string) (*tiktoken.Tiktoken, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.tokenEncodings == nil {
		c.tokenEncodings = make(map[string]*tiktoken.Tiktoken)
	}
	if enc, ok := c.tokenEncodings[model]; ok {
		return enc, nil
	}
	enc, err := tiktoken.EncodingForModel(model)
	if err != nil {
		return nil, err
	}
	c.tokenEncodings[model] = enc
	return enc, nil
}
