# AI Council: Usage Stats & Efficiency Improvements

**Date:** 2026-03-26
**Status:** Approved

## Overview

Add per-model usage stats with token tracking to the AI Council, and reduce costs through smart agreement scoring and per-tool tier defaults.

## Changes

### 1. Token & Per-Model Tracking in the Log

**`providers.py` — `ModelResponse`:**
- Add `input_tokens: int = 0` and `output_tokens: int = 0` fields
- Extract `usage.prompt_tokens` and `usage.completion_tokens` from the LiteLLM response object after each call

**`cost_tracker.py` — `log_query()`:**
- Add `model_details` parameter: list of dicts, each with `{"name": str, "model": str, "cost_usd": float, "input_tokens": int, "output_tokens": int, "latency_ms": int, "succeeded": bool}`
- Store in JSONL entries under `"models_detail"` key alongside existing flat fields for backwards compatibility
- Old entries without `models_detail` are silently skipped by new query functions

**`cost_tracker.py` — new functions:**
- `get_cost_by_model() -> dict[str, {"cost": float, "calls": int, "tokens_in": int, "tokens_out": int}]` — aggregates per-model stats across all log entries
- `get_usage_summary() -> dict` — total tokens in/out, average cost per query, average latency, queries by mode

### 2. Smart Agreement Scoring

Replace unconditional agreement scoring with automatic smart triggering.

**Scoring runs when ALL are true:**
- 3+ models succeeded
- Tier is `balanced` or `full`

**Scoring skips when ANY are true:**
- Fewer than 3 successful models
- Fast tier
- All models failed

**Implementation:**
- Add `_should_score(succeeded_count: int, tier: str) -> bool` helper in `scoring.py`, used by all three strategies
- When scoring runs, track its cost via new `scorer_cost_usd: float = 0.0` field on each result dataclass, included in `total_cost_usd` property
- When scoring is skipped, `agreement_score = None` as today

### 3. Smart Tier Defaults Per Tool

| Tool | Old Default | New Default | Rationale |
|------|------------|-------------|-----------|
| `council_ask` | `full` | `balanced` | General questions don't need 6 premium models |
| `council_debug` | `full` | `balanced` | Debugging benefits from speed; balanced is plenty |
| `council_review` | `full` | `full` | Code review benefits from diverse premium perspectives |
| `council_research` | `full` | `full` | Deep research is the use case for premium models |

**`mcp_server.py`:** Change `tier` parameter default for `council_ask` and `council_debug` to `"balanced"`.

**`cli.py`:** Same — `ask` and `debug` subcommands default to `balanced`.

`config.py` `DEFAULT_TIER` stays `"full"` as the package-level default.

### 4. Enriched `council_costs` Output

Add two new sections to the `council_costs` tool output:

**By Mode** — surfaces the existing `get_cost_by_mode()` (currently dead code):
```
By Mode
  moa: $1.5000
  debate: $1.2345
  review: $0.5000
```

**By Model** — uses new `get_cost_by_model()`, sorted by cost descending:
```
By Model
  Claude Opus 4       $1.2000  (18 calls, 45k in / 32k out)
  GPT-4.1             $0.8500  (18 calls, 40k in / 28k out)
  ...
```

Token counts formatted as `Xk` when >= 1000.

## Files Modified

| File | Changes |
|------|---------|
| `src/ai_council/providers.py` | Add `input_tokens`, `output_tokens` to `ModelResponse`; extract from LiteLLM response |
| `src/ai_council/cost_tracker.py` | Add `model_details` to `log_query()`; add `get_cost_by_model()`, `get_usage_summary()` |
| `src/ai_council/scoring.py` | Add `_should_score()` helper |
| `src/ai_council/strategies/moa.py` | Use `_should_score()`; add `scorer_cost_usd` to `MoAResult`; track scorer cost |
| `src/ai_council/strategies/debate.py` | Use `_should_score()`; add `scorer_cost_usd` to `DebateResult`; track scorer cost |
| `src/ai_council/strategies/redteam.py` | Use `_should_score()`; add `scorer_cost_usd` to `RedTeamResult`; track scorer cost |
| `src/ai_council/mcp_server.py` | Wire up `model_details` in `_log()`; enrich `council_costs`; change defaults for `council_ask`/`council_debug` |
| `src/ai_council/cli.py` | Change defaults for `ask`/`debug`; pass model details to logger |
| `src/ai_council/config.py` | No changes |

## Backwards Compatibility

- JSONL log format is additive — new `models_detail` field is optional. Old entries without it are handled gracefully.
- Existing `get_cost_summary()` and `get_cost_by_tier()` are unchanged.
- `DEFAULT_TIER` in config.py remains `"full"` — only tool-level defaults change.
