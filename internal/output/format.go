package output

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/avicuna/ai-council-personal/internal/config"
	"github.com/avicuna/ai-council-personal/internal/provider"
	"github.com/avicuna/ai-council-personal/internal/strategy"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/pterm/pterm"
)

// Initialize NO_COLOR support at package init
func init() {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		pterm.DisableStyling()
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}

// Cost tracking types (placeholder until cost package is implemented)
type Summary struct {
	Today        float64
	Week         float64
	Month        float64
	AllTime      float64
	QueryCount   int
	QueriesToday int
}

// Color styles
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10"))

	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("11"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	synthesisHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("14"))

	modelInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
)

// RenderMoAResult formats a Mixture of Agents result.
func RenderMoAResult(result *strategy.Result, verbose bool, tier string) string {
	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render("Mode: moa"))
	b.WriteString(" | ")
	b.WriteString(fmt.Sprintf("Tier: %s", tier))
	if result.Synthesis != nil {
		b.WriteString(" | ")
		b.WriteString(fmt.Sprintf("Aggregator: %s", result.Synthesis.Name))
	}
	b.WriteString("\n\n")

	// Per-model responses (if verbose)
	if verbose && len(result.Proposals) > 0 {
		for _, resp := range result.Proposals {
			b.WriteString(successStyle.Render("  ✓ "))
			b.WriteString(resp.Name)
			b.WriteString(modelInfoStyle.Render(fmt.Sprintf(" (%s, %s)",
				formatDuration(time.Duration(resp.LatencyMs)*time.Millisecond),
				formatCost(resp))))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Synthesis
	if result.Synthesis != nil {
		b.WriteString(synthesisHeaderStyle.Render("━━━ Council Synthesis ━━━"))
		b.WriteString("\n")
		b.WriteString(result.Synthesis.Content)
		b.WriteString("\n\n")
	}

	// Footer
	b.WriteString(renderFooter(result, "MoA", tier))

	return b.String()
}

// RenderDebateResult formats a debate result.
func RenderDebateResult(result *strategy.Result, verbose bool, tier string) string {
	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render("Mode: debate"))
	b.WriteString(" | ")
	b.WriteString(fmt.Sprintf("Tier: %s | Rounds: %d", tier, result.Rounds))
	b.WriteString("\n\n")

	// Round-by-round (if verbose)
	if verbose && len(result.Proposals) > 0 {
		for i, resp := range result.Proposals {
			round := (i / 2) + 1 // Assuming 2 models per round
			b.WriteString(dimStyle.Render(fmt.Sprintf("Round %d: ", round)))
			b.WriteString(resp.Name)
			b.WriteString("\n")
			b.WriteString(resp.Content)
			b.WriteString("\n\n")
		}
	}

	// Verdict (synthesis)
	if result.Synthesis != nil {
		b.WriteString(synthesisHeaderStyle.Render("━━━ Council Verdict ━━━"))
		b.WriteString("\n")
		b.WriteString(result.Synthesis.Content)
		b.WriteString("\n\n")
	}

	// Footer
	b.WriteString(renderFooter(result, "Debate", tier))

	return b.String()
}

// RenderRedTeamResult formats a red team result.
func RenderRedTeamResult(result *strategy.Result, verbose bool, tier string) string {
	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render("Mode: red-team"))
	b.WriteString(" | ")
	b.WriteString(fmt.Sprintf("Tier: %s", tier))
	b.WriteString("\n\n")

	// Proposals/attacks/defenses (if verbose)
	if verbose && len(result.Proposals) > 0 {
		b.WriteString(dimStyle.Render("Proposals & Attacks:"))
		b.WriteString("\n")
		for _, resp := range result.Proposals {
			b.WriteString(fmt.Sprintf("  %s:\n", resp.Name))
			b.WriteString(resp.Content)
			b.WriteString("\n\n")
		}
	}

	// Hardened answer (synthesis)
	if result.Synthesis != nil {
		b.WriteString(synthesisHeaderStyle.Render("━━━ Hardened Answer ━━━"))
		b.WriteString("\n")
		b.WriteString(result.Synthesis.Content)
		b.WriteString("\n\n")
	}

	// Footer
	b.WriteString(renderFooter(result, "Red Team", tier))

	return b.String()
}

// renderFooter creates the common footer for all modes.
func renderFooter(result *strategy.Result, mode, tier string) string {
	var parts []string

	// Models count
	parts = append(parts, fmt.Sprintf("Models: %d", len(result.Proposals)))

	// Mode
	parts = append(parts, fmt.Sprintf("Mode: %s", mode))

	// Time
	parts = append(parts, fmt.Sprintf("Time: %s", formatDuration(time.Duration(result.TotalMs)*time.Millisecond)))

	// Cost (placeholder)
	totalCost := 0.0
	for _, resp := range result.Proposals {
		totalCost += estimateCost(resp)
	}
	if result.Synthesis != nil {
		totalCost += estimateCost(result.Synthesis)
	}
	parts = append(parts, fmt.Sprintf("Cost: %s", formatCostValue(totalCost)))

	footer := strings.Join(parts, " | ")

	// Agreement (if available)
	if result.AgreementScore != nil {
		agreementLine := fmt.Sprintf("\nAgreement: %s — %s",
			colorizeAgreementScore(*result.AgreementScore),
			result.AgreementReason)
		footer += agreementLine
	}

	return footer
}

