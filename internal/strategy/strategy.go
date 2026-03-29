package strategy

import (
	"context"

	"github.com/avicuna/ai-council-personal/internal/provider"
)

// Strategy represents a coordination strategy for multiple AI providers.
type Strategy interface {
	Execute(ctx context.Context, opts *Options) (*Result, error)
}

// Options contains all parameters for executing a strategy.
type Options struct {
	Proposers  []provider.Provider
	Aggregator provider.Provider
	Scorer     provider.Provider // nil if scoring disabled (non-full tier)
	Request    *provider.Request
	Progress   chan<- provider.ProgressEvent
	Rounds     int  // only used by debate
	IsFastTier bool // true for fast tier (skips aggregation)
}

// Result contains the output of a strategy execution.
type Result struct {
	Proposals       []*provider.Response
	Synthesis       *provider.Response
	Rounds          int
	AgreementScore  *int
	AgreementReason string
	TotalMs         int64
}
