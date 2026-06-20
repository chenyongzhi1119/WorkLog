# WorkLog

WorkLog is a local Go + SQLite work journal for daily logs, goals, AI-generated plans, reports, and mentor-style review.

## Run

```bash
go run .
```

Then open:

```text
http://localhost:8090
```

The app creates a local SQLite database at `worklog.db`.

## AI Provider

Configure a provider from the settings button in the top bar. The app supports Anthropic and OpenAI-compatible APIs such as DeepSeek, OpenAI, Qwen, GLM, and custom endpoints.

API keys are stored in the local SQLite database, so keep `worklog.db*` private and do not commit it.

## Verify

```bash
go test ./...
```
