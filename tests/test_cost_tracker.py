"""Tests for cost tracker — model details, new query functions."""

from __future__ import annotations

import json
from unittest.mock import patch

from ai_council.cost_tracker import (
    get_cost_by_model,
    get_cost_by_mode,
    get_usage_summary,
    log_query,
)


def test_log_query_with_model_details(tmp_cost_file):
    """log_query writes model_details to JSONL."""
    details = [
        {"name": "Claude", "model": "claude-sonnet-4", "cost_usd": 0.03, "input_tokens": 500, "output_tokens": 800, "latency_ms": 2000, "succeeded": True},
    ]
    with patch("ai_council.cost_tracker.COST_FILE", tmp_cost_file), \
         patch("ai_council.cost_tracker.COST_DIR", tmp_cost_file.parent):
        log_query(
            mode="moa", tier="balanced", models_used=["Claude"],
            models_succeeded=1, total_cost_usd=0.03, total_ms=2000,
            prompt_preview="test", model_details=details,
        )

    entry = json.loads(tmp_cost_file.read_text().strip())
    assert entry["models_detail"] == details


def test_log_query_without_model_details(tmp_cost_file):
    """log_query works without model_details (backwards compat)."""
    with patch("ai_council.cost_tracker.COST_FILE", tmp_cost_file), \
         patch("ai_council.cost_tracker.COST_DIR", tmp_cost_file.parent):
        log_query(
            mode="moa", tier="fast", models_used=["Claude"],
            models_succeeded=1, total_cost_usd=0.01, total_ms=1000,
            prompt_preview="test",
        )

    entry = json.loads(tmp_cost_file.read_text().strip())
    assert "models_detail" not in entry


def test_get_cost_by_model(populated_cost_file):
    """get_cost_by_model aggregates per-model stats."""
    with patch("ai_council.cost_tracker.COST_FILE", populated_cost_file):
        by_model = get_cost_by_model()

    # GPT-4.1 appears in both entries
    assert "GPT-4.1" in by_model
    gpt = by_model["GPT-4.1"]
    assert gpt["cost"] == round(0.02 + 0.04, 4)
    assert gpt["calls"] == 2
    assert gpt["tokens_in"] == 500 + 1000
    assert gpt["tokens_out"] == 600 + 1500

    # Claude Opus 4 appears once
    assert "Claude Opus 4" in by_model
    opus = by_model["Claude Opus 4"]
    assert opus["calls"] == 1

    # Claude Sonnet 4 appears once
    assert "Claude Sonnet 4" in by_model


def test_get_cost_by_model_empty(tmp_cost_file):
    """get_cost_by_model returns empty dict when no data."""
    with patch("ai_council.cost_tracker.COST_FILE", tmp_cost_file):
        assert get_cost_by_model() == {}


def test_get_cost_by_mode(populated_cost_file):
    """get_cost_by_mode aggregates per-mode costs."""
    with patch("ai_council.cost_tracker.COST_FILE", populated_cost_file):
        by_mode = get_cost_by_mode()

    assert by_mode["moa"] == 0.05
    assert by_mode["debate"] == 0.15


def test_get_usage_summary(populated_cost_file):
    """get_usage_summary returns aggregate stats."""
    with patch("ai_council.cost_tracker.COST_FILE", populated_cost_file):
        summary = get_usage_summary()

    assert summary["total_tokens_in"] == 500 + 500 + 1000 + 1000 + 1000
    assert summary["total_tokens_out"] == 800 + 600 + 2000 + 1500 + 1800
    assert summary["total_queries"] == 2
    assert summary["avg_cost_per_query"] == round(0.20 / 2, 4)
    assert summary["avg_latency_ms"] == (3000 + 8000) // 2


def test_get_usage_summary_empty(tmp_cost_file):
    """get_usage_summary returns zeros when no data."""
    with patch("ai_council.cost_tracker.COST_FILE", tmp_cost_file):
        summary = get_usage_summary()

    assert summary["total_queries"] == 0
    assert summary["total_tokens_in"] == 0
    assert summary["avg_cost_per_query"] == 0.0
