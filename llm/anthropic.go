package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type AnthropicProvider struct {
	client anthropic.Client
	model  anthropic.Model
}

func NewAnthropicProvider(apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		client: anthropic.NewClient(option.WithAPIKey(apiKey)),
		model:  anthropic.ModelClaudeSonnet4_6,
	}
}

func buildMessages(messages []Message) []anthropic.MessageParam {
	var params []anthropic.MessageParam
	for _, m := range messages {
		if m.Role == "user" {
			params = append(params, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		} else {
			params = append(params, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		}
	}
	return params
}

func (p *AnthropicProvider) StreamChat(ctx context.Context, system string, messages []Message, onChunk StreamHandler) error {
	stream := p.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  buildMessages(messages),
	})
	defer stream.Close()

	for stream.Next() {
		event := stream.Current()
		switch e := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			if delta, ok := e.Delta.AsAny().(anthropic.TextDelta); ok {
				onChunk(delta.Text)
			}
		}
	}
	return stream.Err()
}

func (p *AnthropicProvider) Chat(ctx context.Context, system string, messages []Message) (string, error) {
	resp, err := p.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     p.model,
		MaxTokens: 4096,
		System:    []anthropic.TextBlockParam{{Text: system}},
		Messages:  buildMessages(messages),
	})
	if err != nil {
		return "", fmt.Errorf("anthropic chat: %w", err)
	}
	var sb strings.Builder
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.TextBlock); ok {
			sb.WriteString(t.Text)
		}
	}
	return sb.String(), nil
}
