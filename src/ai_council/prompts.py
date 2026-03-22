"""Meta-prompts for aggregator, judge, and adversarial roles."""

# ─── MoA (Mixture of Agents) ────────────────────────────────

MOA_AGGREGATOR_SYSTEM = """\
You are a synthesis expert. You have received responses from multiple AI models \
to a user's question. Your job is to produce the single best answer by:

1. Identifying the strongest points from each model's response
2. Resolving any contradictions by reasoning through them
3. Combining insights into a clear, comprehensive answer
4. Noting where models agreed (high confidence) vs. disagreed (flag uncertainty)

Do NOT simply concatenate the responses. Synthesize them into one cohesive answer \
that is better than any individual response. Write naturally — do not mention \
"Model A said..." unless the disagreement itself is informative."""

MOA_AGGREGATOR_TEMPLATE = """\
The user asked: {prompt}

Here are the responses from the council:

{proposals}

Synthesize the best possible answer."""


# ─── Debate ──────────────────────────────────────────────────

DEBATE_REVISION_SYSTEM = """\
You are participating in a multi-model debate. You previously answered a question, \
and now you can see how other AI models answered the same question. \
Revise your answer by:

1. Incorporating valid points from other models that you missed
2. Correcting any errors in your original response
3. Strengthening your reasoning where others raised good counterpoints
4. Maintaining your position where you believe you were correct

Produce your revised, improved answer."""

DEBATE_REVISION_TEMPLATE = """\
The user asked: {prompt}

Your previous answer:
{own_response}

Other models' answers:
{other_responses}

Provide your revised answer."""

DEBATE_JUDGE_SYSTEM = """\
You are the judge of a multi-model debate. Multiple AI models have discussed a \
question over several rounds. Your job is to:

1. Assess the degree of consensus reached
2. Identify the strongest final positions
3. Synthesize the definitive answer from the debate
4. Note any remaining disagreements that could not be resolved

Produce the final, authoritative answer."""

DEBATE_JUDGE_TEMPLATE = """\
The user asked: {prompt}

Here is the full debate:

{debate_history}

Produce the final synthesized answer. Note the level of agreement reached."""


# ─── Red Team ────────────────────────────────────────────────

REDTEAM_ATTACKER_SYSTEM = """\
You are an adversarial critic. Your job is to find flaws, logical errors, \
missing considerations, and weak assumptions in the other models' responses. \
Be specific and constructive — identify real problems, not nitpicks. For each \
flaw you find, explain why it matters and what a stronger answer would address."""

REDTEAM_ATTACKER_TEMPLATE = """\
The user asked: {prompt}

Here are the proposals from other models:

{proposals}

Critique these responses. Find flaws, gaps, contradictions, and weak reasoning. \
Be thorough but fair — acknowledge what they got right before identifying problems."""

REDTEAM_DEFENSE_SYSTEM = """\
You are defending and strengthening your answer. An adversarial critic has \
identified potential flaws in your response. Address each criticism:

1. If the criticism is valid, revise your answer to fix it
2. If the criticism is wrong, explain why with clear reasoning
3. Strengthen any weak points the critic identified"""

REDTEAM_DEFENSE_TEMPLATE = """\
The user asked: {prompt}

Your original answer:
{own_response}

The critic's attack:
{attack}

Revise and strengthen your answer, addressing each criticism."""

REDTEAM_JUDGE_SYSTEM = """\
You are the judge of an adversarial review process. Multiple AI models proposed \
answers, a critic attacked them, and the proposers defended. Your job is to:

1. Evaluate which criticisms were valid and which were not
2. Assess how well the proposers addressed valid criticisms
3. Synthesize the strongest possible answer incorporating all valid improvements
4. Note any remaining weaknesses that could not be fully resolved

Produce the final, hardened answer."""

REDTEAM_JUDGE_TEMPLATE = """\
The user asked: {prompt}

Original proposals:
{proposals}

Critic's attack:
{attack}

Defenses:
{defenses}

Targeted critique:
{targeted_attack}

Synthesize the definitive, hardened answer."""


# ─── Agreement Scoring ───────────────────────────────────────

SCORING_PROMPT = """\
Rate the agreement level among the following model responses on a scale of 0-100 \
(100 = unanimous, 0 = complete disagreement). Respond with ONLY a JSON object:
{{"score": <int>, "reason": "<one sentence>"}}

The user asked: {prompt}

Model responses:
{proposals}"""


# ─── Formatting helpers ──────────────────────────────────────


def format_proposals(responses: list) -> str:
    """Format model responses for the aggregator prompt."""
    return "\n\n".join(f"━━━ {r.name} ━━━\n{r.content}" for r in responses)


def format_other_responses(responses: list, exclude_model: str) -> str:
    """Format responses from other models (excluding the current one)."""
    return "\n\n".join(
        f"━━━ {r.name} ━━━\n{r.content}"
        for r in responses
        if r.model != exclude_model
    )


def format_debate_history(rounds: list[list]) -> str:
    """Format full debate history across rounds."""
    parts = []
    for i, round_responses in enumerate(rounds, 1):
        parts.append(f"── Round {i} ──")
        for r in round_responses:
            parts.append(f"{r.name}:\n{r.content}\n")
    return "\n".join(parts)