// RenderModels formats a model availability table.
func RenderModels(tier string, proposers []config.ModelConfig, aggregator config.ModelConfig) string {
	var b strings.Builder

	b.WriteString(headerStyle.Render(fmt.Sprintf("Models for tier: %s", tier)))
	b.WriteString("\n\n")

	// Table header
	b.WriteString("Role         Model                    Status\n")
	b.WriteString("────────────────────────────────────────────────\n")

	// Proposers
	for _, model := range proposers {
		status := "✓"
		statusStyle := successStyle
		if !config.Available(model.Model) {
			status = "✗ (no API key)"
			statusStyle = errorStyle
		}

		b.WriteString(fmt.Sprintf("Proposer     %-24s %s\n",
			model.Name,
			statusStyle.Render(status)))
	}

	// Aggregator
	aggStatus := "✓"
	aggStatusStyle := successStyle
	if !config.Available(aggregator.Model) {
		aggStatus = "✗ (no API key)"
		aggStatusStyle = errorStyle
	}
	b.WriteString(fmt.Sprintf("Aggregator   %-24s %s\n",
		aggregator.Name,
		aggStatusStyle.Render(aggStatus)))

	return b.String()
}

// RenderCosts formats cost summary tables.
func RenderCosts(summary Summary, byTier map[string]float64, byMode map[string]float64) string {
	var b strings.Builder

	// Overall summary
	b.WriteString(headerStyle.Render("Cost Summary"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Today:      %s\n", formatCostValue(summary.Today)))
	b.WriteString(fmt.Sprintf("This Week:  %s\n", formatCostValue(summary.Week)))
	b.WriteString(fmt.Sprintf("This Month: %s\n", formatCostValue(summary.Month)))
	b.WriteString(fmt.Sprintf("All Time:   %s\n\n", formatCostValue(summary.AllTime)))

	b.WriteString(fmt.Sprintf("Total Queries: %d (today: %d)\n\n", summary.QueryCount, summary.QueriesToday))

	// By tier
	if len(byTier) > 0 {
		b.WriteString(headerStyle.Render("Cost by Tier"))
		b.WriteString("\n\n")
		for tier, cost := range byTier {
			b.WriteString(fmt.Sprintf("%-10s %s\n", tier+":", formatCostValue(cost)))
		}
		b.WriteString("\n")
	}

	// By mode
	if len(byMode) > 0 {
		b.WriteString(headerStyle.Render("Cost by Mode"))
		b.WriteString("\n\n")
		for mode, cost := range byMode {
			b.WriteString(fmt.Sprintf("%-10s %s\n", mode+":", formatCostValue(cost)))
		}
	}

	return b.String()
}

// ProgressTracker manages a progress display using pterm.
type ProgressTracker struct {
	spinner *pterm.SpinnerPrinter
	events  <-chan provider.ProgressEvent
	done    chan struct{}
}

// NewProgressTracker creates a new progress tracker.
func NewProgressTracker(events <-chan provider.ProgressEvent) *ProgressTracker {
	// Disable spinner in tests when race detector is enabled or NO_COLOR is set
	var spinner *pterm.SpinnerPrinter
	if os.Getenv("NO_COLOR") == "" && os.Getenv("PTERM_DISABLE") == "" {
		spinner, _ = pterm.DefaultSpinner.Start("Querying models...")
	}
	return &ProgressTracker{
		spinner: spinner,
		events:  events,
		done:    make(chan struct{}),
	}
}

// Start begins listening for progress events and updating the display.
func (pt *ProgressTracker) Start() {
	go func() {
		defer close(pt.done)
		completed := 0
		failed := 0

		for event := range pt.events {
			switch event.Status {
			case "querying":
				if pt.spinner != nil {
					pt.spinner.UpdateText(fmt.Sprintf("Querying %s...", event.Model))
				}
			case "done":
				completed++
				if pt.spinner != nil {
					pt.spinner.UpdateText(fmt.Sprintf("✓ %s (%s) [%d completed]",
						event.Model,
						formatDuration(event.Latency),
						completed))
				}
			case "failed":
				failed++
				if pt.spinner != nil {
					pt.spinner.Warning(fmt.Sprintf("✗ %s failed: %v [%d failed]",
						event.Model,
						event.Error,
						failed))
				}
			}
		}
	}()
}

// Wait blocks until all progress events are processed.
func (pt *ProgressTracker) Wait() {
	<-pt.done
	if pt.spinner != nil {
		pt.spinner.Success("All models completed")
	}
}

// Stop stops the progress tracker.
func (pt *ProgressTracker) Stop() {
	if pt.spinner != nil {
		pt.spinner.Stop()
	}
}

// Helper functions

// formatDuration formats a duration in human-readable form.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

// formatCostValue formats a cost value with appropriate precision.
func formatCostValue(cost float64) string {
	if cost >= 0.01 {
		return fmt.Sprintf("$%.2f", cost)
	}
	return fmt.Sprintf("$%.4f", cost)
}

// formatCost estimates and formats the cost of a single response.
func formatCost(resp *provider.Response) string {
	return formatCostValue(estimateCost(resp))
}

// estimateCost provides a rough cost estimate based on token counts.
// This is a placeholder until the proper cost tracking is implemented.
func estimateCost(resp *provider.Response) float64 {
	// Rough estimates ($/1M tokens): input $3, output $15
	inputCost := float64(resp.InputTokens) * 3.0 / 1_000_000
	outputCost := float64(resp.OutputTokens) * 15.0 / 1_000_000
	return inputCost + outputCost
}

// colorizeAgreementScore applies color to an agreement score percentage.
func colorizeAgreementScore(score int) string {
	scoreStr := fmt.Sprintf("%d%%", score)
	if score >= 70 {
		return successStyle.Render(scoreStr)
	} else if score >= 40 {
		return warningStyle.Render(scoreStr)
	}
	return errorStyle.Render(scoreStr)
}

// IsColorDisabled returns true if color output is disabled.
func IsColorDisabled() bool {
	return os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" || !isTerminal()
}

// isTerminal checks if stdout is a terminal.
func isTerminal() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
