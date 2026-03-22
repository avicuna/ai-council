# AI Council — Multi-LLM Orchestrator (Personal Edition)

Query up to 6 AI models in parallel and synthesize their best answer. Three orchestration modes: MoA (fast), Debate (thorough), Red Team (adversarial). Built on [LiteLLM](https://github.com/BerriAI/litellm) for universal provider support.

## Models

| Model | Provider | Type |
|-------|----------|------|
| Claude Opus 4.6 | Anthropic | Standard |
| GPT-4.1 | OpenAI | Standard |
| o3 | OpenAI | Reasoning |
| Gemini 2.5 Pro | Google | Standard |
| DeepSeek R1 | DeepSeek | Reasoning |
| Grok 3 | xAI | Standard |

Only models with API keys configured are used. Works with 2+ models.

## Setup

```bash
pip install -e .

# Set API keys (at least 2 required)
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."
export GEMINI_API_KEY="..."
export DEEPSEEK_API_KEY="..."      # Optional
export XAI_API_KEY="..."           # Optional
```

## CLI Usage

```bash
# MoA mode (default) — fast parallel synthesis
council ask "What causes inflation?" --verbose

# Debate mode — multi-round revision
council ask "React vs Vue?" --mode debate --rounds 2 -v

# Red Team mode — adversarial critique + defense
council ask "Is this architecture scalable?" --mode redteam -v

# Preset commands
council review src/main.py          # Code review
council debug "TypeError: ..."      # Debug an error
council research "WebSockets vs SSE" # Research (debate mode)

# Pipe stdin
cat error.log | council debug
cat main.py | council ask "review this" -f src/utils.py

# Check status
council models
```

## MCP Server (Claude Code)

Add to your `.mcp.json`:

```json
{
  "mcpServers": {
    "ai-council": {
      "command": "council-mcp",
      "args": []
    }
  }
}
```

Tools: `council_ask`, `council_review`, `council_debug`, `council_research`, `council_models`

## How It Works

**MoA (Mixture of Agents):** All models answer in parallel → aggregator synthesizes the best response. Fast, good for most questions.

**Debate:** Models answer → see each other's responses → revise over multiple rounds → judge produces verdict. Best for nuanced topics.

**Red Team:** Proposers answer → attacker critiques → proposers defend → attacker targets defenses → judge synthesizes hardened answer. Best for high-stakes decisions.

**Agreement Scoring:** After each run, a separate call scores consensus (0-100%). Low scores suggest using debate mode for deeper analysis.

## Override Models

```bash
export COUNCIL_CLAUDE_MODEL="claude-sonnet-4-20250514"
export COUNCIL_GPT_MODEL="gpt-4o"
export COUNCIL_O3_MODEL="o3-mini"
export COUNCIL_GEMINI_MODEL="gemini/gemini-2.0-flash"
export COUNCIL_DEEPSEEK_MODEL="deepseek/deepseek-chat"
export COUNCIL_GROK_MODEL="xai/grok-3"
export COUNCIL_AGGREGATOR_MODEL="claude-opus-4-20250918"
```

## Architecture

```
src/ai_council/
├── cli.py              CLI (ask, review, debug, research, models)
├── config.py           6 models, env var config, reasoning model detection
├── providers.py        LiteLLM async calls, reasoning model handling
├── prompts.py          MoA + Debate + Red Team + Scoring prompts
├── scoring.py          Agreement scoring (0-100%)
├── mcp_server.py       MCP server for Claude Code
└── strategies/
    ├── moa.py          Mixture of Agents + scoring
    ├── debate.py       Multi-round debate + scoring
    └── redteam.py      Adversarial critique/defense/judge
```

All provider calls go through LiteLLM — supports 100+ models. Just set the API key and model ID.
