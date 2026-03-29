# AI Council Personal — Go Rewrite Implementation Plan

> **For agentic workers:** Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan. Steps use checkbox syntax for tracking.

**Goal:** Rewrite ai-council-personal from Python to Go as a single binary with identical MCP tools and CLI commands.

**Architecture:** Single Go binary using Cobra CLI, native SDKs (anthropic-sdk-go, openai-go, go-genai), goroutine-based parallelism, and go-sdk for stdio MCP server. Layered as `cmd → strategy → provider` with config/cost/prompt as shared packages.

**Tech Stack:** Go 1.22+, Cobra, lipgloss, pterm, anthropic-sdk-go, openai-go, go-genai, go-sdk (MCP)

---

### Task 1: Project Scaffold and Config Layer

**Files:**
- Create: `go.mod`, `main.go`
- Create: `internal/config/models.go` — ModelConfig, tiers, env var overrides, friendly names
- Create: `internal/config/keys.go` — API key detection, validation, availability checks
- Test: `internal/config/models_test.go`, `internal/config/keys_test.go`

**What to build:**

Initialize the Go module (`github.com/avicuna/ai-council-personal`). Install core deps: cobra, lipgloss, pterm.

Port `config.py` into two files. `models.go` defines:

```go
type ModelConfig struct {
    Model       string
    Name        string
    IsReasoning bool
}

type TierConfig struct {
    Name       string
    Proposers  []ModelConfig
    Aggregator ModelConfig
}
```

Tier definitions (fast/balanced/full) as a `map[string]TierConfig`. Each model ID loaded from env var with fallback defaults (e.g., `COUNCIL_CLAUDE_MODEL` → `"claude-opus-4-20250918"`). The `_friendly_name()` mapping ported as a map.

`keys.go` handles API key detection: maps provider keywords in model IDs to env var names (`"claude"→ANTHROPIC_API_KEY`, `"gpt"/"o3"/"o4"→OPENAI_API_KEY`, etc.). `Available(model)` checks if the key is set. `ValidateKeys(tier)` returns missing keys with actionable error message.

`main.go` sets up the Cobra root command with global `--tier` flag (default "full").

**Tests:** Tier resolution returns correct models, env var overrides work, key detection maps correctly, friendly names resolve, reasoning model detection works.

**Verify:**
```bash
go test ./internal/config/ -race -v
```

**Commit:** `feat: project scaffold with config and tier system`

---

### Task 2: Provider Layer — Anthropic

**Files:**
- Create: `internal/provider/provider.go` — Provider interface, Request/Response types, QueryAll parallel execution
- Create: `internal/provider/anthropic.go` — Claude via anthropic-sdk-go
- Test: `internal/provider/provider_test.go`, `internal/provider/anthropic_test.go`

**What to build:**

Define the core provider interface and types in `provider.go`:

```go
type Provider interface {
    Name() string
    Query(ctx context.Context, req *Request) (*Response, error)
    Available() bool
}

type Request struct {
    SystemPrompt string
    UserPrompt   string
    Temperature  *float64 // nil = omit (reasoning models)
    MaxTokens    int
}

type Response struct {
    Content      string
    Model        string
    Name         string
    InputTokens  int
    OutputTokens int
    LatencyMs    int64
}
```

`QueryAll()` function: launch goroutines per provider with `sync.WaitGroup`, per-model timeout via `context.WithTimeout` (30s flash, 90s standard, 180s reasoning), collect responses and errors. Send `ProgressEvent` on a channel as models complete.

