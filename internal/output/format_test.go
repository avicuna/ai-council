package output

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/avicuna/ai-council-personal/internal/config"
	"github.com/avicuna/ai-council-personal/internal/cost"
	"github.com/avicuna/ai-council-personal/internal/provider"
	"github.com/avicuna/ai-council-personal/internal/strategy"
)

// Helper to create test response
func testResponse(name, model, content string, latencyMs int64, inputTokens, outputTokens int) *provider.Response {
	return &provider.Response{
		Content:      content,
		Model:        model,
		Name:         name,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		LatencyMs:    latencyMs,
	}
}

// Helper to create test result
func testResult(proposals []*provider.Response, synthesis *provider.Response, totalMs int64, agreementScore *int, agreementReason string) *strategy.Result {
	return &strategy.Result{
		Proposals:       proposals,
		Synthesis:       synthesis,
		Rounds:          1,
		AgreementScore:  agreementScore,
		AgreementReason: agreementReason,
		TotalMs:         totalMs,
	}
}

func TestRenderMoAResult(t *testing.T) {
	proposals := []*provider.Response{
		testResponse("Claude Sonnet 4", "claude-sonnet-4", "Response 1", 3200, 100, 200),
		testResponse("GPT-4.1", "gpt-4.1", "Response 2", 4100, 120, 180),
		testResponse("Gemini 2.5 Pro", "gemini-2.5-pro", "Response 3", 5700, 110, 220),
	}
	synthesis := testResponse("Claude Sonnet 4", "claude-sonnet-4", "Synthesized response", 2000, 500, 300)
	score := 87
	result := testResult(proposals, synthesis, 10500, &score, "All models converged on key recommendations")

	output := RenderMoAResult(result, false, "balanced")

	// Check header
	if !strings.Contains(output, "Mode: moa") {
		t.Error("Output should contain mode")
	}
	if !strings.Contains(output, "Tier: balanced") {
		t.Error("Output should contain tier")
	}
	if !strings.Contains(output, "Aggregator: Claude Sonnet 4") {
		t.Error("Output should contain aggregator")
	}

	// Check synthesis
	if !strings.Contains(output, "Council Synthesis") {
		t.Error("Output should contain synthesis header")
	}
	if !strings.Contains(output, "Synthesized response") {
		t.Error("Output should contain synthesis content")
	}

	// Check footer
	if !strings.Contains(output, "Models: 3") {
		t.Error("Output should contain model count")
	}
	if !strings.Contains(output, "Mode: MoA") {
		t.Error("Output should contain mode in footer")
	}
	if !strings.Contains(output, "87%") {
		t.Error("Output should contain agreement score")
	}
	if !strings.Contains(output, "All models converged") {
		t.Error("Output should contain agreement reason")
	}

	// Verbose mode should show individual models
	verboseOutput := RenderMoAResult(result, true, "balanced")
	if !strings.Contains(verboseOutput, "Claude Sonnet 4") {
		t.Error("Verbose output should contain model names")
	}
	if !strings.Contains(verboseOutput, "GPT-4.1") {
		t.Error("Verbose output should contain all model names")
	}
}

func TestRenderDebateResult(t *testing.T) {
	proposals := []*provider.Response{
		testResponse("Model A", "model-a", "First argument", 2000, 100, 150),
		testResponse("Model B", "model-b", "Counter argument", 2200, 110, 160),
	}
	synthesis := testResponse("Aggregator", "aggregator", "Final verdict", 1500, 200, 180)
	score := 65
	result := testResult(proposals, synthesis, 5700, &score, "Models reached partial consensus")
	result.Rounds = 2

	output := RenderDebateResult(result, false, "full")

	// Check header
	if !strings.Contains(output, "Mode: debate") {
		t.Error("Output should contain mode")
	}
	if !strings.Contains(output, "Rounds: 2") {
		t.Error("Output should contain rounds")
	}

	// Check verdict
	if !strings.Contains(output, "Council Verdict") {
		t.Error("Output should contain verdict header")
	}
	if !strings.Contains(output, "Final verdict") {
		t.Error("Output should contain verdict content")
	}

	// Verbose mode should show round-by-round
	verboseOutput := RenderDebateResult(result, true, "full")
	if !strings.Contains(verboseOutput, "Model A") {
		t.Error("Verbose output should contain model names")
	}
	if !strings.Contains(verboseOutput, "First argument") {
		t.Error("Verbose output should contain proposal content")
	}
}

