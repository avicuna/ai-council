package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/avicuna/ai-council-personal/internal/config"
)

// mockProvider is a mock implementation of Provider for testing.
type mockProvider struct {
	name      string
	available bool
	delay     time.Duration
	err       error
	response  *Response
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Available() bool {
	return m.available
}

func (m *mockProvider) Query(ctx context.Context, req *Request) (*Response, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if m.err != nil {
		return nil, m.err
	}

	return m.response, nil
}

func TestDetermineTimeout(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected time.Duration
	}{
		{"fast model - haiku", "Claude Haiku 4.5", 30 * time.Second},
		{"fast model - mini", "GPT-4.1 Mini", 30 * time.Second},
		{"fast model - flash", "Gemini Flash", 30 * time.Second},
		{"reasoning model - o3", "o3", 180 * time.Second},
		{"reasoning model - o4", "o4-mini", 180 * time.Second},
		{"reasoning model - deepseek", "DeepSeek Reasoner", 180 * time.Second},
		{"standard model - sonnet", "Claude Sonnet 4", 90 * time.Second},
		{"standard model - gpt", "GPT-4.1", 90 * time.Second},
		{"standard model - gemini", "Gemini 2.5 Pro", 90 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			timeout := determineTimeout(tt.model)
			if timeout != tt.expected {
				t.Errorf("determineTimeout(%q) = %v, want %v", tt.model, timeout, tt.expected)
			}
		})
	}
}

func TestQueryAll_Success(t *testing.T) {
	providers := []Provider{
		&mockProvider{
			name:      "Provider1",
			available: true,
			response: &Response{
				Content:      "Response 1",
				Model:        "model1",
				Name:         "Provider1",
				InputTokens:  10,
				OutputTokens: 20,
				LatencyMs:    100,
			},
		},
		&mockProvider{
			name:      "Provider2",
			available: true,
			response: &Response{
				Content:      "Response 2",
				Model:        "model2",
				Name:         "Provider2",
				InputTokens:  15,
				OutputTokens: 25,
				LatencyMs:    150,
			},
		},
		&mockProvider{
			name:      "Provider3",
			available: true,
			response: &Response{
				Content:      "Response 3",
				Model:        "model3",
				Name:         "Provider3",
				InputTokens:  20,
				OutputTokens: 30,
				LatencyMs:    200,
			},
		},
	}

	req := &Request{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Hello!",
		MaxTokens:    1000,
	}

	progressCh := make(chan ProgressEvent, 10)
	ctx := context.Background()

	// Run QueryAll in a goroutine
	done := make(chan QueryAllResult)
	go func() {
		result := QueryAll(ctx, providers, req, progressCh)
		close(progressCh)
		done <- result
	}()

	// Collect progress events
	var events []ProgressEvent
	for event := range progressCh {
		events = append(events, event)
	}

	// Get result
	result := <-done

	// Verify results
	if len(result.Responses) != 3 {
		t.Errorf("Expected 3 responses, got %d", len(result.Responses))
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got %d: %v", len(result.Errors), result.Errors)
	}

	// Verify progress events (should have 6 events: 3 "querying" + 3 "done")
	if len(events) != 6 {
		t.Errorf("Expected 6 progress events, got %d", len(events))
	}

	// Count event types
	queryingCount := 0
	doneCount := 0
	for _, event := range events {
		switch event.Status {
		case "querying":
			queryingCount++
		case "done":
			doneCount++
		}
	}

	if queryingCount != 3 {
		t.Errorf("Expected 3 'querying' events, got %d", queryingCount)
	}

	if doneCount != 3 {
		t.Errorf("Expected 3 'done' events, got %d", doneCount)
	}
}

func TestQueryAll_PartialFailure(t *testing.T) {
	providers := []Provider{
		&mockProvider{
			name:      "Provider1",
			available: true,
			response: &Response{
				Content: "Success 1",
				Name:    "Provider1",
			},
		},
		&mockProvider{
			name:      "Provider2",
			available: true,
			err:       errors.New("provider2 failed"),
		},
		&mockProvider{
			name:      "Provider3",
			available: true,
			response: &Response{
				Content: "Success 3",
				Name:    "Provider3",
			},
		},
	}

	req := &Request{
		UserPrompt: "Hello!",
		MaxTokens:  1000,
	}

	ctx := context.Background()
	result := QueryAll(ctx, providers, req, nil)

	// Verify that 2 succeeded and 1 failed
	if len(result.Responses) != 2 {
		t.Errorf("Expected 2 responses, got %d", len(result.Responses))
	}

	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}

	if _, ok := result.Errors["Provider2"]; !ok {
		t.Error("Expected error for Provider2")
	}
}

