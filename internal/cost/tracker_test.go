package cost

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewTracker(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "costs.jsonl")

	tracker, err := NewTracker(path)
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	if tracker.path != path {
		t.Errorf("expected path %s, got %s", path, tracker.path)
	}

	// Verify pricing data was loaded
	if len(tracker.prices) == 0 {
		t.Error("pricing data not loaded")
	}

	// Verify specific models
	expectedModels := []string{
		"claude-opus-4-20250918",
		"gpt-4.1",
		"o3",
		"gemini-2.0-flash",
		"deepseek/deepseek-reasoner",
	}

	for _, model := range expectedModels {
		if _, ok := tracker.prices[model]; !ok {
			t.Errorf("expected pricing for %s", model)
		}
	}
}

func TestNewTrackerDefaultPath(t *testing.T) {
	// Create a tracker with default path
	tracker, err := NewTracker("")
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	home, _ := os.UserHomeDir()
	expectedPath := filepath.Join(home, ".ai-council", "costs.jsonl")

	if tracker.path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, tracker.path)
	}
}

func TestEstimateCost(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewTracker(filepath.Join(tmpDir, "costs.jsonl"))
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	tests := []struct {
		name         string
		model        string
		inputTokens  int
		outputTokens int
		expectedCost float64
	}{
		{
			name:         "claude opus",
			model:        "claude-opus-4-20250918",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			expectedCost: 15.0 + 75.0, // $90
		},
		{
			name:         "gpt-4o-mini",
			model:        "gpt-4o-mini",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			expectedCost: 0.15 + 0.60, // $0.75
		},
		{
			name:         "small usage",
			model:        "gpt-4.1",
			inputTokens:  10_000,
			outputTokens: 5_000,
			expectedCost: (10_000.0 / 1_000_000 * 2.0) + (5_000.0 / 1_000_000 * 8.0), // $0.06
		},
		{
			name:         "unknown model",
			model:        "unknown-model",
			inputTokens:  1_000_000,
			outputTokens: 1_000_000,
			expectedCost: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost := tracker.EstimateCost(tt.model, tt.inputTokens, tt.outputTokens)
			if cost != tt.expectedCost {
				t.Errorf("expected cost %.4f, got %.4f", tt.expectedCost, cost)
			}
		})
	}
}

func TestLogQueryAndRead(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewTracker(filepath.Join(tmpDir, "costs.jsonl"))
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	entry := Entry{
		Mode:          "ask",
		Tier:          "full",
		Models:        []string{"claude-opus-4-20250918", "gpt-4.1"},
		SuccessCount:  2,
		CostUSD:       0.15,
		LatencyMs:     2500,
		PromptPreview: "What is the capital of France?",
	}

	// Log entry
	if err := tracker.LogQuery(entry); err != nil {
		t.Fatalf("LogQuery failed: %v", err)
	}

	// Read entries
	entries, err := tracker.readEntries()
	if err != nil {
		t.Fatalf("readEntries failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	readEntry := entries[0]
	if readEntry.Mode != entry.Mode {
		t.Errorf("expected mode %s, got %s", entry.Mode, readEntry.Mode)
	}
	if readEntry.Tier != entry.Tier {
		t.Errorf("expected tier %s, got %s", entry.Tier, readEntry.Tier)
	}
	if readEntry.CostUSD != entry.CostUSD {
		t.Errorf("expected cost %.4f, got %.4f", entry.CostUSD, readEntry.CostUSD)
	}
	if readEntry.Timestamp == "" {
		t.Error("timestamp not set")
	}

	// Verify timestamp is valid RFC3339
	if _, err := time.Parse(time.RFC3339, readEntry.Timestamp); err != nil {
		t.Errorf("invalid timestamp format: %v", err)
	}
}

func TestLogQueryMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewTracker(filepath.Join(tmpDir, "costs.jsonl"))
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	// Log multiple entries
	for i := 0; i < 5; i++ {
		entry := Entry{
			Mode:         "ask",
			Tier:         "full",
			Models:       []string{"claude-opus-4-20250918"},
			SuccessCount: 1,
			CostUSD:      float64(i) * 0.1,
			LatencyMs:    1000,
		}
		if err := tracker.LogQuery(entry); err != nil {
			t.Fatalf("LogQuery failed: %v", err)
		}
	}

	// Read entries
	entries, err := tracker.readEntries()
	if err != nil {
		t.Fatalf("readEntries failed: %v", err)
	}

	if len(entries) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(entries))
	}

	// Verify costs
	for i, entry := range entries {
		expectedCost := float64(i) * 0.1
		if entry.CostUSD != expectedCost {
			t.Errorf("entry %d: expected cost %.2f, got %.2f", i, expectedCost, entry.CostUSD)
		}
	}
}

