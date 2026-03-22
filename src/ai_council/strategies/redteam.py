"""Red Team — adversarial critique and defense."""

from __future__ import annotations

import asyncio
from dataclasses import dataclass

from ai_council.config import get_aggregator, get_proposers
from ai_council.prompts import (
    REDTEAM_ATTACKER_SYSTEM,
    REDTEAM_ATTACKER_TEMPLATE,
    REDTEAM_DEFENSE_SYSTEM,
    REDTEAM_DEFENSE_TEMPLATE,
    REDTEAM_JUDGE_SYSTEM,
    REDTEAM_JUDGE_TEMPLATE,
    format_proposals,
)
from ai_council.providers import ModelResponse, call_model, call_models_parallel
from ai_council.scoring import score_agreement


@dataclass
class RedTeamResult:
    proposals: list[ModelResponse]
    initial_attack: ModelResponse
    defenses: list[ModelResponse]
    targeted_attack: ModelResponse
    synthesis: ModelResponse
    total_ms: int = 0
    agreement_score: int | None = None
    agreement_reason: str | None = None


def _select_attacker(prompt: str, proposers: list) -> int:
    """Select attacker index via prompt hash. Deterministic per prompt."""
    return hash(prompt) % len(proposers)


async def run_redteam(prompt: str) -> RedTeamResult:
    all_proposers = get_proposers()

    if len(all_proposers) < 2:
        from ai_council.strategies.moa import run_moa
        moa = await run_moa(prompt)
        return RedTeamResult(
            proposals=moa.proposals, initial_attack=moa.synthesis,
            defenses=[], targeted_attack=moa.synthesis,
            synthesis=moa.synthesis, total_ms=moa.total_ms,
        )

    attacker_idx = _select_attacker(prompt, all_proposers)
    attacker_cfg = all_proposers[attacker_idx]
    proposer_cfgs = [p for i, p in enumerate(all_proposers) if i != attacker_idx]

    total = 0

    # Round 1: Proposers answer
    proposals = await call_models_parallel(
        proposer_cfgs, [{"role": "user", "content": prompt}]
    )
    total += sum(p.latency_ms for p in proposals)

    ok_proposals = [p for p in proposals if p.succeeded]
    if not ok_proposals:
        fail = ModelResponse("", "redteam", "All proposers failed.", 0, "no proposals")
        return RedTeamResult(
            proposals=proposals, initial_attack=fail,
            defenses=[], targeted_attack=fail, synthesis=fail, total_ms=total,
        )

    # Round 1: Attacker critiques
    attack_prompt = REDTEAM_ATTACKER_TEMPLATE.format(
        prompt=prompt, proposals=format_proposals(ok_proposals)
    )
    initial_attack = await call_model(
        attacker_cfg,
        [{"role": "system", "content": REDTEAM_ATTACKER_SYSTEM},
         {"role": "user", "content": attack_prompt}],
    )
    total += initial_attack.latency_ms

    # Round 2: Proposers defend
    defense_tasks = []
    for cfg in proposer_cfgs:
        own = next((p for p in ok_proposals if p.model == cfg.model), None)
        if not own:
            continue
        defense_prompt = REDTEAM_DEFENSE_TEMPLATE.format(
            prompt=prompt, own_response=own.content,
            attack=initial_attack.content if initial_attack.succeeded else "No attack available.",
        )
        defense_tasks.append(
            call_model(cfg, [
                {"role": "system", "content": REDTEAM_DEFENSE_SYSTEM},
                {"role": "user", "content": defense_prompt},
            ])
        )
    defenses = list(await asyncio.gather(*defense_tasks))
    total += sum(d.latency_ms for d in defenses)

    # Round 2: Attacker targets defenses
    ok_defenses = [d for d in defenses if d.succeeded]
    targeted_prompt = REDTEAM_ATTACKER_TEMPLATE.format(
        prompt=prompt,
        proposals=format_proposals(ok_defenses) if ok_defenses else "No defenses.",
    )
    targeted_attack = await call_model(
        attacker_cfg,
        [{"role": "system", "content": REDTEAM_ATTACKER_SYSTEM},
         {"role": "user", "content": targeted_prompt}],
    )
    total += targeted_attack.latency_ms

    # Final: Judge synthesizes hardened answer
    judge_prompt = REDTEAM_JUDGE_TEMPLATE.format(
        prompt=prompt,
        proposals=format_proposals(ok_proposals),
        attack=initial_attack.content if initial_attack.succeeded else "N/A",
        defenses=format_proposals(ok_defenses) if ok_defenses else "N/A",
        targeted_attack=targeted_attack.content if targeted_attack.succeeded else "N/A",
    )
    synthesis = await call_model(
        get_aggregator(),
        [{"role": "system", "content": REDTEAM_JUDGE_SYSTEM},
         {"role": "user", "content": judge_prompt}],
    )
    total += synthesis.latency_ms

    # Score agreement on defenses (or proposals if no defenses)
    score_targets = ok_defenses if ok_defenses else ok_proposals
    score, reason = await score_agreement(score_targets, prompt)

    return RedTeamResult(
        proposals=proposals, initial_attack=initial_attack,
        defenses=defenses, targeted_attack=targeted_attack,
        synthesis=synthesis, total_ms=total,
        agreement_score=score, agreement_reason=reason,
    )