func TestRenderRedTeamResult(t *testing.T) {
	proposals := []*provider.Response{
		testResponse("Proposer", "model-p", "Initial proposal", 2000, 100, 150),
		testResponse("Attacker", "model-a", "Attack vector", 2200, 110, 160),
	}
	synthesis := testResponse("Defender", "model-d", "Hardened answer", 1800, 220, 200)
	result := testResult(proposals, synthesis, 6000, nil, "")

	output := RenderRedTeamResult(result, false, "balanced")

	// Check header
	if !strings.Contains(output, "Mode: red-team") {
		t.Error("Output should contain mode")
	}

	// Check hardened answer
	if !strings.Contains(output, "Hardened Answer") {
		t.Error("Output should contain hardened answer header")
	}
	if !strings.Contains(output, "Hardened answer") {
		t.Error("Output should contain hardened answer content")
	}

	// Verbose mode should show proposals/attacks
	verboseOutput := RenderRedTeamResult(result, true, "balanced")
	if !strings.Contains(verboseOutput, "Proposals & Attacks") {
		t.Error("Verbose output should contain section header")
	}
	if !strings.Contains(verboseOutput, "Proposer") {
		t.Error("Verbose output should contain model names")
	}
}

func TestRenderModels(t *testing.T) {
	proposers := []config.ModelConfig{
		{Model: "claude-sonnet-4", Name: "Claude Sonnet 4", IsReasoning: false},
		{Model: "gpt-4.1", Name: "GPT-4.1", IsReasoning: false},
		{Model: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", IsReasoning: false},
	}
	aggregator := config.ModelConfig{Model: "claude-sonnet-4", Name: "Claude Sonnet 4", IsReasoning: false}

	output := RenderModels("balanced", proposers, aggregator)

	// Check header
	if !strings.Contains(output, "Models for tier: balanced") {
		t.Error("Output should contain tier")
	}

	// Check table structure
	if !strings.Contains(output, "Role") {
		t.Error("Output should contain table header")
	}
	if !strings.Contains(output, "Proposer") {
		t.Error("Output should contain proposer role")
	}
	if !strings.Contains(output, "Aggregator") {
		t.Error("Output should contain aggregator role")
	}

	// Check model names
	if !strings.Contains(output, "Claude Sonnet 4") {
		t.Error("Output should contain model names")
	}
	if !strings.Contains(output, "GPT-4.1") {
		t.Error("Output should contain all model names")
	}
}

func TestRenderCosts(t *testing.T) {
	summary := &cost.Summary{
		Today:        1.25,
		Week:         8.50,
		Month:        32.75,
		AllTime:      156.40,
		QueryCount:   523,
		QueriesToday: 15,
	}
	byTier := []cost.TierBreakdown{
		{Tier: "fast", CostUSD: 5.20, Queries: 50},
		{Tier: "balanced", CostUSD: 15.80, Queries: 100},
		{Tier: "full", CostUSD: 135.40, Queries: 373},
	}
	byMode := []cost.ModeBreakdown{
		{Mode: "moa", CostUSD: 100.50, Queries: 350},
		{Mode: "debate", CostUSD: 35.90, Queries: 120},
		{Mode: "red-team", CostUSD: 20.00, Queries: 53},
	}

	output := RenderCosts(summary, byTier, byMode)

	// Check summary
	if !strings.Contains(output, "Cost Summary") {
		t.Error("Output should contain summary header")
	}
	if !strings.Contains(output, "$1.25") {
		t.Error("Output should contain today's cost")
	}
	if !strings.Contains(output, "$156.40") {
		t.Error("Output should contain all-time cost")
	}
	if !strings.Contains(output, "523") {
		t.Error("Output should contain query count")
	}

	// Check by-tier
	if !strings.Contains(output, "Cost by Tier") {
		t.Error("Output should contain tier breakdown")
	}
	if !strings.Contains(output, "fast") {
		t.Error("Output should contain tier names")
	}

	// Check by-mode
	if !strings.Contains(output, "Cost by Mode") {
		t.Error("Output should contain mode breakdown")
	}
	if !strings.Contains(output, "moa") {
		t.Error("Output should contain mode names")
	}
}

func TestAgreementScoreColoring(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{90, "90%"},  // green
		{70, "70%"},  // green
		{65, "65%"},  // yellow
		{40, "40%"},  // yellow
		{35, "35%"},  // red
		{10, "10%"},  // red
	}

	for _, tt := range tests {
		result := colorizeAgreementScore(tt.score)
		// Should contain the percentage value
		if !strings.Contains(result, tt.expected) {
			t.Errorf("Score %d should contain %q, got %q", tt.score, tt.expected, result)
		}
	}
}

