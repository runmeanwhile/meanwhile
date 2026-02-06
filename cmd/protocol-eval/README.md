# protocol-eval

Protocol evaluation harness for Meanwhile.

## What it does

- Runs protocol variants across one or more models and scenarios.
- Computes deterministic transcript metrics.
- Optionally scores each run using a judge model.
- Persists JSON report to `artifacts/evals/...`.
- Optionally compares against a baseline report and fails on regressions.

## Usage

```bash
go run ./cmd/protocol-eval \
  -protocol brainstorming \
  -models gpt-5.2-chat-latest,gpt-5.1-codex \
  -runs 2 \
  -rounds 5 \
  -dataset evals/datasets/brainstorming-core.json \
  -judge-model gpt-5.2-chat-latest
```

With regression gating:

```bash
go run ./cmd/protocol-eval \
  -protocol brainstorming \
  -models gpt-5.2-chat-latest \
  -runs 2 \
  -baseline artifacts/evals/brainstorming/BASELINE/report.json \
  -max-overall-drop 0.25 \
  -max-critical-drop 0.40
```

## Notes

- Requires `OPENAI_API_KEY` in environment.
- Current implementation supports `brainstorming` protocol.
- Reports are self-contained JSON and can be checked into CI artifacts.
