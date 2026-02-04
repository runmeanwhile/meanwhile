# Server Package Notes (for coding agents)

Focus: inbound HTTP handlers for human responses.

Handlers
- `HumanResponseHandler`: accepts JSON with `request_id` + `response`, resolves session via request registry, and calls `Respond()`.
- `SlackCommandHandler`: parses slash command text (`respond <request_id> <response>`).
- `HumanRequestInboxHandler`: read-only JSON list of human requests for inbox UIs.

Security
- HMAC verification via `X-Meanwhile-Signature` (hex) or body signature.
- Slack handler validates Slack signing secret.

Operational notes
- Keep responses small and deterministic; these are meant for webhook/command use.
- Consider adding rate limiting upstream if exposed publicly.
