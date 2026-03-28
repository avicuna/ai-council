# AI Council Personal — Go Rewrite Design Spec

**Date:** 2026-03-28
**Status:** Draft
**Scope:** Full rewrite of ai-council-personal from Python to Go

## Overview

Rewrite the AI Council personal edition (~2,000 lines of Python) as a single Go binary. The tool queries up to 6 LLM models in parallel, orchestrates them via 3 strategies (MoA, Debate, Red Team), and synthesizes a final answer. It runs as both a CLI tool and an MCP server for Claude Code.

### Goals

- Single binary distribution (no Python runtime, no venv, no pip)
- Fast startup (~10ms vs ~500ms)
- Drop-in replacement for the Python edition (same MCP tool names, same CLI commands)
- Native Go concurrency (goroutines) replacing asyncio

### Non-Goals

- Plugin architecture for third-party providers
- Streaming responses (progress spinners are sufficient)
- OpenTelemetry / observability infrastructure
- Config file system (env vars are sufficient)
- HTTP MCP transport (stdio only — HTTP is for the work edition)

## Architecture

### Single Binary, Cobra Subcommands

```
council-personal ask "prompt" [--mode moa|debate|redteam] [--tier fast|balanced|full] [--file path] [--verbose] [--rounds N]
council-personal review <file_path> [--tier] [--verbose]
council-personal debug [error_text] [--file] [--tier] [--verbose]
council-personal research "topic" [--mode moa|debate] [--tier] [--verbose] [--rounds N]
council-personal models [--tier]
council-personal costs
council-personal mcp                  # stdio MCP server for Claude Code
```

### Module & Location

- **Module:** `github.com/avicuna/ai-council-personal`
- **Location:** `~/dev/ai-council-personal/` (in-place replacement of Python edition)
- **Go version:** 1.22+

### Project Structure

```
ai-council-personal/
├── go.mod
├── go.sum
├── main.go                     # Entry point, cobra root command
├── pricing.json                # Embedded token pricing table
├── cmd/
│   ├── ask.go                  # council-personal ask
│   ├── review.go               # council-personal review
│   ├── debug.go                # council-personal debug
│   ├── research.go             # council-personal research
│   ├── models.go               # council-personal models
│   ├── costs.go                # council-personal costs
│   └── mcp.go                  # council-personal mcp (stdio MCP server)
├── internal/
│   ├── provider/
│   │   ├── provider.go         # Provider interface, Request/Response types, parallel execution
│   │   ├── anthropic.go        # Anthropic SDK (Claude models)
│   │   ├── openai.go           # OpenAI SDK (GPT, o3, o4-mini, DeepSeek, Grok)
│   │   └── google.go           # Google GenAI SDK (Gemini)
│   ├── strategy/
│   │   ├── strategy.go         # Strategy interface
│   │   ├── moa.go              # Mixture of Agents
│   │   ├── debate.go           # Multi-round debate
│   │   └── redteam.go          # Adversarial red team
│   ├── config/
│   │   ├── models.go           # ModelConfig, tiers, env var overrides
│   │   └── keys.go             # API key detection, availability, startup validation
│   ├── scoring/
│   │   └── agreement.go        # Agreement scoring via GPT-4o-mini
│   ├── cost/
│   │   └── tracker.go          # JSONL cost log (~/.ai-council/costs.jsonl), summaries
│   ├── prompt/
│   │   └── prompts.go          # Meta-prompts for synthesizer/judge/attacker
│   └── output/
│       └── format.go           # Terminal formatting (lipgloss/pterm), progress spinners
├── Dockerfile
└── README.md
```

### Dependency Direction

```
cmd → strategy → provider
         ↓
      scoring
         ↓
       output

config ← (read by all packages)
cost   ← (written by cmd layer after strategy completes)
prompt ← (read by strategy layer)
```

Imports flow downward only. `provider` knows nothing about `strategy`. `output` is never imported in MCP code paths.

## Provider Layer

### Interface

```go
type Provider interface {
    Name() string
    Query(ctx context.Context, req *Request) (*Response, error)
    EstimateCost(inputTokens, outputTokens int) float64
    Available() bool  // checks if API key is set
}
```

### Types

```go
type Request struct {
    SystemPrompt string
    UserPrompt   string
    Temperature  *float64  // nil = provider default (critical for reasoning models)
    MaxTokens    int
}

type Response struct {
    Content      string
    Model        string
    Name         string
    InputTokens  int
    OutputTokens int
    LatencyMs    int64
    CostUSD      float64
}
```

No `Error` field in Response — use Go's idiomatic `(Response, error)` return.

### Provider Implementations

| File | SDK | Models | Notes |
|------|-----|--------|-------|
| `anthropic.go` | `anthropic-sdk-go` | Claude Opus, Sonnet, Haiku | Messages API directly |
| `openai.go` | `openai-go` | GPT-4.1, GPT-4o-mini, o3, o4-mini, DeepSeek R1, Grok 3 | Shared constructor with custom base URL for DeepSeek/Grok |
| `google.go` | `googleapis/go-genai` | Gemini 2.5 Pro, Gemini Flash | |