func TestGetSummary(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewTracker(filepath.Join(tmpDir, "costs.jsonl"))
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	now := time.Now().UTC()
	yesterday := now.AddDate(0, 0, -1)
	weekAgo := now.AddDate(0, 0, -8)
	monthAgo := now.AddDate(0, 0, -31)

	entries := []Entry{
		{Timestamp: now.Format(time.RFC3339), CostUSD: 1.0, Mode: "ask", Tier: "full"},
		{Timestamp: yesterday.Format(time.RFC3339), CostUSD: 2.0, Mode: "ask", Tier: "full"},
		{Timestamp: weekAgo.Format(time.RFC3339), CostUSD: 3.0, Mode: "ask", Tier: "full"},
		{Timestamp: monthAgo.Format(time.RFC3339), CostUSD: 4.0, Mode: "ask", Tier: "full"},
	}

	for _, entry := range entries {
		if err := tracker.LogQuery(entry); err != nil {
			t.Fatalf("LogQuery failed: %v", err)
		}
	}

	summary, err := tracker.GetSummary()
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	// Today should include entries from today
	if summary.Today != 1.0 {
		t.Errorf("expected today cost 1.0, got %.2f", summary.Today)
	}

	// Week should include today and yesterday
	if summary.Week != 3.0 {
		t.Errorf("expected week cost 3.0, got %.2f", summary.Week)
	}

	// Month should include today, yesterday, and week ago
	if summary.Month != 6.0 {
		t.Errorf("expected month cost 6.0, got %.2f", summary.Month)
	}

	// All time should include all entries
	if summary.AllTime != 10.0 {
		t.Errorf("expected all time cost 10.0, got %.2f", summary.AllTime)
	}

	// Query count
	if summary.QueryCount != 4 {
		t.Errorf("expected query count 4, got %d", summary.QueryCount)
	}

	if summary.QueriesToday != 1 {
		t.Errorf("expected queries today 1, got %d", summary.QueriesToday)
	}
}

func TestGetByTier(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewTracker(filepath.Join(tmpDir, "costs.jsonl"))
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	entries := []Entry{
		{Mode: "ask", Tier: "full", CostUSD: 1.0, Models: []string{"model1"}},
		{Mode: "ask", Tier: "full", CostUSD: 2.0, Models: []string{"model1"}},
		{Mode: "ask", Tier: "balanced", CostUSD: 3.0, Models: []string{"model2"}},
		{Mode: "ask", Tier: "fast", CostUSD: 4.0, Models: []string{"model3"}},
	}

	for _, entry := range entries {
		if err := tracker.LogQuery(entry); err != nil {
			t.Fatalf("LogQuery failed: %v", err)
		}
	}

	breakdown, err := tracker.GetByTier()
	if err != nil {
		t.Fatalf("GetByTier failed: %v", err)
	}

	if len(breakdown) != 3 {
		t.Fatalf("expected 3 tiers, got %d", len(breakdown))
	}

	tierMap := make(map[string]TierBreakdown)
	for _, b := range breakdown {
		tierMap[b.Tier] = b
	}

	// Verify full tier
	if full, ok := tierMap["full"]; ok {
		if full.CostUSD != 3.0 {
			t.Errorf("expected full tier cost 3.0, got %.2f", full.CostUSD)
		}
		if full.Queries != 2 {
			t.Errorf("expected full tier queries 2, got %d", full.Queries)
		}
	} else {
		t.Error("full tier not found")
	}

	// Verify balanced tier
	if balanced, ok := tierMap["balanced"]; ok {
		if balanced.CostUSD != 3.0 {
			t.Errorf("expected balanced tier cost 3.0, got %.2f", balanced.CostUSD)
		}
		if balanced.Queries != 1 {
			t.Errorf("expected balanced tier queries 1, got %d", balanced.Queries)
		}
	} else {
		t.Error("balanced tier not found")
	}
}

