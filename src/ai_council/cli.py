"""AI Council CLI — Personal Edition.

Ask multiple AI models and synthesize their best answer.
Supports tiered routing (fast/balanced/full) and per-query cost tracking.
"""

from __future__ import annotations

import asyncio
import sys

import click
from rich.console import Console
from rich.markup import escape

from ai_council.config import DEFAULT_TIER, VALID_TIERS, get_aggregator, get_all_proposers, get_proposers

console = Console()

# ─── Shared CLI options ──────────────────────────────────────

def _tier_option(default: str = DEFAULT_TIER):
    """Create a tier option with a specific default."""
    return click.option(
        "--tier", "-t",
        type=click.Choice(VALID_TIERS),
        default=default,
        help=f"Model tier: fast (~10x cheaper), balanced (mid), full (premium). Default: {default}.",
    )


def _score_color(score: int | None) -> str:
    """Color-code agreement score."""
    if score is None:
        return "[dim]N/A[/dim]"
    if score >= 70:
        return f"[green]{score}%[/green]"
    if score >= 40:
        return f"[yellow]{score}%[/yellow]"
    return f"[red]{score}%[/red]"


def _cost_str(cost: float) -> str:
    """Format cost for display."""
    if cost < 0.01:
        return f"[dim]${cost:.4f}[/dim]"
    return f"[dim]${cost:.2f}[/dim]"


def _build_model_details(result) -> list[dict]:
    """Extract per-model detail dicts from a strategy result."""
    responses = []
    if hasattr(result, "proposals"):
        responses.extend(result.proposals)
    if hasattr(result, "rounds"):
        for rnd in result.rounds:
            responses.extend(rnd)
    if hasattr(result, "defenses"):
        responses.extend(result.defenses)
    if hasattr(result, "initial_attack"):
        responses.append(result.initial_attack)
    if hasattr(result, "targeted_attack"):
        responses.append(result.targeted_attack)
    if hasattr(result, "synthesis"):
        responses.append(result.synthesis)

    details = []
    for r in responses:
        if not r.model:
            continue
        details.append({
            "name": r.name,
            "model": r.model,
            "cost_usd": r.cost_usd,
            "input_tokens": r.input_tokens,
            "output_tokens": r.output_tokens,
            "latency_ms": r.latency_ms,
            "succeeded": r.succeeded,
        })
    return details


def _log_cost(result, mode: str, prompt: str) -> None:
    """Log query cost to persistent tracker."""
    from ai_council.cost_tracker import log_query
    models = [p.name for p in result.proposals if hasattr(result, "proposals")]
    succeeded = len(result.succeeded) if hasattr(result, "succeeded") else 0
    log_query(
        mode=mode,
        tier=getattr(result, "tier", DEFAULT_TIER),
        models_used=models,
        models_succeeded=succeeded,
        total_cost_usd=result.total_cost_usd,
        total_ms=result.total_ms,
        prompt_preview=prompt,
        model_details=_build_model_details(result),
    )


def _format_tokens(n: int) -> str:
    """Format token count: 1500 -> '1.5k', 800 -> '800'."""
    if n >= 1000:
        return f"{n / 1000:.1f}k"
    return str(n)


def _read_stdin() -> str | None:
    """Read from stdin if piped."""
    if not sys.stdin.isatty():
        return sys.stdin.read().strip()
    return None


@click.group()
def cli():
    """AI Council — Multi-LLM Orchestrator (Personal Edition)"""


