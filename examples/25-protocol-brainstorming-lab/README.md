# Protocol Brainstorming Lab

Run locally:

```bash
go run ./examples/25-protocol-brainstorming-lab/main.go
```

What this example demonstrates (dogfooding Meanwhile):

- Discovery-first brainstorming flow (problem framing -> discovery -> challenge -> ideation -> critique -> evidence gate)
- A memory corpus loaded from real repo context:
  - `VISION.md` (copied into `examples/25-protocol-brainstorming-lab/localdata/memory/VISION.md` at runtime)
  - entire `docs/` folder
  - rewritten memory from all `AGENTS.md` files
  - `issues/` markdown files (if present)
  - root `README.md`
- Tools:
  - `memory_search` (searches the dogfood memory corpus)
  - `hmw_generate` (generates diverse HMW prompts)

Optional env vars:

- `MEANWHILE_MODEL` (default: `gpt-4o-mini`)
- `DUMP_METADATA_JSON=1` to print full structured metadata

Example:

```bash
MEANWHILE_MODEL=gpt-5.2-chat-latest DUMP_METADATA_JSON=1 go run ./examples/25-protocol-brainstorming-lab/main.go
```
