"""AI Council CLI — ask multiple AI models and synthesize their best answer."""

from __future__ import annotations

import asyncio

import click
from rich.console import Console

from ai_council.config import get_aggregator, get_proposers

console = Console()


@click.group()
def cli():
    """AI Council — Multi-LLM Orchestrator"""


@cli.command()
@click.argument("prompt")
@click.option("--mode", "-m", type=click.Choice(["moa", "debate"]), default="moa")
@click.option("--verbose", "-v", is_flag=True, help="Show individual model responses.")
@click.option("--rounds", "-r", default=3, help="Max debate rounds.")
def ask(prompt: str, mode: str, verbose: bool, rounds: int):
    """Ask the council a question."""
    agg = get_aggregator()
    console.print(f"[dim]Mode: {mode} | Aggregator: {agg.name}[/dim]\n")

    if mode == "moa":
        asyncio.run(_moa(prompt, verbose))
    else:
        asyncio.run(_debate(prompt, verbose, rounds))


async def _moa(prompt: str, verbose: bool):
    from ai_council.strategies.moa import run_moa

    with console.status("[bold cyan]Consulting the council..."):
        result = await run_moa(prompt)

    if verbose:
        for p in result.proposals:
            if p.succeeded:
                console.print(f"\n[bold yellow]━━━ {p.name} ━━━[/bold yellow] [dim]({p.latency_ms}ms)[/dim]")
                console.print(p.content)
            else:
                console.print(f"\n[bold red]━━━ {p.name} (FAILED) ━━━[/bold red]")
                console.print(f"[red]{p.error}[/red]")
        console.print()

    console.print("[bold green]━━━ Council Synthesis ━━━[/bold green]")
    if result.synthesis.succeeded:
        console.print(result.synthesis.content)
    else:
        console.print(f"[red]Synthesis failed: {result.synthesis.error}[/red]")

    n = len(result.succeeded)
    console.print(f"\n[dim]Models: {n}/{len(result.proposals)} | Mode: MoA | Time: {result.total_ms / 1000:.1f}s[/dim]")


async def _debate(prompt: str, verbose: bool, max_rounds: int):
    from ai_council.strategies.debate import run_debate

    with console.status("[bold cyan]Debate in progress..."):
        result = await run_debate(prompt, max_rounds=max_rounds)

    if verbose:
        for i, rnd in enumerate(result.rounds, 1):
            console.print(f"\n[bold magenta]── Round {i} ──[/bold magenta]")
            for r in rnd:
                if r.succeeded:
                    console.print(f"\n[bold yellow]━━━ {r.name} ━━━[/bold yellow] [dim]({r.latency_ms}ms)[/dim]")
                    console.print(r.content)
                else:
                    console.print(f"\n[bold red]━━━ {r.name} (FAILED) ━━━[/bold red]")
                    console.print(f"[red]{r.error}[/red]")
        console.print()

    console.print("[bold green]━━━ Council Verdict ━━━[/bold green]")
    if result.synthesis.succeeded:
        console.print(result.synthesis.content)
    else:
        console.print(f"[red]Synthesis failed: {result.synthesis.error}[/red]")

    console.print(f"\n[dim]Rounds: {result.num_rounds} | Mode: Debate | Time: {result.total_ms / 1000:.1f}s[/dim]")


@cli.command()
def models():
    """Show configured models and API key status."""
    import os

    console.print("[bold]Proposers:[/bold]")
    for m in get_proposers():
        status = "[green]✓[/green]" if m.available else "[red]✗[/red]"
        console.print(f"  {status} {m.name} ({m.model})")

    agg = get_aggregator()
    status = "[green]✓[/green]" if agg.available else "[red]✗[/red]"
    console.print("\n[bold]Aggregator:[/bold]")
    console.print(f"  {status} {agg.name} ({agg.model})")

    console.print("\n[bold]API Keys:[/bold]")
    for label, env in [("Anthropic", "ANTHROPIC_API_KEY"), ("OpenAI", "OPENAI_API_KEY"), ("Gemini", "GEMINI_API_KEY")]:
        key = os.environ.get(env, "")
        s = "[green]✓[/green]" if key else "[red]✗[/red]"
        masked = f"{key[:8]}...{key[-4:]}" if len(key) > 12 else "(not set)"
        console.print(f"  {s} {label} (${env}): {masked}")

    console.print("\n[dim]Override models via env vars: COUNCIL_ANTHROPIC_MODEL, COUNCIL_OPENAI_MODEL, COUNCIL_GEMINI_MODEL, COUNCIL_AGGREGATOR_MODEL[/dim]")


if __name__ == "__main__":
    cli()
