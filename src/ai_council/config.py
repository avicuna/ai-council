"""Configuration for AI Council — Personal Edition.

Premium model lineup with direct API keys. No gateway, no restrictions.
Supports tiered routing: fast (cheap), balanced (mid), full (premium).

Required env vars (set whichever providers you want):
    export ANTHROPIC_API_KEY="sk-ant-..."
    export OPENAI_API_KEY="sk-..."
    export GEMINI_API_KEY="..."
    export DEEPSEEK_API_KEY="..."      # Optional
    export XAI_API_KEY="..."           # Optional (Grok)
"""

from __future__ import annotations

import os
from dataclasses import dataclass

# ─── Default models per tier ─────────────────────────────────
# Override any model via env vars: COUNCIL_CLAUDE_MODEL, etc.

# Full tier (premium)
CLAUDE_MODEL = os.environ.get("COUNCIL_CLAUDE_MODEL", "claude-opus-4-20250918")
GPT_MODEL = os.environ.get("COUNCIL_GPT_MODEL", "gpt-4.1")
O3_MODEL = os.environ.get("COUNCIL_O3_MODEL", "o3")
GEMINI_MODEL = os.environ.get("COUNCIL_GEMINI_MODEL", "gemini/gemini-2.5-pro")
DEEPSEEK_MODEL = os.environ.get("COUNCIL_DEEPSEEK_MODEL", "deepseek/deepseek-reasoner")
GROK_MODEL = os.environ.get("COUNCIL_GROK_MODEL", "xai/grok-3")

AGGREGATOR_MODEL = os.environ.get("COUNCIL_AGGREGATOR_MODEL", CLAUDE_MODEL)

# Models where reasoning is built-in — no temperature, system→user prefix
REASONING_MODELS = {"o3", "o3-mini", "o4-mini", "deepseek/deepseek-reasoner"}

# Provider → env var mapping for API key detection
_PROVIDER_KEYS: list[tuple[tuple[str, ...], str]] = [
    (("claude", "anthropic"), "ANTHROPIC_API_KEY"),
    (("gpt", "o1", "o3", "o4", "openai/", "ft:"), "OPENAI_API_KEY"),
    (("gemini",), "GEMINI_API_KEY"),
    (("deepseek",), "DEEPSEEK_API_KEY"),
    (("xai", "grok"), "XAI_API_KEY"),
]

# ─── Tier definitions ────────────────────────────────────────
# Each tier: (list of (model_id, friendly_name), aggregator_model_id)

TIERS: dict[str, dict] = {
    "fast": {
        "models": [
            ("claude-haiku-4-5-20251001", "Claude Haiku 4.5"),
            ("gpt-4o-mini", "GPT-4o-mini"),
            ("gemini/gemini-2.0-flash", "Gemini Flash"),
        ],
        "aggregator": "claude-haiku-4-5-20251001",
    },
    "balanced": {
        "models": [
            ("claude-sonnet-4-20250514", "Claude Sonnet 4"),
            ("gpt-4.1", "GPT-4.1"),
            ("gemini/gemini-2.5-pro", "Gemini 2.5 Pro"),
        ],
        "aggregator": "claude-sonnet-4-20250514",
    },
    "full": {
        "models": [
            (CLAUDE_MODEL, None),   # None = auto-resolve name
            (GPT_MODEL, None),
            (O3_MODEL, None),
            (GEMINI_MODEL, None),
            (DEEPSEEK_MODEL, None),
            (GROK_MODEL, None),
        ],
        "aggregator": AGGREGATOR_MODEL,
    },
}

VALID_TIERS = list(TIERS.keys())
DEFAULT_TIER = "full"


@dataclass
class ModelConfig:
    """A model and its LiteLLM identifier."""

    model: str
    name: str

    @property
    def is_reasoning(self) -> bool:
        """Check if this is a reasoning model (derived from REASONING_MODELS)."""
        return self.model in REASONING_MODELS

    @property
    def available(self) -> bool:
        """Check if the required API key is set for this model."""
        model_lower = self.model.lower()
        for prefixes, env_var in _PROVIDER_KEYS:
            if any(p in model_lower for p in prefixes):
                return bool(os.environ.get(env_var, "").strip())
        return True  # Unknown provider — try anyway


def _resolve_tier(tier: str) -> dict:
    """Resolve a tier name to its definition, with validation."""
    if tier not in TIERS:
        import warnings
        warnings.warn(f"Unknown tier {tier!r}, falling back to {DEFAULT_TIER!r}. Valid: {VALID_TIERS}")
        return TIERS[DEFAULT_TIER]
    return TIERS[tier]


def _build_models(tier: str) -> list[ModelConfig]:
    """Build the model list for a given tier."""
    tier_def = _resolve_tier(tier)
    models = []
    for model_id, name in tier_def["models"]:
        friendly = name if name else _friendly_name(model_id)
        models.append(ModelConfig(model=model_id, name=friendly))
    return models


def get_proposers(tier: str = DEFAULT_TIER) -> list[ModelConfig]:
    """Return proposer models with available API keys for the given tier."""
    return [m for m in _build_models(tier) if m.available]


def get_all_proposers(tier: str = DEFAULT_TIER) -> list[ModelConfig]:
    """Return ALL proposer models for display (regardless of key availability)."""
    return _build_models(tier)


def get_aggregator(tier: str = DEFAULT_TIER) -> ModelConfig:
    """Return the aggregator model for the given tier."""
    tier_def = _resolve_tier(tier)
    agg_model = tier_def["aggregator"]
    return ModelConfig(model=agg_model, name=_friendly_name(agg_model))


_FRIENDLY_NAMES = {
    "claude-opus-4-20250918": "Claude Opus 4.6",
    "claude-sonnet-4-20250514": "Claude Sonnet 4",
    "claude-haiku-4-5-20251001": "Claude Haiku 4.5",
    "gpt-4.1": "GPT-4.1",
    "gpt-4.1-mini": "GPT-4.1 Mini",
    "gpt-4o": "GPT-4o",
    "gpt-4o-mini": "GPT-4o-mini",
    "o3": "o3",
    "o3-mini": "o3-mini",
    "o4-mini": "o4-mini",
    "gemini/gemini-2.5-pro": "Gemini 2.5 Pro",
    "gemini/gemini-2.0-flash": "Gemini Flash",
    "deepseek/deepseek-reasoner": "DeepSeek R1",
    "deepseek/deepseek-chat": "DeepSeek V3",
    "xai/grok-3": "Grok 3",
}


def _friendly_name(model: str) -> str:
    """Convert model ID to a human-friendly name."""
    if model in _FRIENDLY_NAMES:
        return _FRIENDLY_NAMES[model]
    # Fallback: strip provider prefix and title-case
    base = model.rsplit("/", 1)[-1]
    return base.replace("-", " ").title()