`anthropic.go`: Implement `AnthropicProvider` using `anthropic-sdk-go`. Constructor takes API key and model config. `Query()` calls the Messages API, measures latency, extracts token counts from response. No reasoning model handling needed here (Claude isn't a reasoning model).

**Tests:** Mock the SDK client. Test that Query builds correct request params, measures latency, extracts tokens. Test QueryAll with 3 mock providers — verify parallel execution, partial failure handling (one returns error, others succeed), timeout enforcement.

**Verify:**
```bash
go test ./internal/provider/ -race -v
```

**Commit:** `feat: provider interface and Anthropic implementation`

---

### Task 3: Provider Layer — OpenAI and Google

**Files:**
- Create: `internal/provider/openai.go` — GPT, o3, o4-mini, DeepSeek, Grok via openai-go
- Create: `internal/provider/google.go` — Gemini via go-genai
- Test: `internal/provider/openai_test.go`, `internal/provider/google_test.go`

**What to build:**

`openai.go`: Shared `NewOpenAICompatProvider(name, baseURL, apiKey, model, isReasoning)` constructor. For reasoning models: convert system message to user prefix (`[System instructions]\n...\n\n`), omit temperature, use `max_completion_tokens` instead of `max_tokens`. Factory functions:
- `NewGPTProvider(key, model)` — base URL: OpenAI default
- `NewO3Provider(key, model)` — same base URL, `isReasoning=true`
- `NewDeepSeekProvider(key, model)` — base URL: `https://api.deepseek.com/v1`, `isReasoning=true`
- `NewGrokProvider(key, model)` — base URL: `https://api.x.ai/v1`

`google.go`: `GeminiProvider` using `go-genai`. Map Request fields to Gemini's content format. Add retry wrapper (2 retries with jittered backoff on 429/5xx) since go-genai has no built-in retries.

Both implement the `Provider` interface from Task 2.

**Tests:** Verify reasoning model message conversion (system→user prefix). Verify temperature omission for reasoning models. Verify retry logic for Google provider. Verify each factory sets correct base URL.

**Verify:**
```bash
go test ./internal/provider/ -race -v
```

**Commit:** `feat: OpenAI-compat and Google provider implementations`

---

### Task 4: Prompt Templates and Scoring

**Files:**
- Create: `internal/prompt/prompts.go` — all system prompts and template formatting functions
- Create: `internal/scoring/agreement.go` — agreement scoring via cheap model
- Test: `internal/prompt/prompts_test.go`, `internal/scoring/agreement_test.go`

**What to build:**

Port `prompts.py` constants directly into Go string constants. Port the three formatting helpers:
- `FormatProposals(responses)` — joins as `"━━━ Name ━━━\ncontent"`
- `FormatOtherResponses(responses, excludeModel)` — same but excludes one
- `FormatDebateHistory(rounds)` — formats all rounds with headers

Port `scoring.py`: `ScoreAgreement()` takes proposals + prompt, calls GPT-4o-mini with the scoring prompt, returns `(score int, reason string, err error)`. `ParseAgreementScore()` tries JSON parse then regex fallback. Use a 5-second timeout — scoring should never block the main result.

The scoring function needs a Provider to call — accept one as a parameter rather than constructing internally.

**Tests:** Template formatting produces expected output. Score parsing handles valid JSON, malformed JSON with regex fallback, and garbage input. Test with <2 proposals returns nil score.

**Verify:**
```bash
go test ./internal/prompt/ ./internal/scoring/ -race -v
```

**Commit:** `feat: prompt templates and agreement scoring`

---

### Task 5: MoA Strategy

**Files:**
- Create: `internal/strategy/strategy.go` — Strategy interface and Result type
- Create: `internal/strategy/moa.go` — Mixture of Agents
- Test: `internal/strategy/moa_test.go`

**What to build:**

Define the strategy interface in `strategy.go`:

```go
type Strategy interface {
    Execute(ctx context.Context, opts *Options) (*Result, error)
}

type Options struct {
    Proposers  []provider.Provider
    Aggregator provider.Provider
    Scorer     provider.Provider // nil if scoring disabled (non-full tier)
    Request    *provider.Request
    Progress   chan<- ProgressEvent
    Rounds     int // only used by debate
}

type Result struct {
    Proposals      []*provider.Response
    Synthesis      *provider.Response
    Rounds         int
    AgreementScore *int
    AgreementReason string
    TotalMs        int64
}
```

MoA strategy: QueryAll proposers → filter successes (need ≥2, or ≥1 for fast tier) → call aggregator with synthesis prompt → optionally score agreement. Fast tier skips aggregation (returns first successful response as synthesis).

**Tests:** Mock providers. Test happy path (3 succeed → synthesis). Test partial failure (1 fails, 2 succeed → still synthesizes). Test all fail → error. Test fast tier skips aggregation. Test scoring is called when scorer is provided.

**Verify:**
```bash
go test ./internal/strategy/ -race -v
```

**Commit:** `feat: MoA strategy implementation`

---

### Task 6: Debate and Red Team Strategies

**Files:**
- Create: `internal/strategy/debate.go` — multi-round debate
- Create: `internal/strategy/redteam.go` — adversarial red team
- Test: `internal/strategy/debate_test.go`, `internal/strategy/redteam_test.go`

**What to build:**

**Debate:** Round 1: QueryAll independently. Rounds 2-N: each proposer revises seeing others' previous answers (parallel per round). Check `ctx.Done()` between rounds. Judge synthesizes from full history. Score agreement on final round.

**Red Team:** Select attacker via `sha256(prompt) % len(proposers)`. Non-attackers propose → attacker critiques → proposers defend → attacker retargets → judge synthesizes hardened answer. Score agreement on defenses.

Both use the same `Result` type from Task 5. Red team falls back to MoA if <2 providers available.

**Tests:** Debate: verify correct number of rounds, revision prompts include others' responses, early stop on <2 successes. Red Team: verify attacker selection is deterministic, attacker gets different prompt than proposers, fallback to MoA with 1 provider.

**Verify:**
```bash
go test ./internal/strategy/ -race -v
```

**Commit:** `feat: debate and red team strategies`

---

### Task 7: Cost Tracker

**Files:**
- Create: `internal/cost/tracker.go` — JSONL cost logging and summary queries
- Create: `pricing.json` — embedded token pricing table
- Test: `internal/cost/tracker_test.go`

**What to build:**

Port `cost_tracker.py`. Append-only JSONL at `~/.ai-council/costs.jsonl`. Use `os.O_APPEND|os.O_CREATE|os.O_WRONLY` for cross-process safety.

```go
type Entry struct {
    Timestamp    string   `json:"ts"`
    Mode         string   `json:"mode"`
    Tier         string   `json:"tier"`
    Models       []string `json:"models"`
    SuccessCount int      `json:"succeeded"`
    CostUSD      float64  `json:"cost_usd"`
    LatencyMs    int64    `json:"latency_ms"`
    PromptPreview string  `json:"prompt"`
}
```

`LogQuery()` appends entry. `GetSummary()` returns today/week/month/all-time totals + query counts. `GetByTier()` and `GetByMode()` return breakdowns. Use `//go:embed pricing.json` for token pricing. `EstimateCost(model, inputTokens, outputTokens)` looks up rates.

**Tests:** Use temp dir for JSONL file. Test log+read roundtrip, summary date bucketing, malformed line handling, pricing lookup for known models, missing model returns zero cost.

**Verify:**
```bash
go test ./internal/cost/ -race -v
```

**Commit:** `feat: cost tracking with embedded pricing table`

---

### Task 8: CLI Output Formatting

**Files:**
- Create: `internal/output/format.go` — terminal rendering with lipgloss/pterm
- Test: `internal/output/format_test.go`

**What to build:**

Port the Rich console formatting from `cli.py` to lipgloss+pterm. Functions:

- `RenderMoAResult(result, verbose, tier)` — header line, per-model responses (if verbose), synthesis, footer (models/mode/time/cost/agreement)
- `RenderDebateResult(result, verbose, tier)` — round-by-round if verbose, verdict, footer
- `RenderRedTeamResult(result, verbose, tier)` — proposals/attacks/defenses if verbose, hardened answer, footer
- `RenderModels(tier, proposers, aggregator)` — model table with availability status
- `RenderCosts(summary, byTier, byMode)` — cost summary table
- `NewProgressTracker()` — pterm spinner that updates as models complete via ProgressEvent channel

Respect `NO_COLOR` env var and non-TTY detection (degrade to plain text).

**Tests:** Test with `NO_COLOR=1` produces clean output without ANSI codes. Test agreement score color coding (green ≥70, yellow ≥40, red <40).

**Verify:**
```bash
go test ./internal/output/ -race -v
```

**Commit:** `feat: terminal output formatting with lipgloss and pterm`

---

### Task 9: CLI Commands

**Files:**
- Create: `cmd/ask.go`, `cmd/review.go`, `cmd/debug.go`, `cmd/research.go`
- Create: `cmd/models.go`, `cmd/costs.go`
- Update: `main.go` — register all subcommands
- Test: `cmd/ask_test.go` (representative CLI test)

**What to build:**

Port `cli.py` Click commands to Cobra subcommands. Each command:
1. Reads prompt from args, `--file`, or stdin
2. Resolves tier, builds provider list from config, validates API keys
3. Constructs strategy, runs it with progress channel
4. Renders output via `internal/output`
5. Logs cost via `internal/cost`

`ask.go`: `--mode` (moa/debate/redteam), `--verbose`, `--rounds`, `--file`, `--tier`
`review.go`: Reads file, constructs review prompt, runs MoA verbose
`debug.go`: Reads error from arg or stdin, constructs debug prompt, runs MoA
`research.go`: Constructs research prompt, runs debate with configurable rounds
`models.go`: Displays tier model table
`costs.go`: Displays cost summary

Wire a shared helper that all commands use for the provider-construction → strategy-run → render → log-cost pipeline to avoid duplication.

**Tests:** Test ask command with mock providers via dependency injection. Verify flag parsing, stdin reading, strategy selection.

**Verify:**
```bash
go test ./cmd/ -race -v && go build -o /dev/null .
```

**Commit:** `feat: CLI commands (ask, review, debug, research, models, costs)`

---

### Task 10: MCP Server

**Files:**
- Create: `cmd/mcp.go` — stdio MCP server entry point
- Test: `cmd/mcp_test.go`

**What to build:**

Port `mcp_server.py` using `modelcontextprotocol/go-sdk`. Register the same tool names with identical parameter schemas:

| Tool | Maps To |
|------|---------|
| `council_ask` | ask with all options |
| `council_review` | review |
| `council_debug` | debug |
| `council_research` | research |
| `council_costs` | costs summary |
| `council_models` | models list |
| `council_usage` | last query token usage |
| `council_status` | version info |
| `council_changelog` | recent changes |
| `council` | generic escape-hatch |

All logging to stderr (never stdout — MCP protocol uses stdout). The `internal/output` package must NOT be imported in this code path. Format results as plain text strings matching the Python edition's output format.

**Tests:** Spawn `go run . mcp` as subprocess, send MCP initialize + tool list request over stdin, verify all tools are registered with correct schemas.

**Verify:**
```bash
go test ./cmd/ -race -v
```

**Commit:** `feat: stdio MCP server with all council tools`

---

### Task 11: Integration Test and Migration Cleanup

**Files:**
- Update: `README.md` — Go installation instructions, updated usage
- Create: `.gitignore` — Go build artifacts
- Modify: `go.mod` — ensure all deps are tidied

**What to build:**

Run a full integration test: build the binary, invoke `council-personal models --tier balanced` and verify it lists 3 models. If API keys are available, run `council-personal ask "What is 2+2?" --tier fast` and verify it returns a response.

Create the `pre-go-rewrite` branch preserving the Python code:
```bash
git branch pre-go-rewrite
```

Remove all Python files (`src/`, `pyproject.toml`, `.python-version`, etc.), keeping `docs/` and `.git/`.

Update README with:
- Install: `go install github.com/avicuna/ai-council-personal@latest`
- Or: `go build -o council-personal .`
- MCP config: command is now `council-personal mcp` (was `council-personal-mcp`)
- Required env vars per tier

Run `go mod tidy` and verify clean build.

**Verify:**
```bash
go build -o council-personal . && ./council-personal models --tier balanced
go test ./... -race -v
```

**Commit:** `feat: complete Go rewrite, remove Python code`

---

## Task Dependency Graph

```
Task 1 (config) ──→ Task 2 (provider interface + anthropic) ──→ Task 3 (openai + google)
                                    ↓
Task 4 (prompts + scoring) ────→ Task 5 (MoA) ──→ Task 6 (debate + redteam)
                                                          ↓
Task 7 (cost tracker) ──→ Task 8 (output) ──→ Task 9 (CLI) ──→ Task 10 (MCP)
                                                                      ↓
                                                               Task 11 (integration + cleanup)
```

Tasks 1, 4, 7 can start in parallel. Tasks 2-3 depend on 1. Tasks 5-6 depend on 2-4. Tasks 9-10 depend on 5-8. Task 11 depends on everything.
