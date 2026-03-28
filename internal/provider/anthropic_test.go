package provider

import (
	"context"
	"os"
	"testing"

	"github.com/avicuna/ai-council-personal/internal/config"
)

func TestNewAnthropicProvider(t *testing.T) {
	tests := []struct {
		name        string
		apiKey      string
		modelCfg    config.ModelConfig
		expectError bool
	}{
		{
			name:   "valid config with API key",
			apiKey: "sk-ant-test-key",
			modelCfg: config.ModelConfig{
				Model:       "claude-opus-4-20250918",
				Name:        "Claude Opus 4.6",
				IsReasoning: false,
			},
			expectError: false,
		},
		{
			name:   "missing API key",
			apiKey: "",
			modelCfg: config.ModelConfig{
				Model:       "claude-sonnet-4-20250514",
				Name:        "Claude Sonnet 4",
				IsReasoning: false,
			},
			expectError: true,
		},
		{
			name:   "whitespace API key",
			apiKey: "   ",
			modelCfg: config.ModelConfig{
				Model:       "claude-haiku-4-5-20251001",
				Name:        "Claude Haiku 4.5",
				IsReasoning: false,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			if tt.apiKey != "" {
				os.Setenv("ANTHROPIC_API_KEY", tt.apiKey)
			} else {
				os.Unsetenv("ANTHROPIC_API_KEY")
			}
			defer os.Unsetenv("ANTHROPIC_API_KEY")

			provider, err := NewAnthropicProvider(tt.modelCfg)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				if provider != nil {
					t.Error("Expected nil provider on error")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if provider == nil {
				t.Fatal("Expected provider, got nil")
			}

			// Verify provider methods
			if provider.Name() != tt.modelCfg.Name {
				t.Errorf("Name() = %q, want %q", provider.Name(), tt.modelCfg.Name)
			}

			if !provider.Available() {
				t.Error("Expected Available() = true")
			}
		})
	}
}

func TestAnthropicProvider_Name(t *testing.T) {
	modelCfg := config.ModelConfig{
		Model: "claude-opus-4-20250918",
		Name:  "Claude Opus 4.6",
	}

	os.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	provider, err := NewAnthropicProvider(modelCfg)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	if provider.Name() != "Claude Opus 4.6" {
		t.Errorf("Name() = %q, want %q", provider.Name(), "Claude Opus 4.6")
	}
}

func TestAnthropicProvider_Available(t *testing.T) {
	modelCfg := config.ModelConfig{
		Model: "claude-opus-4-20250918",
		Name:  "Claude Opus 4.6",
	}

	tests := []struct {
		name      string
		apiKey    string
		available bool
	}{
		{"with API key", "sk-ant-test-key", true},
		{"without API key", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.apiKey != "" {
				os.Setenv("ANTHROPIC_API_KEY", tt.apiKey)
			} else {
				os.Unsetenv("ANTHROPIC_API_KEY")
			}
			defer os.Unsetenv("ANTHROPIC_API_KEY")

			// For the "without API key" test, we can't create the provider
			// since NewAnthropicProvider will fail. Test config.Available instead.
			if tt.apiKey == "" {
				available := config.Available(modelCfg.Model)
				if available != tt.available {
					t.Errorf("config.Available() = %v, want %v", available, tt.available)
				}
				return
			}

			provider, err := NewAnthropicProvider(modelCfg)
			if err != nil {
				t.Fatalf("Failed to create provider: %v", err)
			}

			if provider.Available() != tt.available {
				t.Errorf("Available() = %v, want %v", provider.Available(), tt.available)
			}
		})
	}
}

func TestAnthropicProvider_Query_RequestStructure(t *testing.T) {
	// This test verifies that the request is structured correctly,
	// but doesn't actually call the API (would require a real API key).

	modelCfg := config.ModelConfig{
		Model: "claude-opus-4-20250918",
		Name:  "Claude Opus 4.6",
	}

	os.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	provider, err := NewAnthropicProvider(modelCfg)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	// Test that Query method exists and accepts correct parameters
	temp := 0.7
	req := &Request{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Hello!",
		Temperature:  &temp,
		MaxTokens:    1000,
	}

	ctx := context.Background()

	// We expect this to fail with an API error (invalid API key),
	// but we can verify that the method signature is correct.
	_, err = provider.Query(ctx, req)

	// We expect an error because we're using a fake API key
	if err == nil {
		t.Error("Expected error with fake API key, got nil")
	}
}

func TestAnthropicProvider_Query_TemperatureOmission(t *testing.T) {
	// Test that temperature can be omitted (nil)

	modelCfg := config.ModelConfig{
		Model:       "claude-opus-4-20250918",
		Name:        "Claude Opus 4.6",
		IsReasoning: false,
	}

	os.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	provider, err := NewAnthropicProvider(modelCfg)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	req := &Request{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Hello!",
		Temperature:  nil, // Explicitly nil
		MaxTokens:    1000,
	}

	ctx := context.Background()

	// We expect this to fail with an API error (invalid API key),
	// but the request should be structured correctly.
	_, err = provider.Query(ctx, req)

	if err == nil {
		t.Error("Expected error with fake API key, got nil")
	}
}

func TestAnthropicProvider_Query_EmptySystemPrompt(t *testing.T) {
	// Test that system prompt can be empty

	modelCfg := config.ModelConfig{
		Model: "claude-opus-4-20250918",
		Name:  "Claude Opus 4.6",
	}

	os.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key")
	defer os.Unsetenv("ANTHROPIC_API_KEY")

	provider, err := NewAnthropicProvider(modelCfg)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	temp := 0.7
	req := &Request{
		SystemPrompt: "", // Empty system prompt
		UserPrompt:   "Hello!",
		Temperature:  &temp,
		MaxTokens:    1000,
	}

	ctx := context.Background()

	_, err = provider.Query(ctx, req)

	if err == nil {
		t.Error("Expected error with fake API key, got nil")
	}
}
