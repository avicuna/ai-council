"""Shared fixtures for AI Council tests."""

from __future__ import annotations

import json
import os
from pathlib import Path

import pytest


@pytest.fixture
def tmp_cost_file(tmp_path: Path) -> Path:
    """Return a temporary cost file path and patch COST_FILE to use it."""
    return tmp_path / "costs.jsonl"


@pytest.fixture
def sample_entries() -> list[dict]:
    """Sample JSONL entries for cost tracker tests."""
    return [
        {
            "ts": "2026-03-26T10:00:00+00:00",
            "mode": "moa",
            "tier": "balanced",
            "models": ["Claude Sonnet 4", "GPT-4.1"],
            "succeeded": 2,
            "cost_usd": 0.05,
            "latency_ms": 3000,
            "prompt": "test prompt",
            "models_detail": [
                {"name": "Claude Sonnet 4", "model": "claude-sonnet-4-20250514", "cost_usd": 0.03, "input_tokens": 500, "output_tokens": 800, "latency_ms": 2000, "succeeded": True},
                {"name": "GPT-4.1", "model": "gpt-4.1", "cost_usd": 0.02, "input_tokens": 500, "output_tokens": 600, "latency_ms": 1500, "succeeded": True},
            ],
        },
        {
            "ts": "2026-03-26T11:00:00+00:00",
            "mode": "debate",
            "tier": "full",
            "models": ["Claude Opus 4", "GPT-4.1", "o3"],
            "succeeded": 3,
            "cost_usd": 0.15,
            "latency_ms": 8000,
            "prompt": "another prompt",
            "models_detail": [
                {"name": "Claude Opus 4", "model": "claude-opus-4-20250514", "cost_usd": 0.08, "input_tokens": 1000, "output_tokens": 2000, "latency_ms": 5000, "succeeded": True},
                {"name": "GPT-4.1", "model": "gpt-4.1", "cost_usd": 0.04, "input_tokens": 1000, "output_tokens": 1500, "latency_ms": 4000, "succeeded": True},
                {"name": "o3", "model": "o3", "cost_usd": 0.03, "input_tokens": 1000, "output_tokens": 1800, "latency_ms": 6000, "succeeded": True},
            ],
        },
    ]


@pytest.fixture
def populated_cost_file(tmp_cost_file: Path, sample_entries: list[dict]) -> Path:
    """Write sample entries to a temporary cost file."""
    with open(tmp_cost_file, "w") as f:
        for entry in sample_entries:
            f.write(json.dumps(entry) + "\n")
    return tmp_cost_file
