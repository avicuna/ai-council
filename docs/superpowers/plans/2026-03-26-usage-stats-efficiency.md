# Usage Stats & Efficiency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-model usage stats with token tracking, smart agreement scoring, per-tool tier defaults, and enriched `council_costs` output.

**Architecture:** Enrich the existing JSONL log with per-model detail rows. Add computed summary functions to `cost_tracker.py`. Make agreement scoring conditional based on tier and success count. Change default tiers at the tool level in `mcp_server.py` and `cli.py`.

**Tech Stack:** Python 3.11+, LiteLLM, pytest

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `pyproject.toml` | Modify | Add pytest dev dependency |
| `src/ai_council/providers.py` | Modify | Extract token counts from LiteLLM responses |
| `src/ai_council/cost_tracker.py` | Modify | Accept `model_details`, add `get_cost_by_model()` and `get_usage_summary()` |
| `src/ai_council/scoring.py` | Modify | Add `should_score()` helper |
| `src/ai_council/strategies/moa.py` | Modify | Use `should_score()`, track scorer cost, expose model details |
| `src/ai_council/strategies/debate.py` | Modify | Use `should_score()`, track scorer cost, expose model details |
| `src/ai_council/strategies/redteam.py` | Modify | Use `should_score()`, track scorer cost, expose model details |
| `src/ai_council/mcp_server.py` | Modify | New tier defaults, pass model details to logger, enrich `council_costs` |
| `src/ai_council/cli.py` | Modify | New tier defaults, pass model details to logger, enrich `costs` command |
| `tests/test_cost_tracker.py` | Create | Tests for new cost tracker functions |
| `tests/test_scoring.py` | Create | Tests for `should_score()` |
| `tests/test_providers.py` | Create | Tests for token extraction |

---

### Task 1: Set Up Test Infrastructure

**Files:**
- Modify: `pyproject.toml`
- Create: `tests/__init__.py`
- Create: `tests/conftest.py`

- [ ] **Step 1: Add pytest to pyproject.toml**

```toml
[project.optional-dependencies]
dev = ["pytest>=8.0", "pytest-asyncio>=0.23"]

[tool.pytest.ini_options]
asyncio_mode = "auto"
```

Add this after the existing `[tool.ruff]` section in `pyproject.toml`.

- [ ] **Step 2: Create test boilerplate**

Create `tests/__init__.py` (empty file).

Create `tests/conftest.py`:

```python
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
```

- [ ] **Step 3: Install dev dependencies**

Run: `cd /Users/avicuna/Dev/ai-council && pip install -e ".[dev]"`
Expected: Successful install including pytest and pytest-asyncio.

- [ ] **Step 4: Verify pytest runs**

Run: `cd /Users/avicuna/Dev/ai-council && python -m pytest tests/ -v --co`
Expected: "no tests ran" (no test files yet), but no import errors.

- [ ] **Step 5: Commit**

```bash
cd /Users/avicuna/Dev/ai-council
git add pyproject.toml tests/__init__.py tests/conftest.py
git commit -m "chore: add pytest test infrastructure"
```

---

### Task 2: Add Token Counts to ModelResponse

**Files:**
- Create: `tests/test_providers.py`
- Modify: `src/ai_council/providers.py:28-36` (ModelResponse dataclass)
- Modify: `src/ai_council/providers.py:87-127` (call_model function)

- [ ] **Step 1: Write the failing test**

Create `tests/test_providers.py`:

```python
"""Tests for provider layer — token extraction."""

from ai_council.providers import ModelResponse


def test_model_response_has_token_fields():
    """ModelResponse includes input/output token counts."""
    r = ModelResponse(
        model="test-model",
        name="Test",
        content="hello",
        latency_ms=100,
    )
    assert r.input_tokens == 0
    assert r.output_tokens == 0


def test_model_response_with_tokens():
    """ModelResponse stores token counts when provided."""
    r = ModelResponse(
        model="test-model",
        name="Test",
        content="hello",
        latency_ms=100,
        input_tokens=500,
        output_tokens=800,
    )
    assert r.input_tokens == 500
    assert r.output_tokens == 800
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/avicuna/Dev/ai-council && python -m pytest tests/test_providers.py -v`
Expected: FAIL — `ModelResponse.__init__() got an unexpected keyword argument 'input_tokens'`

- [ ] **Step 3: Add token fields to ModelResponse**

