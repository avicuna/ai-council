"""AI Council MCP Server — Personal Edition.

Expose council tools to Claude Code via MCP protocol.
Add to .mcp.json:
    {
        "mcpServers": {
            "ai-council": {
                "command": "council-mcp",
                "args": []
            }
        }
    }
"""

from __future__ import annotations

from pathlib import Path

from mcp.server.fastmcp import FastMCP

from ai_council.config import DEFAULT_TIER, VALID_TIERS, get_aggregator, get_all_proposers, get_proposers
from ai_council.cost_tracker import log_query
from ai_council.strategies.moa import run_moa
from ai_council.strategies.debate import run_debate
from ai_council.strategies.redteam import run_redteam

mcp = FastMCP(
    "ai-council",
    instructions=(
        "Multi-LLM orchestrator — queries up to 6 AI models (Claude, GPT, o3, Gemini, "
        "DeepSeek, Grok) in parallel and synthesizes their responses. "
        "Three modes: MoA (fast synthesis), Debate (multi-round revision), "
        "Red Team (adversarial critique + defense). "
        "Three tiers: fast (cheap, 3 models), balanced (mid, 3 models), full (premium, 6 models)."
    ),
)


def _cost_line(cost: float) -> str:
    if cost < 0.01:
        return f"Cost: ${cost:.4f}"
    return f"Cost: ${cost:.2f}"


def _log(result, mode: str, prompt: str) -> None:
    models = [p.name for p in result.proposals] if hasattr(result, "proposals") else []
    succeeded = len(result.succeeded) if hasattr(result, "succeeded") else 0
    log_query(
        mode=mode,
        tier=getattr(result, "tier", DEFAULT_TIER),
        models_used=models,
        models_succeeded=succeeded,
        total_cost_usd=result.total_cost_usd,
        total_ms=result.total_ms,
        prompt_preview=prompt,
    )


def _format_moa_result(result, verbose: bool = True) -> str:
    parts = []
    if verbose:
        for p in result.proposals:
            if p.succeeded:
                parts.append(f"━━━ {p.name} ({p.latency_ms}ms, ${p.cost_usd:.4f}) ━━━\n{p.content}")
            else:
                parts.append(f"━━━ {p.name} (FAILED) ━━━\n{p.error}")

    parts.append(f"\n━━━ Council Synthesis ━━━\n{result.synthesis.content}")

    n = len(result.succeeded)
    parts.append(f"\n[Models: {n}/{len(result.proposals)} | Mode: MoA | Tier: {result.tier} | Time: {result.total_ms / 1000:.1f}s | {_cost_line(result.total_cost_usd)}]")
    if result.agreement_score is not None:
        parts.append(f"Agreement: {result.agreement_score}%")
    return "\n\n".join(parts)


def _format_debate_result(result, verbose: bool = True) -> str:
    parts = []
    if verbose:
        for i, rnd in enumerate(result.rounds, 1):
            parts.append(f"── Round {i} ──")
            for r in rnd:
                if r.succeeded:
                    parts.append(f"{r.name} ({r.latency_ms}ms, ${r.cost_usd:.4f}):\n{r.content}")
                else:
                    parts.append(f"{r.name} (FAILED): {r.error}")

    parts.append(f"\n━━━ Council Verdict ━━━\n{result.synthesis.content}")
    parts.append(f"\n[Rounds: {result.num_rounds} | Mode: Debate | Tier: {result.tier} | Time: {result.total_ms / 1000:.1f}s | {_cost_line(result.total_cost_usd)}]")
    if result.agreement_score is not None:
        parts.append(f"Agreement: {result.agreement_score}%")
    return "\n\n".join(parts)


def _format_redteam_result(result, verbose: bool = True) -> str:
    parts = []
    if verbose:
        for p in result.proposals:
            if p.succeeded:
                parts.append(f"━━━ {p.name} — Proposal ({p.latency_ms}ms, ${p.cost_usd:.4f}) ━━━\n{p.content}")
            else:
                parts.append(f"━━━ {p.name} — Proposal (FAILED) ━━━\n{p.error}")

        if result.initial_attack.succeeded:
            parts.append(f"\n━━━ Attacker — Critique ({result.initial_attack.latency_ms}ms) ━━━\n{result.initial_attack.content}")

        for d in result.defenses:
            if d.succeeded:
                parts.append(f"\n━━━ {d.name} — Defense ({d.latency_ms}ms) ━━━\n{d.content}")

        if result.targeted_attack.succeeded:
            parts.append(f"\n━━━ Attacker — Targeted Critique ({result.targeted_attack.latency_ms}ms) ━━━\n{result.targeted_attack.content}")

    parts.append(f"\n━━━ Hardened Answer ━━━\n{result.synthesis.content}")
    parts.append(f"\n[Mode: Red Team | Tier: {result.tier} | Time: {result.total_ms / 1000:.1f}s | {_cost_line(result.total_cost_usd)}]")
    if result.agreement_score is not None:
        parts.append(f"Agreement: {result.agreement_score}%")
    return "\n\n".join(parts)


