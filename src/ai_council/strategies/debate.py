"""Debate — multi-round revision until consensus."""

from __future__ import annotations

import asyncio
from dataclasses import dataclass

from ai_council.config import DEFAULT_TIER, get_aggregator, get_proposers
from ai_council.prompts import (
    DEBATE_JUDGE_SYSTEM,
    DEBATE_JUDGE_TEMPLATE,
    DEBATE_REVISION_SYSTEM,
    DEBATE_REVISION_TEMPLATE,
    format_debate_history,
    format_other_responses,
)
from ai_council.providers import ModelResponse, call_model, call_models_parallel
from ai_council.scoring import score_agreement, should_score


@dataclass
class DebateResult:
    rounds: list[list[ModelResponse]]
    synthesis: ModelResponse
    num_rounds: int
    total_ms: int
    tier: str = DEFAULT_TIER
    agreement_score: int | None = None
    agreement_reason: str | None = None
    scorer_cost_usd: float = 0.0

    @property
    def succeeded(self) -> list[ModelResponse]:
        """Return succeeded responses from the final round."""
        if not self.rounds:
            return []
        return [r for r in self.rounds[-1] if r.succeeded]

    @property
    def total_cost_usd(self) -> float:
        cost = self.synthesis.cost_usd + self.scorer_cost_usd
        for rnd in self.rounds:
            cost += sum(r.cost_usd for r in rnd)
        return cost


async def run_debate(prompt: str, max_rounds: int = 3, tier: str = DEFAULT_TIER) -> DebateResult:
    proposers = get_proposers(tier)
    rounds: list[list[ModelResponse]] = []
    total = 0

    # Round 1: independent answers
    r1 = await call_models_parallel(proposers, [{"role": "user", "content": prompt}])
    rounds.append(r1)
    total += sum(r.latency_ms for r in r1)

    prev = r1
    for _ in range(2, max_rounds + 1):
        ok = [r for r in prev if r.succeeded]
        if len(ok) < 2:
            break

        tasks = []
        for cfg in proposers:
            own = next((r for r in ok if r.model == cfg.model), None)
            if not own:
                continue
            revision_prompt = DEBATE_REVISION_TEMPLATE.format(
                prompt=prompt,
                own_response=own.content,
                other_responses=format_other_responses(ok, cfg.model),
            )
            tasks.append(
                call_model(cfg, [
                    {"role": "system", "content": DEBATE_REVISION_SYSTEM},
                    {"role": "user", "content": revision_prompt},
                ])
            )

        rnd = list(await asyncio.gather(*tasks))
        rounds.append(rnd)
        total += sum(r.latency_ms for r in rnd)
        prev = rnd

    # Judge synthesizes
    judge_prompt = DEBATE_JUDGE_TEMPLATE.format(
        prompt=prompt, debate_history=format_debate_history(rounds)
    )
    synthesis = await call_model(
        get_aggregator(tier),
        [{"role": "system", "content": DEBATE_JUDGE_SYSTEM},
         {"role": "user", "content": judge_prompt}],
    )
    total += synthesis.latency_ms

    # Score agreement on final round
    final_ok = [r for r in prev if r.succeeded]
    score = None
    reason = None
    scorer_cost = 0.0
    if should_score(len(final_ok), tier):
        score, reason, scorer_cost = await score_agreement(final_ok, prompt)

    return DebateResult(
        rounds=rounds, synthesis=synthesis, num_rounds=len(rounds), total_ms=total,
        tier=tier, agreement_score=score, agreement_reason=reason,
        scorer_cost_usd=scorer_cost,
    )