func TestQueryAll_Timeout(t *testing.T) {
	// Create a provider with a long delay
	providers := []Provider{
		&mockProvider{
			name:      "SlowProvider",
			available: true,
			delay:     200 * time.Millisecond, // Longer than our test timeout
			response: &Response{
				Content: "This should timeout",
				Name:    "SlowProvider",
			},
		},
	}

	req := &Request{
		UserPrompt: "Hello!",
		MaxTokens:  1000,
	}

	// Use a short context timeout to test timeout behavior
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	result := QueryAll(ctx, providers, req, nil)

	// Verify that it timed out
	if len(result.Errors) != 1 {
		t.Errorf("Expected 1 error, got %d", len(result.Errors))
	}

	if err, ok := result.Errors["SlowProvider"]; !ok {
		t.Error("Expected error for SlowProvider")
	} else if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

func TestQueryAll_NilProgressChannel(t *testing.T) {
	providers := []Provider{
		&mockProvider{
			name:      "Provider1",
			available: true,
			response: &Response{
				Content: "Success",
				Name:    "Provider1",
			},
		},
	}

	req := &Request{
		UserPrompt: "Hello!",
		MaxTokens:  1000,
	}

	ctx := context.Background()
	// Test that nil progress channel doesn't cause panic
	result := QueryAll(ctx, providers, req, nil)

	if len(result.Responses) != 1 {
		t.Errorf("Expected 1 response, got %d", len(result.Responses))
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got %d", len(result.Errors))
	}
}

func TestNewProvider(t *testing.T) {
	tests := []struct {
		name        string
		modelCfg    config.ModelConfig
		expectError bool
		expectType  string
	}{
		{
			name: "claude model",
			modelCfg: config.ModelConfig{
				Model: "claude-opus-4-20250918",
				Name:  "Claude Opus 4.6",
			},
			expectError: false,
			expectType:  "*provider.AnthropicProvider",
		},
		{
			name: "anthropic prefix",
			modelCfg: config.ModelConfig{
				Model: "anthropic/claude-sonnet",
				Name:  "Claude Sonnet",
			},
			expectError: false,
			expectType:  "*provider.AnthropicProvider",
		},
		{
			name: "unsupported model",
			modelCfg: config.ModelConfig{
				Model: "gpt-4.1",
				Name:  "GPT-4.1",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := NewProvider(tt.modelCfg)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				// Only fail if we expected success and got an error about API key
				// (We can't test with actual API keys in unit tests)
				if !strings.Contains(err.Error(), "API_KEY") {
					t.Errorf("Unexpected error: %v", err)
				}
				return
			}

			if provider == nil {
				t.Error("Expected provider, got nil")
			}
		})
	}
}

func TestNewProvider_DefaultGPTModelsSendMaxCompletionTokens(t *testing.T) {
	// The GPT-6 models reject max_tokens with a 400; they need max_completion_tokens.
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()
	t.Setenv("OPENAI_BASE_URL", srv.URL)
	t.Setenv("OPENAI_API_KEY", "test-key")

	for _, tier := range config.ValidTiers() {
		for _, cfg := range config.GetAllProposers(tier) {
			if !strings.HasPrefix(cfg.Model, "gpt-") {
				continue
			}
			p, err := NewProvider(cfg)
			if err != nil {
				t.Fatalf("NewProvider(%s): %v", cfg.Model, err)
			}
			bodies = nil
			if _, err := p.Query(context.Background(), &Request{UserPrompt: "hi", MaxTokens: 100}); err != nil {
				t.Fatalf("Query(%s): %v", cfg.Model, err)
			}
			if len(bodies) != 1 {
				t.Fatalf("%s: expected 1 request, got %d", cfg.Model, len(bodies))
			}
			if _, ok := bodies[0]["max_tokens"]; ok {
				t.Errorf("%s (%s tier): request sent max_tokens", cfg.Model, tier)
			}
			if bodies[0]["max_completion_tokens"] != float64(100) {
				t.Errorf("%s (%s tier): max_completion_tokens = %v, want 100", cfg.Model, tier, bodies[0]["max_completion_tokens"])
			}
		}
	}
}
