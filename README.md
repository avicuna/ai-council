# AI Council — Multi-LLM Orchestrator (Personal)

Ask multiple AI models a question and get a synthesized best answer. Models are queried in parallel via [LiteLLM](https://github.com/BerriAI/litellm).

## Setup

```bash
pip install -e .

# Set your API keys
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."
export GEMINI_API_KEY="..."
```

## Usage

```bash
# MoA mode (default) — all models propose, one synthesizes
council ask "What causes inflation?" --verbose

# Debate mode — models discuss over multiple rounds
council ask "React vs Vue?" --mode debate --verbose --rounds 2

# Check model/key status
council models
```

## How It Works

**MoA (Mixture of Agents):** All models answer in parallel → aggregator synthesizes the best response.

**Debate:** Models answer → see each other's responses → revise → repeat → judge produces verdict.

## Override Models

```bash
export COUNCIL_ANTHROPIC_MODEL="claude-opus-4-20250918"
export COUNCIL_OPENAI_MODEL="gpt-4o"
export COUNCIL_GEMINI_MODEL="gemini/gemini-1.5-pro"
export COUNCIL_AGGREGATOR_MODEL="claude-opus-4-20250918"
```

## Architecture

```
src/ai_council/
├── cli.py              CLI (ask, models)
├── config.py           Model config from env vars
├── providers.py        LiteLLM async calls
├── prompts.py          Aggregator/judge prompts
└── strategies/
    ├── moa.py          Mixture of Agents
    └── debate.py       Multi-round debate
```

All provider calls go through LiteLLM — supports 100+ models out of the box. Just set the right API key and model ID.
