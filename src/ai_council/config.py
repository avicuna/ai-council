"""Configuration for AI Council.

Simple env-var based config. No profiles, no gateway — just direct API keys.
Set these in your shell:
    export ANTHROPIC_API_KEY="sk-ant-..."
    export OPENAI_API_KEY="sk-..."
    export GEMINI_API_KEY="..."
"""

from __future__ import annotations

import os
from dataclasses import dataclass

# Default models — change these to experiment
ANTHROPIC_MODEL = os.environ.get("COUNCIL_ANTHROPIC_MODEL", "claude-sonnet-4-20250514")
OPENAI_MODEL = os.environ.get("COUNCIL_OPENAI_MODEL", "gpt-4o-mini")
GEMINI_MODEL = os.environ.get("COUNCIL_GEMINI_MODEL", "gemini/gemini-2.0-flash")
AGGREGATOR_MODEL = os.environ.get("COUNCIL_AGGREGATOR_MODEL", ANTHROPIC_MODEL)


@dataclass
class ModelConfig:
    """A model and its LiteLLM identifier."""
    model: str
    name: str  # Human-friendly display name

    @property
    def available(self) -> bool:
        """Check if the required API key is set for this model."""
        if "claude" in self.model or "anthropic" in self.model:
            return bool(os.environ.get("ANTHROPIC_API_KEY"))
        if "gpt" in self.model or "o1" in self.model or "o3" in self.model:
            return bool(os.environ.get("OPENAI_API_KEY"))
        if "gemini" in self.model:
            return bool(os.environ.get("GEMINI_API_KEY"))
        return True  # Unknown provider — try anyway


def get_proposers() -> list[ModelConfig]:
    """Return the list of proposer models."""
    return [
        ModelConfig(model=ANTHROPIC_MODEL, name=_friendly_name(ANTHROPIC_MODEL)),
        ModelConfig(model=OPENAI_MODEL, name=_friendly_name(OPENAI_MODEL)),
        ModelConfig(model=GEMINI_MODEL, name=_friendly_name(GEMINI_MODEL)),
    ]


def get_aggregator() -> ModelConfig:
    """Return the aggregator model."""
    return ModelConfig(model=AGGREGATOR_MODEL, name=_friendly_name(AGGREGATOR_MODEL))


def _friendly_name(model: str) -> str:
    """Convert model ID to a human-friendly name."""
    names = {
        "claude-opus-4-20250918": "Claude Opus 4.6",
        "claude-sonnet-4-20250514": "Claude Sonnet 4",
        "claude-haiku-4-5-20251001": "Claude Haiku 4.5",
        "gpt-4o": "GPT-4o",
        "gpt-4o-mini": "GPT-4o-mini",
        "gpt-4.1": "GPT-4.1",
        "o3-mini": "o3-mini",
        "gemini/gemini-2.0-flash": "Gemini Flash",
        "gemini/gemini-1.5-pro": "Gemini Pro",
    }
    if model in names:
        return names[model]
    if "opus" in model.lower():
        return "Claude Opus"
    if "sonnet" in model.lower():
        return "Claude Sonnet"
    if "haiku" in model.lower():
        return "Claude Haiku"
    return model