func TestGetByMode(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewTracker(filepath.Join(tmpDir, "costs.jsonl"))
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	entries := []Entry{
		{Mode: "ask", Tier: "full", CostUSD: 1.0, Models: []string{"model1"}},
		{Mode: "ask", Tier: "full", CostUSD: 2.0, Models: []string{"model1"}},
		{Mode: "debate", Tier: "full", CostUSD: 3.0, Models: []string{"model2"}},
		{Mode: "redteam", Tier: "full", CostUSD: 4.0, Models: []string{"model3"}},
	}

	for _, entry := range entries {
		if err := tracker.LogQuery(entry); err != nil {
			t.Fatalf("LogQuery failed: %v", err)
		}
	}

	breakdown, err := tracker.GetByMode()
	if err != nil {
		t.Fatalf("GetByMode failed: %v", err)
	}

	if len(breakdown) != 3 {
		t.Fatalf("expected 3 modes, got %d", len(breakdown))
	}

	modeMap := make(map[string]ModeBreakdown)
	for _, b := range breakdown {
		modeMap[b.Mode] = b
	}

	// Verify ask mode
	if ask, ok := modeMap["ask"]; ok {
		if ask.CostUSD != 3.0 {
			t.Errorf("expected ask mode cost 3.0, got %.2f", ask.CostUSD)
		}
		if ask.Queries != 2 {
			t.Errorf("expected ask mode queries 2, got %d", ask.Queries)
		}
	} else {
		t.Error("ask mode not found")
	}
}

func TestMalformedLineHandling(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "costs.jsonl")

	// Write a file with valid and malformed lines
	data := `{"ts":"2026-03-28T10:00:00Z","mode":"ask","tier":"full","models":["model1"],"succeeded":1,"cost_usd":1.0,"latency_ms":1000,"prompt":"test"}
this is not json
{"ts":"2026-03-28T11:00:00Z","mode":"ask","tier":"full","models":["model2"],"succeeded":1,"cost_usd":2.0,"latency_ms":1000,"prompt":"test2"}
{"invalid": "incomplete
{"ts":"2026-03-28T12:00:00Z","mode":"ask","tier":"full","models":["model3"],"succeeded":1,"cost_usd":3.0,"latency_ms":1000,"prompt":"test3"}
`

	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	tracker, err := NewTracker(path)
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	entries, err := tracker.readEntries()
	if err != nil {
		t.Fatalf("readEntries failed: %v", err)
	}

	// Should have 3 valid entries despite malformed lines
	if len(entries) != 3 {
		t.Errorf("expected 3 valid entries, got %d", len(entries))
	}

	// Verify the valid entries - check only non-zero costs
	validEntries := make([]Entry, 0)
	for _, entry := range entries {
		if entry.CostUSD > 0 {
			validEntries = append(validEntries, entry)
		}
	}

	if len(validEntries) != 3 {
		t.Fatalf("expected 3 entries with non-zero cost, got %d", len(validEntries))
	}

	expectedCosts := []float64{1.0, 2.0, 3.0}
	for i, entry := range validEntries {
		if entry.CostUSD != expectedCosts[i] {
			t.Errorf("entry %d: expected cost %.2f, got %.2f", i, expectedCosts[i], entry.CostUSD)
		}
	}
}

func TestEmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewTracker(filepath.Join(tmpDir, "costs.jsonl"))
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	// Summary on empty file
	summary, err := tracker.GetSummary()
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if summary.AllTime != 0 {
		t.Errorf("expected all time cost 0, got %.2f", summary.AllTime)
	}
	if summary.QueryCount != 0 {
		t.Errorf("expected query count 0, got %d", summary.QueryCount)
	}

	// Tier breakdown on empty file
	tierBreakdown, err := tracker.GetByTier()
	if err != nil {
		t.Fatalf("GetByTier failed: %v", err)
	}
	if len(tierBreakdown) != 0 {
		t.Errorf("expected 0 tier breakdown entries, got %d", len(tierBreakdown))
	}

	// Mode breakdown on empty file
	modeBreakdown, err := tracker.GetByMode()
	if err != nil {
		t.Fatalf("GetByMode failed: %v", err)
	}
	if len(modeBreakdown) != 0 {
		t.Errorf("expected 0 mode breakdown entries, got %d", len(modeBreakdown))
	}
}