In `src/ai_council/providers.py`, modify the `ModelResponse` dataclass:

```python
@dataclass
class ModelResponse:
    model: str
    name: str
    content: str
    latency_ms: int
    error: str | None = None
    cost_usd: float = 0.0
    input_tokens: int = 0
    output_tokens: int = 0

    @property
    def succeeded(self) -> bool:
        return self.error is None
```

- [ ] **Step 4: Extract tokens from LiteLLM response**

In `src/ai_council/providers.py`, in the `call_model` function, after the cost estimation block (after `cost = completion_cost(...)` try/except), extract tokens from the response:

```python
        # Extract token usage
        input_tokens = 0
        output_tokens = 0
        if hasattr(response, "usage") and response.usage:
            input_tokens = getattr(response.usage, "prompt_tokens", 0) or 0
            output_tokens = getattr(response.usage, "completion_tokens", 0) or 0

        return ModelResponse(
            model=config.model,
            name=config.name,
            content=response.choices[0].message.content or "",
            latency_ms=elapsed,
            cost_usd=cost,
            input_tokens=input_tokens,
            output_tokens=output_tokens,
        )
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/avicuna/Dev/ai-council && python -m pytest tests/test_providers.py -v`
Expected: 2 passed.

- [ ] **Step 6: Commit**

```bash
cd /Users/avicuna/Dev/ai-council
git add src/ai_council/providers.py tests/test_providers.py
git commit -m "feat: add input/output token tracking to ModelResponse"
```

---

### Task 3: Add `should_score()` to scoring.py

**Files:**
- Create: `tests/test_scoring.py`
- Modify: `src/ai_council/scoring.py`

- [ ] **Step 1: Write the failing tests**

Create `tests/test_scoring.py`:

```python
"""Tests for agreement scoring logic."""

from ai_council.scoring import should_score


def test_should_score_balanced_3_models():
    """Score when balanced tier and 3+ succeeded."""
    assert should_score(succeeded_count=3, tier="balanced") is True


def test_should_score_full_4_models():
    """Score when full tier and 4+ succeeded."""
    assert should_score(succeeded_count=4, tier="full") is True


def test_should_not_score_fast_tier():
    """Never score on fast tier."""
    assert should_score(succeeded_count=4, tier="fast") is False


def test_should_not_score_2_models():
    """Don't score with only 2 models."""
    assert should_score(succeeded_count=2, tier="full") is False


def test_should_not_score_1_model():
    """Don't score with only 1 model."""
    assert should_score(succeeded_count=1, tier="balanced") is False


def test_should_not_score_0_models():
    """Don't score with no models."""
    assert should_score(succeeded_count=0, tier="full") is False
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/avicuna/Dev/ai-council && python -m pytest tests/test_scoring.py -v`
Expected: FAIL — `ImportError: cannot import name 'should_score'`

- [ ] **Step 3: Implement `should_score()`**

In `src/ai_council/scoring.py`, add above the existing `parse_agreement_score` function:

```python
def should_score(succeeded_count: int, tier: str) -> bool:
    """Decide whether agreement scoring is worth the API call.

    Runs when 3+ models succeeded and tier is balanced or full.
    Skips on fast tier (scorer cost is proportionally too high)
    and when fewer than 3 models succeeded (binary agreement is uninformative).
    """
    return succeeded_count >= 3 and tier in ("balanced", "full")
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/avicuna/Dev/ai-council && python -m pytest tests/test_scoring.py -v`
Expected: 6 passed.

- [ ] **Step 5: Commit**

```bash
cd /Users/avicuna/Dev/ai-council
git add src/ai_council/scoring.py tests/test_scoring.py
git commit -m "feat: add should_score() for smart agreement scoring"
```

---

### Task 4: Enrich `cost_tracker.py` with Model Details and New Queries

**Files:**
- Create: `tests/test_cost_tracker.py`
- Modify: `src/ai_council/cost_tracker.py`

- [ ] **Step 1: Write the failing tests**

Create `tests/test_cost_tracker.py`:

```python
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/avicuna/Dev/ai-council && python -m pytest tests/test_cost_tracker.py -v`
Expected: FAIL — `ImportError: cannot import name 'get_cost_by_model'` and `cannot import name 'get_usage_summary'`

- [ ] **Step 3: Update `log_query()` to accept model_details**

In `src/ai_council/cost_tracker.py`, modify `log_query`:

