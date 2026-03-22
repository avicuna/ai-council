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

from ai_council.config import get_aggregator, get_all_proposers, get_proposers
from ai_council.strategies.moa import run_moa
from ai_council.strategies.debate import run_debate
from ai_council.strategies.redteam import run_redteam

mcp = FastMCP(
    "ai-council",
    instructions=(
        "Multi-LLM orchestrator — queries up to 6 AI models (Claude, GPT, o3, Gemini, "
        "DeepSeek, Grok) in parallel and synthesizes their responses. "
        "Three modes: MoA (fast synthesis), Debate (multi-round revision), "
        "Red Team (adversarial critique + defense)."
    ),
)


def _format_moa_result(result, verbose: bool = True) -> str:
    parts = []
    if verbose:
        for p in result.proposals:
            if p.succeeded:
                parts.append(f"━━━ {p.name} ({p.latency_ms}ms) ━━━\n{p.content}")
            else:
                parts.append(f"━━━ {p.name} (FAILED) ━━━\n{p.error}")

    parts.append(f"\n━━━ Council Synthesis ━━━\n{result.synthesis.content}")

    n = len(result.succeeded)
    parts.append(f"\n[Models: {n}/{len(result.proposals)} | Mode: MoA | Time: {result.total_ms / 1000:.1f}s]")
    if result.agreement_score is not None:
        parts.append(f"Agreement: {result.agreement_score}%")
        if result.agreement_reason:
            parts.append(f"Reason: {result.agreement_reason}")
    return "\n\n".join(parts)


def _format_debate_result(result, verbose: bool = True) -> str:
    parts = []
    if verbose:
        for i, rnd in enumerate(result.rounds, 1):
            parts.append(f"── Round {i} ──")
            for r in rnd:
                if r.succeeded:
                    parts.append(f"{r.name} ({r.latency_ms}ms):\n{r.content}")
                else:
                    parts.append(f"{r.name} (FAILED): {r.error}")

    parts.append(f"\n━━━ Council Verdict ━━━\n{result.synthesis.content}")
    parts.append(f"\n[Rounds: {result.num_rounds} | Mode: Debate | Time: {result.total_ms / 1000:.1f}s]")
    if result.agreement_score is not None:
        parts.append(f"Agreement: {result.agreement_score}%")
        if result.agreement_reason:
            parts.append(f"Reason: {result.agreement_reason}")
    return "\n\n".join(parts)


def _format_redteam_result(result, verbose: bool = True) -> str:
    parts = []
    if verbose:
        for p in result.proposals:
            if p.succeeded:
                parts.append(f"━━━ {p.name} — Proposal ({p.latency_ms}ms) ━━━\n{p.content}")
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
    parts.append(f"\n[Mode: Red Team | Time: {result.total_ms / 1000:.1f}s]")
    if result.agreement_score is not None:
        parts.append(f"Agreement: {result.agreement_score}%")
    return "\n\n".join(parts)


@mcp.tool()
async def council_ask(
    prompt: str, mode: str = "moa", verbose: bool = True, rounds: int = 3
) -> str:
    """Ask multiple AI models a question and get a synthesized answer.

    Queries up to 6 models (Claude Opus, GPT-4.1, o3, Gemini Pro, DeepSeek R1, Grok 3)
    in parallel and synthesizes their best answer.

    Args:
        prompt: The question or prompt to send to all models.
        mode: "moa" (fast synthesis), "debate" (multi-round revision), or "redteam" (adversarial critique).
        verbose: If True, include each model's individual response.
        rounds: Max debate rounds (debate mode only, default 3).
    """
    if mode == "debate":
        result = await run_debate(prompt, max_rounds=rounds)
        return _format_debate_result(result, verbose)
    elif mode == "redteam":
        result = await run_redteam(prompt)
        return _format_redteam_result(result, verbose)
    else:
        result = await run_moa(prompt)
        return _format_moa_result(result, verbose)


@mcp.tool()
async def council_review(file_path: str) -> str:
    """Code review a file using multiple AI models.

    Each model independently reviews for bugs, performance issues, edge cases,
    and improvements. The aggregator synthesizes the best review.

    Args:
        file_path: Absolute path to the file to review.
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
    result = await run_moa(prompt)
    return _format_moa_result(result)


@mcp.tool()
async def council_debug(error: str) -> str:
    """Debug an error using multiple AI models.

    Each model independently analyzes the error and suggests root cause,
    fixes, and prevention strategies. The aggregator synthesizes the best diagnosis.

    Args:
        error: The error message, stack trace, or description of the issue.
    """
    prompt = (
        f"Debug this error. Identify:\n"
        f"1. The most likely root cause\n"
        f"2. What to check first\n"
        f"3. How to fix it\n"
        f"4. How to prevent it in the future\n\n"
        f"--- Error ---\n{error}"
    )
    result = await run_moa(prompt)
    return _format_moa_result(result)


@mcp.tool()
async def council_research(topic: str, rounds: int = 2) -> str:
    """Research a topic using multiple AI models in debate mode.

    Models discuss over multiple rounds, refining their answers after seeing
    each other's responses. Produces a thorough, well-rounded analysis.

    Args:
        topic: The topic to research.
        rounds: Number of debate rounds (default 2).
    """
    prompt = (
        f"Research this topic thoroughly. Cover:\n"
        f"1. Current state and best practices\n"
        f"2. Key trade-offs and considerations\n"
        f"3. Practical recommendations with reasoning\n"
        f"4. Common pitfalls to avoid\n\n"
        f"Topic: {topic}"
    )
    result = await run_debate(prompt, max_rounds=rounds)
    return _format_debate_result(result)


@mcp.tool()
async def council_models() -> str:
    """Show the configured AI models and their status."""
    import os

    lines = []
    lines.append("Proposers:")
    for m in get_all_proposers():
        status = "✓" if m.available else "✗"
        reasoning = " (reasoning)" if m.is_reasoning else ""
        lines.append(f"  {status} {m.name} ({m.model}){reasoning}")

    active = get_proposers()
    lines.append(f"\nActive: {len(active)} models with API keys configured")

    agg = get_aggregator()
    lines.append(f"\nAggregator: {agg.name} ({agg.model})")

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