DeepSeek and Grok use `openai.go` with custom base URLs via a shared `NewOpenAICompatProvider(name, baseURL, apiKey, model)` constructor. Provider-specific response post-processing handled in each constructor's response mapper.

### Reasoning Model Handling

Handled inside each provider, transparent to strategy layer:
- Strip `Temperature` parameter (leave nil)
- Convert system messages to user message prefix: `[System instructions] ...\n\n`
- Use `max_completion_tokens` instead of `max_tokens`
- Applies to: o3, o4-mini, DeepSeek R1

### Parallel Execution

```go
func QueryAll(ctx context.Context, providers []Provider, req *Request, progress chan<- ProgressEvent) ([]*Response, []error) {
    responses := make([]*Response, len(providers))
    errors := make([]error, len(providers))
    var wg sync.WaitGroup

    for i, p := range providers {
        i, p := i, p
        wg.Add(1)
        go func() {
            defer wg.Done()
            timeout := timeoutFor(p.Name())
            ctx, cancel := context.WithTimeout(ctx, timeout)
            defer cancel()

            progress <- ProgressEvent{Model: p.Name(), Status: "querying"}
            resp, err := p.Query(ctx, req)
            if err != nil {
                errors[i] = err
                progress <- ProgressEvent{Model: p.Name(), Status: "failed"}
            } else {
                responses[i] = resp
                progress <- ProgressEvent{Model: p.Name(), Status: "done", Latency: time.Duration(resp.LatencyMs) * time.Millisecond}
            }
        }()
    }
    wg.Wait()
    return responses, errors
}
```

Uses `sync.WaitGroup` (not `errgroup.WithContext`) so one failure doesn't cancel others.

### Per-Model Timeouts

| Model Category | Timeout | Examples |
|---|---|---|
| Flash/mini | 30s | Claude Haiku, GPT-4o-mini, Gemini Flash |
| Standard | 90s | Claude Sonnet, GPT-4.1, Gemini 2.5 Pro |
| Reasoning | 180s | o3, o4-mini, DeepSeek R1 |

### Retry Middleware

Each provider wraps SDK calls with:
- Retry on 429 (rate limit) and 500/502/503 — up to 2 retries with jittered exponential backoff
- Respect `Retry-After` header when present
- No retry on 400 (bad request) or 401 (auth failure)
- Note: anthropic-sdk-go and openai-go have built-in retries (2 by default). go-genai does NOT — add retry wrapper for Google provider.

### Cost Calculation

Token pricing stored in embedded `pricing.json` via `//go:embed`. Updated manually with each release. Format:

```json
{
  "claude-opus-4-20250918": {"input_per_mtok": 15.0, "output_per_mtok": 75.0},
  "gpt-4.1": {"input_per_mtok": 2.0, "output_per_mtok": 8.0}
}
```

Each provider's `EstimateCost()` method reads from this table.

### API Key Validation

Keys checked at startup before any queries. Fail fast with actionable error:

```
Error: missing API keys for balanced tier: OPENAI_API_KEY, GEMINI_API_KEY

Set them with:
  export OPENAI_API_KEY="sk-..."
  export GEMINI_API_KEY="..."
```

Required keys per tier:
- **fast:** ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY
- **balanced:** ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY
- **full:** All of the above + DEEPSEEK_API_KEY, XAI_API_KEY

## Strategy Layer

### Interface

```go
type Strategy interface {
    Execute(ctx context.Context, providers []Provider, aggregator Provider, req *Request, progress chan<- ProgressEvent) (*Result, error)
    MinRequired() int  // minimum successful responses needed
}

type Result struct {
    Proposals  []*Response  // individual model responses
    Synthesis  *Response    // aggregated answer
    Rounds     int          // number of rounds (debate/redteam)
}
```

### Mixture of Agents (MoA)

1. Query all proposers in parallel → collect responses
2. Fast tier: return first response as synthesis (no aggregation)
3. Balanced/Full tier: call aggregator (tier-appropriate Claude model) with all proposals
4. Full tier only: call scoring model (GPT-4o-mini) for agreement score

**Minimum required:** 2 successful responses for synthesis, 1 for fast tier.

### Debate

1. Round 1: All proposers answer independently (parallel)
2. Rounds 2-N: Each proposer sees others' answers and revises (parallel per round)
3. Stop after max rounds or if < 2 proposers succeeded
4. Judge phase: aggregator synthesizes from all rounds
5. Full tier only: score agreement on final round

**Minimum required:** 2 successful responses.

Context cancellation checked between rounds via `select { case <-ctx.Done(): ... }`.

### Red Team

1. Select attacker deterministically: `sha256(prompt) % len(proposers)`
2. All non-attacker proposers answer in parallel
3. Attacker critiques all proposals
4. Proposers defend against critique (parallel)
5. Attacker targets defenses
6. Judge synthesizes hardened answer
7. Full tier only: score agreement on defenses

**Minimum required:** 2 (1 attacker + 1 defender).

## Tier System