@mcp.tool()
async def council_ask(
    prompt: str, mode: str = "moa", verbose: bool = True, rounds: int = 3, tier: str = "full"
) -> str:
    """Ask multiple AI models a question and get a synthesized answer.

    Queries up to 6 models in parallel and synthesizes their best answer.

    Args:
        prompt: The question or prompt to send to all models.
        mode: "moa" (fast synthesis), "debate" (multi-round revision), or "redteam" (adversarial critique).
        verbose: If True, include each model's individual response.
        rounds: Max debate rounds (debate mode only, default 3).
        tier: Model tier — "fast" (cheap: Haiku+4o-mini+Flash), "balanced" (Sonnet+4.1+Gemini Pro), or "full" (all 6 premium).
    """
    if tier not in VALID_TIERS:
        tier = DEFAULT_TIER

    if mode == "debate":
        result = await run_debate(prompt, max_rounds=rounds, tier=tier)
        _log(result, "debate", prompt)
        return _format_debate_result(result, verbose)
    elif mode == "redteam":
        result = await run_redteam(prompt, tier=tier)
        _log(result, "redteam", prompt)
        return _format_redteam_result(result, verbose)
    else:
        result = await run_moa(prompt, tier=tier)
        _log(result, "moa", prompt)
        return _format_moa_result(result, verbose)


@mcp.tool()
async def council_review(file_path: str, tier: str = "full") -> str:
    """Code review a file using multiple AI models.

    Args:
        file_path: Absolute path to the file to review.
        tier: Model tier — "fast", "balanced", or "full" (default).
    """
    path = Path(file_path)
    if not path.exists():
        return f"Error: File not found: {file_path}"

    code = path.read_text()
    prompt = (
        f"Review this code for bugs, performance issues, edge cases, and improvements. "
        f"Be specific — reference line numbers where possible.\n\n"
        f"--- {path.name} ---\n{code}"
    )
    result = await run_moa(prompt, tier=tier)
    _log(result, "review", prompt)
    return _format_moa_result(result)


@mcp.tool()
async def council_debug(error: str, tier: str = "full") -> str:
    """Debug an error using multiple AI models.

    Args:
        error: The error message, stack trace, or description of the issue.
        tier: Model tier — "fast", "balanced", or "full" (default).
    """
    prompt = (
        f"Debug this error. Identify:\n"
        f"1. The most likely root cause\n"
        f"2. What to check first\n"
        f"3. How to fix it\n"
        f"4. How to prevent it in the future\n\n"
        f"--- Error ---\n{error}"
    )
    result = await run_moa(prompt, tier=tier)
    _log(result, "debug", prompt)
    return _format_moa_result(result)


@mcp.tool()
async def council_research(topic: str, rounds: int = 2, tier: str = "full") -> str:
    """Research a topic using multiple AI models in debate mode.

    Args:
        topic: The topic to research.
        rounds: Number of debate rounds (default 2).
        tier: Model tier — "fast", "balanced", or "full" (default).
    """
    prompt = (
        f"Research this topic thoroughly. Cover:\n"
        f"1. Current state and best practices\n"
        f"2. Key trade-offs and considerations\n"
        f"3. Practical recommendations with reasoning\n"
        f"4. Common pitfalls to avoid\n\n"
        f"Topic: {topic}"
    )
    result = await run_debate(prompt, max_rounds=rounds, tier=tier)
    _log(result, "research", prompt)
    return _format_debate_result(result)


@mcp.tool()
async def council_costs() -> str:
    """Show spending summary and budget tracking."""
    from ai_council.cost_tracker import get_cost_summary, get_cost_by_tier

    summary = get_cost_summary()
    by_tier = get_cost_by_tier()

    lines = [
        "Spending Summary",
        f"  Today:      ${summary['today']:.4f}  ({summary['queries_today']} queries)",
        f"  This week:  ${summary['week']:.4f}",
        f"  This month: ${summary['month']:.4f}",
        f"  All time:   ${summary['all_time']:.4f}  ({summary['query_count']} queries)",
    ]

    if by_tier:
        lines.append("\nBy Tier")
        for tier, cost in by_tier.items():
            lines.append(f"  {tier}: ${cost:.4f}")

    return "\n".join(lines)


@mcp.tool()
async def council_models(tier: str = "full") -> str:
    """Show the configured AI models and their status.

    Args:
        tier: Which tier to show — "fast", "balanced", or "full" (default).
    """
    import os

    if tier not in VALID_TIERS:
        tier = DEFAULT_TIER

    lines = [f"Tier: {tier}", "", "Proposers:"]
    for m in get_all_proposers(tier):
        status = "✓" if m.available else "✗"
        reasoning = " (reasoning)" if m.is_reasoning else ""
        lines.append(f"  {status} {m.name} ({m.model}){reasoning}")

    active = get_proposers(tier)
    lines.append(f"\nActive: {len(active)} models with API keys configured")

    agg = get_aggregator(tier)
    lines.append(f"\nAggregator: {agg.name} ({agg.model})")

    lines.append("\nAll tiers:")
    for t in VALID_TIERS:
        n = len(get_proposers(t))
        marker = " <-- current" if t == tier else ""
        lines.append(f"  {t}: {n} models{marker}")

    lines.append("\nAPI Keys:")
    for label, env in [
        ("Anthropic", "ANTHROPIC_API_KEY"),
        ("OpenAI", "OPENAI_API_KEY"),
        ("Google", "GEMINI_API_KEY"),
        ("DeepSeek", "DEEPSEEK_API_KEY"),
        ("xAI", "XAI_API_KEY"),
    ]:
        status = "✓" if os.environ.get(env) else "✗"
        lines.append(f"  {status} {label} (${env})")

    return "\n".join(lines)


def main():
    mcp.run(transport="stdio")


if __name__ == "__main__":
    main()
