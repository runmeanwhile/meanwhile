# Scheduler Package Notes (for coding agents)

Focus: pluggable scheduling drivers + polling worker.

Key types/flows
- `Driver` persists `Job` records and **must remove** jobs returned by `ClaimDue`.
- `Worker` polls driver at an interval and dispatches via handler.
- Drivers: in-memory (simple), Redis (sorted set + hash storage).

Contracts and pitfalls
- `ClaimDue` should be idempotent for a given job ID (remove when claimed).
- Jobs are identified by `Job.ID` and can be rescheduled by overwriting.
- Prefer short handler work; treat worker as a poller, not a job runner.

Used by engine
- Timeout scheduling uses job type `human.request.timeout` and request ID as job ID.
