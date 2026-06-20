package llm

import "context"

type Message struct {
	Role    string
	Content string
}

type StreamHandler func(chunk string)

type Provider interface {
	StreamChat(ctx context.Context, system string, messages []Message, onChunk StreamHandler) error
	Chat(ctx context.Context, system string, messages []Message) (string, error)
}
