"""Provider layer — all calls go through LiteLLM.

LiteLLM handles API keys automatically from environment variables:
    ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY
"""

from __future__ import annotations

import asyncio
import time
from dataclasses import dataclass

from litellm import acompletion

from ai_council.config import ModelConfig


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
        response = await acompletion(model=config.model, messages=messages)
        elapsed = int((time.monotonic() - start) * 1000)
        return ModelResponse(
            model=config.model,
            name=config.name,
            content=response.choices[0].message.content,
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
