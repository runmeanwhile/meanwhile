# Request Registry Package Notes (for coding agents)

Focus: mapping human request IDs to session IDs for inbound responses.

Key types/flows
- Implements `engine.RequestRegistry` (Register, Resolve, Delete).
- In-memory and Redis implementations exist; Redis supports optional TTL.

Behavior notes
- Register is idempotent for the same request → session mapping.
- Resolve returns `engine.ErrRequestNotFound` when missing.
- Delete should return `engine.ErrRequestNotFound` if already removed.
