# Integration Package Notes (for coding agents)

Focus: outbound human escalation delivery and routing.

Key types/flows
- `Integration` interface delivers an `integration.Request` to a channel/contact.
- `Registry` stores integrations by ID; `Router` selects preferred + fallback.
- Router order: preferred channel first, then remaining channels sorted for determinism.
- `FormatPlainText` and `FormatMarkdown` shape human-facing payloads.

Design constraints
- No hard-coded channel keywords; channels are user-provided strings.
- Failures should return explicit errors; engine emits `human.request.failed`.
- Keep payloads minimal (avoid leaking sensitive context).

Integrations shipped
- Slack: sends text via slack-go client.
- Email: SMTP sender with STARTTLS when available.
- Webhook: POST JSON to contact URL, only http/https allowed.
