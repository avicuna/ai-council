# Working in ai-council

A Go MCP server and CLI that queries several LLM providers in parallel. Single binary.

## Build and test

- `go build -o council-personal .` and `go test ./...`. The Go tests sit beside the code (`cmd/`, `internal/`). The Python files in `tests/` are left over from the old Python version and don't run.

## Cost

- Every live call bills each provider. Tests must never reach a real API; fake the providers. Don't run live council calls to check a change unless the owner asks.
- Prefer changes that cut tokens or calls. When choosing between an opt-in flag and automatic behavior, default to automatic behavior that adapts to tier and model count. Add a parameter only when the choice is genuinely subjective, like the tier.
