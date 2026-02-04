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
- Tool call events attach span attributes.
