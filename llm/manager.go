package llm

import (
	"context"
	"fmt"
	"sync"
	"worklog/db"
)

const (
	SettingProviderType = "provider_type"
	SettingAPIKey       = "provider_api_key"
	SettingBaseURL      = "provider_base_url"
	SettingModel        = "provider_model"
)

// ProviderManager holds the active provider and allows hot-swapping.
type ProviderManager struct {
	mu       sync.RWMutex
	provider Provider
}

// Get returns the current provider (safe for concurrent use).
func (pm *ProviderManager) Get() Provider {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.provider
}

// Reload reads settings from DB and rebuilds the provider.
func (pm *ProviderManager) Reload() error {
	p, err := BuildFromDB()
	if err != nil {
		return err
	}
	pm.mu.Lock()
	pm.provider = p
	pm.mu.Unlock()
	return nil
}

// BuildFromDB reads provider config from DB and returns a Provider.
// Falls back to Anthropic with env var if no DB config exists.
func BuildFromDB() (Provider, error) {
	keys := []string{SettingProviderType, SettingAPIKey, SettingBaseURL, SettingModel}
	cfg, err := db.GetAllSettings(keys)
	if err != nil {
		return nil, err
	}

	ptype := cfg[SettingProviderType]
	apiKey := cfg[SettingAPIKey]
	baseURL := cfg[SettingBaseURL]
	model := cfg[SettingModel]

	if ptype == "" || apiKey == "" {
		return nil, fmt.Errorf("no provider configured")
	}

	switch ptype {
	case "anthropic":
		return NewAnthropicProvider(apiKey), nil
	case "openai_compat":
		if baseURL == "" {
			return nil, fmt.Errorf("base_url required for openai_compat provider")
		}
		if model == "" {
			model = "deepseek-chat"
		}
		return NewOpenAICompatProvider(apiKey, baseURL, model), nil
	default:
		return nil, fmt.Errorf("unknown provider type: %s", ptype)
	}
}

// ProviderManager also implements Provider so it can be passed directly to handlers.
func (pm *ProviderManager) StreamChat(ctx context.Context, system string, messages []Message, onChunk StreamHandler) error {
	p := pm.Get()
	if p == nil {
		return fmt.Errorf("未配置 AI 提供商，请在设置中配置 API Key")
	}
	return p.StreamChat(ctx, system, messages, onChunk)
}

func (pm *ProviderManager) Chat(ctx context.Context, system string, messages []Message) (string, error) {
	p := pm.Get()
	if p == nil {
		return "", fmt.Errorf("未配置 AI 提供商，请在设置中配置 API Key")
	}
	return p.Chat(ctx, system, messages)
}

// NewManager creates a ProviderManager with an initial provider.
// fallback is used when no DB config exists (e.g. ANTHROPIC_API_KEY env var).
func NewManager(fallback Provider) *ProviderManager {
	pm := &ProviderManager{}
	// Try to load from DB; if not configured, use fallback
	if p, err := BuildFromDB(); err == nil {
		pm.provider = p
	} else {
		pm.provider = fallback
	}
	return pm
}
