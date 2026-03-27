"""Agreement scoring via separate follow-up call."""

from __future__ import annotations

import json
import re

from ai_council.config import ModelConfig
from ai_council.prompts import SCORING_PROMPT, format_proposals
from ai_council.providers import ModelResponse, call_model

# Use a cheap, fast model for scoring
SCORING_MODEL = ModelConfig(
    model="gpt-4o-mini",
    name="Scorer",
)


def should_score(succeeded_count: int, tier: str) -> bool:
    """Decide whether agreement scoring is worth the API call.

    Runs when 3+ models succeeded and tier is balanced or full.
    Skips on fast tier (scorer cost is proportionally too high)
    and when fewer than 3 models succeeded (binary agreement is uninformative).
    """
    return succeeded_count >= 3 and tier in ("balanced", "full")


def parse_agreement_score(text: str) -> tuple[int | None, str | None]:
    """Parse agreement score from model response. JSON first, regex fallback."""
    if not text:
        return None, None

    try:
        data = json.loads(text)
        return int(data["score"]), data.get("reason")
    except (json.JSONDecodeError, KeyError, ValueError, TypeError):
        pass

    # Regex fallback for malformed JSON
    match = re.search(r'"score"\s*:\s*(\d+)', text)
    if match:
        reason_match = re.search(r'"reason"\s*:\s*"([^"]+)"', text)
        return int(match.group(1)), reason_match.group(1) if reason_match else None

    return None, None


async def score_agreement(
    proposals: list[ModelResponse], prompt: str
) -> tuple[int | None, str | None, float]:
    """Score agreement among proposals. Returns (score, reason, cost_usd)."""
    ok = [p for p in proposals if p.succeeded]
    if len(ok) < 2:
        return None, None, 0.0

    scoring_prompt = SCORING_PROMPT.format(
        prompt=prompt, proposals=format_proposals(ok)
    )
    result = await call_model(
        SCORING_MODEL,
        [{"role": "user", "content": scoring_prompt}],
    )

    if not result.succeeded:
        return None, None, result.cost_usd

    score, reason = parse_agreement_score(result.content)
    return score, reason, result.cost_usd
