package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/avicuna/ai-council-personal/internal/config"
)

// Provider is the interface that all LLM providers must implement.
type Provider interface {
	Name() string
	Query(ctx context.Context, req *Request) (*Response, error)
	Available() bool
}

// Request represents a query to an LLM provider.
type Request struct {
	SystemPrompt string
	UserPrompt   string
	Temperature  *float64 // nil = omit (reasoning models)
	MaxTokens    int
}

// Response represents a response from an LLM provider.
type Response struct {
	Content      string
	Model        string
	Name         string
	InputTokens  int
	OutputTokens int
	LatencyMs    int64
}

// ProgressEvent represents progress updates during QueryAll.
type ProgressEvent struct {
	Model   string
	Status  string // "querying", "done", "failed"
	Latency time.Duration
	Error   error
}

// QueryAllResult contains results and errors from QueryAll.
type QueryAllResult struct {
	Responses []Response
	Errors    map[string]error // model name -> error
}

// QueryAll queries multiple providers in parallel and collects results.
// It sends progress events on the provided channel as models complete.
// If progressCh is nil, no progress events are sent.
func QueryAll(ctx context.Context, providers []Provider, req *Request, progressCh chan<- ProgressEvent) QueryAllResult {
	result := QueryAllResult{
		Responses: make([]Response, 0, len(providers)),
		Errors:    make(map[string]error),
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, provider := range providers {
		wg.Add(1)
		go func(p Provider) {
			defer wg.Done()

			name := p.Name()
			start := time.Now()

			// Send "querying" event
			if progressCh != nil {
				progressCh <- ProgressEvent{
					Model:  name,
					Status: "querying",
				}
			}

			// Determine timeout based on model name
			timeout := determineTimeout(name)
			queryCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Execute query
			resp, err := p.Query(queryCtx, req)
			latency := time.Since(start)

			mu.Lock()
			if err != nil {
				result.Errors[name] = err
				if progressCh != nil {
					progressCh <- ProgressEvent{
						Model:   name,
						Status:  "failed",
						Latency: latency,
						Error:   err,
					}
				}
			} else {
				result.Responses = append(result.Responses, *resp)
				if progressCh != nil {
					progressCh <- ProgressEvent{
						Model:   name,
						Status:  "done",
						Latency: latency,
					}
				}
			}
			mu.Unlock()
		}(provider)
	}

	wg.Wait()
	return result
}

// determineTimeout returns the timeout duration based on model characteristics.
func determineTimeout(modelName string) time.Duration {
	nameLower := strings.ToLower(modelName)

	// Check for reasoning models first (o3, o4, deepseek-reasoner)
	// Must be before fast model check since o4-mini contains "mini"
	if strings.Contains(nameLower, "o3") ||
		strings.Contains(nameLower, "o4") ||
		strings.Contains(nameLower, "reasoner") {
		return 180 * time.Second
	}

	// Check for fast models (haiku, flash, mini suffix)
	// Use word boundaries for "mini" to avoid matching "gemini"
	if strings.Contains(nameLower, "haiku") ||
		strings.Contains(nameLower, "flash") ||
		strings.HasSuffix(nameLower, "mini") ||
		strings.HasSuffix(nameLower, "-mini") {
		return 30 * time.Second
	}

	// Standard models
	return 90 * time.Second
}

// NewProvider creates a provider based on the model configuration.
func NewProvider(modelCfg config.ModelConfig) (Provider, error) {
	modelLower := strings.ToLower(modelCfg.Model)

	// Determine provider type
	if strings.Contains(modelLower, "claude") || strings.Contains(modelLower, "anthropic") {
		return NewAnthropicProvider(modelCfg)
	}

	// Other providers will be added in future tasks
	return nil, fmt.Errorf("unsupported model: %s", modelCfg.Model)
}
