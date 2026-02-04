# Automatic Session Memory

**Goal:** Automatically capture durable session memory when a session ends, using a user-supplied prompt with a safe default, and persist it in the configured memory store.

---

## Overview

When `Engine.CloseSession` is called, the engine can optionally:

1. Build a conversation context from memory.
2. Run a memory prompt via the configured provider.
3. Store the result as a `memory.summary` event with a **string payload**.

This keeps memory flexible: the payload is just text, so it can be plain sentences or
newline-separated key/value pairs depending on your prompt.

If you configure a memory store with `engine.WithMemoryStore`, memory automation is
enabled by default.

---

## Default Prompt

The default prompt produces a concise memory string (not JSON):

```
You are a memory extractor.
Write a concise memory string that captures durable facts, preferences, and key findings.
Use newline-separated "key: value" when helpful, otherwise short sentences.
Rules:
- Do NOT store secrets, API keys, or credentials.
- Skip ephemeral details unless the user says they are important.
- Be brief and precise.
- Return plain text only.
```

---

## Prompt Overrides

Users can override the prompt via configuration or session metadata.

**Priority order:**
1. Session-specific prompt override
2. Global Memory Automation prompt
3. Default prompt

Session metadata key: `engine.MemoryAutomationPromptKey`

To disable globally, pass `engine.WithMemoryAutomation(config.MemoryAutomationConfig{Enabled: false})`
or set `engine.MemoryAutomationEnabledKey` to `false` per session.

---

## Configuration Surface

```go
type MemoryAutomationConfig struct {
    Enabled        bool                    `json:"enabled" yaml:"enabled"`
    ProviderID     string                  `json:"provider_id" yaml:"provider_id"`
    Model          string                  `json:"model" yaml:"model"`
    Prompt         string                  `json:"prompt" yaml:"prompt"`
    Params         map[string]any          `json:"params" yaml:"params"`
    Context        MemoryAutomationContext `json:"context" yaml:"context"`
    TimeoutSeconds int                     `json:"timeout_seconds" yaml:"timeout_seconds"`
    StoreEvent     string                  `json:"store_event" yaml:"store_event"`
}

type MemoryAutomationContext struct {
    RecentMessages     int  `json:"recent_messages" yaml:"recent_messages"`
    TokenLimit         int  `json:"token_limit" yaml:"token_limit"`
    IncludeToolResults bool `json:"include_tool_results" yaml:"include_tool_results"`
}
```

---

## Event Storage

Summaries are stored as a `memory.summary` event with a string payload:

```go
ev := event.New(event.MemorySummary, sessionID, memoryText)
```

If you want to tag the memory as being about a specific agent, set
`engine.MemoryAutomationSubjectKey` in session metadata. The engine will store that
value in `Event.AgentID`.

---

## Example Usage (config)

```yaml
global:
  memory:
    store: "postgres"
    params:
      dsn: "postgres://..."
  memory_automation:
    enabled: true
    provider_id: "openai"
    model: "gpt-5-mini"
    prompt: |
      Remember durable facts as key: value lines.
      Avoid secrets. Keep it short.
    params:
      temperature: 0.2
      max_tokens: 400
    context:
      recent_messages: 20
      token_limit: 4000
```

---

## Failure Handling

- If the provider call fails, the session still closes.
- If the response is empty, nothing is stored.
- If no messages exist, memory automation is skipped.

---

## Notes

- Memory automation stores **text only**. You can choose whatever format you want in the prompt.
- A simple idempotency check prevents summaries when no new messages were added since the last summary.