@cli.command()
@click.argument("prompt", required=False)
@click.option("--mode", "-m", type=click.Choice(["moa", "debate", "redteam"]), default="moa")
@click.option("--verbose", "-v", is_flag=True, help="Show individual model responses.")
@click.option("--rounds", "-r", default=3, help="Max debate rounds.")
@click.option("--file", "-f", "file_path", type=click.Path(exists=True), help="Attach a file.")
@_tier_option("balanced")
def ask(prompt: str | None, mode: str, verbose: bool, rounds: int, file_path: str | None, tier: str):
    """Ask the council a question."""
    parts = []
    if prompt:
        parts.append(prompt)
    if file_path:
        from pathlib import Path
        content = Path(file_path).read_text()
        parts.append(f"\n--- {Path(file_path).name} ---\n{content}")
    stdin_data = _read_stdin()
    if stdin_data:
        parts.append(stdin_data)

    if not parts:
        console.print("[red]Error: provide a prompt, --file, or pipe via stdin.[/red]")
        raise SystemExit(1)

    full_prompt = "\n\n".join(parts)
    available = get_proposers(tier)
    agg = get_aggregator(tier)
    console.print(f"[dim]Mode: {mode} | Tier: {tier} | Models: {len(available)} | Aggregator: {agg.name}[/dim]\n")

    if mode == "moa":
        asyncio.run(_run_moa(full_prompt, verbose, tier))
    elif mode == "debate":
        asyncio.run(_run_debate(full_prompt, verbose, rounds, tier))
    else:
        asyncio.run(_run_redteam(full_prompt, verbose, tier))


@cli.command()
@click.argument("file_path", type=click.Path(exists=True))
@_tier_option("full")
def review(file_path: str, tier: str):
    """Code review a file using the council."""
    from pathlib import Path
    code = Path(file_path).read_text()
    prompt = (
        f"Review this code for bugs, performance issues, edge cases, and improvements. "
        f"Be specific — reference line numbers where possible.\n\n"
        f"--- {Path(file_path).name} ---\n{code}"
    )
    available = get_proposers(tier)
    console.print(f"[dim]Reviewing {file_path} | Tier: {tier} | Models: {len(available)}[/dim]\n")
    asyncio.run(_run_moa(prompt, verbose=True, tier=tier))


@cli.command()
@click.argument("error", required=False)
@_tier_option("balanced")
def debug(error: str | None, tier: str):
    """Debug an error using the council."""
    parts = []
    if error:
        parts.append(error)
    stdin_data = _read_stdin()
    if stdin_data:
        parts.append(stdin_data)

    if not parts:
        console.print("[red]Error: provide an error message or pipe via stdin.[/red]")
        raise SystemExit(1)

    error_text = "\n\n".join(parts)
    prompt = (
        f"Debug this error. Identify:\n"
        f"1. The most likely root cause\n"
        f"2. What to check first\n"
        f"3. How to fix it\n"
        f"4. How to prevent it in the future\n\n"
        f"--- Error ---\n{error_text}"
    )
    console.print(f"[dim]Debugging | Tier: {tier}[/dim]\n")
    asyncio.run(_run_moa(prompt, verbose=True, tier=tier))


@cli.command()
@click.argument("topic")
@click.option("--rounds", "-r", default=2, help="Number of debate rounds.")
@_tier_option("full")
def research(topic: str, rounds: int, tier: str):
    """Research a topic using debate mode."""
    prompt = (
        f"Research this topic thoroughly. Cover:\n"
        f"1. Current state and best practices\n"
        f"2. Key trade-offs and considerations\n"
        f"3. Practical recommendations with reasoning\n"
        f"4. Common pitfalls to avoid\n\n"
        f"Topic: {topic}"
    )
    available = get_proposers(tier)
    console.print(f"[dim]Researching (debate, {rounds} rounds) | Tier: {tier} | Models: {len(available)}[/dim]\n")
    asyncio.run(_run_debate(prompt, verbose=True, max_rounds=rounds, tier=tier))


