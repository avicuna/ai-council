package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/avicuna/ai-council-personal/internal/config"
)

// AnthropicProvider implements the Provider interface for Anthropic's Claude models.
type AnthropicProvider struct {
	client anthropic.Client
	config config.ModelConfig
}

// NewAnthropicProvider creates a new Anthropic provider.
// The API key is read from the ANTHROPIC_API_KEY environment variable.
func NewAnthropicProvider(modelCfg config.ModelConfig) (*AnthropicProvider, error) {
	// Verify API key is available
	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY environment variable is not set")
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	return &AnthropicProvider{
		client: client,
		config: modelCfg,
	}, nil
}

// Name returns the human-friendly name of the provider.
func (p *AnthropicProvider) Name() string {
	return p.config.Name
}

// Available checks if the provider is available (API key is set).
func (p *AnthropicProvider) Available() bool {
	return config.Available(p.config.Model)
}

// Query sends a request to the Anthropic API and returns the response.
func (p *AnthropicProvider) Query(ctx context.Context, req *Request) (*Response, error) {
	start := time.Now()

	// Build the messages array
	messages := []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(req.UserPrompt)),
	}

	// Build the request parameters
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(p.config.Model),
		MaxTokens: int64(req.MaxTokens),
		Messages:  messages,
	}

	// Add system prompt if provided
	if req.SystemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{
				Type: "text",
				Text: req.SystemPrompt,
			},
		}
	}

	// Add temperature if provided (omit for reasoning models)
	if req.Temperature != nil {
		params.Temperature = anthropic.Float(*req.Temperature)
	}

	// Make the API call
	message, err := p.client.Messages.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	// Calculate latency
	latency := time.Since(start)

	// Extract text content from response
	var content strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			content.WriteString(block.Text)
		}
	}

	// Build response
	resp := &Response{
		Content:      content.String(),
		Model:        string(message.Model),
		Name:         p.config.Name,
		InputTokens:  int(message.Usage.InputTokens),
		OutputTokens: int(message.Usage.OutputTokens),
		LatencyMs:    latency.Milliseconds(),
	}

	return resp, nil
}
