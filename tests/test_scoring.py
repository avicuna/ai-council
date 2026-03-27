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
