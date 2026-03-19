"""Mixture of Agents — parallel proposals + synthesis."""

from __future__ import annotations

from dataclasses import dataclass

from ai_council.config import get_aggregator, get_proposers
from ai_council.prompts import MOA_SYSTEM, MOA_TEMPLATE, format_proposals
from ai_council.providers import ModelResponse, call_model, call_models_parallel


@dataclass
class MoAResult:
    proposals: list[ModelResponse]
    synthesis: ModelResponse
    total_ms: int

    @property
    def succeeded(self) -> list[ModelResponse]:
        return [p for p in self.proposals if p.succeeded]


async def run_moa(prompt: str) -> MoAResult:
    proposals = await call_models_parallel(
        get_proposers(), [{"role": "user", "content": prompt}]
    )

    ok = [p for p in proposals if p.succeeded]
    if not ok:
        return MoAResult(
            proposals=proposals,
            synthesis=ModelResponse("", "aggregator", "All models failed.", 0, "no proposals"),
            total_ms=sum(p.latency_ms for p in proposals),
        )

    agg_prompt = MOA_TEMPLATE.format(prompt=prompt, proposals=format_proposals(ok))
    synthesis = await call_model(
        get_aggregator(),
        [{"role": "system", "content": MOA_SYSTEM}, {"role": "user", "content": agg_prompt}],
    )

    return MoAResult(
        proposals=proposals,
        synthesis=synthesis,
        total_ms=sum(p.latency_ms for p in proposals) + synthesis.latency_ms,
    )
