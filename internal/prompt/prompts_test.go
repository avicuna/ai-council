package prompt

import (
	"strings"
	"testing"

	"github.com/avicuna/ai-council-personal/internal/provider"
)

func TestFormatProposals(t *testing.T) {
	tests := []struct {
		name      string
		responses []provider.Response
		want      string
	}{
		{
			name:      "empty responses",
			responses: []provider.Response{},
			want:      "",
		},
		{
			name: "single response",
			responses: []provider.Response{
				{Name: "claude-opus-4", Content: "This is the answer."},
			},
			want: "━━━ claude-opus-4 ━━━\nThis is the answer.",
		},
		{
			name: "multiple responses",
			responses: []provider.Response{
				{Name: "claude-opus-4", Content: "Answer one."},
				{Name: "gpt-5-pro", Content: "Answer two."},
				{Name: "gemini-2.5-pro", Content: "Answer three."},
			},
			want: "━━━ claude-opus-4 ━━━\nAnswer one.\n\n━━━ gpt-5-pro ━━━\nAnswer two.\n\n━━━ gemini-2.5-pro ━━━\nAnswer three.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatProposals(tt.responses)
			if got != tt.want {
				t.Errorf("FormatProposals() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestFormatOtherResponses(t *testing.T) {
	responses := []provider.Response{
		{Name: "claude-opus-4", Content: "Answer one."},
		{Name: "gpt-5-pro", Content: "Answer two."},
		{Name: "gemini-2.5-pro", Content: "Answer three."},
	}

	tests := []struct {
		name          string
		excludeModel  string
		wantContains  []string
		wantExcludes  []string
	}{
		{
			name:         "exclude claude",
			excludeModel: "claude-opus-4",
			wantContains: []string{"gpt-5-pro", "gemini-2.5-pro", "Answer two", "Answer three"},
			wantExcludes: []string{"claude-opus-4", "Answer one"},
		},
		{
			name:         "exclude gpt",
			excludeModel: "gpt-5-pro",
			wantContains: []string{"claude-opus-4", "gemini-2.5-pro", "Answer one", "Answer three"},
			wantExcludes: []string{"gpt-5-pro", "Answer two"},
		},
		{
			name:         "exclude nonexistent model",
			excludeModel: "nonexistent",
			wantContains: []string{"claude-opus-4", "gpt-5-pro", "gemini-2.5-pro"},
			wantExcludes: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatOtherResponses(responses, tt.excludeModel)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("FormatOtherResponses() missing %q in output:\n%s", want, got)
				}
			}

			for _, exclude := range tt.wantExcludes {
				if strings.Contains(got, exclude) {
					t.Errorf("FormatOtherResponses() should not contain %q in output:\n%s", exclude, got)
				}
			}
		})
	}
}

func TestFormatDebateHistory(t *testing.T) {
	tests := []struct {
		name   string
		rounds []DebateRound
		want   string
	}{
		{
			name:   "empty rounds",
			rounds: []DebateRound{},
			want:   "",
		},
		{
			name: "single round",
			rounds: []DebateRound{
				{
					RoundNum: 1,
					Responses: []provider.Response{
						{Name: "claude-opus-4", Content: "Initial answer."},
					},
				},
			},
			want: "═══ Round 1 ═══\n━━━ claude-opus-4 ━━━\nInitial answer.",
		},
		{
			name: "multiple rounds",
			rounds: []DebateRound{
				{
					RoundNum: 1,
					Responses: []provider.Response{
						{Name: "claude-opus-4", Content: "Round 1 answer."},
						{Name: "gpt-5-pro", Content: "Round 1 other."},
					},
				},
				{
					RoundNum: 2,
					Responses: []provider.Response{
						{Name: "claude-opus-4", Content: "Round 2 answer."},
						{Name: "gpt-5-pro", Content: "Round 2 other."},
					},
				},
			},
			want: "═══ Round 1 ═══\n━━━ claude-opus-4 ━━━\nRound 1 answer.\n\n━━━ gpt-5-pro ━━━\nRound 1 other.\n\n═══ Round 2 ═══\n━━━ claude-opus-4 ━━━\nRound 2 answer.\n\n━━━ gpt-5-pro ━━━\nRound 2 other.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDebateHistory(tt.rounds)
			if got != tt.want {
				t.Errorf("FormatDebateHistory() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestPromptConstants(t *testing.T) {
	// Verify all prompt constants are non-empty
	prompts := []struct {
		name  string
		value string
	}{
		{"MOAAggregatorSystem", MOAAggregatorSystem},
		{"MOAAggregatorTemplate", MOAAggregatorTemplate},
		{"DebateRevisionSystem", DebateRevisionSystem},
		{"DebateRevisionTemplate", DebateRevisionTemplate},
		{"DebateJudgeSystem", DebateJudgeSystem},
		{"DebateJudgeTemplate", DebateJudgeTemplate},
		{"RedTeamAttackerSystem", RedTeamAttackerSystem},
		{"RedTeamAttackerTemplate", RedTeamAttackerTemplate},
		{"RedTeamDefenseSystem", RedTeamDefenseSystem},
		{"RedTeamDefenseTemplate", RedTeamDefenseTemplate},
		{"RedTeamJudgeSystem", RedTeamJudgeSystem},
		{"RedTeamJudgeTemplate", RedTeamJudgeTemplate},
		{"ScoringPrompt", ScoringPrompt},
	}

	for _, p := range prompts {
		t.Run(p.name, func(t *testing.T) {
			if p.value == "" {
				t.Errorf("prompt constant %s is empty", p.name)
			}
		})
	}
}

func TestPromptTemplatesContainPlaceholders(t *testing.T) {
	// Verify templates contain expected placeholders
	tests := []struct {
		name         string
		template     string
		placeholders []string
	}{
		{
			name:         "MOAAggregatorTemplate",
			template:     MOAAggregatorTemplate,
			placeholders: []string{"{prompt}", "{proposals}"},
		},
		{
			name:         "DebateRevisionTemplate",
			template:     DebateRevisionTemplate,
			placeholders: []string{"{prompt}", "{own_response}", "{other_responses}"},
		},
		{
			name:         "DebateJudgeTemplate",
			template:     DebateJudgeTemplate,
			placeholders: []string{"{prompt}", "{debate_history}"},
		},
		{
			name:         "RedTeamAttackerTemplate",
			template:     RedTeamAttackerTemplate,
			placeholders: []string{"{prompt}", "{proposals}"},
		},
		{
			name:         "RedTeamDefenseTemplate",
			template:     RedTeamDefenseTemplate,
			placeholders: []string{"{prompt}", "{own_response}", "{attack}"},
		},
		{
			name:         "RedTeamJudgeTemplate",
			template:     RedTeamJudgeTemplate,
			placeholders: []string{"{prompt}", "{proposals}", "{attack}", "{defenses}", "{targeted_attack}"},
		},
		{
			name:         "ScoringPrompt",
			template:     ScoringPrompt,
			placeholders: []string{"{prompt}", "{proposals}"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, placeholder := range tt.placeholders {
				if !strings.Contains(tt.template, placeholder) {
					t.Errorf("template %s missing placeholder %s", tt.name, placeholder)
				}
			}
		})
	}
}
