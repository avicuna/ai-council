# AI Council — Multi-LLM Orchestrator (Personal Edition)

Query up to 6 AI models in parallel and synthesize their best answer. Three orchestration modes, three cost tiers, per-query cost tracking. Built on [LiteLLM](https://github.com/BerriAI/litellm) for universal provider support.

## Tiers

| Tier | Models | Aggregator | Use Case |
|------|--------|------------|----------|
| **fast** | Haiku 4.5, GPT-4o-mini, Gemini Flash | Haiku 4.5 | Quick questions, ~10x cheaper |
| **balanced** | Sonnet 4, GPT-4.1, Gemini 2.5 Pro | Sonnet 4 | Good quality, moderate cost |
| **full** | Opus 4.6, GPT-4.1, o3, Gemini Pro, DeepSeek R1, Grok 3 | Opus 4.6 | Maximum quality, 6 models |

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

# Use a cheaper tier for simple questions
council ask "What is HTTP?" --tier fast -v

# Balanced tier for everyday use
council ask "React vs Vue?" --tier balanced --mode debate -v

# Full tier (default) for important questions
council ask "Design a caching strategy" --mode redteam -v

# Preset commands (all support --tier)
council review src/main.py --tier balanced
council debug "TypeError: ..."
council research "WebSockets vs SSE" --tier full

# Pipe stdin
cat error.log | council debug
cat main.py | council ask "review this" -f src/utils.py

# Budget tracking
council costs                       # Spending summary
council models                      # Model status (default: full)
council models -t fast              # Show fast tier models
```

## Cost Tracking

Every query logs its cost to `~/.ai-council/costs.jsonl`. View your spending:

```bash
council costs
# Spending Summary
#   Today:      $0.1234  (5 queries)
#   This week:  $0.8901
#   This month: $2.3456
#   All time:   $12.3456  (142 queries)
#
# By Tier
#   fast: $1.2345
#   balanced: $3.4567
#   full: $7.5544
```

Cost is also shown in every query's footer:
```
Models: 6/6 | Mode: MoA | Tier: full | Time: 12.3s | Agreement: 85% | Cost: $0.0234
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

Tools: `council_ask`, `council_review`, `council_debug`, `council_research`, `council_costs`, `council_models`

All MCP tools support a `tier` parameter (`"fast"`, `"balanced"`, `"full"`).

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
├── cli.py              CLI (ask, review, debug, research, models, costs)
├── config.py           Tiered model config, env var overrides
├── providers.py        LiteLLM async calls, cost tracking, reasoning model handling
├── prompts.py          MoA + Debate + Red Team + Scoring prompts
├── scoring.py          Agreement scoring (0-100%)
├── cost_tracker.py     Persistent cost logging (~/.ai-council/costs.jsonl)
├── mcp_server.py       MCP server for Claude Code
└── strategies/
    ├── moa.py          Mixture of Agents + scoring + cost
    ├── debate.py       Multi-round debate + scoring + cost
    └── redteam.py      Adversarial critique/defense/judge + cost
```

All provider calls go through LiteLLM — supports 100+ models. Just set the API key and model ID.
