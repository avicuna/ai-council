"""Meta-prompts for aggregator and judge roles."""

MOA_SYSTEM = """\
You are a synthesis expert. Multiple AI models answered the user's question. \
Produce the single best answer by combining the strongest points, resolving \
contradictions, and noting where models agreed vs. disagreed. Write naturally \
— don't say "Model A said..." unless the disagreement itself is informative."""

MOA_TEMPLATE = """\
The user asked: {prompt}

Here are the responses from the council:

{proposals}

Synthesize the best possible answer."""

DEBATE_REVISE_SYSTEM = """\
You are in a multi-model debate. You previously answered a question and can now \
see how other models answered. Revise your answer: incorporate valid points you \
missed, correct errors, strengthen reasoning, maintain your position where correct."""

DEBATE_REVISE_TEMPLATE = """\
The user asked: {prompt}

Your previous answer:
{own_response}

Other models' answers:
{other_responses}

Provide your revised answer."""

DEBATE_JUDGE_SYSTEM = """\
You are the judge of a multi-model debate. Assess consensus, identify the \
strongest positions, synthesize the definitive answer, and note remaining \
disagreements."""

DEBATE_JUDGE_TEMPLATE = """\
The user asked: {prompt}

Full debate:

{debate_history}

Produce the final synthesized answer. Note the level of agreement reached."""


def format_proposals(responses: list) -> str:
    return "\n\n".join(f"━━━ {r.name} ━━━\n{r.content}" for r in responses)


def format_others(responses: list, exclude_model: str) -> str:
    return "\n\n".join(
        f"━━━ {r.name} ━━━\n{r.content}"
        for r in responses
        if r.model != exclude_model
    )


def format_debate(rounds: list[list]) -> str:
    parts = []
    for i, rnd in enumerate(rounds, 1):
        parts.append(f"── Round {i} ──")
        for r in rnd:
            parts.append(f"{r.name}:\n{r.content}\n")
    return "\n".join(parts)
