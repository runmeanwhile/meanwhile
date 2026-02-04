# Example 16: Memory Automation

This example demonstrates automatic session memory capture on session close.

## What it does

- Runs a short session with an agent
- Closes the session to trigger memory automation
- Reads the `memory.summary` event from the memory store

## Run

```bash
go run examples/16-memory-automation/main.go
```

You need `OPENAI_API_KEY` in your environment.
