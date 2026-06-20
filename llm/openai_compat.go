package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// OpenAICompatProvider works with any OpenAI-compatible API:
// DeepSeek, Qwen, GLM, OpenAI, Groq, etc.
type OpenAICompatProvider struct {
	apiKey  string
	baseURL string
	model   string
	client  *http.Client
}

func NewOpenAICompatProvider(apiKey, baseURL, model string) *OpenAICompatProvider {
	return &OpenAICompatProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		client:  &http.Client{},
	}
}

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (p *OpenAICompatProvider) buildBody(system string, messages []Message, stream bool) ([]byte, error) {
	msgs := []oaiMessage{{Role: "system", Content: system}}
	for _, m := range messages {
		msgs = append(msgs, oaiMessage{Role: m.Role, Content: m.Content})
	}
	return json.Marshal(map[string]any{
		"model":    p.model,
		"messages": msgs,
		"stream":   stream,
	})
}

func (p *OpenAICompatProvider) doRequest(ctx context.Context, body []byte, stream bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(b))
	}
	return resp, nil
}

func (p *OpenAICompatProvider) StreamChat(ctx context.Context, system string, messages []Message, onChunk StreamHandler) error {
	body, err := p.buildBody(system, messages, true)
	if err != nil {
		return err
	}
	resp, err := p.doRequest(ctx, body, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			onChunk(chunk.Choices[0].Delta.Content)
		}
	}
	return scanner.Err()
}

func (p *OpenAICompatProvider) Chat(ctx context.Context, system string, messages []Message) (string, error) {
	body, err := p.buildBody(system, messages, false)
	if err != nil {
		return "", err
	}
	resp, err := p.doRequest(ctx, body, false)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response from API")
	}
	return result.Choices[0].Message.Content, nil
}
