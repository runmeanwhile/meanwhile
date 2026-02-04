# Hooks as Interrupts

Hooks are interruptions. They can observe, modify, or block messages, turns, and tools.

## Available Hook Phases

- **PreMessage**: before the protocol receives a message.
- **PreTurn** / **PostTurn**: around every agent run.
- **PreTool** / **PostTool**: around tool calls.
- **Stop**: when a loop is about to stop.

Turn hooks can inject or modify run requests, enabling chair-like interjections without rewriting protocol logic.
