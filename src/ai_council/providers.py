"""Provider layer — all calls go through LiteLLM.

LiteLLM handles API keys automatically from environment variables:
    ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY,
    DEEPSEEK_API_KEY, XAI_API_KEY

Reasoning models (o3, DeepSeek R1) get special handling:
    - System messages are converted to user message prefixes
    - Temperature parameter is omitted
"""

from __future__ import annotations

import asyncio
import time
from dataclasses import dataclass

from litellm import acompletion

from ai_council.config import ModelConfig, REASONING_MODELS

# Timeout for individual model calls (seconds)
MODEL_TIMEOUT = 180


@dataclass
class ModelResponse:
    model: str
    name: str
    content: str
    latency_ms: int
    error: str | None = None

    @property
    def succeeded(self) -> bool:
        return self.error is None


def _prepare_messages(config: ModelConfig, messages: list[dict]) -> list[dict]:
    """Prepare messages for model, handling reasoning model quirks."""
    if not config.is_reasoning and config.model not in REASONING_MODELS:
        return messages

    # Reasoning models: convert system messages to user message prefix
    prepared = []
    system_parts = []
    for msg in messages:
        if msg["role"] == "system":
            system_parts.append(msg["content"])
        else:
            prepared.append(msg)

    if system_parts:
        system_text = "\n\n".join(system_parts)
        if not prepared:
            # Only system messages — convert to a user message
            prepared = [{"role": "user", "content": system_text}]
        elif prepared[0]["role"] == "user":
            # Prepend system content to first user message
            prepared[0] = {
                "role": "user",
                "content": f"[System instructions]\n{system_text}\n\n[User query]\n{prepared[0]['content']}",
            }
        else:
            prepared.insert(0, {"role": "user", "content": system_text})

    return prepared


def _model_kwargs(config: ModelConfig) -> dict:
    """Build extra kwargs for LiteLLM based on model type."""
    kwargs: dict = {"timeout": MODEL_TIMEOUT}

    if config.is_reasoning or config.model in REASONING_MODELS:
        # Reasoning models: no temperature, use max_completion_tokens
        kwargs["max_completion_tokens"] = 16384
    else:
        kwargs["temperature"] = 0.7
        kwargs["max_tokens"] = 8192

    return kwargs


async def call_model(config: ModelConfig, messages: list[dict]) -> ModelResponse:
    """Call a single model via LiteLLM."""
    if not config.available:
        return ModelResponse(
            model=config.model,
            name=config.name,
            content="",
            latency_ms=0,
            error="API key not set",
        )

    start = time.monotonic()
    try:
        prepared = _prepare_messages(config, messages)
        kwargs = _model_kwargs(config)
        response = await acompletion(model=config.model, messages=prepared, **kwargs)
        elapsed = int((time.monotonic() - start) * 1000)
        return ModelResponse(
            model=config.model,
            name=config.name,
            content=response.choices[0].message.content or "",
            latency_ms=elapsed,
        )
    except Exception as e:
        elapsed = int((time.monotonic() - start) * 1000)
        return ModelResponse(
            model=config.model,
            name=config.name,
            content="",
            latency_ms=elapsed,
            error=str(e),
        )


async def call_models_parallel(
    configs: list[ModelConfig], messages: list[dict]
) -> list[ModelResponse]:
    """Call multiple models in parallel."""
    return list(await asyncio.gather(*[call_model(c, messages) for c in configs]))
