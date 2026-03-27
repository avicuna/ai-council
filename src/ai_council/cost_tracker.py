"""Persistent cost tracking — append-only JSONL log.

Logs every query to ~/.ai-council/costs.jsonl for budget monitoring.
"""

from __future__ import annotations

import json
import os
from datetime import datetime, timezone
from pathlib import Path

COST_DIR = Path.home() / ".ai-council"
COST_FILE = COST_DIR / "costs.jsonl"


def log_query(
    mode: str,
    tier: str,
    models_used: list[str],
    models_succeeded: int,
    total_cost_usd: float,
    total_ms: int,
    prompt_preview: str = "",
    model_details: list[dict] | None = None,
) -> None:
    """Append a cost entry to the log file."""
    COST_DIR.mkdir(parents=True, exist_ok=True)

    entry = {
        "ts": datetime.now(timezone.utc).isoformat(),
        "mode": mode,
        "tier": tier,
        "models": models_used,
        "succeeded": models_succeeded,
        "cost_usd": round(total_cost_usd, 6),
        "latency_ms": total_ms,
        "prompt": prompt_preview[:100],  # First 100 chars for context
    }

    if model_details:
        entry["models_detail"] = model_details

    with open(COST_FILE, "a") as f:
        f.write(json.dumps(entry) + "\n")


def _load_entries() -> list[dict]:
    """Load all cost entries."""
    if not COST_FILE.exists():
        return []
    entries = []
    for line in COST_FILE.read_text().splitlines():
        line = line.strip()
        if line:
            try:
                entries.append(json.loads(line))
            except json.JSONDecodeError:
                continue
    return entries


def get_cost_summary() -> dict:
    """Return spending summary: today, this week, this month, all time."""
    entries = _load_entries()
    if not entries:
        return {
            "today": 0.0,
            "week": 0.0,
            "month": 0.0,
            "all_time": 0.0,
            "query_count": 0,
            "queries_today": 0,
        }

    now = datetime.now(timezone.utc)
    today_start = now.replace(hour=0, minute=0, second=0, microsecond=0)
    week_ago = now.timestamp() - 7 * 86400
    month_start = now.replace(day=1, hour=0, minute=0, second=0, microsecond=0)

    today_cost = 0.0
    week_cost = 0.0
    month_cost = 0.0
    all_cost = 0.0
    queries_today = 0

    for e in entries:
        cost = e.get("cost_usd", 0.0)

        try:
            ts = datetime.fromisoformat(e["ts"])
        except (KeyError, ValueError):
            all_cost += cost  # Count cost even if timestamp is bad
            continue

        all_cost += cost
        if ts >= today_start:
            today_cost += cost
            queries_today += 1
        if ts.timestamp() >= week_ago:
            week_cost += cost
        if ts >= month_start:
            month_cost += cost

    return {
        "today": round(today_cost, 4),
        "week": round(week_cost, 4),
        "month": round(month_cost, 4),
        "all_time": round(all_cost, 4),
        "query_count": len(entries),
        "queries_today": queries_today,
    }


def get_cost_by_tier() -> dict[str, float]:
    """Return total cost broken down by tier."""
    entries = _load_entries()
    by_tier: dict[str, float] = {}
    for e in entries:
        tier = e.get("tier", "full")
        by_tier[tier] = by_tier.get(tier, 0.0) + e.get("cost_usd", 0.0)
    return {k: round(v, 4) for k, v in sorted(by_tier.items())}


def get_cost_by_mode() -> dict[str, float]:
    """Return total cost broken down by mode (moa, debate, redteam, etc.)."""
    entries = _load_entries()
    by_mode: dict[str, float] = {}
    for e in entries:
        mode = e.get("mode", "unknown")
        by_mode[mode] = by_mode.get(mode, 0.0) + e.get("cost_usd", 0.0)
    return {k: round(v, 4) for k, v in sorted(by_mode.items())}


def get_cost_by_model() -> dict[str, dict]:
    """Return per-model aggregated stats: cost, calls, tokens in/out.

    Only counts entries that have models_detail (new format).
    """
    entries = _load_entries()
    by_model: dict[str, dict] = {}

    for e in entries:
        for m in e.get("models_detail", []):
            name = m.get("name", "unknown")
            if name not in by_model:
                by_model[name] = {"cost": 0.0, "calls": 0, "tokens_in": 0, "tokens_out": 0}
            by_model[name]["cost"] = round(by_model[name]["cost"] + m.get("cost_usd", 0.0), 4)
            by_model[name]["calls"] += 1
            by_model[name]["tokens_in"] += m.get("input_tokens", 0)
            by_model[name]["tokens_out"] += m.get("output_tokens", 0)

    return by_model


def get_usage_summary() -> dict:
    """Return aggregate usage stats: total tokens, averages."""
    entries = _load_entries()
    if not entries:
        return {
            "total_tokens_in": 0,
            "total_tokens_out": 0,
            "total_queries": 0,
            "avg_cost_per_query": 0.0,
            "avg_latency_ms": 0,
        }

    total_in = 0
    total_out = 0
    total_cost = 0.0
    total_latency = 0

    for e in entries:
        total_cost += e.get("cost_usd", 0.0)
        total_latency += e.get("latency_ms", 0)
        for m in e.get("models_detail", []):
            total_in += m.get("input_tokens", 0)
            total_out += m.get("output_tokens", 0)

    n = len(entries)
    return {
        "total_tokens_in": total_in,
        "total_tokens_out": total_out,
        "total_queries": n,
        "avg_cost_per_query": round(total_cost / n, 4),
        "avg_latency_ms": total_latency // n,
    }