```python
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
        "prompt": prompt_preview[:100],
    }

    if model_details:
        entry["models_detail"] = model_details

    with open(COST_FILE, "a") as f:
        f.write(json.dumps(entry) + "\n")
```

- [ ] **Step 4: Add `get_cost_by_model()`**

In `src/ai_council/cost_tracker.py`, add after `get_cost_by_mode()`:

```python
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
```

- [ ] **Step 5: Add `get_usage_summary()`**

In `src/ai_council/cost_tracker.py`, add after `get_cost_by_model()`:

```python
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
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd /Users/avicuna/Dev/ai-council && python -m pytest tests/test_cost_tracker.py -v`
Expected: 8 passed.

- [ ] **Step 7: Commit**

```bash
cd /Users/avicuna/Dev/ai-council
git add src/ai_council/cost_tracker.py tests/test_cost_tracker.py
git commit -m "feat: add per-model cost/token tracking and usage summary queries"
```

---

### Task 5: Update Strategies — Smart Scoring + Scorer Cost Tracking

**Files:**
- Modify: `src/ai_council/strategies/moa.py`
- Modify: `src/ai_council/strategies/debate.py`
- Modify: `src/ai_council/strategies/redteam.py`

- [ ] **Step 1: Update MoA strategy**

In `src/ai_council/strategies/moa.py`, add import for `should_score`:

```python
from ai_council.scoring import score_agreement, should_score
```

Add `scorer_cost_usd` field to `MoAResult`:

```python
@dataclass
class MoAResult:
    proposals: list[ModelResponse]
    synthesis: ModelResponse
    total_ms: int
    tier: str = DEFAULT_TIER
    agreement_score: int | None = None
    agreement_reason: str | None = None
    scorer_cost_usd: float = 0.0

    @property
    def succeeded(self) -> list[ModelResponse]:
        return [p for p in self.proposals if p.succeeded]

    @property
    def total_cost_usd(self) -> float:
        return sum(p.cost_usd for p in self.proposals) + self.synthesis.cost_usd + self.scorer_cost_usd
```

Update `run_moa` to use smart scoring:

```python
async def run_moa(prompt: str, tier: str = DEFAULT_TIER) -> MoAResult:
    proposers = get_proposers(tier)
    proposals = await call_models_parallel(
        proposers, [{"role": "user", "content": prompt}]
    )

    ok = [p for p in proposals if p.succeeded]
    if not ok:
        return MoAResult(
            proposals=proposals,
            synthesis=ModelResponse("", "aggregator", "All models failed.", 0, "no proposals"),
            total_ms=sum(p.latency_ms for p in proposals),
            tier=tier,
        )

    agg_prompt = MOA_AGGREGATOR_TEMPLATE.format(
        prompt=prompt, proposals=format_proposals(ok)
    )
    synthesis = await call_model(
        get_aggregator(tier),
        [{"role": "system", "content": MOA_AGGREGATOR_SYSTEM},
         {"role": "user", "content": agg_prompt}],
    )

    score = None
    reason = None
    scorer_cost = 0.0
    if should_score(len(ok), tier):
        score, reason = await score_agreement(ok, prompt)
        # score_agreement calls call_model internally; estimate scorer cost
        # We can't easily get it from score_agreement without changing its signature,
        # so we use a known estimate for gpt-4o-mini scoring calls
        # TODO: This will be improved when we refactor score_agreement to return cost

    return MoAResult(
        proposals=proposals,
        synthesis=synthesis,
        total_ms=sum(p.latency_ms for p in proposals) + synthesis.latency_ms,
        tier=tier,
        agreement_score=score,
        agreement_reason=reason,
        scorer_cost_usd=scorer_cost,
    )
```

- [ ] **Step 2: Update score_agreement to return cost**

Actually, to properly track scorer cost, modify `score_agreement` in `src/ai_council/scoring.py` to return the cost too:

```python
async def score_agreement(
    proposals: list[ModelResponse], prompt: str
) -> tuple[int | None, str | None, float]:
    """Score agreement among proposals. Returns (score, reason, cost_usd)."""
    ok = [p for p in proposals if p.succeeded]
    if len(ok) < 2:
        return None, None, 0.0

    scoring_prompt = SCORING_PROMPT.format(
        prompt=prompt, proposals=format_proposals(ok)
    )
    result = await call_model(
        SCORING_MODEL,
        [{"role": "user", "content": scoring_prompt}],
    )

    if not result.succeeded:
        return None, None, result.cost_usd

    score, reason = parse_agreement_score(result.content)
    return score, reason, result.cost_usd
```

