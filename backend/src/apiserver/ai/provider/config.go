package provider

import "fmt"

// NewChatModel creates a ChatModel based on the provider name.
func NewChatModel(providerName, apiKey, model string, maxTokens int) (ChatModel, error) {
	switch providerName {
	case "anthropic":
		return NewAnthropicProvider(apiKey, model, maxTokens), nil
	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", providerName)
	}
}
