"""AI Council CLI — Personal Edition.

Ask multiple AI models and synthesize their best answer.
"""

from __future__ import annotations

import asyncio
import sys

import click
from rich.console import Console
from rich.markup import escape

from ai_council.config import get_aggregator, get_all_proposers, get_proposers

console = Console()


def _score_color(score: int | None) -> str:
    """Color-code agreement score."""
    if score is None:
        return "[dim]N/A[/dim]"
    if score >= 70:
        return f"[green]{score}%[/green]"
    if score >= 40:
        return f"[yellow]{score}%[/yellow]"
    return f"[red]{score}%[/red]"


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
def ask(prompt: str | None, mode: str, verbose: bool, rounds: int, file_path: str | None):
    """Ask the council a question."""
    # Build prompt from args + file + stdin
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
    available = get_proposers()
    agg = get_aggregator()
    console.print(f"[dim]Mode: {mode} | Models: {len(available)} | Aggregator: {agg.name}[/dim]\n")

    if mode == "moa":
        asyncio.run(_run_moa(full_prompt, verbose))
    elif mode == "debate":
        asyncio.run(_run_debate(full_prompt, verbose, rounds))
    else:
        asyncio.run(_run_redteam(full_prompt, verbose))


@cli.command()
@click.argument("file_path", type=click.Path(exists=True))
def review(file_path: str):
    """Code review a file using the full council."""
    from pathlib import Path
    code = Path(file_path).read_text()
    prompt = (
        f"Review this code for bugs, performance issues, edge cases, and improvements. "
        f"Be specific — reference line numbers where possible.\n\n"
        f"--- {Path(file_path).name} ---\n{code}"
    )
    console.print(f"[dim]Reviewing {file_path}...[/dim]\n")
    asyncio.run(_run_moa(prompt, verbose=True))


@cli.command()
@click.argument("error", required=False)
def debug(error: str | None):
    """Debug an error using the full council."""
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
    console.print("[dim]Debugging...[/dim]\n")
    asyncio.run(_run_moa(prompt, verbose=True))


@cli.command()
@click.argument("topic")
@click.option("--rounds", "-r", default=2, help="Number of debate rounds.")
def research(topic: str, rounds: int):
    """Research a topic using debate mode."""
    prompt = (
        f"Research this topic thoroughly. Cover:\n"
        f"1. Current state and best practices\n"
        f"2. Key trade-offs and considerations\n"
        f"3. Practical recommendations with reasoning\n"
        f"4. Common pitfalls to avoid\n\n"
        f"Topic: {topic}"
    )
    console.print(f"[dim]Researching (debate, {rounds} rounds)...[/dim]\n")
    asyncio.run(_run_debate(prompt, verbose=True, max_rounds=rounds))


@cli.command()
def models():
    """Show configured models and API key status."""
    import os

    console.print("[bold]Proposers:[/bold]")
    for m in get_all_proposers():
        status = "[green]✓[/green]" if m.available else "[red]✗[/red]"
        reasoning = " [dim](reasoning)[/dim]" if m.is_reasoning else ""
        console.print(f"  {status} {m.name} ({m.model}){reasoning}")

    agg = get_aggregator()
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

    active = get_proposers()
    console.print(f"\n[dim]Active models: {len(active)} | Override via COUNCIL_*_MODEL env vars[/dim]")


# ─── Strategy runners ────────────────────────────────────────


async def _run_moa(prompt: str, verbose: bool):
    from ai_council.strategies.moa import run_moa

    with console.status("[bold cyan]Consulting the council..."):
        result = await run_moa(prompt)

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
    console.print(f"\n[dim]Models: {n}/{len(result.proposals)} | Mode: MoA | Time: {result.total_ms / 1000:.1f}s | Agreement: [/dim]{score}")

    if result.agreement_score is not None and result.agreement_score < 50:
        console.print("[dim italic]Tip: Low agreement — try --mode debate for deeper analysis.[/dim italic]")


async def _run_debate(prompt: str, verbose: bool, max_rounds: int):
    from ai_council.strategies.debate import run_debate

    with console.status("[bold cyan]Debate in progress..."):
        result = await run_debate(prompt, max_rounds=max_rounds)

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
    console.print(f"\n[dim]Rounds: {result.num_rounds} | Mode: Debate | Time: {result.total_ms / 1000:.1f}s | Agreement: [/dim]{score}")


async def _run_redteam(prompt: str, verbose: bool):
    from ai_council.strategies.redteam import run_redteam

    with console.status("[bold cyan]Red team in progress..."):
        result = await run_redteam(prompt)

    if verbose:
        # Initial proposals
        for p in result.proposals:
            if p.succeeded:
                console.print(f"\n[bold yellow]━━━ {p.name} — Proposal ━━━[/bold yellow] [dim]({p.latency_ms}ms)[/dim]")
                console.print(escape(p.content))
            else:
                console.print(f"\n[bold red]━━━ {p.name} — Proposal (FAILED) ━━━[/bold red]")

        # Attack
        if result.initial_attack.succeeded:
            console.print(f"\n[bold red]━━━ Attacker — Critique ━━━[/bold red] [dim]({result.initial_attack.latency_ms}ms)[/dim]")
            console.print(escape(result.initial_attack.content))

        # Defenses
        for d in result.defenses:
            if d.succeeded:
                console.print(f"\n[bold blue]━━━ {d.name} — Defense ━━━[/bold blue] [dim]({d.latency_ms}ms)[/dim]")
                console.print(escape(d.content))

        # Targeted attack
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
    console.print(f"\n[dim]Mode: Red Team | Time: {result.total_ms / 1000:.1f}s | Agreement: [/dim]{score}")


if __name__ == "__main__":
    cli()