- [ ] **Step 3: Update MoA to use the 3-tuple return**

Replace the scoring block in `run_moa` (remove the TODO comment):

```python
    score = None
    reason = None
    scorer_cost = 0.0
    if should_score(len(ok), tier):
        score, reason, scorer_cost = await score_agreement(ok, prompt)

    return MoAResult(
        proposals=proposals,
        synthesis=synthesis,
        total_ms=sum(p.latency_ms for p in proposals) + synthesis.latency_ms,
        tier=tier,
        agreement_score=score,
        agreement_reason=reason,
        scorer_cost_usd=scorer_cost,
    )
```

- [ ] **Step 4: Update Debate strategy**

In `src/ai_council/strategies/debate.py`, add import:

```python
from ai_council.scoring import score_agreement, should_score
```

Add `scorer_cost_usd` to `DebateResult`:

```python
@dataclass
class DebateResult:
    rounds: list[list[ModelResponse]]
    synthesis: ModelResponse
    num_rounds: int
    total_ms: int
    tier: str = DEFAULT_TIER
    agreement_score: int | None = None
    agreement_reason: str | None = None
    scorer_cost_usd: float = 0.0

    @property
    def total_cost_usd(self) -> float:
        cost = self.synthesis.cost_usd + self.scorer_cost_usd
        for rnd in self.rounds:
            cost += sum(r.cost_usd for r in rnd)
        return cost
```

Replace the scoring block at the end of `run_debate`:

```python
    # Score agreement on final round
    final_ok = [r for r in prev if r.succeeded]
    score = None
    reason = None
    scorer_cost = 0.0
    if should_score(len(final_ok), tier):
        score, reason, scorer_cost = await score_agreement(final_ok, prompt)

    return DebateResult(
        rounds=rounds, synthesis=synthesis, num_rounds=len(rounds), total_ms=total,
        tier=tier, agreement_score=score, agreement_reason=reason,
        scorer_cost_usd=scorer_cost,
    )
```

- [ ] **Step 5: Update Red Team strategy**

In `src/ai_council/strategies/redteam.py`, add import:

```python
from ai_council.scoring import score_agreement, should_score
```

Add `scorer_cost_usd` to `RedTeamResult`:

```python
@dataclass
class RedTeamResult:
    proposals: list[ModelResponse]
    initial_attack: ModelResponse
    defenses: list[ModelResponse]
    targeted_attack: ModelResponse
    synthesis: ModelResponse
    total_ms: int = 0
    tier: str = DEFAULT_TIER
    agreement_score: int | None = None
    agreement_reason: str | None = None
    scorer_cost_usd: float = 0.0

    @property
    def total_cost_usd(self) -> float:
        cost = sum(p.cost_usd for p in self.proposals)
        cost += self.initial_attack.cost_usd
        cost += sum(d.cost_usd for d in self.defenses)
        cost += self.targeted_attack.cost_usd
        cost += self.synthesis.cost_usd
        cost += self.scorer_cost_usd
        return cost
```

Replace the scoring block at the end of `run_redteam`:

```python
    # Score agreement on defenses (or proposals if no defenses)
    score_targets = ok_defenses if ok_defenses else ok_proposals
    score = None
    reason = None
    scorer_cost = 0.0
    if should_score(len(score_targets), tier):
        score, reason, scorer_cost = await score_agreement(score_targets, prompt)

    return RedTeamResult(
        proposals=proposals, initial_attack=initial_attack,
        defenses=defenses, targeted_attack=targeted_attack,
        synthesis=synthesis, total_ms=total, tier=tier,
        agreement_score=score, agreement_reason=reason,
        scorer_cost_usd=scorer_cost,
    )
```

- [ ] **Step 6: Run all tests**

Run: `cd /Users/avicuna/Dev/ai-council && python -m pytest tests/ -v`
Expected: All tests pass (the scoring tests from Task 3 still pass, provider tests from Task 2 still pass).

- [ ] **Step 7: Commit**

```bash
cd /Users/avicuna/Dev/ai-council
git add src/ai_council/scoring.py src/ai_council/strategies/
git commit -m "feat: smart agreement scoring — skip when low value, track scorer cost"
```

---

### Task 6: Update MCP Server — New Defaults, Model Details, Enriched Costs

