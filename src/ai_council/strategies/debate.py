"""Debate — multi-round revision until consensus."""

from __future__ import annotations

import asyncio
from dataclasses import dataclass

from ai_council.config import get_aggregator, get_proposers
from ai_council.prompts import (
    DEBATE_JUDGE_SYSTEM,
    DEBATE_JUDGE_TEMPLATE,
    DEBATE_REVISE_SYSTEM,
    DEBATE_REVISE_TEMPLATE,
    format_debate,
    format_others,
)
from ai_council.providers import ModelResponse, call_model, call_models_parallel


@dataclass
class DebateResult:
    rounds: list[list[ModelResponse]]
    synthesis: ModelResponse
    num_rounds: int
    total_ms: int


async def run_debate(prompt: str, max_rounds: int = 3) -> DebateResult:
    proposers = get_proposers()
    rounds: list[list[ModelResponse]] = []
    total = 0

    # Round 1: independent
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
            revision_prompt = DEBATE_REVISE_TEMPLATE.format(
                prompt=prompt,
                own_response=own.content,
                other_responses=format_others(ok, cfg.model),
            )
            tasks.append(
                call_model(cfg, [
                    {"role": "system", "content": DEBATE_REVISE_SYSTEM},
                    {"role": "user", "content": revision_prompt},
                ])
            )

        rnd = list(await asyncio.gather(*tasks))
        rounds.append(rnd)
        total += sum(r.latency_ms for r in rnd)
        prev = rnd

    # Judge
    judge_prompt = DEBATE_JUDGE_TEMPLATE.format(
        prompt=prompt, debate_history=format_debate(rounds)
    )
    synthesis = await call_model(
        get_aggregator(),
        [{"role": "system", "content": DEBATE_JUDGE_SYSTEM}, {"role": "user", "content": judge_prompt}],
    )
    total += synthesis.latency_ms

    return DebateResult(rounds=rounds, synthesis=synthesis, num_rounds=len(rounds), total_ms=total)