func TestConcurrentWrites(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewTracker(filepath.Join(tmpDir, "costs.jsonl"))
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	// Write concurrently
	done := make(chan bool)
	numWrites := 10

	for i := 0; i < numWrites; i++ {
		go func(idx int) {
			entry := Entry{
				Mode:         "ask",
				Tier:         "full",
				Models:       []string{"model1"},
				SuccessCount: 1,
				CostUSD:      float64(idx),
				LatencyMs:    1000,
			}
			if err := tracker.LogQuery(entry); err != nil {
				t.Errorf("LogQuery failed: %v", err)
			}
			done <- true
		}(i)
	}

	// Wait for all writes
	for i := 0; i < numWrites; i++ {
		<-done
	}

	// Read entries
	entries, err := tracker.readEntries()
	if err != nil {
		t.Fatalf("readEntries failed: %v", err)
	}

	if len(entries) != numWrites {
		t.Errorf("expected %d entries, got %d", numWrites, len(entries))
	}
}

func TestCustomTimestamp(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewTracker(filepath.Join(tmpDir, "costs.jsonl"))
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	customTime := "2025-01-15T10:30:00Z"
	entry := Entry{
		Timestamp:    customTime,
		Mode:         "ask",
		Tier:         "full",
		Models:       []string{"model1"},
		SuccessCount: 1,
		CostUSD:      1.0,
		LatencyMs:    1000,
	}

	if err := tracker.LogQuery(entry); err != nil {
		t.Fatalf("LogQuery failed: %v", err)
	}

	entries, err := tracker.readEntries()
	if err != nil {
		t.Fatalf("readEntries failed: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Timestamp != customTime {
		t.Errorf("expected timestamp %s, got %s", customTime, entries[0].Timestamp)
	}
}

func TestPricingDataCompleteness(t *testing.T) {
	tmpDir := t.TempDir()
	tracker, err := NewTracker(filepath.Join(tmpDir, "costs.jsonl"))
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	// Verify all expected models have pricing
	expectedModels := map[string]struct{}{
		"claude-opus-4-20250918":         {},
		"claude-sonnet-4-20250514":       {},
		"claude-haiku-4-5-20241022":      {},
		"gpt-4.1":                        {},
		"gpt-4o-mini":                    {},
		"o3":                             {},
		"o4-mini":                        {},
		"gemini-2.5-pro-preview-05-06":   {},
		"gemini-2.0-flash":               {},
		"deepseek/deepseek-reasoner":     {},
		"xai/grok-3":                     {},
	}

	for model := range expectedModels {
		pricing, ok := tracker.prices[model]
		if !ok {
			t.Errorf("missing pricing for model %s", model)
			continue
		}

		if pricing.InputPerMTok <= 0 {
			t.Errorf("model %s has invalid input pricing: %.2f", model, pricing.InputPerMTok)
		}
		if pricing.OutputPerMTok <= 0 {
			t.Errorf("model %s has invalid output pricing: %.2f", model, pricing.OutputPerMTok)
		}
	}
}

func TestJSONLFileFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "costs.jsonl")
	tracker, err := NewTracker(path)
	if err != nil {
		t.Fatalf("NewTracker failed: %v", err)
	}

	// Log an entry
	entry := Entry{
		Mode:         "ask",
		Tier:         "full",
		Models:       []string{"model1", "model2"},
		SuccessCount: 2,
		CostUSD:      1.23,
		LatencyMs:    1500,
	}

	if err := tracker.LogQuery(entry); err != nil {
		t.Fatalf("LogQuery failed: %v", err)
	}

	// Read raw file
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	// Verify it's valid JSON on a single line
	lines := []byte{}
	for _, b := range data {
		if b != '\n' {
			lines = append(lines, b)
		}
	}

	var parsed Entry
	if err := json.Unmarshal(lines, &parsed); err != nil {
		t.Fatalf("file is not valid JSONL: %v", err)
	}

	// Verify content matches
	if parsed.Mode != entry.Mode {
		t.Errorf("expected mode %s, got %s", entry.Mode, parsed.Mode)
	}
	if parsed.CostUSD != entry.CostUSD {
		t.Errorf("expected cost %.2f, got %.2f", entry.CostUSD, parsed.CostUSD)
	}
}