@cli.command()
@_tier_option()
def models(tier: str):
    """Show configured models and API key status."""
    import os

    console.print(f"[bold]Tier: {tier}[/bold]\n")

    console.print("[bold]Proposers:[/bold]")
    for m in get_all_proposers(tier):
        status = "[green]✓[/green]" if m.available else "[red]✗[/red]"
        reasoning = " [dim](reasoning)[/dim]" if m.is_reasoning else ""
        console.print(f"  {status} {m.name} ({m.model}){reasoning}")

    agg = get_aggregator(tier)
    status = "[green]✓[/green]" if agg.available else "[red]✗[/red]"
    console.print(f"\n[bold]Aggregator:[/bold]\n  {status} {agg.name} ({agg.model})")

    console.print("\n[bold]API Keys:[/bold]")
    for label, env in [
        ("Anthropic", "ANTHROPIC_API_KEY"),
        ("OpenAI", "OPENAI_API_KEY"),
        ("Google", "GEMINI_API_KEY"),
        ("DeepSeek", "DEEPSEEK_API_KEY"),
        ("xAI", "XAI_API_KEY"),
    ]:
        key = os.environ.get(env, "")
        s = "[green]✓[/green]" if key else "[red]✗[/red]"
        masked = f"{key[:8]}...{key[-4:]}" if len(key) > 12 else "(not set)"
        console.print(f"  {s} {label} (${env}): {masked}")

    active = get_proposers(tier)
    console.print(f"\n[bold]All tiers:[/bold]")
    for t in VALID_TIERS:
        n = len(get_proposers(t))
        marker = " [bold cyan]<--[/bold cyan]" if t == tier else ""
        console.print(f"  {t}: {n} models available{marker}")


@cli.command()
def costs():
    """Show spending summary."""
    from ai_council.cost_tracker import get_cost_summary, get_cost_by_tier, get_cost_by_mode, get_cost_by_model

    summary = get_cost_summary()
    by_tier = get_cost_by_tier()
    by_mode = get_cost_by_mode()
    by_model = get_cost_by_model()

    console.print("[bold]Spending Summary[/bold]\n")
    console.print(f"  Today:      ${summary['today']:.4f}  ({summary['queries_today']} queries)")
    console.print(f"  This week:  ${summary['week']:.4f}")
    console.print(f"  This month: ${summary['month']:.4f}")
    console.print(f"  All time:   ${summary['all_time']:.4f}  ({summary['query_count']} queries)")

    if by_tier:
        console.print("\n[bold]By Tier[/bold]\n")
        for tier, cost in by_tier.items():
            console.print(f"  {tier}: ${cost:.4f}")

    if by_mode:
        console.print("\n[bold]By Mode[/bold]\n")
        for mode, cost in by_mode.items():
            console.print(f"  {mode}: ${cost:.4f}")

    if by_model:
        console.print("\n[bold]By Model[/bold]\n")
        sorted_models = sorted(by_model.items(), key=lambda x: x[1]["cost"], reverse=True)
        for name, stats in sorted_models:
            tokens_in = _format_tokens(stats["tokens_in"])
            tokens_out = _format_tokens(stats["tokens_out"])
            console.print(f"  {name:<22} ${stats['cost']:.4f}  ({stats['calls']} calls, {tokens_in} in / {tokens_out} out)")

    console.print(f"\n[dim]Log: ~/.ai-council/costs.jsonl[/dim]")


# ─── Strategy runners ────────────────────────────────────────


async def _run_moa(prompt: str, verbose: bool, tier: str = DEFAULT_TIER):
    from ai_council.strategies.moa import run_moa

    with console.status("[bold cyan]Consulting the council..."):
        result = await run_moa(prompt, tier=tier)

    if verbose:
        for p in result.proposals:
            if p.succeeded:
                console.print(f"\n[bold yellow]━━━ {p.name} ━━━[/bold yellow] [dim]({p.latency_ms}ms)[/dim]")
                console.print(escape(p.content))
            else:
                console.print(f"\n[bold red]━━━ {p.name} (FAILED) ━━━[/bold red]")
                console.print(f"[red]{escape(str(p.error))}[/red]")
        console.print()

    console.print("[bold green]━━━ Council Synthesis ━━━[/bold green]")
    if result.synthesis.succeeded:
        console.print(escape(result.synthesis.content))
    else:
        console.print(f"[red]Synthesis failed: {escape(str(result.synthesis.error))}[/red]")

    n = len(result.succeeded)
    score = _score_color(result.agreement_score)
    cost = _cost_str(result.total_cost_usd)
    console.print(f"\n[dim]Models: {n}/{len(result.proposals)} | Mode: MoA | Tier: {tier} | Time: {result.total_ms / 1000:.1f}s | Agreement: [/dim]{score}[dim] | Cost: [/dim]{cost}")

    if result.agreement_score is not None and result.agreement_score < 50:
        console.print("[dim italic]Tip: Low agreement — try --mode debate for deeper analysis.[/dim italic]")

    _log_cost(result, "moa", prompt)