func TestNOCOLOR(t *testing.T) {
	// Set NO_COLOR environment variable
	os.Setenv("NO_COLOR", "1")
	defer os.Unsetenv("NO_COLOR")

	// Re-initialize to pick up NO_COLOR
	if os.Getenv("NO_COLOR") != "" {
		// Disable styling for this test
	}

	proposals := []*provider.Response{
		testResponse("Claude Sonnet 4", "claude-sonnet-4", "Response 1", 3200, 100, 200),
	}
	synthesis := testResponse("Claude Sonnet 4", "claude-sonnet-4", "Synthesized response", 2000, 500, 300)
	score := 87
	result := testResult(proposals, synthesis, 5200, &score, "High agreement")

	output := RenderMoAResult(result, false, "balanced")

	// Output should not contain ANSI escape codes
	// ANSI codes start with \x1b[ or \033[
	if strings.Contains(output, "\x1b[") || strings.Contains(output, "\033[") {
		t.Error("Output should not contain ANSI codes when NO_COLOR is set")
	}

	// Should still contain the actual content
	if !strings.Contains(output, "Mode: moa") {
		t.Error("Output should still contain readable content")
	}
	if !strings.Contains(output, "87%") {
		t.Error("Output should still contain agreement score")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{500 * time.Millisecond, "500ms"},
		{999 * time.Millisecond, "999ms"},
		{1 * time.Second, "1.0s"},
		{1500 * time.Millisecond, "1.5s"},
		{10500 * time.Millisecond, "10.5s"},
		{65 * time.Second, "65.0s"},
	}

	for _, tt := range tests {
		result := formatDuration(tt.duration)
		if result != tt.expected {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.expected)
		}
	}
}

func TestFormatCostValue(t *testing.T) {
	tests := []struct {
		cost     float64
		expected string
	}{
		{0.00, "$0.0000"},
		{0.0001, "$0.0001"},
		{0.0050, "$0.0050"},
		{0.0099, "$0.0099"},
		{0.01, "$0.01"},
		{0.10, "$0.10"},
		{1.00, "$1.00"},
		{1.234, "$1.23"},
		{10.567, "$10.57"},
		{100.999, "$101.00"},
	}

	for _, tt := range tests {
		result := formatCostValue(tt.cost)
		if result != tt.expected {
			t.Errorf("formatCostValue(%.4f) = %q, want %q", tt.cost, result, tt.expected)
		}
	}
}

func TestProgressTracker(t *testing.T) {
	// Disable pterm for this test to avoid race conditions in pterm's internal spinner
	os.Setenv("PTERM_DISABLE", "1")
	defer os.Unsetenv("PTERM_DISABLE")

	// Create a test event channel
	events := make(chan provider.ProgressEvent, 10)

	// Create tracker
	tracker := NewProgressTracker(events)
	tracker.Start()

	// Send some events
	events <- provider.ProgressEvent{
		Model:  "Model A",
		Status: "querying",
	}
	events <- provider.ProgressEvent{
		Model:   "Model A",
		Status:  "done",
		Latency: 2 * time.Second,
	}
	events <- provider.ProgressEvent{
		Model:   "Model B",
		Status:  "querying",
	}
	events <- provider.ProgressEvent{
		Model:   "Model B",
		Status:  "failed",
		Latency: 1 * time.Second,
		Error:   nil,
	}

	// Close channel and wait
	close(events)
	tracker.Wait()
	tracker.Stop()

	// Test passes if no panic occurs
}

func TestEstimateCost(t *testing.T) {
	resp := testResponse("Model", "model", "Content", 1000, 1000, 2000)
	cost := estimateCost(resp)

	// Should be roughly (1000 * 3 + 2000 * 15) / 1M = 0.033
	if cost < 0.03 || cost > 0.04 {
		t.Errorf("estimateCost() = %.4f, want ~0.033", cost)
	}
}

func TestIsColorDisabled(t *testing.T) {
	// Save original value
	original := os.Getenv("NO_COLOR")
	defer func() {
		if original != "" {
			os.Setenv("NO_COLOR", original)
		} else {
			os.Unsetenv("NO_COLOR")
		}
	}()

	// Test with NO_COLOR set
	os.Setenv("NO_COLOR", "1")
	if !IsColorDisabled() {
		t.Error("IsColorDisabled() should return true when NO_COLOR is set")
	}

	// Test with NO_COLOR unset
	os.Unsetenv("NO_COLOR")
	// Result depends on terminal detection, so we just check it doesn't panic
	_ = IsColorDisabled()
}
