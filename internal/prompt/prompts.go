package prompt

import (
	"fmt"
	"strings"

	"github.com/avicuna/ai-council-personal/internal/provider"
)

// MoA (Mixture of Agents) prompts
const (
	MOAAggregatorSystem = "You are an expert synthesizer. You will receive multiple AI model responses to the same question. Your job is to identify the strongest points from each response, resolve any contradictions, and produce a single, comprehensive answer that is better than any individual response. Be direct and authoritative — don't mention 'the models said' or reference the synthesis process."

	MOAAggregatorTemplate = "Original question: {prompt}\n\nModel responses:\n{proposals}\n\nSynthesize the best possible answer from these responses."
)

// Debate prompts
const (
	DebateRevisionSystem = "You are participating in a multi-model debate. You previously answered a question, and now you can see how other models answered. Revise your answer: incorporate valid points from others, correct any errors in your original response, and strengthen your reasoning. Be direct — don't reference the debate process."

	DebateRevisionTemplate = "Original question: {prompt}\n\nYour previous answer:\n{own_response}\n\nOther models' answers:\n{other_responses}\n\nRevise your answer."

	DebateJudgeSystem = "You are the final judge in a multi-model debate. You will see multiple rounds of responses where models refined their answers after seeing each other's work. Synthesize the definitive answer — take the strongest reasoning from any round, resolve remaining disagreements, and produce the best possible response. Be direct and authoritative."

	DebateJudgeTemplate = "Original question: {prompt}\n\nDebate history:\n{debate_history}\n\nSynthesize the definitive answer."
)

// Red Team prompts
const (
	RedTeamAttackerSystem = "You are a critical reviewer. Your job is to find flaws, logical errors, missing considerations, and weak assumptions in the other models' responses. Be specific and constructive — identify real problems, not nitpicks."

	RedTeamAttackerTemplate = "Original question: {prompt}\n\nProposed answers:\n{proposals}\n\nCritique these responses. Find flaws, gaps, and weak reasoning."

	RedTeamDefenseSystem = "Your answer was critiqued by a reviewer. Address their criticism: fix genuine errors, strengthen weak points, and defend valid reasoning. If the critic found real problems, acknowledge and fix them. If the criticism was unfair, explain why your approach is correct."

	RedTeamDefenseTemplate = "Original question: {prompt}\n\nYour original answer:\n{own_response}\n\nCritique of your answer:\n{attack}\n\nRevise and defend your answer."

	RedTeamJudgeSystem = "You are the final judge in an adversarial review. You will see proposals, critiques, and defenses. Synthesize a hardened answer that incorporates valid criticisms and strong defenses. The result should be more robust than any individual response."

	RedTeamJudgeTemplate = "Original question: {prompt}\n\nOriginal proposals:\n{proposals}\n\nCritique:\n{attack}\n\nDefenses:\n{defenses}\n\nTargeted critique:\n{targeted_attack}\n\nSynthesize a hardened, definitive answer."
)

// Scoring prompt
const (
	ScoringPrompt = "Rate the agreement level among the following model responses on a scale of 0-100 (100 = unanimous agreement, 0 = complete disagreement). Consider both the conclusions and the reasoning.\n\nOriginal question: {prompt}\n\nModel responses:\n{proposals}\n\nRespond with ONLY a JSON object: {\"score\": <int>, \"reason\": \"<one sentence>\"}"
)

// FormatProposals formats model responses as a block of proposals.
// Each response is formatted as:
//
//	━━━ ModelName ━━━
//	content
func FormatProposals(responses []provider.Response) string {
	if len(responses) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, resp := range responses {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("━━━ %s ━━━\n%s", resp.Name, resp.Content))
	}
	return sb.String()
}

// FormatOtherResponses formats model responses excluding one specific model.
// Same format as FormatProposals, but skips the response matching excludeModel.
func FormatOtherResponses(responses []provider.Response, excludeModel string) string {
	filtered := make([]provider.Response, 0, len(responses))
	for _, resp := range responses {
		if resp.Name != excludeModel {
			filtered = append(filtered, resp)
		}
	}
	return FormatProposals(filtered)
}

// DebateRound represents a single round in a debate.
type DebateRound struct {
	RoundNum  int
	Responses []provider.Response
}

// FormatDebateHistory formats all debate rounds with headers.
// Each round is formatted as:
//
//	═══ Round N ═══
//	━━━ ModelName ━━━
//	content
func FormatDebateHistory(rounds []DebateRound) string {
	if len(rounds) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, round := range rounds {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(fmt.Sprintf("═══ Round %d ═══\n", round.RoundNum))
		sb.WriteString(FormatProposals(round.Responses))
	}
	return sb.String()
}