**Files:**
- Modify: `src/ai_council/mcp_server.py`

- [ ] **Step 1: Update `_log()` to pass model details**

Replace the `_log` function:

```python
def _build_model_details(result) -> list[dict]:
    """Extract per-model detail dicts from a strategy result."""
    details = []
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
        model_details=_build_model_details(result),
    )
```

- [ ] **Step 2: Change default tiers for council_ask and council_debug**

In `council_ask`, change `tier: str = "full"` to `tier: str = "balanced"` and update the docstring:

```python
@mcp.tool()
async def council_ask(
    prompt: str, mode: str = "moa", verbose: bool = True, rounds: int = 3, tier: str = "balanced"
) -> str:
    """Ask multiple AI models a question and get a synthesized answer.

    Queries up to 6 models in parallel and synthesizes their best answer.

    Args:
        prompt: The question or prompt to send to all models.
        mode: "moa" (fast synthesis), "debate" (multi-round revision), or "redteam" (adversarial critique).
        verbose: If True, include each model's individual response.
        rounds: Max debate rounds (debate mode only, default 3).
        tier: Model tier — "fast" (cheap), "balanced" (default), or "full" (premium). Override with "full" for complex questions.
    """
```

In `council_debug`, change `tier: str = "full"` to `tier: str = "balanced"` and update the docstring:

```python
@mcp.tool()
async def council_debug(error: str, tier: str = "balanced") -> str:
    """Debug an error using multiple AI models.

    Args:
        error: The error message, stack trace, or description of the issue.
        tier: Model tier — "fast", "balanced" (default), or "full".
    """
```

- [ ] **Step 3: Enrich `council_costs` output**

Replace `council_costs`:

```python
def _format_tokens(n: int) -> str:
    """Format token count: 1500 -> '1.5k', 800 -> '800'."""
    if n >= 1000:
        return f"{n / 1000:.1f}k"
    return str(n)


@mcp.tool()
async def council_costs() -> str:
    """Show spending summary and budget tracking."""
    from ai_council.cost_tracker import (
        get_cost_summary,
        get_cost_by_tier,
        get_cost_by_mode,
        get_cost_by_model,
    )

    summary = get_cost_summary()
    by_tier = get_cost_by_tier()
    by_mode = get_cost_by_mode()
    by_model = get_cost_by_model()

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

    if by_mode:
        lines.append("\nBy Mode")
        for mode, cost in by_mode.items():
            lines.append(f"  {mode}: ${cost:.4f}")

    if by_model:
        lines.append("\nBy Model")
        # Sort by cost descending
        sorted_models = sorted(by_model.items(), key=lambda x: x[1]["cost"], reverse=True)
        for name, stats in sorted_models:
            tokens_in = _format_tokens(stats["tokens_in"])
            tokens_out = _format_tokens(stats["tokens_out"])
            lines.append(
                f"  {name:<22} ${stats['cost']:.4f}  ({stats['calls']} calls, {tokens_in} in / {tokens_out} out)"
            )

    return "\n".join(lines)
```

- [ ] **Step 4: Verify imports are correct**

Make sure the top of `mcp_server.py` still imports `log_query`:

```python
from ai_council.cost_tracker import log_query
```

No additional imports needed at the top — `council_costs` uses lazy imports inside the function body.

- [ ] **Step 5: Commit**

```bash
cd /Users/avicuna/Dev/ai-council
git add src/ai_council/mcp_server.py
git commit -m "feat: enrich MCP server — smart tier defaults, model details logging, rich cost output"
```

---

### Task 7: Update CLI — New Defaults, Model Details, Enriched Costs

**Files:**
- Modify: `src/ai_council/cli.py`

- [ ] **Step 1: Change default tiers for ask and debug**

The CLI uses a shared `_tier_option` with `default=DEFAULT_TIER` (which is `"full"`). We need per-command defaults. Replace the shared option with per-command options.

Remove the shared `_tier_option` definition and replace with a factory:

```python
def _tier_option(default: str = DEFAULT_TIER):
    """Create a tier option with a specific default."""
    return click.option(
        "--tier", "-t",
        type=click.Choice(VALID_TIERS),
        default=default,
        help=f"Model tier: fast (~10x cheaper), balanced (mid), full (premium). Default: {default}.",
    )
```