async def _run_debate(prompt: str, verbose: bool, max_rounds: int, tier: str = DEFAULT_TIER):
    from ai_council.strategies.debate import run_debate

    with console.status("[bold cyan]Debate in progress..."):
        result = await run_debate(prompt, max_rounds=max_rounds, tier=tier)

    if verbose:
        for i, rnd in enumerate(result.rounds, 1):
            console.print(f"\n[bold magenta]── Round {i} ──[/bold magenta]")
            for r in rnd:
                if r.succeeded:
                    console.print(f"\n[bold yellow]━━━ {r.name} ━━━[/bold yellow] [dim]({r.latency_ms}ms)[/dim]")
                    console.print(escape(r.content))
                else:
                    console.print(f"\n[bold red]━━━ {r.name} (FAILED) ━━━[/bold red]")
                    console.print(f"[red]{escape(str(r.error))}[/red]")
        console.print()

    console.print("[bold green]━━━ Council Verdict ━━━[/bold green]")
    if result.synthesis.succeeded:
        console.print(escape(result.synthesis.content))
    else:
        console.print(f"[red]Synthesis failed: {escape(str(result.synthesis.error))}[/red]")

    score = _score_color(result.agreement_score)
    cost = _cost_str(result.total_cost_usd)
    console.print(f"\n[dim]Rounds: {result.num_rounds} | Mode: Debate | Tier: {tier} | Time: {result.total_ms / 1000:.1f}s | Agreement: [/dim]{score}[dim] | Cost: [/dim]{cost}")

    _log_cost(result, "debate", prompt)


async def _run_redteam(prompt: str, verbose: bool, tier: str = DEFAULT_TIER):
    from ai_council.strategies.redteam import run_redteam

    with console.status("[bold cyan]Red team in progress..."):
        result = await run_redteam(prompt, tier=tier)

    if verbose:
        for p in result.proposals:
            if p.succeeded:
                console.print(f"\n[bold yellow]━━━ {p.name} — Proposal ━━━[/bold yellow] [dim]({p.latency_ms}ms)[/dim]")
                console.print(escape(p.content))
            else:
                console.print(f"\n[bold red]━━━ {p.name} — Proposal (FAILED) ━━━[/bold red]")

        if result.initial_attack.succeeded:
            console.print(f"\n[bold red]━━━ Attacker — Critique ━━━[/bold red] [dim]({result.initial_attack.latency_ms}ms)[/dim]")
            console.print(escape(result.initial_attack.content))

        for d in result.defenses:
            if d.succeeded:
                console.print(f"\n[bold blue]━━━ {d.name} — Defense ━━━[/bold blue] [dim]({d.latency_ms}ms)[/dim]")
                console.print(escape(d.content))

        if result.targeted_attack.succeeded:
            console.print(f"\n[bold red]━━━ Attacker — Targeted Critique ━━━[/bold red] [dim]({result.targeted_attack.latency_ms}ms)[/dim]")
            console.print(escape(result.targeted_attack.content))

        console.print()

    console.print("[bold green]━━━ Hardened Answer ━━━[/bold green]")
    if result.synthesis.succeeded:
        console.print(escape(result.synthesis.content))
    else:
        console.print(f"[red]Synthesis failed: {escape(str(result.synthesis.error))}[/red]")

    score = _score_color(result.agreement_score)
    cost = _cost_str(result.total_cost_usd)
    console.print(f"\n[dim]Mode: Red Team | Tier: {tier} | Time: {result.total_ms / 1000:.1f}s | Agreement: [/dim]{score}[dim] | Cost: [/dim]{cost}")

    _log_cost(result, "redteam", prompt)


if __name__ == "__main__":
    cli()