| Tier | Models | Count | Default For | Aggregator |
|------|--------|-------|-------------|------------|
| fast | Claude Haiku 4.5, GPT-4o-mini, Gemini Flash | 3 | — | Claude Haiku 4.5 |
| balanced | Claude Sonnet 4, GPT-4.1, Gemini 2.5 Pro | 3 | MCP tools | Claude Sonnet 4 |
| full | Claude Opus 4, GPT-4.1, o3, Gemini 2.5 Pro, DeepSeek R1, Grok 3 | 6 | CLI default | Claude Opus 4 |

All model IDs configurable via environment variables (e.g., `COUNCIL_CLAUDE_MODEL`).

## MCP Server

### Transport

Stdio only, via `modelcontextprotocol/go-sdk` (v1.4.1). Invoked as `council-personal mcp`.

### Tools

Matching the Python edition exactly:

| Tool | Parameters | Description |
|------|-----------|-------------|
| `council_ask` | prompt, mode?, verbose?, rounds?, tier? | Main query tool |
| `council_review` | file_path?, file_content?, tier? | Code review |
| `council_debug` | error, tier? | Error diagnosis |
| `council_research` | topic, rounds?, tier? | Deep research (debate mode) |
| `council_usage` | — | Token usage from last query |
| `council_models` | — | List tiers and available models |
| `council_status` | — | Server version info |
| `council_changelog` | last_n? | Show recent changes |
| `council` | action, params? | Generic escape-hatch (schema never changes) |

### stdout Safety

All logging redirected to stderr when running as MCP server. The `internal/output/` package (lipgloss/pterm) is never imported in the MCP code path. Build-time verification: MCP entry point imports only `internal/provider`, `internal/strategy`, `internal/config`, `internal/cost`, `internal/scoring`, `internal/prompt`.

## Cost Tracking

### Storage

Append-only JSONL at `~/.ai-council/costs.jsonl`. Each entry:

```json
{
  "timestamp": "2026-03-28T10:30:00Z",
  "mode": "moa",
  "tier": "balanced",
  "models": ["claude-sonnet-4", "gpt-4.1", "gemini-2.5-pro"],
  "success_count": 3,
  "cost_usd": 0.0234,
  "latency_ms": 8500,
  "prompt_preview": "Review this architecture..."
}
```

### Concurrency Safety

In-process mutex on the Logger struct. Cross-process safety via `O_APPEND` (sufficient for single-user tool).

### Queries

`council-personal costs` displays:
- Today / this week / this month / all-time totals
- By tier breakdown
- By mode breakdown
- Log file location

## CLI Output

### Terminal Formatting

Using lipgloss for styling and pterm for spinners/progress:

```
Mode: moa | Tier: balanced | Aggregator: Claude Sonnet 4

  ✓ Claude Sonnet 4 (3.2s, $0.012)
  ✓ GPT-4.1 (4.1s, $0.008)
  ⠋ Gemini 2.5 Pro...
  ✓ Gemini 2.5 Pro (5.7s, $0.006)
  ⠋ Synthesizing...

━━━ Council Synthesis ━━━
{synthesized response}

Models: 3/3 | Mode: MoA | Time: 10.5s | Cost: $0.038
Agreement: 87% — All models converged on key recommendations
```

Progress spinners update in real-time as models complete. Failed models shown with error reason.

### NO_COLOR / CI Detection

Respect `NO_COLOR` env var and detect non-TTY output for CI/pipe usage. Degrade to plain text.

### Input Methods

- Direct argument: `council-personal ask "What is gRPC?"`
- File attachment: `council-personal ask "Review" -f file.py`
- Stdin piping: `cat error.log | council-personal debug`
- Combined: file + stdin + prompt

## Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/anthropics/anthropic-sdk-go` | Claude API |
| `github.com/openai/openai-go` | OpenAI, DeepSeek, Grok (OpenAI-compat) |
| `github.com/googleapis/go-genai` | Gemini API |
| `github.com/modelcontextprotocol/go-sdk` | MCP server (stdio) |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/pterm/pterm` | Progress spinners, tables |
| `github.com/charmbracelet/lipgloss` | Terminal styling |

## Testing Strategy

### Unit Tests

- Cost calculation from pricing table (deterministic)
- Tier resolution and model config (deterministic)
- Prompt template rendering (deterministic)
- Reasoning model parameter adjustment (deterministic)
- API key validation logic (deterministic)
- Agreement score parsing (deterministic)
- Attacker selection (deterministic — SHA256-based)

### Integration Tests

- Record real API responses with go-vcr or similar
- Replay for strategy testing (MoA synthesis, debate rounds, red team flow)
- Verify partial failure handling (inject errors for some providers)

### MCP Protocol Tests

- Spawn `council-personal mcp` as subprocess
- Send MCP protocol messages over stdin
- Verify tool registration and response format

### Race Detection

All tests run with `go test -race` from day one.

## Migration Plan

1. Create git branch `pre-go-rewrite` preserving the Python code
2. Clear the working directory (keep `.git`, `docs/`)
3. Initialize Go module and build incrementally
4. After Go version is functional, update Claude Code MCP config:
   - Command: `council-personal mcp` (was `council-personal-mcp`)
5. Uninstall Python package: `pip uninstall ai-council-personal`

## Open Questions

None — all decisions resolved during brainstorming.
