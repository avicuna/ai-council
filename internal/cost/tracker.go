package cost

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed pricing.json
var pricingJSON []byte

// ModelPricing represents the pricing for a model.
type ModelPricing struct {
	InputPerMTok  float64 `json:"input_per_mtok"`
	OutputPerMTok float64 `json:"output_per_mtok"`
}

// Entry represents a single cost tracking entry.
type Entry struct {
	Timestamp     string   `json:"ts"`
	Mode          string   `json:"mode"`
	Tier          string   `json:"tier"`
	Models        []string `json:"models"`
	SuccessCount  int      `json:"succeeded"`
	CostUSD       float64  `json:"cost_usd"`
	LatencyMs     int64    `json:"latency_ms"`
	PromptPreview string   `json:"prompt"`
}

// Summary represents cost summary statistics.
type Summary struct {
	Today        float64 `json:"today"`
	Week         float64 `json:"week"`
	Month        float64 `json:"month"`
	AllTime      float64 `json:"all_time"`
	QueryCount   int     `json:"query_count"`
	QueriesToday int     `json:"queries_today"`
}

// TierBreakdown represents cost breakdown by tier.
type TierBreakdown struct {
	Tier     string  `json:"tier"`
	CostUSD  float64 `json:"cost_usd"`
	Queries  int     `json:"queries"`
}

// ModeBreakdown represents cost breakdown by mode.
type ModeBreakdown struct {
	Mode     string  `json:"mode"`
	CostUSD  float64 `json:"cost_usd"`
	Queries  int     `json:"queries"`
}

// Tracker manages cost logging and retrieval.
type Tracker struct {
	path   string
	mu     sync.Mutex
	prices map[string]ModelPricing
}

// NewTracker creates a new cost tracker.
// If path is empty, defaults to ~/.ai-council/costs.jsonl
func NewTracker(path string) (*Tracker, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("get home dir: %w", err)
		}
		path = filepath.Join(home, ".ai-council", "costs.jsonl")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// Load pricing data
	prices := make(map[string]ModelPricing)
	if err := json.Unmarshal(pricingJSON, &prices); err != nil {
		return nil, fmt.Errorf("parse pricing: %w", err)
	}

	return &Tracker{
		path:   path,
		prices: prices,
	}, nil
}

// EstimateCost estimates the cost for a model based on token usage.
// Returns 0 if the model is not found in the pricing table.
func (t *Tracker) EstimateCost(model string, inputTokens, outputTokens int) float64 {
	pricing, ok := t.prices[model]
	if !ok {
		return 0
	}

	inputCost := (float64(inputTokens) / 1_000_000) * pricing.InputPerMTok
	outputCost := (float64(outputTokens) / 1_000_000) * pricing.OutputPerMTok

	return inputCost + outputCost
}

// LogQuery appends a cost entry to the log file.
func (t *Tracker) LogQuery(entry Entry) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Set timestamp if not provided
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	// Marshal entry
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	// Append to file with O_APPEND for cross-process safety
	f, err := os.OpenFile(t.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write entry: %w", err)
	}

	return nil
}

// readEntries reads all entries from the log file.
func (t *Tracker) readEntries() ([]Entry, error) {
	data, err := os.ReadFile(t.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Entry{}, nil
		}
		return nil, fmt.Errorf("read file: %w", err)
	}

	var entries []Entry
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}

		var entry Entry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Log malformed line but continue processing
			fmt.Fprintf(os.Stderr, "Warning: malformed entry at line %d: %v\n", i+1, err)
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// GetSummary returns cost summary statistics.
func (t *Tracker) GetSummary() (*Summary, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entries, err := t.readEntries()
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	startOfWeek := now.AddDate(0, 0, -7)
	startOfMonth := now.AddDate(0, 0, -30)

	summary := &Summary{}

	for _, entry := range entries {
		ts, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			continue
		}

		summary.AllTime += entry.CostUSD
		summary.QueryCount++

		if ts.After(startOfDay) || ts.Equal(startOfDay) {
			summary.Today += entry.CostUSD
			summary.QueriesToday++
		}

		if ts.After(startOfWeek) {
			summary.Week += entry.CostUSD
		}

		if ts.After(startOfMonth) {
			summary.Month += entry.CostUSD
		}
	}

	return summary, nil
}

// GetByTier returns cost breakdown by tier.
func (t *Tracker) GetByTier() ([]TierBreakdown, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entries, err := t.readEntries()
	if err != nil {
		return nil, err
	}

	tierMap := make(map[string]*TierBreakdown)
	for _, entry := range entries {
		tier := entry.Tier
		if tier == "" {
			tier = "unknown"
		}

		if _, ok := tierMap[tier]; !ok {
			tierMap[tier] = &TierBreakdown{Tier: tier}
		}

		tierMap[tier].CostUSD += entry.CostUSD
		tierMap[tier].Queries++
	}

	result := make([]TierBreakdown, 0, len(tierMap))
	for _, breakdown := range tierMap {
		result = append(result, *breakdown)
	}

	return result, nil
}

// GetByMode returns cost breakdown by mode.
func (t *Tracker) GetByMode() ([]ModeBreakdown, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	entries, err := t.readEntries()
	if err != nil {
		return nil, err
	}

	modeMap := make(map[string]*ModeBreakdown)
	for _, entry := range entries {
		mode := entry.Mode
		if mode == "" {
			mode = "unknown"
		}

		if _, ok := modeMap[mode]; !ok {
			modeMap[mode] = &ModeBreakdown{Mode: mode}
		}

		modeMap[mode].CostUSD += entry.CostUSD
		modeMap[mode].Queries++
	}

	result := make([]ModeBreakdown, 0, len(modeMap))
	for _, breakdown := range modeMap {
		result = append(result, *breakdown)
	}

	return result, nil
}
