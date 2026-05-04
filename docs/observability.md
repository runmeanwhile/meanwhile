# Observability

Meanwhile exposes a small telemetry abstraction so you can plug in tracing backends without changing core logic.

## Telemetry Client

```go
client := langfuse.NewClient(langfuse.Config{
    PublicKey: "pk_...",
    SecretKey: "sk_...",
    Endpoint:  "https://cloud.langfuse.com/api/public/otel",
    ServiceName: "meanwhile",
})

eng, _ := engine.New(engine.WithTelemetryClient(client))
```

## Langfuse

The Langfuse adapter uses OpenTelemetry HTTP (OTLP). It sends spans to:

```
https://cloud.langfuse.com/api/public/otel/v1/traces
```

Use the `PublicKey` and `SecretKey` from your Langfuse project.

## Events and Spans

- `session.run` trace per session run.
- `agent.run` span per agent execution.
- `tool.run` child span per tool execution.
- Agent spans include OpenTelemetry events for:
  - `agent.message.delta`
  - `agent.reasoning.delta`
  - `agent.reasoning.summary.delta`
  - `agent.message.completed`
  - `tool.call`
  - `provider.raw.event`
- Tool spans include OpenTelemetry events for:
  - `tool.call.started`
  - `tool.call.completed`
  - `tool.call.error`
  - `tool.call.awaiting`

All telemetry events carry stable attributes such as `session_id`, `agent_id`, `tool_id`, `tool_call_id`, `event_type`, and relevant text/result fields. Image bytes and image data URLs are omitted from telemetry attributes; the trace records the image event and points back to the original application artifact instead of duplicating media.

## Recommended attributes for persona evaluation

If you plan to refine personas later (for example in Studio), capture these now so historical traces are usable:

- `protocol_id`
- `session_id`
- `agent_name`
- `model`
- `persona_id`
- `persona_version`
- `prompt_version`
- `eval_scenario_id` (when running eval harnesses)
- `eval_variant` (runtime/prompt variant label)
- `run_id` (for repeated experiments)

## Runtime boundary for refinement

- Runtime should not auto-edit persona prompts during normal execution.
- Refinement should happen in a separate evaluation/control plane:
  - collect traces
  - score outcomes
  - propose prompt/persona changes
  - approve and roll out a new persona version