Update decorators on commands:
- `ask`: change `@_tier_option` to `@_tier_option("balanced")`
- `debug`: change `@_tier_option` to `@_tier_option("balanced")`
- `review`: change `@_tier_option` to `@_tier_option("full")`
- `research`: change `@_tier_option` to `@_tier_option("full")`
- `models`: change `@_tier_option` to `@_tier_option()`
- `costs`: no tier option, no change needed

- [ ] **Step 2: Update `_log_cost()` to pass model details**

Replace `_log_cost`:

```python
def _build_model_details(result) -> list[dict]:
    """Extract per-model detail dicts from a strategy result."""
    details = []
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
```

- [ ] **Step 3: Enrich the `costs` command**

Replace the `costs` command:

```python
def _format_tokens(n: int) -> str:
    """Format token count: 1500 -> '1.5k', 800 -> '800'."""
    if n >= 1000:
        return f"{n / 1000:.1f}k"
    return str(n)


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
```

- [ ] **Step 4: Run all tests**

Run: `cd /Users/avicuna/Dev/ai-council && python -m pytest tests/ -v`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/avicuna/Dev/ai-council
git add src/ai_council/cli.py
git commit -m "feat: enrich CLI — smart tier defaults, model details logging, rich cost output"
```

---

### Task 8: Deduplicate `_build_model_details` and `_format_tokens`

Both `mcp_server.py` and `cli.py` now have identical `_build_model_details` and `_format_tokens` functions. Extract them.

**Files:**
- Modify: `src/ai_council/cost_tracker.py`
- Modify: `src/ai_council/mcp_server.py`
- Modify: `src/ai_council/cli.py`

- [ ] **Step 1: Move helpers to cost_tracker.py**

Add to `src/ai_council/cost_tracker.py`:

```python
def format_tokens(n: int) -> str:
    """Format token count for display: 1500 -> '1.5k', 800 -> '800'."""
    if n >= 1000:
        return f"{n / 1000:.1f}k"
    return str(n)


def build_model_details(result) -> list[dict]:
    """Extract per-model detail dicts from a strategy result for logging."""
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
```

- [ ] **Step 2: Update mcp_server.py imports and remove duplicate**

Remove `_build_model_details` and `_format_tokens` from `mcp_server.py`. Update imports:

```python
from ai_council.cost_tracker import log_query, build_model_details, format_tokens
```

In `_log()`, replace `_build_model_details(result)` with `build_model_details(result)`.
In `council_costs`, replace `_format_tokens(...)` with `format_tokens(...)`.

- [ ] **Step 3: Update cli.py imports and remove duplicate**

Remove `_build_model_details` and `_format_tokens` from `cli.py`. Update the `_log_cost` function:

```python
def _log_cost(result, mode: str, prompt: str) -> None:
    """Log query cost to persistent tracker."""
    from ai_council.cost_tracker import log_query, build_model_details
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
        model_details=build_model_details(result),
    )
```

In the `costs` command, import and use `format_tokens`:

```python
from ai_council.cost_tracker import get_cost_summary, get_cost_by_tier, get_cost_by_mode, get_cost_by_model, format_tokens
```

Replace `_format_tokens(...)` with `format_tokens(...)`.

- [ ] **Step 4: Run all tests**

Run: `cd /Users/avicuna/Dev/ai-council && python -m pytest tests/ -v`
Expected: All tests pass.

- [ ] **Step 5: Commit**

```bash
cd /Users/avicuna/Dev/ai-council
git add src/ai_council/cost_tracker.py src/ai_council/mcp_server.py src/ai_council/cli.py
git commit -m "refactor: extract shared helpers to cost_tracker module"
```

---

### Task 9: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `cd /Users/avicuna/Dev/ai-council && python -m pytest tests/ -v`
Expected: All tests pass.

- [ ] **Step 2: Verify imports work end-to-end**

Run: `cd /Users/avicuna/Dev/ai-council && python -c "from ai_council.mcp_server import mcp; print('MCP OK')"`
Expected: `MCP OK`

Run: `cd /Users/avicuna/Dev/ai-council && python -c "from ai_council.cli import cli; print('CLI OK')"`
Expected: `CLI OK`

- [ ] **Step 3: Verify CLI costs command runs**

Run: `cd /Users/avicuna/Dev/ai-council && council costs`
Expected: Spending summary output with By Tier, By Mode, and By Model sections (By Model may be empty if no new-format entries exist yet).

- [ ] **Step 4: Commit any final fixes if needed**
