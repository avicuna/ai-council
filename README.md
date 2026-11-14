# AI Council — Multi-LLM Orchestrator (Personal Edition)

Query several AI models in parallel and synthesize their best answer. Three orchestration modes, three cost tiers, per-query cost tracking. Single Go binary — no runtime dependencies.

## Install

```bash
# Build from source
git clone https://github.com/avicuna/ai-council.git
cd ai-council
go build -o council-personal .

# Move to PATH
mv council-personal ~/go/bin/  # or /usr/local/bin/
```

## Setup

```bash
# Set API keys (at least 2 required)
export ANTHROPIC_API_KEY="sk-ant-..."
export OPENAI_API_KEY="sk-..."
export GEMINI_API_KEY="..."        # Optional, used with COUNCIL_GEMINI_MODEL
```

## Tiers

| Tier | Models | Aggregator | Use Case |
|------|--------|------------|----------|
| **fast** | Haiku 5.5, GPT-6 Luna | Haiku 5.5 | Quick questions, cheapest |
| **balanced** | Sonnet 5.5, GPT-6.1 Terra | Sonnet 5.5 | Good quality, moderate cost |
| **full** | Opus 5.5, GPT-6.1 Sol | Opus 5.5 | Maximum quality |

Set `COUNCIL_GEMINI_MODEL` (for example `gemini/gemini-3.8-flash`) to add Gemini to the full tier.

Only models with API keys configured are used. Works with 2+ models.

## CLI Usage

```bash
# MoA mode (default) — fast parallel synthesis
council-personal ask "What causes inflation?" --verbose

# Use a cheaper tier for simple questions
council-personal ask "What is HTTP?" --tier fast -v

# Debate mode for nuanced topics
council-personal ask "React vs Vue?" --tier balanced --mode debate -v

# Red team mode for high-stakes decisions
council-personal ask "Design a caching strategy" --mode redteam -v

# Preset commands (all support --tier)
council-personal review main.go --tier balanced
council-personal debug "TypeError: ..."
council-personal research "WebSockets vs SSE" --tier full

# Pipe stdin
cat error.log | council-personal debug
cat main.go | council-personal ask "review this" -f utils.go

# Budget tracking
council-personal costs
council-personal models
council-personal models -t fast
```

## Cost Tracking

Every query logs its cost to `~/.ai-council/costs.jsonl`. View spending with `council-personal costs`.

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
      "command": "council-personal",
      "args": ["mcp"]
    }
  }
}
```

Tools: `council_ask`, `council_review`, `council_debug`, `council_research`, `council_costs`, `council_models`, `council_usage`, `council_status`, `council`

All MCP tools support a `tier` parameter (`"fast"`, `"balanced"`, `"full"`).

## How It Works

**MoA (Mixture of Agents):** All models answer in parallel → aggregator synthesizes the best response. Fast, good for most questions.

**Debate:** Models answer → see each other's responses → revise over multiple rounds → judge produces verdict. Best for nuanced topics.

**Red Team:** Proposers answer → attacker critiques → proposers defend → attacker targets defenses → judge synthesizes hardened answer. Best for high-stakes decisions.

**Agreement Scoring:** After each run, a separate call scores consensus (0-100%). Low scores suggest using debate mode for deeper analysis.

## Override Models

```bash
# These apply to the full tier.
export COUNCIL_CLAUDE_MODEL="claude-sonnet-5-5"
export COUNCIL_GPT_MODEL="gpt-6.1-terra"
export COUNCIL_GEMINI_MODEL="gemini/gemini-3.8-flash"   # off unless set
export COUNCIL_AGGREGATOR_MODEL="claude-opus-5-5"
```

## Architecture

```
cmd/                        CLI commands (Cobra)
├── ask.go                  council-personal ask
├── review.go               council-personal review
├── debug.go                council-personal debug
├── research.go             council-personal research
├── models.go               council-personal models
├── costs.go                council-personal costs
├── mcp.go                  council-personal mcp (stdio MCP server)
├── common.go               Shared pipeline logic
└── root.go                 Command registration

internal/
├── config/                 Tiered model config, env var overrides, API key validation
├── provider/               Native SDK calls (Anthropic, OpenAI, Google), parallel execution
├── strategy/               MoA, Debate, Red Team orchestration
├── prompt/                 System prompts and templates
├── scoring/                Agreement scoring (0-100%)
├── cost/                   Persistent cost logging (~/.ai-council/costs.jsonl)
└── output/                 Terminal formatting (lipgloss, pterm)
```
