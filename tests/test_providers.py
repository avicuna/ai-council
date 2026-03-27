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
