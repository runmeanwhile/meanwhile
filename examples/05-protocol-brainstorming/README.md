# Protocol Brainstorming

Run the baseline example:

```bash
go run ./examples/05-protocol-brainstorming/main.go
```

Run multi-variant comparison (runtime assembly + prompt variants):

```bash
go run ./cmd/brainstorm-compare -runs 3 -rounds 5
```

The comparison command writes a JSON report with:

- per-run transcripts
- per-run quality metrics
- per-variant aggregated metrics
