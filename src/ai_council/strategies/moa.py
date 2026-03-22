"""Mixture of Agents — parallel proposals + synthesis."""

from __future__ import annotations

from dataclasses import dataclass

from ai_council.config import get_aggregator, get_proposers
from ai_council.prompts import MOA_AGGREGATOR_SYSTEM, MOA_AGGREGATOR_TEMPLATE, format_proposals
from ai_council.providers import ModelResponse, call_model, call_models_parallel
from ai_council.scoring import score_agreement


@dataclass
class MoAResult:
    proposals: list[ModelResponse]
    synthesis: ModelResponse
    total_ms: int
    agreement_score: int | None = None
    agreement_reason: str | None = None

    @property
    def succeeded(self) -> list[ModelResponse]:
        return [p for p in self.proposals if p.succeeded]


async def run_moa(prompt: str) -> MoAResult:
    proposers = get_proposers()
    proposals = await call_models_parallel(
        proposers, [{"role": "user", "content": prompt}]
    )

    ok = [p for p in proposals if p.succeeded]
    if not ok:
        return MoAResult(
            proposals=proposals,
            synthesis=ModelResponse("", "aggregator", "All models failed.", 0, "no proposals"),
            total_ms=sum(p.latency_ms for p in proposals),
        )

    agg_prompt = MOA_AGGREGATOR_TEMPLATE.format(
        prompt=prompt, proposals=format_proposals(ok)
    )
    synthesis = await call_model(
        get_aggregator(),
        [{"role": "system", "content": MOA_AGGREGATOR_SYSTEM},
         {"role": "user", "content": agg_prompt}],
    )

    # Score agreement in parallel with nothing (fire and forget would be ideal,
    # but we want the result)
    score, reason = await score_agreement(ok, prompt)

    return MoAResult(
        proposals=proposals,
        synthesis=synthesis,
        total_ms=sum(p.latency_ms for p in proposals) + synthesis.latency_ms,
        agreement_score=score,
        agreement_reason=reason,
    )
