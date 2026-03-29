package provider

import (
	"context"
	"fmt"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// OpenAICompatProvider implements Provider for OpenAI-compatible APIs.
// This includes GPT models, o3/o4 reasoning models, DeepSeek, and Grok.
type OpenAICompatProvider struct {
	name        string
	client      openai.Client
	model       string
	isReasoning bool
}

// NewOpenAICompatProvider creates a new OpenAI-compatible provider.
// For reasoning models (o3, o4, DeepSeek R1), set isReasoning=true to:
//   - Convert system messages to user prefix
//   - Omit temperature parameter
//   - Use max_completion_tokens instead of max_tokens
func NewOpenAICompatProvider(name, baseURL, apiKey, model string, isReasoning bool) (*OpenAICompatProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("%s: API key is required", name)
	}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}

	client := openai.NewClient(opts...)

	return &OpenAICompatProvider{
		name:        name,
		client:      client,
		model:       model,
		isReasoning: isReasoning,
	}, nil
}

// NewGPTProvider creates a provider for OpenAI GPT models.
func NewGPTProvider(apiKey, model string) (*OpenAICompatProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	return NewOpenAICompatProvider("gpt", "", apiKey, model, false)
}

// NewO3Provider creates a provider for OpenAI o3/o4 reasoning models.
func NewO3Provider(apiKey, model string) (*OpenAICompatProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	return NewOpenAICompatProvider("o3", "", apiKey, model, true)
}

// NewDeepSeekProvider creates a provider for DeepSeek models.
func NewDeepSeekProvider(apiKey, model string) (*OpenAICompatProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("DEEPSEEK_API_KEY")
	}
	return NewOpenAICompatProvider("deepseek", "https://api.deepseek.com/v1", apiKey, model, true)
}

// NewGrokProvider creates a provider for X.AI Grok models.
func NewGrokProvider(apiKey, model string) (*OpenAICompatProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("XAI_API_KEY")
	}
	return NewOpenAICompatProvider("grok", "https://api.x.ai/v1", apiKey, model, false)
}

// Name returns the provider name.
func (p *OpenAICompatProvider) Name() string {
	return p.name
}

// Available checks if the provider is available.
// Since the provider is created with a valid client in the constructor,
// this always returns true.
func (p *OpenAICompatProvider) Available() bool {
	return true
}

// Query sends a request to the OpenAI-compatible API.
func (p *OpenAICompatProvider) Query(ctx context.Context, req *Request) (*Response, error) {
	messages := p.buildMessages(req)

	params := openai.ChatCompletionNewParams{
		Model:    p.model, // ChatModel is just a string alias
		Messages: messages,
	}

	// Reasoning models use max_completion_tokens and omit temperature
	if p.isReasoning {
		if req.MaxTokens > 0 {
			params.MaxCompletionTokens = openai.Int(int64(req.MaxTokens))
		}
	} else {
		// Standard models use max_tokens and temperature
		if req.MaxTokens > 0 {
			params.MaxTokens = openai.Int(int64(req.MaxTokens))
		}
		if req.Temperature != nil {
			params.Temperature = openai.Float(*req.Temperature)
		}
	}

	completion, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("%s query failed: %w", p.name, err)
	}

	if len(completion.Choices) == 0 {
		return nil, fmt.Errorf("%s returned no choices", p.name)
	}

	return &Response{
		Content:      completion.Choices[0].Message.Content,
		Model:        completion.Model,
		Name:         p.name,
		InputTokens:  int(completion.Usage.PromptTokens),
		OutputTokens: int(completion.Usage.CompletionTokens),
	}, nil
}

// buildMessages constructs the messages array for the API request.
// For reasoning models, system messages are converted to a user message prefix.
func (p *OpenAICompatProvider) buildMessages(req *Request) []openai.ChatCompletionMessageParamUnion {
	if p.isReasoning {
		// Reasoning models: combine system and user prompts into a single user message
		userContent := req.UserPrompt
		if req.SystemPrompt != "" {
			userContent = fmt.Sprintf("[System instructions]\n%s\n\n%s", req.SystemPrompt, req.UserPrompt)
		}
		return []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(userContent),
		}
	}

	// Standard models: separate system and user messages
	messages := make([]openai.ChatCompletionMessageParamUnion, 0, 2)
	if req.SystemPrompt != "" {
		messages = append(messages, openai.SystemMessage(req.SystemPrompt))
	}
	messages = append(messages, openai.UserMessage(req.UserPrompt))
	return messages
}
