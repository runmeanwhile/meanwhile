# Context Management Implementation Plan

## Scope
Focus on core functional requirements (token limits, tool-result growth, and long-running session context). Excludes enterprise-specific needs like PII redaction.

## Goals
- Prevent unbounded prompt growth during a single run.
- Keep prompts within a configurable token/size budget.
- Preserve critical system instructions and recent conversational context.
- Control tool-result size without losing key outcomes.
- Support long-running sessions via periodic summarization.

## Non-Goals
- PII/sensitive data redaction.
- Multi-tenant policy enforcement.
- Advanced governance/audit pipelines.

## Proposed Core Capabilities
1) **Context Policy**
   - A pluggable policy that decides which messages/tool outputs are included for a given run.
   - Inputs: system prompt, prior messages, tool outputs, memory snippets, budget.
   - Output: ordered message list used for provider request.

2) **Token/Size Budgeting**
   - Hard budget set per agent/session/run.
   - Priority: system + pinned > recent dialogue > tool outputs > older dialogue.
   - Uses provider token estimators when available, falls back to approximate counting (4 chars ≈ 1 token).

3) **Tool Output Controls**
   - Truncate or summarize tool outputs when they exceed thresholds.
   - Optionally store full outputs out-of-band and inject references + compact summaries.

4) **Rolling Window**
   - Retain last N user/assistant turns if budget allows.
   - Drop oldest turns first.

5) **Summarization Checkpoints**
   - When history exceeds a threshold, summarize earlier turns into a compact summary message.
   - Store summary in memory, replace older turns in prompt with summary.

## Plan by Phase

### Phase 1: Introduce Policy Surface (No Behavior Change by Default)
**Code**
- Add `context.Policy` interface.
- Add `context.Selector` default implementation that returns input unchanged.
- Wire policy into `Session.RunAgent(...)` (and protocol runner), but keep default no-op.

**Tests**
- New tests asserting default policy preserves message ordering and content.
- Regression tests for current tool-loop behavior unchanged.

### Phase 2: Token Budgeting + Rolling Window
**Code**
- Implement budgeted selector:
  - Keep system/pinned messages.
  - Include most recent messages until budget is hit.
  - Drop oldest turns first.
- Add configuration:
  - `MaxPromptTokens` (approx) at agent/session/run levels.

**Tests**
- Verify trimming retains system + most recent N messages.
- Verify budget behavior with mixed tool + assistant + user messages.

### Phase 3: Tool Output Control
**Code**
- Introduce max tool output size (`MaxToolOutputChars`).
- Truncate tool outputs or create summary part.
- Optional: store full output in memory with a reference ID.

**Tests**
- Tool outputs exceeding threshold are truncated or summarized.
- Output under threshold remains intact.

### Phase 4: Summarization Checkpoints
**Code**
- Trigger summarization when history exceeds thresholds.
- Store summary in memory and inject summary message in place of older turns.
- Avoid repeated summarization with checkpoint tracking.

**Tests**
- Summary replaces older turns.
- Summary preserves latest turns unchanged.
- Summary is stable across repeated runs without new messages.

## Integration Points
- `engine.Session.RunAgent(...)` (main prompt construction).
- `memory.BuildConversationContext(...)` for cross-run retrieval.
- `engine.memoryAutomation` as a post-run summary fallback.

## Configuration Shape (Draft)
```yaml
context:
  max_prompt_tokens: 4000
  max_tool_output_chars: 2000
  rolling_window: 10
  summarization:
    enabled: true
    threshold_tokens: 6000
  auto_summarize:
    summarize_at_tokens: 4000
    min_keep_messages: 6
```

## Success Criteria
- No provider request exceeds configured budget.
- Tool outputs no longer dominate the prompt.
- Long-running sessions remain usable without manual trimming.
- All existing tests pass and new tests are green.
